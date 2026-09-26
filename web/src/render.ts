// Every view renders HTML strings into the fixed hosts in index.html.
// Gestures and taps are bound once, by delegation, in gestures.ts, so a
// render never has to rebind anything.

import * as api from "./api";
import type { Issue } from "./api.gen";
import { D, ancestors, blocksOf, descendants, eff, get, isReady, kids, laneOf, openBlockers, pool, progress, projectOf, shortId, type Item } from "./data";
import { PC, S, SORTS, matches, queryString, stOf, treeRows, tyOf, type TreeRow } from "./state";
import { syncHistory } from "./nav";
import { $, age, esc, fmtDate, md, store } from "./util";

export function l2HTML(i: Item, opts: { proj?: boolean } = {}): string {
  const bits: string[] = [];
  const [tg, tc] = tyOf(i.type);
  if (S.cols.has("prio")) bits.push(`<span style="color:${PC[i.priority] || "var(--muted)"}">P${i.priority}</span>`);
  if (S.cols.has("type")) bits.push(`<span style="color:${tc}">${tg} ${esc(i.type)}</span>`);
  if (S.cols.has("who") && i.assignee) bits.push(`<span style="color:var(--dim)">@${esc(i.assignee)}</span>`);
  if (S.cols.has("age")) bits.push(`<span>${age(i.upd)}</span>`);
  const pr = progress(i.id);
  if (S.cols.has("kids") && pr) bits.push(`<span style="color:var(--green)">${pr.done}/${pr.all}</span>`);
  if (S.cols.has("deps")) {
    const ob = openBlockers(i).length;
    if (ob) bits.push(`<span style="color:var(--red)">⊘${ob}</span>`);
    const bl = blocksOf(i).filter(x => !x.closed_like).length;
    if (bl) bits.push(`<span style="color:var(--orange)">→${bl}</span>`);
  }
  if (S.cols.has("notes") && i.comment_count) bits.push(`<span>✎${i.comment_count}</span>`);
  if (S.cols.has("labels") && i.labels.length) bits.push(`<span style="color:var(--cyan)">#${esc(i.labels.join(" #"))}</span>`);
  if (opts.proj && D.project.all) bits.push(`<span style="color:var(--accent)">${esc(projectOf(i))}</span>`);
  return `<div class="l2">${bits.join("<span>·</span>")}</div>`;
}

export function rowHTML(r: TreeRow, flat = false): string {
  if (r.hdr) return `<div class="ph" data-hdr="${esc(r.key)}" role="button" tabindex="0"><span>${r.folded ? "▸" : "▾"}</span><span>${esc(r.hdr)}</span><span class="n">${r.n}</span></div>`;
  const i = r.i!, st = stOf(eff(i));
  const cls = ["row", S.cursor === i.id ? "sel" : "", r.dim ? "dim" : "", S.marks.has(i.id) ? "marked" : "", S.flash.has(i.id) ? "flash" : ""].join(" ");
  const chev = flat ? "" : `<span class="chev ${r.hasKids ? "" : "none"}" data-chev aria-hidden="true">${r.hasKids ? (r.folded ? "▸" : "▾") : "·"}</span>`;
  const mk = S.marking ? `<span class="mk">${S.marks.has(i.id) ? "▪" : "▫"}</span>` : "";
  const label = `${shortId(i.id)} ${i.t}, ${st.w}, P${i.priority} ${i.type}${S.marks.has(i.id) ? ", marked" : ""}`;
  return `<div class="${cls}" data-id="${esc(i.id)}" role="listitem" tabindex="0" aria-label="${esc(label)}"><div class="ul"></div><div class="ur"></div><div class="rc" style="--d:${flat ? 0 : Math.min(r.d, 6)}"><div class="l1">${mk}${chev}<span class="st" style="color:${st.c}">${st.g}</span><span class="id">${esc(shortId(i.id))}</span><span class="t">${esc(i.t)}</span></div>${l2HTML(i, { proj: true })}</div></div>`;
}

/* ================= shell ================= */

export function render(): void {
  if (!D.loaded) return;
  renderHeader(); renderChips(); renderBar(); renderStale();
  (["tree", "board", "search", "more", "graph"] as const).forEach(v => { $("#v" + v[0].toUpperCase() + v.slice(1)).hidden = S.view !== v; });
  if (S.view === "tree") renderTree();
  if (S.view === "board") renderBoard();
  if (S.view === "search") renderSearch();
  if (S.view === "more") renderMore();
  if (S.view === "graph") renderGraph();
  renderDetail();
  syncQuery();
  syncHistory();
}

