// View state, the TUI palette as glyph tables, and the filter that decides
// which rows show. The text query itself is evaluated on the server.

import { D, ancestors, descendants, eff, get, isReady, kids, pool, projectOf, type Item } from "./data";
import { store } from "./util";

export type View = "tree" | "board" | "search" | "more" | "graph";
export type SortKey = "priority" | "updated" | "created" | "id" | "status";

export interface Sheet { kind: string; id?: string; ids?: string[]; parent?: string | null; [k: string]: unknown }

export const S = {
  view: "tree" as View,
  /** status chips: a row shows when its effective status is in the set, or the set is empty */
  st: new Set<string>(store.get<string[]>("chips", ["open", "in_progress", "blocked"])),
  ready: false,
  /** the TUI query the tree applies, evaluated by GET api/query */
  text: "",
  qres: null as null | { text: string; version: number; ids: Set<string>; error: string },
  sort: store.get<SortKey>("sort", "priority"),
  cols: new Set<string>(store.get<string[]>("cols", ["type", "prio", "who", "age", "deps", "kids", "notes"])),
  wrap: store.get("wrap", false),
  textSize: store.get("textSize", 100),
  folded: new Set<string>(),
  focus: null as null | { id: string; mode: "branch" | "subtree" },
  cursor: null as string | null,
  marks: new Set<string>(),
  marking: false,
  lastMark: null as string | null,
  chips: store.get("chipRow", true),
  detail: null as null | { stack: string[]; size: "half" | "full" },
  sheet: null as Sheet | null,
  follow: store.get("follow", false),
  board: {
    group: store.get<"status" | "priority" | "type">("boardGroup", "status"),
    col: null as string | null,
    folded: new Set<string>(),
    lanes: store.get<"off" | "rows" | "rail">("boardLanes", "rows"),
    lane: null as string | null,
    laneFold: new Set<string>(),
    keep: false,
  },
  graph: [] as string[],
  flash: new Set<string>(),
  log: [] as { at: number; cmd: string; ok: boolean }[],
  live: "connecting" as "connecting" | "live" | "stale",
  retryIn: 0,
};

export const ST: Record<string, { g: string; c: string; w: string }> = {
  open: { g: "○", c: "var(--green)", w: "open" },
  in_progress: { g: "◐", c: "var(--cyan)", w: "in_progress" },
  blocked: { g: "⊘", c: "var(--red)", w: "blocked" },
  deferred: { g: "◌", c: "var(--orange)", w: "deferred" },
  review: { g: "◑", c: "var(--yellow)", w: "review" },
  pinned: { g: "⊙", c: "var(--accent)", w: "pinned" },
  hooked: { g: "↳", c: "var(--accent)", w: "hooked" },
  closed: { g: "✓", c: "var(--muted)", w: "closed" },
};
export const stOf = (s: string) => ST[s] || { g: "·", c: "var(--dim)", w: s };

export const TY: Record<string, [string, string]> = {
  epic: ["♦", "var(--accent)"], feature: ["▲", "var(--feat)"], task: ["✔", "var(--blue)"],
  bug: ["●", "var(--red)"], chore: ["○", "var(--cyan)"], decision: ["◆", "var(--orange)"],
};
export const tyOf = (t: string): [string, string] => TY[t] || ["·", "var(--muted)"];
export const PC = ["var(--red)", "var(--orange)", "var(--dim)", "var(--muted)", "var(--muted)"];

const STATUS_ORDER = ["in_progress", "blocked", "open", "review", "hooked", "pinned", "deferred", "closed"];
const rank = (i: Item) => { const n = STATUS_ORDER.indexOf(eff(i)); return n < 0 ? 3 : n; };
export const SORTS: Record<SortKey, (a: Item, b: Item) => number> = {
  priority: (a, b) => a.priority - b.priority || b.upd - a.upd,
  updated: (a, b) => b.upd - a.upd,
  created: (a, b) => b.crt - a.crt,
  id: (a, b) => a.id.localeCompare(b.id, undefined, { numeric: true }),
  status: (a, b) => rank(a) - rank(b) || a.priority - b.priority,
};
export const SORT_HELP: Record<SortKey, string> = {
  priority: "P0 first, then most recently updated", updated: "most recent first", created: "newest first",
  id: "natural ID order", status: "in progress, blocked, open, deferred, closed",
};

/** textMatches is the server's verdict on S.text; a pending query hides nothing. */
function textMatches(i: Item): boolean {
  if (!S.text.trim()) return true;
  if (!S.qres || S.qres.text !== S.text) return true;
  return S.qres.ids.has(i.id);
}

export function chipMatches(i: Item): boolean {
  if (S.st.size && !S.st.has(eff(i))) {
    // Statuses without a chip of their own (review, pinned, hooked) ride with open.
    if (!(S.st.has("open") && !(eff(i) in CHIP_STATUSES))) return false;
  }
  return !S.ready || isReady(i);
}
const CHIP_STATUSES: Record<string, true> = { open: true, in_progress: true, blocked: true, deferred: true, closed: true };

export const matches = (i: Item): boolean => chipMatches(i) && textMatches(i);

/** queryString shows the whole filter the way the TUI query bar would. */
export function queryString(): string {
  const parts: string[] = [];
  if (S.st.size) parts.push("status:" + [...S.st].join(","));
  if (S.ready) parts.push("ready");
  if (S.text.trim()) parts.push(S.text.trim());
  return parts.join(" ");
}

export interface TreeRow { i?: Item; d: number; dim?: boolean; hasKids?: boolean; folded?: boolean; hdr?: string; n?: number; key?: string }

export function treeRows(): { rows: TreeRow[]; matchCount: number } {
  const P = pool();
  const match = new Set(P.filter(matches).map(i => i.id));
  let show = new Set(match);
  for (const id of match) for (const a of ancestors(id)) show.add(a);
  if (S.focus && get(S.focus.id)) {
    const keep = new Set([S.focus.id, ...descendants(S.focus.id)]);
    if (S.focus.mode === "branch") ancestors(S.focus.id).forEach(a => keep.add(a));
    show = new Set([...show].filter(x => keep.has(x)));
  }
  const cmp = SORTS[S.sort];
  const out: TreeRow[] = [];
  const seen = new Set<string>();
  const walk = (id: string, d: number) => {
    if (seen.has(id)) return;
    seen.add(id);
    const i = get(id)!;
    const ks = kids(id).filter(k => show.has(k)).map(k => get(k)!).sort(cmp);
    out.push({ i, d, dim: !match.has(id), hasKids: ks.length > 0, folded: S.folded.has(id) });
    if (!S.folded.has(id)) ks.forEach(k => walk(k.id, d + 1));
  };
  const roots = [...show].map(x => get(x)!).filter(i => !i.parent || !show.has(i.parent)).sort(cmp);
  if (D.project.all) {
    const names = [...new Set(roots.map(projectOf))].sort();
    for (const p of names) {
      const key = "proj:" + p;
      out.push({ hdr: p, d: 0, n: [...show].filter(x => projectOf(get(x)!) === p).length, folded: S.folded.has(key), key });
      if (!S.folded.has(key)) roots.filter(r => projectOf(r) === p).forEach(r => walk(r.id, 0));
    }
  } else roots.forEach(r => walk(r.id, 0));
  return { rows: out, matchCount: match.size };
}

export function saveChips(): void { store.set("chips", [...S.st]); }
