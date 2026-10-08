// Browser history for navigation inside the app. The place the user is at
// (view, detail stack, graph path) is derived from S after every render and
// mirrored into history, so the many code paths that change S need not know
// about history at all.
//
//   new place == this entry's      same view, same depth   anything else
//   previous place (‹, ✕, Esc)    (cursor, sibling swipe)  (view, open, relation)
//            │                              │                     │
//      history.back()                 replaceState            pushState
//
// The URL carries the view and the top issue, #/board/bd-12, so a reload or a
// shared link opens the same place. The detail stack itself lives only in
// history.state.
//
// An open sheet is part of the place, so Back closes the sheet and nothing
// else. A sheet's form state cannot be rebuilt, so a step that lands on a
// sheet's entry with no sheet open walks on past it. Leaving a sheet for
// another place takes over the sheet's entry, so no entry ever follows a
// sheet's, and Forward onto one has nowhere further to go.

import { get } from "./data";
import { S, defaultDetailSize, type View } from "./state";

interface Place { v: View; d: string[] | null; g: string[]; s?: boolean }
interface Entry { place: Place; n: number; prev: Place | null }

const VIEWS: View[] = ["tree", "board", "search", "more", "graph"];

let restoring = false;
/** set between our own history.back() and its popstate, when history.state is still the entry being left */
let backing = 0;

function here(): Place {
  return { v: S.view, d: S.detail ? [...S.detail.stack] : null, g: S.view === "graph" ? [...S.graph] : [], ...(S.sheet ? { s: true } : {}) };
}

const same = (a: Place | null, b: Place | null) => JSON.stringify(a) === JSON.stringify(b);

function hashOf(p: Place): string {
  const id = p.d ? p.d[p.d.length - 1] : p.g[p.g.length - 1];
  return `#/${p.v}${id ? "/" + encodeURIComponent(id) : ""}`;
}

function entry(): Entry | null {
  const e = history.state as Entry | null;
  return e && e.place && typeof e.n === "number" ? e : null;
}

/** parseHash reads #/view/id. The id is not checked: the issues may not have loaded yet. */
export function parseHash(h = location.hash): { v: View; id: string } | null {
  const m = /^#\/([a-z]+)(?:\/(.+))?$/.exec(h);
  if (!m || !VIEWS.includes(m[1] as View)) return null;
  let id = "";
  try { id = m[2] ? decodeURIComponent(m[2]) : ""; } catch { /* a mangled id opens the view alone */ }
  return { v: m[1] as View, id };
}

/**
 * started is false until boot has applied the URL. A reload keeps
 * history.state, so a render before the issues load would read the empty
 * place as a closed detail and step Back off the issue the URL names.
 */
let started = false;

/** startHistory lets syncHistory record places; boot calls it once the URL is applied. */
export function startHistory(): void { started = true; }

/** syncHistory records where the app is now. render() calls it last. */
export function syncHistory(): void {
  if (!started || restoring || backing) return;
  const now = here(), cur = entry();
  if (!cur) { history.replaceState({ place: now, n: 0, prev: null } satisfies Entry, "", hashOf(now)); return; }
  if (same(cur.place, now)) return;
  if (cur.n > 0 && same(cur.prev, now)) {
    // The popstate that follows restores this same place, so nothing redraws.
    backing = window.setTimeout(() => { backing = 0; }, 1000);
    history.back();
    return;
  }
  const p = cur.place;
  if (p.s || (p.v === now.v && !p.d === !now.d && (p.d?.length ?? 0) === (now.d?.length ?? 0) && p.g.length === now.g.length && !now.s)) {
    history.replaceState({ ...cur, place: now } satisfies Entry, "", hashOf(now));
    return;
  }
  history.pushState({ place: now, n: cur.n + 1, prev: p } satisfies Entry, "", hashOf(now));
}

/**
 * syncSoon records the place once the code that opened or closed a sheet has
 * run on, so an action that closes its sheet and then opens an issue records
 * one step, not a Back followed by a lost open.
 */
export function syncSoon(): void { queueMicrotask(syncHistory); }

/** canGoBack is false on the entry the app opened on, where Back would leave the app. */
export const canGoBack = (): boolean => (entry()?.n ?? 0) > 0;

/** openHash sets the view and the issue a link names, once the issues have loaded. */
export function openHash(h = parseHash()): boolean {
  if (!h) return false;
  S.view = h.v;
  if (h.id && get(h.id)) {
    if (h.v === "graph") S.graph = [h.id];
    else S.detail = { stack: [h.id], size: S.detail?.size ?? defaultDetailSize() };
    S.cursor = h.id;
  }
  return true;
}

export function bindHistory(render: () => void, closeSheet: () => void): void {
  window.addEventListener("popstate", () => {
    if (backing) { clearTimeout(backing); backing = 0; }
    const e = entry();
    if (e?.place.s && !S.sheet) { history.back(); return; }
    restoring = true;
    try {
      if (S.sheet) closeSheet();
      if (e) {
        const p = e.place, ids = (p.d || []).filter(id => get(id));
        S.view = p.v;
        S.detail = ids.length ? { stack: ids, size: S.detail?.size ?? defaultDetailSize() } : null;
        if (p.v === "graph") S.graph = [...p.g];
        if (ids.length) S.cursor = ids[ids.length - 1];
      } else {
        // A hash typed or pasted into the address bar makes an entry without state.
        S.detail = null;
        openHash();
      }
      render();
    } finally { restoring = false; }
    // An issue deleted since the entry was made drops out of the restored place.
    const cur = entry(), now = here();
    history.replaceState({ place: now, n: cur?.n ?? 0, prev: cur?.prev ?? null } satisfies Entry, "", hashOf(now));
  });
}