function renderHeader(): void {
  const P = pool();
  const n = (s: string) => P.filter(i => eff(i) === s).length;
  const pname = D.project.all ? "all" : D.project.name;
  const bad = S.live === "stale" || (D.health && !D.health.ok);
  $("#hd").innerHTML = `<button class="proj" data-act="projects" aria-label="Switch project">b9s <small>·</small> ${esc(pname)}${D.project.read_only ? ` <span class="ro">ro</span>` : ""} <small>▾</small></button>
    <button class="dot ${bad ? "bad" : ""}" id="hdot" data-act="health" aria-label="Data source health">●</button>
    <span class="cnt"><b>${n("open")}</b> open · <i>${n("in_progress")}</i> prog${n("blocked") ? ` · <span style="color:var(--red)">${n("blocked")}</span> blk` : ""}</span>
    <button class="ib follow ${S.follow ? "on" : ""}" data-act="follow" aria-label="Follow mode" aria-pressed="${S.follow}">F</button>
    <button class="ib" data-act="create" aria-label="Create issue">+</button>
    <button class="ib" data-act="help" aria-label="Gestures">?</button>`;
}

function renderChips(): void {
  const el = $("#chips");
  el.hidden = !S.chips;
  const P = pool();
  const cnt = (s: string) => P.filter(i => eff(i) === s).length;
  const fmt = (n: number) => (n >= 1000 ? (n / 1000).toFixed(1) + "k" : String(n));
  const chip = (k: string, key: string, label: string, color: string, on: boolean, n: number | null) =>
    `<button class="chip ${on ? "on" : ""}" data-chip="${key}" style="color:${color}" aria-pressed="${on}"><span class="k">${k}</span>${label}${n != null ? ` <span class="n">${fmt(n)}</span>` : ""}</button>`;
  let h = chip("o", "open", "Open", "var(--green)", S.st.has("open"), cnt("open"))
    + chip("i", "in_progress", "In prog", "var(--cyan)", S.st.has("in_progress"), cnt("in_progress"))
    + chip("b", "blocked", "Blocked", "var(--red)", S.st.has("blocked"), cnt("blocked"))
    + chip("d", "deferred", "Deferred", "var(--orange)", S.st.has("deferred"), cnt("deferred"))
    + chip("C", "closed", "Closed", "var(--muted)", S.st.has("closed"), cnt("closed"))
    + chip("r", "ready", "Ready", "var(--yellow)", S.ready, P.filter(isReady).length)
    + chip("a", "all", "All", "var(--dim)", S.st.size === 0 && !S.ready, null);
  h += `<span class="sep"></span>`;
  if (S.text.trim()) {
    const err = S.qres && S.qres.text === S.text && S.qres.error;
    h += `<button class="chip q ${err ? "err" : ""}" data-act="search"><span>⌕ ${esc(S.text.trim())}</span></button><button class="chip" data-act="clearq" aria-label="Clear query">✕</button>`;
  } else h += `<button class="chip" data-act="search" style="color:var(--muted)">⌕ query</button>`;
  h += `<button class="chip" data-act="sort" style="color:var(--dim)">⇅ ${S.sort}</button>`;
  el.innerHTML = h;
}

function renderBar(): void {
  const el = $("#bar");
  if (S.marking) {
    el.className = "bar bulk";
    el.innerHTML = `<button class="cnt" data-act="markall">${S.marks.size} marked<br><span style="font-weight:400;color:var(--muted)">tap: all</span></button>
      <button data-act="bulkstatus"><span class="g">◐</span>Status</button>
      <button data-act="bulkclose"><span class="g">✓</span>Close</button>
      <button class="red" data-act="bulkdelete"><span class="g">⌫</span>Delete</button>
      <button data-act="unmark"><span class="g">✕</span>Done</button>`;
    return;
  }
  el.className = "bar";
  const tab = (v: string, g: string, l: string) => {
    const on = S.view === v || (v === "more" && S.view === "graph");
    return `<button class="${on ? "on" : ""}" data-view="${v}" aria-current="${on ? "page" : "false"}"><span class="g">${g}</span>${l}</button>`;
  };
  el.innerHTML = tab("tree", "≣", "Tree") + tab("board", "▥", "Board") + tab("search", "⌕", "Search") + tab("more", "⋯", "More");
}

export function renderStale(): void {
  const el = $("#stale");
  if (S.live !== "stale") { el.hidden = true; return; }
  el.hidden = false;
  el.innerHTML = `<span>Offline: showing the last snapshot${S.retryIn ? ` · retry in ${Math.round(S.retryIn / 1000)} s` : ""}</span><button data-act="retrylive">Retry now</button>`;
}

export const treeList = () => $("#treeList");

function renderTree(): void {
  const fb = $("#fbar");
  const fi = S.focus && get(S.focus.id);
  fb.innerHTML = S.focus && fi ? `<div class="fbar"><span>${S.focus.mode === "branch" ? "⎇ branch" : "⌖ subtree"} · ${esc(shortId(fi.id))} ${esc(fi.t)}</span><button data-act="unfocus">✕ clear</button></div>` : "";
  const { rows } = treeRows();
  const el = $("#treeRows");
  el.className = S.wrap ? "wrap" : "";
  el.innerHTML = rows.length ? rows.map(r => rowHTML(r)).join("")
    : `<div class="empty">No issues match <b>${esc(queryString())}</b>.<br><br><button data-act="allq">Show all statuses</button></div>`;
  treeList().style.paddingBottom = S.detail ? detailHeightPx() + "px" : "";
}

/* ================= board ================= */

export const GROUPS: Record<string, [string, string][]> = {
  status: [["open", "Open"], ["in_progress", "In prog"], ["blocked", "Blocked"], ["closed", "Closed"]],
  priority: [["0", "P0"], ["1", "P1"], ["2", "P2"], ["3", "P3"], ["4", "P4"]],
  type: [["epic", "Epic"], ["feature", "Feature"], ["task", "Task"], ["bug", "Bug"], ["chore", "Chore"]],
};

/** colOf puts every status without a column of its own (deferred, review, …) in Open. */
export function colOf(i: Item): string {
  const g = S.board.group;
  if (g === "status") { const e = eff(i); return e === "in_progress" || e === "blocked" || e === "closed" ? e : "open"; }
  if (g === "priority") return String(i.priority);
  return i.type;
}

export function boardCols(): [string, string][] {
  let cols = GROUPS[S.board.group];
  if (S.board.group === "status") {
    cols = cols.filter(([k]) => k === "open" ? (!S.st.size || S.st.has("open") || S.st.has("deferred")) : (!S.st.size ? k !== "closed" : S.st.has(k)));
  }
  return cols;
}

/**
 * wide matches a laptop, desktop browser or tablet: room for every column
 * side by side. A phone on its side is wide but too short for it.
 */
export const WIDE = "(min-width: 720px) and (min-height: 500px)";
export const wide = () => window.matchMedia(WIDE).matches;

function boardItems(key: string): Item[] {
  const lane = S.board.lanes === "rail" ? S.board.lane : null;
  const keep = S.focus ? new Set([S.focus.id, ...descendants(S.focus.id)]) : null;
  return pool().filter(i => {
    if (i.type === "epic" && S.board.group !== "type" && S.board.lanes !== "off") return false;
    if (colOf(i) !== key || !matches(i)) return false;
    if (keep && !keep.has(i.id)) return false;
    return !lane || (laneOf(i) || "none") === lane;
  }).sort(SORTS[S.sort]);
}

export function openCols(): [string, string][] { return boardCols().filter(([k]) => !S.board.folded.has(k)); }

function activeCol(): [string, string] | null {
  const cols = openCols();
  if (!cols.length) return null;
  if (!cols.some(([k]) => k === S.board.col)) S.board.col = cols[0][0];
  return cols.find(([k]) => k === S.board.col) || null;
}

function cardHTML(i: Item): string {
  const st = stOf(eff(i));
  const [, tc] = tyOf(i.type);
  return `<div class="card ${S.cursor === i.id ? "sel" : ""} ${S.flash.has(i.id) ? "flash" : ""}" data-card="${esc(i.id)}" tabindex="0" role="listitem" aria-label="${esc(shortId(i.id) + " " + i.t + ", " + st.w)}" style="--c:${tc}"><div class="l1"><span class="st" style="color:${st.c}">${st.g}</span><span class="id">${esc(shortId(i.id))}</span><span class="t">${esc(i.t)}</span></div>${l2HTML(i, { proj: true })}</div>`;
}

/** laneKeys orders epic lanes the way both boards do: biggest first, loose cards last. */
function laneKeys(items: Item[]): [string, Item[]][] {
  const lanes = new Map<string, Item[]>();
  items.forEach(i => { const l = laneOf(i) || "none"; const a = lanes.get(l); if (a) a.push(i); else lanes.set(l, [i]); });
  return [...lanes].sort((a, b) => Number(a[0] === "none") - Number(b[0] === "none") || b[1].length - a[1].length);
}

function laneHTML(l: string, n: number): string {
  const f = S.board.laneFold.has(l), e = get(l);
  return `<button class="lane" data-lane="${esc(l)}" aria-expanded="${!f}"><span>${f ? "▸" : "▾"} ${l === "none" ? "·" : "♦ " + esc(shortId(l))}</span><span class="t">${l === "none" ? "No epic" : esc(e ? e.t : l)}</span><span class="n">${n}</span></button>`;
}

function railHTML(inCols: string[]): string {
  const counts = new Map<string, number>();
  pool().filter(i => inCols.includes(colOf(i)) && matches(i) && i.type !== "epic").forEach(i => { const l = laneOf(i) || "none"; counts.set(l, (counts.get(l) || 0) + 1); });
  const total = [...counts.values()].reduce((a, b) => a + b, 0);
  return `<div class="brail"><button class="${!S.board.lane ? "on" : ""}" data-rlane="">all<b>${total}</b></button>`
    + [...counts].sort((a, b) => b[1] - a[1]).map(([l, n]) => `<button class="${S.board.lane === l ? "on" : ""}" data-rlane="${esc(l)}">${l === "none" ? "no epic" : "♦" + esc(shortId(l))}<b>${n}</b></button>`).join("") + `</div>`;
}

function optsTab(): string {
  return `<button class="tab" data-act="boardopts" style="color:var(--muted)">⚙ ${S.board.group}${S.board.lanes !== "off" ? " · lanes " + S.board.lanes : ""}</button>`;
}

/**
 * renderWideBoard lays every column out side by side in one grid, so an
 * epic lane is a row across all of them and the columns scroll together.
 */
function renderWideBoard(v: HTMLElement): void {
  const cols = boardCols();
  const per = new Map(cols.map(([k]) => [k, boardItems(k)]));
  const lanes = S.board.lanes !== "off" && S.board.group !== "type";
  const folded = (k: string) => S.board.folded.has(k);
  const tmpl = cols.map(([k]) => (folded(k) ? "34px" : "minmax(240px, 1fr)")).join(" ");
  const head = cols.map(([k, l]) => {
    const n = per.get(k)!.length;
    return folded(k)
      ? `<button class="whead rail" data-tab="${esc(k)}" data-dcol="${esc(k)}" aria-label="Unfold ${esc(l)}" title="Unfold ${esc(l)}"><span>${esc(l)}</span><b>${n}</b></button>`
      : `<button class="whead" data-tab="${esc(k)}" data-dcol="${esc(k)}" style="--hc:${S.board.group === "status" ? stOf(k).c : "var(--line)"}" aria-label="${esc(l)}, ${n} cards. Fold" title="Fold ${esc(l)} (z)"><span>${esc(l)}</span><b>${n}</b></button>`;
  }).join("");
  const cell = (k: string, arr: Item[]) => folded(k)
    ? `<div class="wcell rail" data-dcol="${esc(k)}"></div>`
    : `<div class="wcell" data-dcol="${esc(k)}" role="list">${arr.map(cardHTML).join("")}</div>`;
  let rows = "";
  if (S.board.lanes === "rows" && lanes) {
    for (const [l, arr] of laneKeys(cols.filter(([k]) => !folded(k)).flatMap(([k]) => per.get(k)!))) {
      rows += laneHTML(l, arr.length);
      if (!S.board.laneFold.has(l)) rows += cols.map(([k]) => cell(k, per.get(k)!.filter(i => (laneOf(i) || "none") === l))).join("");
    }
  } else rows = cols.map(([k]) => cell(k, per.get(k)!)).join("");
  const total = cols.reduce((n, [k]) => n + (folded(k) ? 0 : per.get(k)!.length), 0);
  if (!total) rows += `<div class="empty wempty">No cards match <b>${esc(queryString() || "the filter")}</b>.</div>`;
  const rail = S.board.lanes === "rail" && lanes ? railHTML(cols.map(([k]) => k)) : "";
  const tools = optsTab()
    + (S.board.folded.size ? `<button class="tab unf" data-act="unfoldall">Z unfold</button>` : "")
    + `<span class="whint">drag cards between columns · h j k l move · z folds</span>`;
  const old = document.getElementById("wboard");
  const top = old ? old.scrollTop : 0, left = old ? old.scrollLeft : 0;
  v.innerHTML = `<div class="tabs" id="tabs">${tools}</div><div class="bbody" id="bbody">${rail}<div class="wboard" id="wboard" style="grid-template-columns:${tmpl}">${head}${rows}</div></div>`;
  const nb = document.getElementById("wboard")!;
  nb.scrollTop = top; nb.scrollLeft = left;
  S.board.keep = false;
}

export function renderBoard(): void {
  const v = $("#vBoard");
  if (wide()) { renderWideBoard(v); return; }
  const cols = boardCols();
  const cur = activeCol();
  const folded = cols.filter(([k]) => S.board.folded.has(k));
  let tabs = cols.map(([k, l]) => {
    const n = boardItems(k).length;
    if (S.board.folded.has(k)) return `<button class="tab rail" data-tab="${esc(k)}" aria-label="Unfold ${esc(l)}">${esc(l.slice(0, 4))} ${n}</button>`;
    return `<button class="tab ${cur && cur[0] === k ? "on" : ""}" data-tab="${esc(k)}">${esc(l)} <b>${n}</b></button>`;
  }).join("");
  if (folded.length) tabs += `<button class="tab unf" data-act="unfoldall">Z unfold</button>`;
  tabs += optsTab();
  let body: string;
  if (!cur) body = `<div class="bcol"><div class="empty">Every column is folded.<br><br><button data-act="unfoldall">Unfold all</button></div></div>`;
  else {
    const items = boardItems(cur[0]);
    let inner = "";
    if (!items.length) inner = `<div class="empty">No cards in ${esc(cur[1])}.</div>`;
    else if (S.board.lanes === "rows" && S.board.group !== "type") {
      for (const [l, arr] of laneKeys(items)) {
        inner += laneHTML(l, arr.length);
        if (!S.board.laneFold.has(l)) inner += arr.map(cardHTML).join("");
      }
    } else inner = items.map(cardHTML).join("");
    let rail = "";
    if (S.board.lanes === "rail" && S.board.group !== "type") rail = railHTML([cur[0]]);
    body = `${rail}<div class="bcol" id="bcol" role="list">${inner}</div><div class="edgehint l">‹</div><div class="edgehint r">›</div>`;
  }
  const old = document.getElementById("bcol");
  const keepScroll = old ? old.scrollTop : 0;
  v.innerHTML = `<div class="tabs" id="tabs">${tabs}</div><div class="bbody" id="bbody">${body}</div>`;
  const nc = document.getElementById("bcol");
  if (nc && S.board.keep) nc.scrollTop = keepScroll;
  S.board.keep = false;
}

/* ================= search ================= */

const PRED = ["status:", "type:", "priority:", "assignee:", "label:", "id:", "title:", "project:", "!status:closed"];

export function suggestions(val: string): { t: string; v: boolean }[] {
  const tok = val.split(/\s+/).pop() || "";
  const m = tok.match(/^(!?)(\w+):(.*)$/);
  if (!m) return PRED.filter(p => !tok || p.startsWith(tok)).map(t => ({ t, v: false }));
  const k = m[2].toLowerCase(), v = m[3].toLowerCase();
  const P = pool();
  let vals: string[] = [];
  if (k === "status") vals = [...new Set(P.map(i => i.status))].sort();
  if (k === "type") vals = [...new Set(P.map(i => i.type))].sort();
  if (k === "priority") vals = ["0", "1", "2", "3", "4"];
  if (k === "assignee") vals = [...new Set(P.map(i => i.assignee).filter(Boolean))].sort();
  if (k === "label") vals = [...new Set(P.flatMap(i => i.labels))].sort();
  if (k === "project") vals = [...new Set(P.map(projectOf))].sort();
  if (k === "id") vals = P.filter(i => i.type === "epic" && !i.closed_like).map(i => shortId(i.id));
  return vals.filter(x => x.toLowerCase().startsWith(v) && x.toLowerCase() !== v).slice(0, 30).map(t => ({ t, v: true }));
}

function renderSuggestions(): void {
  const val = $<HTMLInputElement>("#sIn").value;
  $("#sSugg").innerHTML = suggestions(val).map(s => `<button class="chip ${s.v ? "v" : ""}" data-sugg="${esc(s.t)}">${esc(s.t)}</button>`).join("")
    || `<span class="chip" style="color:var(--muted)">no completions</span>`;
}

/** The last search result, keyed by the text and data version it answers. */
const search = { text: "", version: -1, ids: null as string[] | null, error: "", pending: "" };
let searchTimer = 0;

export function renderSearch(): void {
  const q = $<HTMLInputElement>("#sIn").value.trim();
  renderSuggestions();
  if (!q) {
    const rec = store.get<string[]>("recent", []);
    $("#sList").innerHTML = (rec.length ? `<div class="sec">Recent queries</div>` + rec.map(r => `<button class="item" data-recent="${esc(r)}"><span class="g">↺</span><span class="x">${esc(r)}</span></button>`).join("") : "")
      + `<div class="sec">Syntax</div><div class="md" style="padding:0 16px"><p><code>status:open</code> <code>type:bug</code> <code>priority:1</code> <code>assignee:alice</code> <code>label:mobile</code> <code>project:b9s</code> <code>id:foit</code> <code>title:graph</code>. The same field twice means either value. <code>!</code> negates, and a bare <code>!closed</code> means not that status. Plain words match ID and title, fuzzily. It is the TUI's query language, evaluated by b9s.</p></div>`;
    return;
  }
  if (search.text !== q || search.version !== D.version) {
    if (search.pending !== q + "@" + D.version) {
      search.pending = q + "@" + D.version;
      clearTimeout(searchTimer);
      searchTimer = window.setTimeout(() => {
        const ver = D.version;
        api.query(q).then(r => { Object.assign(search, { text: q, version: ver, ids: r.ids, error: r.error || "" }); },
          e => { Object.assign(search, { text: q, version: ver, ids: [], error: e instanceof Error ? e.message : String(e) }); })
          .finally(() => { search.pending = ""; if (S.view === "search" && $<HTMLInputElement>("#sIn").value.trim() === q) renderSearch(); });
      }, 150);
    }
    if (search.text !== q) { $("#sList").innerHTML = `<div class="shead"><span>Searching…</span></div>`; return; }
  }
  if (search.error) { $("#sList").innerHTML = `<div class="empty err">${esc(search.error)}</div>`; return; }
  const res = (search.ids || []).map(get).filter((x): x is Item => !!x).sort(SORTS[S.sort]);
  $("#sList").innerHTML = `<div class="shead"><span>${res.length} match${res.length === 1 ? "" : "es"}${res.length > 200 ? ", first 200 shown" : ""}</span><button data-act="applysearch">Apply to tree ›</button></div><div class="${S.wrap ? "wrap" : ""}" id="sRows" role="list">${res.slice(0, 200).map(i => rowHTML({ i, d: 0 }, true)).join("")}</div>`;
}

/**
 * syncQuery fetches the server's verdict on the tree's text query whenever
 * the text or the data changed since the last one.
 */
let queryInFlight = "";
export function syncQuery(): void {
  const text = S.text.trim();
  if (!text) { S.qres = null; return; }
  if (S.qres && S.qres.text === S.text && S.qres.version === D.version) return;
  const key = S.text + "@" + D.version;
  if (queryInFlight === key) return;
  queryInFlight = key;
  const want = S.text, ver = D.version;
  api.query(text).then(
    r => { S.qres = { text: want, version: ver, ids: new Set(r.ids), error: r.error || "" }; },
    e => { S.qres = { text: want, version: ver, ids: new Set(), error: e instanceof Error ? e.message : String(e) }; },
  ).finally(() => { if (queryInFlight === key) queryInFlight = ""; if (S.text === want) render(); });
}

/* ================= more / graph ================= */

function renderMore(): void {
  const it = (a: string, g: string, x: string, sub: string, v?: string | null, on?: boolean) =>
    `<button class="item" data-act="${a}"><span class="g">${g}</span><span class="x">${x}${sub ? `<small>${sub}</small>` : ""}</span>${v != null ? `<span class="v ${on ? "on" : ""}">${esc(v)}</span>` : ""}</button>`;
  const h = D.health;
  $("#moreList").innerHTML =
    `<div class="sec">Project</div>`
    + it("projects", "▤", "Projects", "slots 1-9 · all projects", D.project.all ? "all" : D.project.name)
    + it("health", "●", "Data source health", "source · watcher · fallbacks (D)", h ? (h.ok ? "ok" : "problem") : "…", !!h && h.ok)
    + it("identity", "@", "Acting as", "author of your issues and comments", D.actor || "unknown")
    + `<div class="sec">Views</div>`
    + it("graph", "⋔", "Dependency graph", "blocked-by and blocks from the cursor", null)
    + it("treeopts", "≣", "Tree options", "sort, columns, wrap, text size, expand and collapse all", null)
    + it("boardopts", "▥", "Board options", "group, epic lanes, folded columns", S.board.group)
    + it("follow", "F", "Follow mode", "jump to each issue a live change touches", S.follow ? "on" : "off", S.follow)
    + it("togglechips", "⌄", "Filter chip row", "also: swipe down on the header", S.chips ? "shown" : "hidden", S.chips)
    + `<div class="sec">Connection</div>`
    + it("pairing", "⌁", "Pairing", "b9s web · tailscale serve · pairing link", null)
    + it("log", "$", "Write log", "every bd command this browser asked for", String(S.log.length))
    + it("reload", "↻", "Reload", "also: pull down on the tree", null)
    + it("help", "?", "Gestures", "every gesture and its TUI key", null);
}

interface GRow { id: string; depth: number; cycle: boolean }

/** chain walks blocked-by or blocks edges depth-first; an ID already on the path marks a cycle. */
export function chain(id: string, next: (i: Item) => string[], limit = 200): GRow[] {
  const out: GRow[] = [];
  const walk = (x: string, depth: number, path: string[]) => {
    const i = get(x);
    if (!i || out.length >= limit) return;
    for (const n of next(i)) {
      if (!get(n) || out.length >= limit) continue;
      const cycle = path.includes(n);
      out.push({ id: n, depth, cycle });
      if (!cycle && depth < 12) walk(n, depth + 1, [...path, n]);
    }
  };
  walk(id, 0, [id]);
  return out;
}

const blockedByOf = (i: Item) => i.blocked_by;
const blocksIds = (i: Item) => blocksOf(i).map(x => x.id);

function renderGraph(): void {
  const v = $("#vGraph");
  if (!S.graph.length || !get(S.graph[S.graph.length - 1])) {
    const P = pool();
    const c = (S.cursor && get(S.cursor)) || P.find(i => i.blocked_by.length) || P[0];
    if (!c) { v.innerHTML = `<div class="empty">No issues.</div>`; return; }
    S.graph = [c.id];
  }
  const id = S.graph[S.graph.length - 1], i = get(id)!, st = stOf(eff(i));
  const rowsOf = (rs: GRow[]) => rs.map(r => {
    const html = rowHTML({ i: get(r.id)!, d: 0 }, true);
    const pad = `<div class="rc" style="--d:${Math.min(r.depth, 6)}">`;
    const cyc = r.cycle ? `<span class="err" title="cycle">↻</span>` : "";
    return html.replace(`<div class="rc" style="--d:0">`, pad).replace(`<span class="st"`, `${cyc}<span class="st"`);
  }).join("");
  const sec = (label: string, rs: GRow[]) => rs.length ? `<div class="gsec">${label} <b>${rs.length}</b></div>` + rowsOf(rs) : "";
  const flat = (ids: string[]) => ids.filter(x => get(x)).map(x => ({ id: x, depth: 0, cycle: false }));
  const up = i.parent && get(i.parent) ? [i.parent] : [];
  const bb = chain(id, blockedByOf), bl = chain(id, blocksIds);
  v.innerHTML = `<div class="crumbs">${S.graph.map((g, n) => `<button data-crumb="${n}">${esc(shortId(g))}</button>`).join("<span style='color:var(--muted)'>›</span>")}</div>
    <div class="scroll" id="gList" role="list"><button class="gnode" data-open="${esc(id)}" style="display:block;width:calc(100% - 24px);text-align:left"><div class="l1"><span class="st" style="color:${st.c}">${st.g}</span><span class="id">${esc(i.id)}</span></div><div class="t2">${esc(i.t)}</div>${l2HTML(i)}<div class="pager" style="margin-top:4px">tap for detail · tap a row below to go there</div></button>
    ${sec("Blocked by", bb)}${sec("Blocks", bl)}${sec("Parent", flat(up))}${sec("Children", flat(kids(id)))}${sec("Discovered from", flat(i.discovered_from))}
    ${!bb.length && !bl.length ? `<div class="empty">No blocking relations. Parent and children are still listed.</div>` : ""}</div>`;
}

/* ================= detail sheet ================= */

const sat = () => parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--sat")) || 0;
export const appH = () => $("#app").clientHeight;
export const appW = () => $("#app").clientWidth;
export const halfH = () => Math.round(appH() * 0.52);
export const fullH = () => appH() - sat() - 36;
/** detailHeightPx is what the sheet covers of a list; the wide side panel covers none. */
export const detailHeightPx = () => (wide() ? 0 : S.detail && S.detail.size === "full" ? fullH() : halfH());

/** siblings are the rows next to an issue in the current tree, which the detail pages through. */
export function siblings(id: string): string[] {
  const i = get(id);
  if (!i) return [id];
  const { rows } = treeRows();
  const sib = rows.filter(r => r.i && (r.i.parent || "") === (i.parent || "")).map(r => r.i!.id);
  return sib.includes(id) ? sib : [id];
}

/** Full issues from api/issue, kept until the data version moves on. */
const full = new Map<string, { version: number; issue: Issue }>();
const fetching = new Set<string>();

export function fullIssue(id: string): Issue | null {
  const c = full.get(id);
  if (c && c.version === D.version) return c.issue;
  const key = id + "@" + D.version;
  if (!fetching.has(key)) {
    fetching.add(key);
    const ver = D.version;
    api.issue(id).then(issue => { full.set(id, { version: ver, issue }); }, () => { /* the snapshot still shows the lean fields */ })
      .finally(() => { fetching.delete(key); if (S.detail && S.detail.stack[S.detail.stack.length - 1] === id) renderDetail(); });
  }
  return c ? c.issue : null;
}

function relHTML(x: string): string {
  const j = get(x);
  if (!j) return "";
  const st = stOf(eff(j));
  return `<button class="rel" data-nav="${esc(x)}"><span class="st" style="color:${st.c}">${st.g}</span><span class="id">${esc(shortId(x))}</span><span class="t">${esc(j.t)}</span></button>`;
}

function textSec(label: string, text: string | undefined, loading: boolean): string {
  if (text) return `<div class="dsec">${label}</div><div class="md">${md(text)}</div>`;
  if (label === "Description") return `<div class="dsec">${label}</div><div class="md"><p style="color:var(--muted)">${loading ? "Loading…" : "No description."}</p></div>`;
  return "";
}

export function renderDetail(): void {
  const host = $("#detailHost");
  $("#app").classList.toggle("hasdetail", !!S.detail);
  if (!S.detail) { host.innerHTML = ""; return; }
  const id = S.detail.stack[S.detail.stack.length - 1], i = get(id);
  if (!i) { S.detail = null; host.innerHTML = ""; $("#app").classList.remove("hasdetail"); return; }
  const f = fullIssue(id);
  const st = stOf(eff(i)), [tg, tc] = tyOf(i.type);
  const sib = siblings(id), pos = sib.indexOf(id);
  const pr = progress(id);
  const ob = i.blocked_by.filter(x => get(x)), bl = blocksOf(i).map(x => x.id);
  const existing = host.querySelector<HTMLElement>(".detail");
  const scrollTop = existing && existing.dataset.id === id ? existing.querySelector(".dbody")!.scrollTop : 0;
  const cms = f ? f.comments : [];
  const ro = D.project.read_only;
  host.innerHTML = `<section class="detail ${existing ? "" : "enter"}" data-id="${esc(id)}" style="--h:${detailHeightPx()}px" aria-label="Issue ${esc(shortId(id))}" role="dialog">
    <div class="dhead" id="dhead"><div class="grab"></div>
      <div class="dtop">${S.detail.stack.length > 1 ? `<button class="back" data-act="dback">‹ ${esc(shortId(S.detail.stack[S.detail.stack.length - 2]))}</button>` : ""}<span class="id">${esc(i.id)}</span><span class="sp"></span><span class="pager">${pos + 1}/${sib.length} ‹ swipe ›</span><button data-act="dsize" aria-label="Resize">${S.detail.size === "full" ? "⌄" : "⌃"}</button><button data-act="dclose" aria-label="Close">✕</button></div>
      <div class="dtitle">${esc(i.t)}</div>
      <div class="pills"><span class="pill" style="color:${st.c}">${st.g} ${esc(i.status === eff(i) ? st.w : i.status + " · " + st.w)}</span><span class="pill" style="color:${PC[i.priority] || "var(--muted)"}">P${i.priority}</span><span class="pill" style="color:${tc}">${tg} ${esc(i.type)}</span>${i.labels.map(l => `<span class="pill" style="color:var(--cyan)">#${esc(l)}</span>`).join("")}</div>
    </div>
    <div class="dbody" id="dbody">
      <dl class="meta"><dt>assignee</dt><dd>${i.assignee ? "@" + esc(i.assignee) : "—"}</dd><dt>creator</dt><dd>${esc(i.created_by || "—")}</dd><dt>created</dt><dd>${fmtDate(i.crt)}${i.crt ? " · " + age(i.crt) : ""}</dd><dt>updated</dt><dd>${fmtDate(i.upd)}${i.upd ? " · " + age(i.upd) : ""}</dd>${i.cls ? `<dt>closed</dt><dd>${fmtDate(i.cls)}</dd>` : ""}${i.defer_until ? `<dt>deferred</dt><dd>until ${esc(i.defer_until.slice(0, 10))}</dd>` : ""}${D.project.all ? `<dt>project</dt><dd>${esc(projectOf(i))}</dd>` : ""}</dl>
      ${i.parent && get(i.parent) ? `<div class="dsec">Parent</div>${relHTML(i.parent)}` : ""}
      ${pr ? `<div class="dsec"><span>Children ${pr.done}/${pr.all}</span><button data-act="dfocus">⌖ subtree</button></div><div class="bar2"><i style="width:${Math.round(100 * pr.done / pr.all)}%"></i></div>${kids(id).map(k => get(k)!).sort(SORTS.status).map(k => relHTML(k.id)).join("")}` : ""}
      ${ob.length ? `<div class="dsec"><span style="color:var(--red)">Blocked by ${ob.length}</span><button data-act="dgraph">⋔ graph</button></div>${ob.map(relHTML).join("")}` : ""}
      ${bl.length ? `<div class="dsec"><span style="color:var(--orange)">Blocks ${bl.length}</span><button data-act="dgraph">⋔ graph</button></div>${bl.map(relHTML).join("")}` : ""}
      ${i.discovered_from.some(x => get(x)) ? `<div class="dsec">Discovered from</div>${i.discovered_from.map(relHTML).join("")}` : ""}
      ${i.related.some(x => get(x)) ? `<div class="dsec">Related</div>${i.related.map(relHTML).join("")}` : ""}
      ${textSec("Description", f?.description, !f)}${textSec("Design", f?.design, !f)}${textSec("Acceptance", f?.acceptance, !f)}${textSec("Notes", f?.notes, !f)}
      <div class="dsec"><span>Comments ${f ? cms.length : i.comment_count}</span>${ro ? "" : `<button data-act="comment">+ add</button>`}</div>
      ${cms.map(c => `<div class="cm"><div class="who"><b>${esc(c.author || "unknown")}</b> · ${age(Date.parse(c.created_at) || 0)}</div><div class="md">${md(c.text)}</div></div>`).join("") || `<div class="md"><p style="color:var(--muted)">${f || !i.comment_count ? "No comments." : "Loading…"}</p></div>`}
    </div>
    <div class="dacts">${ro ? "" : `<button data-act="status"><span class="g">◐</span>Status</button><button data-act="edit"><span class="g">✎</span>Edit</button><button data-act="comment"><span class="g">✉</span>Comment</button>`}<button data-act="dbranch"><span class="g">⎇</span>Branch</button><button data-act="actions"><span class="g">⋯</span>More</button></div>
  </section>`;
  const sec = host.querySelector<HTMLElement>(".detail")!;
  sec.querySelector(".dbody")!.scrollTop = scrollTop;
  if (!existing) requestAnimationFrame(() => requestAnimationFrame(() => sec.classList.remove("enter")));
}

export function revealInTree(id: string): void { ancestors(id).forEach(a => S.folded.delete(a)); }

export function ensureVisible(id: string): void {
  if (S.view !== "tree") return;
  const tl = treeList();
  const row = $("#treeRows").querySelector<HTMLElement>(`.row[data-id="${CSS.escape(id)}"]`);
  if (!row) return;
  const avail = tl.clientHeight - (S.detail ? detailHeightPx() : 0);
  const top = row.offsetTop - tl.offsetTop;
  if (top < tl.scrollTop || top + row.offsetHeight > tl.scrollTop + avail) tl.scrollTo({ top: Math.max(0, top - Math.max(44, avail / 3)) });
}
