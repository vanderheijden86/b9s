// Every view renders HTML strings into the fixed hosts in index.html.
// Gestures and taps are bound once, by delegation, in gestures.ts, so a
// render never has to rebind anything.

import * as api from "./api";
import type { Issue } from "./api.gen";
import { D, ancestors, blocksOf, descendants, eff, get, isReady, kids, laneOf, openBlockers, pool, progress, projectOf, shortId, type Item } from "./data";
import { DEFAULT_CHIPS, PC, S, SORTS, chipRowVisible, laneHeight, matches, nextLaneHeight, queryString, stOf, textMatches, treeRows, tyOf, typeTerms, type LaneHeight, type TreeRow } from "./state";
import { syncHistory } from "./nav";
import { renderMermaidBlocks } from "./mermaid";
import { renderMemory, renderMemoryDetail } from "./memory";
import { $, $opt, age, desktop, esc, fmtDate, md, sizeClass, split, store } from "./util";

const metaSep = '<span class="msep">-</span>';
const sizeClassNames = ["phone", "tablet", "desktop", "wide"] as const;
let cachedDetailPlace: { id: string; place: TextPlace } | null = null;
let suppressDetailScrollCacheUntil = 0;
let detailResizeGuardBound = false;

function currentSizeClass(): (typeof sizeClassNames)[number] | null {
  const app = $opt("#app");
  return sizeClassNames.find(c => app?.classList.contains(c)) || null;
}

function bindDetailResizeGuard(): void {
  if (detailResizeGuardBound) return;
  detailResizeGuardBound = true;
  window.addEventListener("resize", () => { suppressDetailScrollCacheUntil = performance.now() + 300; }, { passive: true });
}

function statusDotHTML(status: string): string {
  return `<span class="st" style="--sc:${stOf(status).c}" aria-hidden="true"></span>`;
}

function statusMetaHTML(status: string): string {
  const st = stOf(status);
  return `<span class="mi stw" style="--sc:${st.c}"><span class="sdot" aria-hidden="true"></span>${esc(st.w)}</span>`;
}

export function l2HTML(i: Item, opts: { proj?: boolean; status?: boolean } = {}): string {
  const bits: string[] = [];
  if (S.cols.has("status") && opts.status !== false) bits.push(statusMetaHTML(eff(i)));
  if (S.cols.has("prio")) bits.push(`<span class="mi prio">P${i.priority}</span>`);
  if (S.cols.has("type")) bits.push(`<span class="mi typ">${esc(i.type)}</span>`);
  if (S.cols.has("who") && i.assignee) bits.push(`<span class="mi who">@${esc(i.assignee)}</span>`);
  if (S.cols.has("age")) bits.push(`<span class="mi age">${age(i.upd)}</span>`);
  const pr = progress(i.id);
  if (S.cols.has("kids") && pr) bits.push(`<span class="mi count">${pr.done}/${pr.all}</span>`);
  if (S.cols.has("deps")) {
    const ob = openBlockers(i).length;
    if (ob) bits.push(`<span class="mi count">blocked-by ${ob}</span>`);
    const bl = blocksOf(i).filter(x => !x.closed_like).length;
    if (bl) bits.push(`<span class="mi count">blocks ${bl}</span>`);
  }
  if (S.cols.has("notes") && i.comment_count) bits.push(`<span class="mi count">comments ${i.comment_count}</span>`);
  if (S.cols.has("labels") && i.labels.length) bits.push(`<span class="mi label">#${esc(i.labels.join(" #"))}</span>`);
  if (opts.proj && D.project.all) bits.push(`<span class="mi proj">${esc(projectOf(i))}</span>`);
  return `<div class="l2">${bits.join(metaSep)}</div>`;
}

export function rowHTML(r: TreeRow, flat = false): string {
  if (r.hdr) return `<div class="ph" data-hdr="${esc(r.key)}" role="button" tabindex="0"><span>${r.folded ? "▸" : "▾"}</span><span>${esc(r.hdr)}</span><span class="n">${r.n}</span></div>`;
  const i = r.i!, st = stOf(eff(i));
  const cls = ["row", S.cursor === i.id ? "sel" : "", r.dim ? "dim" : "", S.marks.has(i.id) ? "marked" : "", S.flash.has(i.id) ? "flash" : ""].join(" ");
  const chev = flat ? "" : `<span class="chev ${r.hasKids ? "" : "none"}" data-chev aria-hidden="true">${r.hasKids ? (r.folded ? "▸" : "▾") : "·"}</span>`;
  const mk = S.marking ? `<span class="mk">${S.marks.has(i.id) ? "▪" : "▫"}</span>` : "";
  const label = `${shortId(i.id)} ${i.t}, ${st.w}, P${i.priority} ${i.type}${S.marks.has(i.id) ? ", marked" : ""}`;
  return `<div class="${cls}" data-id="${esc(i.id)}" role="listitem" tabindex="0" aria-label="${esc(label)}"><div class="ul"></div><div class="ur"></div><div class="rc" style="--d:${flat ? 0 : Math.min(r.d, 6)}"><div class="l1">${mk}${chev}<span class="id">${esc(shortId(i.id))}</span><span class="t">${esc(i.t)}</span></div>${l2HTML(i, { proj: true })}</div></div>`;
}

/* ================= shell ================= */

export function render(): void {
  if (!D.loaded) return;
  // Both reads come while the layout is still clean, so the reads inside the views below force none,
  // and the detail's place comes first: once the size class moves, the layout no longer holds it (textPlace).
  const oldClass = currentSizeClass();
  const currentPlace = detailPlace();
  const nextClass = sizeClass();
  const was = oldClass && oldClass !== nextClass && cachedDetailPlace && (!currentPlace || cachedDetailPlace.id === currentPlace.id)
    ? cachedDetailPlace
    : currentPlace;
  renderHeader(); renderChips(); renderBar(); renderStale();
  (["tree", "board", "search", "more", "graph", "memory"] as const).forEach(v => { $("#v" + v[0].toUpperCase() + v.slice(1)).hidden = S.view !== v; });
  if (S.view === "tree") renderTree();
  if (S.view === "board") renderBoard();
  if (S.view === "search") renderSearch();
  if (S.view === "more") renderMore();
  if (S.view === "graph") renderGraph();
  if (S.view === "memory") renderMemory();
  renderDetail(was);
  syncQuery();
  syncHistory();
}

/** renderBanner fills the strip a server's --banner asks for, or hides it. */
export function renderBanner(text: string, link: string): void {
  const el = $("#banner");
  el.hidden = !text;
  if (!text) return;
  const href = /^https?:\/\//.test(link) ? link : "";
  el.innerHTML = `<span class="t">${esc(text)}</span>${href ? `<a href="${esc(href)}" target="_blank" rel="noopener">Get b9s ↗</a>` : ""}`;
}

function renderHeader(): void {
  const P = pool();
  const n = (s: string) => P.filter(i => eff(i) === s).length;
  const pname = D.project.all ? "all" : D.project.name;
  const bad = S.live === "stale" || (D.health && !D.health.ok);
  const proj = `<span class="prompt">$</span> b9s <small>/</small> ${esc(pname)}${D.project.read_only ? ` <span class="ro">ro</span>` : ""}`;
  const stat = (label: string, color: string, count: number) => `<span class="hstat" style="--sc:${color}"><span class="sdot" aria-hidden="true"></span>${count} ${label}</span>`;
  $("#hd").innerHTML = (D.public ? `<span class="proj">${proj}</span>` : `<button class="proj" data-act="projects" aria-label="Switch project">${proj} <small>▾</small></button>`) + `
    <button class="dot ${bad ? "bad" : ""}" id="hdot" data-act="health" aria-label="Data source health">●</button>
    <span class="cnt">${stat("open", "var(--open)", n("open"))}${stat("in progress", "var(--progress)", n("in_progress"))}${n("blocked") ? stat("blocked", "var(--danger)", n("blocked")) : ""}</span>
    <button class="ib follow ${S.follow ? "on" : ""}" data-act="follow" aria-label="Follow mode" aria-pressed="${S.follow}">F</button>
    <button class="ib" data-act="create" aria-label="Create issue">+</button>
    <button class="ib" data-act="help" aria-label="Gestures">?</button>`;
}

/**
 * renderChips draws the status and type filters. On the tree a status chip counts every issue
 * with that status; on the board it counts cards, with the board's own filter
 * (boardCard), so the Open chip and the Open box header agree.
 */
function renderChips(): void {
  const el = $("#chips");
  el.hidden = !chipRowVisible();
  const P = pool();
  const onBoard = S.view === "board";
  const keep = onBoard ? focusSet() : null;
  const counted = onBoard ? P.filter(i => boardCounted(i, keep)) : P;
  const cnt = (s: string) => counted.filter(i => eff(i) === s).length;
  const fmt = (n: number) => (n >= 1000 ? (n / 1000).toFixed(1) + "k" : String(n));
  const chip = (k: string, key: string, label: string, color: string, on: boolean, n: number | null) =>
    `<button class="chip ${on ? "on" : ""}" data-chip="${key}" style="--sc:${color}" aria-pressed="${on}" title="${esc(k)}">${label}${n != null ? ` <span class="n">${fmt(n)}</span>` : ""}</button>`;
  let h = chip("o", "open", "Open", "var(--open)", S.st.has("open"), cnt("open"))
    + chip("i", "in_progress", "In prog", "var(--progress)", S.st.has("in_progress"), cnt("in_progress"))
    + chip("b", "blocked", "Blocked", "var(--danger)", S.st.has("blocked"), cnt("blocked"))
    + chip("d", "deferred", "Deferred", "var(--priority)", S.st.has("deferred"), cnt("deferred"))
    + chip("C", "closed", "Closed", "var(--muted)", S.st.has("closed"), cnt("closed"))
    + chip("r", "ready", "Ready", "var(--priority)", S.ready, counted.filter(isReady).length)
    + chip("a", "all", "All", "var(--dim)", S.st.size === 0 && !S.ready, null);
  h += `<span class="sep"></span>`;
  const types = typeTerms();
  for (const name of ["epic", "feature", "task", "bug", "chore"]) {
    h += chip(`Show ${name} issues`, `type:${name}`, name[0].toUpperCase() + name.slice(1), tyOf(name)[1], types.includes(`type:${name}`), null);
  }
  h += `<span class="sep"></span>`;
  if (S.text.trim()) {
    const err = S.qres && S.qres.text === S.text && S.qres.error;
    h += `<button class="chip q ${err ? "err" : ""}" data-act="search"><span>⌕ ${esc(S.text.trim())}</span></button><button class="chip" data-act="clearq" aria-label="Clear query">✕</button>`;
  } else h += `<button class="chip" data-act="search">⌕ query</button>`;
  h += `<button class="chip" data-act="sort">⇅ ${S.sort}</button>`;
  // The phone board has no toolbar of its own, so its options sit at the end of the chip row.
  if (onBoard && !split()) h += `<button class="chip" data-act="boardopts" aria-label="Board options">⚙ ${esc(S.board.group)}</button>`;
  el.innerHTML = h;
}

function renderBar(): void {
  const el = $("#bar");
  const covered = !!S.detail && !split();
  el.toggleAttribute("aria-hidden", covered);
  el.toggleAttribute("inert", covered);
  // The sidebar of the desktop classes carries the project above the views and the connection
  // below them, as the direction A mockup does (docs/mockups/responsive-web/shots/a-desktop.png).
  const side = desktop();
  const brand = side ? `<div class="sbrand"><b>b9s</b><small>${esc(D.project.all ? "all projects" : D.project.name)}${D.project.read_only ? " · ro" : ""}</small></div>` : "";
  const live = S.live === "live" ? "connected" : S.live === "stale" ? "offline" : "connecting";
  const foot = side ? `<div class="sfoot"><span class="slive ${S.live}"><span class="sdot" aria-hidden="true"></span>${live}</span>${D.public ? "" : `<button data-act="projects">project switcher</button>`}</div>` : "";
  if (S.marking) {
    el.className = "bar bulk";
    el.innerHTML = brand + `<button class="cnt" data-act="markall">${S.marks.size} marked<br><span style="font-weight:400;color:var(--muted)">tap: all</span></button>
      <button data-act="bulkstatus"><span class="g" aria-hidden="true">◐</span>Status</button>
      <button data-act="bulkclose"><span class="g" aria-hidden="true">✓</span>Close</button>
      <button class="red" data-act="bulkdelete"><span class="g" aria-hidden="true">⌫</span>Delete</button>
      <button data-act="unmark"><span class="g" aria-hidden="true">✕</span>Done</button>` + foot;
    return;
  }
  el.className = "bar";
  const tab = (v: string, g: string, l: string) => {
    const on = S.view === v || (v === "more" && S.view === "graph");
    return `<button class="${on ? "on" : ""}" data-view="${v}" aria-current="${on ? "page" : "false"}"><span class="g" aria-hidden="true">${g}</span>${l}</button>`;
  };
  el.innerHTML = brand + (S.list ? tab("tree", "☰", "List") : tab("tree", "≣", "Tree")) + tab("board", "▥", "Board") + tab("memory", "◇", "Memory") + tab("search", "⌕", "Search") + tab("more", "⋯", "More") + foot;
}

export function renderStale(): void {
  const el = $("#stale");
  // Tests wait on this before they act: the render that follows a connect
  // replaces every row.
  document.body.dataset.live = S.live;
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
  el.innerHTML = rows.length ? rows.map(r => rowHTML(r, S.list)).join("")
    : `<div class="empty">No issues match <b>${esc(queryString())}</b>.<br><br><button data-act="allq">Show all statuses</button></div>`;
  treeList().style.paddingBottom = S.detail ? detailHeightPx() + "px" : "";
}

/* ================= board ================= */

const CLOSED_BATCH = 50;

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

/** defaultChips says whether the status chips are a first visit's set, or none: anything narrower is a filter the user chose. */
const defaultChips = (): boolean => !S.st.size || DEFAULT_CHIPS.every(k => S.st.has(k));

/**
 * phoneBoardCols adds the Closed box to the status board under the default
 * chips, which leave closed out, and under none: a phone has no bar that
 * counts the closed work, so the box, folded by default, is where it is. A
 * narrower filter shows Closed only with its chip on, as the wide board does.
 */
function phoneBoardCols(): [string, string][] {
  const cols = boardCols();
  if (S.board.group === "status" && defaultChips() && !cols.some(([k]) => k === "closed")) return [...cols, ["closed", "Closed"]];
  return cols;
}

/** focusSet is the focused issue and its subtree, the only issues the board shows while a focus is on. */
const focusSet = (): Set<string> | null => (S.focus ? new Set([S.focus.id, ...descendants(S.focus.id)]) : null);

/**
 * boardCard says whether an issue can be a card on the board: epics are lane
 * headers unless lanes are off or the board groups by type, and a focus keeps
 * only its subtree. The status chips and the text query come on top (inBoard).
 */
function boardCard(i: Item, keep: Set<string> | null): boolean {
  if (i.type === "epic" && S.board.group !== "type" && S.board.lanes !== "off") return false;
  return !keep || keep.has(i.id);
}

const inBoard = (i: Item, keep: Set<string> | null): boolean => boardCard(i, keep) && matches(i);

/**
 * boardCounted is the board's filter without the status chips. The chip row
 * counts cards with it, so a chip's number is what its box holds, and the
 * Closed column fills with it: the chips choose that column (boardCols,
 * phoneBoardCols), and every card in it is closed, so they have nothing left
 * to say about its cards. The default chips, which leave closed out, would
 * otherwise empty the Closed box a phone shows under them.
 */
const boardCounted = (i: Item, keep: Set<string> | null): boolean => boardCard(i, keep) && textMatches(i) && (!S.ready || isReady(i));

function boardItems(key: string): Item[] {
  const keep = focusSet();
  const ok = key === "closed" ? (i: Item) => boardCounted(i, keep) : (i: Item) => inBoard(i, keep);
  return pool().filter(i => colOf(i) === key && ok(i)).sort(SORTS[S.sort]);
}

function phoneBoardItems(key: string): Item[] {
  const items = boardItems(key);
  if (key !== "closed") return items;
  return [...items].sort((a, b) => (b.cls || b.upd) - (a.cls || a.upd));
}

export function openCols(): [string, string][] { return boardCols().filter(([k]) => !S.board.folded.has(k)); }

/** colColor is the dot of a column header: the status, priority or type colour of the column's key. */
function colColor(k: string): string {
  if (S.board.group === "status") return stOf(k).c;
  if (S.board.group === "priority") return PC[Number(k)] || "var(--muted)";
  return tyOf(k)[1];
}

/** lanesShown says whether cards sit under epic lane headers, so a card's epic is already in view. */
const lanesShown = () => S.board.lanes !== "off" && S.board.group !== "type";

/** railShown says whether the board draws the epic rail, whose lanes step through their heights (LaneHeight) rather than fold. */
export const railShown = (): boolean => split() && S.board.lanes === "rail" && lanesShown();

/**
 * parentLineHTML names a card's direct parent when its lane header names only
 * the epic above it, so a grandchild does not read as a child of the epic.
 */
function parentLineHTML(i: Item): string {
  const p = i.parent ? get(i.parent) : undefined;
  if (!p || !lanesShown() || p.id === laneOf(i)) return "";
  if (S.cursor !== i.id && i.parent && cellIds.has(i.parent)) return "";
  return `<div class="l3">↳ <span class="e">${esc(shortId(p.id))}</span> ${esc(p.t)}</div>`;
}

/**
 * cardTagsHTML are a card's right-aligned tags in the terminal's order
 * (docs/adr/0030): the open blocker, the dispatcher lane, the downstream
 * count, and P0 or P1. Lower priorities and the age wait for the open card.
 */
function cardTagsHTML(i: Item): string {
  const tags: string[] = [];
  const ob = i.closed_like ? [] : openBlockers(i);
  if (ob.length) tags.push(`<span class="tb">blocked ${esc(shortId(ob[0]))}</span>`);
  const stage = i.labels.find(l => l.startsWith("lane-stage="));
  if (stage) tags.push(`<span class="tl">lane: ${esc(stage.slice(11).trim())}</span>`);
  const bl = blocksOf(i).filter(x => !x.closed_like).length;
  if (bl) tags.push(`<span class="tk">blocks ${bl}</span>`);
  if (i.priority <= 1) tags.push(`<span class="tp">P${i.priority}</span>`);
  return tags.length ? `<span class="tags">${tags.join("")}</span>` : "";
}

/**
 * cardHTML draws a card as one line: type icon, ID, title and tags. The card
 * under the cursor opens into a box with the wrapped title, the meta line and
 * the parent. depth indents a card below its parent in the same cell.
 */
function cardHTML(i: Item, depth = 0): string {
  const st = stOf(eff(i));
  const sel = S.cursor === i.id;
  const open = sel ? `${l2HTML(i, { proj: true, status: S.board.group !== "status" })}` : "";
  return `<div class="card ${sel ? "sel" : ""} ${S.flash.has(i.id) ? "flash" : ""}" data-card="${esc(i.id)}" tabindex="0" role="button" aria-label="${esc(shortId(i.id) + " " + i.t + ", " + st.w)}" style="--d:${depth}"><div class="l1">${statusDotHTML(eff(i))}<span class="id">${esc(shortId(i.id))}</span><span class="t">${esc(i.t)}</span>${cardTagsHTML(i)}</div>${open}${parentLineHTML(i)}</div>`;
}

/**
 * cellParent is the card a card hangs under inside one cell: its parent, when
 * that parent sits in the same cell and is not the lane's epic, which the
 * lane header names already.
 */
function cellParent(i: Item, inCell: Set<string>): string | null {
  const p = i.parent;
  if (!p || !inCell.has(p) || p === laneOf(i)) return null;
  const pi = get(p);
  return pi && laneOf(pi) === laneOf(i) ? p : null;
}

/**
 * treeOrder keeps a cell's sort but puts each child straight below its
 * parent, the terminal's board_hierarchy.go. A parent cycle cannot hide a
 * card: every card not reached from a root is appended at depth 0.
 */
export function treeOrder(arr: Item[]): [Item, number][] {
  const inCell = new Set(arr.map(i => i.id));
  const under = new Map<string, Item[]>();
  const roots: Item[] = [];
  for (const i of arr) {
    const p = cellParent(i, inCell);
    if (!p) { roots.push(i); continue; }
    const k = under.get(p);
    if (k) k.push(i); else under.set(p, [i]);
  }
  const out: [Item, number][] = [], seen = new Set<string>();
  const walk = (i: Item, d: number) => {
    if (seen.has(i.id)) return;
    seen.add(i.id); out.push([i, d]);
    for (const c of under.get(i.id) || []) walk(c, d + 1);
  };
  roots.forEach(i => walk(i, 0));
  arr.forEach(i => walk(i, 0));
  return out;
}

/** cellIds are the cards of the cell being drawn, so a card indented under its parent skips the parent line. */
let cellIds = new Set<string>();

function cellCards(arr: Item[]): string {
  cellIds = new Set(arr.map(i => i.id));
  const html = treeOrder(arr).map(([i, d]) => cardHTML(i, d)).join("");
  cellIds = new Set();
  return html;
}

/** laneKeys orders epic lanes the way both boards do: biggest first, loose cards last. */
function laneKeys(items: Item[]): [string, Item[]][] {
  const lanes = new Map<string, Item[]>();
  items.forEach(i => { const l = laneOf(i) || "none"; const a = lanes.get(l); if (a) a.push(i); else lanes.set(l, [i]); });
  return [...lanes].sort((a, b) => Number(a[0] === "none") - Number(b[0] === "none") || b[1].length - a[1].length);
}

/**
 * laneHTML is an epic's lane header: its chevron and ID open the epic, and the
 * rest of the header, title included, folds the lane so the fold target is wide.
 */
function laneHTML(l: string, n: number, facts = false): string {
  const f = S.board.laneFold.has(l), e = get(l), pr = e ? progress(l) : null;
  const ef = facts && e ? epicFacts(l) : null;
  const done = ef ? { done: ef.done, all: ef.all } : pr;
  const bar = done && done.all ? `<span class="bar" title="${done.done}/${done.all} done"><i style="width:${Math.round((100 * done.done) / done.all)}%"></i></span>` : "";
  const lo = l === "none" ? "" : " data-lopen";
  const more = ef && e ? `<span class="lf">${epicCountsHTML(ef)}${ef.urgent ? `<span class="tp">P1 ${ef.urgent}</span>` : ""}${epicMetaHTML(e, ef)}</span>` : "";
  const sum = ef && e && e.summary ? `<span class="ls">${esc(e.summary)}</span>` : "";
  return `<button class="lane${ef ? " lfacts" : ""}" data-lane="${esc(l)}" aria-expanded="${!f}"><span class="ch"${lo}>${f ? "▸" : "▾"}</span><span class="ei"${lo}>${l === "none" ? "none" : esc(shortId(l))}</span><span class="t">${l === "none" ? "No epic" : esc(e ? e.t : l)}</span>${more}${bar}<span class="n">${ef ? `${ef.done}/${ef.all}` : n}</span>${sum}</button>`;
}

interface EpicFacts { done: number; all: number; urgent: number; wip: number; waiting: number; ready: number; last: number }

/**
 * epicFacts counts an epic's issues below it, whatever the filter, with the
 * terminal's rules (pkg/ui/board_epics.go): an open issue is in progress,
 * else waiting on an open blocker, else ready unless deferred.
 */
export function epicFacts(id: string): EpicFacts {
  const e = get(id);
  const f: EpicFacts = { done: 0, all: 0, urgent: 0, wip: 0, waiting: 0, ready: 0, last: e ? e.upd : 0 };
  for (const d of descendants(id)) {
    const i = get(d);
    if (!i) continue;
    f.all++;
    f.last = Math.max(f.last, i.upd);
    if (i.closed_like) { f.done++; continue; }
    if (i.priority <= 1) f.urgent++;
    if (i.status === "in_progress") f.wip++;
    else if (openBlockers(i).length) f.waiting++;
    else if (i.status !== "deferred") f.ready++;
  }
  return f;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** epicMetaHTML is the epic's labels, owner, due date and last activity. */
function epicMetaHTML(e: Item, f: EpicFacts): string {
  const bits: string[] = [];
  if (e.labels.length) bits.push(`<span class="lb">#${esc(e.labels.join(" #"))}</span>`);
  if (e.assignee) bits.push(`<span>@${esc(e.assignee)}</span>`);
  const due = e.due ? new Date(e.due) : null;
  if (due && !isNaN(+due)) bits.push(`<span class="${!e.closed_like && +due < Date.now() ? "od" : ""}">due ${due.getUTCDate()} ${MONTHS[due.getUTCMonth()]}</span>`);
  if (f.last) bits.push(`<span>${age(f.last)}</span>`);
  return bits.join("<span>·</span>");
}

function epicCountsHTML(f: EpicFacts): string {
  return [f.wip ? `<span class="cw">wip ${f.wip}</span>` : "", f.waiting ? `<span class="cb">waiting ${f.waiting}</span>` : "", f.ready ? `<span class="cr">ready ${f.ready}</span>` : ""].filter(Boolean).join("<span>·</span>");
}

/**
 * LANE_HEIGHTS give the epic cell's button for each height of its lane: the
 * glyph and the label say how tall the lane is, the title what the next
 * step makes it.
 */
const LANE_HEIGHTS: Record<LaneHeight, { glyph: string; label: string; step: string }> = {
  compact: { glyph: "▸", label: "Compact lane", step: "Compact: as tall as the epic" },
  open: { glyph: "▿", label: "Open lane", step: "Open: every open and in progress card" },
  full: { glyph: "▾", label: "Full lane", step: "Full: every card" },
};

/**
 * epicCellHTML is one epic in the rail's first column, the terminal's epic
 * cell: title, first sentence, progress over every issue below the epic,
 * what is moving, and who owns it by when. Its button steps the lane's height.
 */
function epicCellHTML(l: string, n: number): string {
  const e = get(l), h = laneHeight(l), now = LANE_HEIGHTS[h];
  const label = l === "none" ? "No epic" : `${shortId(l)} ${e ? e.t : l}`;
  let body = "";
  if (e) {
    const f = epicFacts(l);
    const pct = f.all ? Math.round((100 * f.done) / f.all) : 0;
    if (e.summary) body += `<div class="es">${esc(e.summary)}</div>`;
    if (f.all) body += `<div class="ep"><i style="width:${pct}%"></i></div><div class="en">${f.done}/${f.all}${f.urgent ? ` · <span class="tp">P1 ${f.urgent}</span>` : ""}</div>`;
    const counts = epicCountsHTML(f);
    if (counts) body += `<div class="ec">${counts}</div>`;
    const meta = epicMetaHTML(e, f);
    if (meta) body += `<div class="em">${meta}</div>`;
  } else body = `<div class="ec">${n} issue${n === 1 ? "" : "s"}</div>`;
  return `<div class="wepic lt ${S.cursor === l ? "sel" : ""}" data-epic="${esc(l)}" tabindex="0" aria-label="${esc(label)}, ${n} issues">`
    + `<div class="eh"><button data-lane="${esc(l)}" aria-label="${now.label}" title="${LANE_HEIGHTS[nextLaneHeight(h)].step} (Tab)">${now.glyph}</button>${l === "none" ? "<span>none</span>" : `<span class="ei">${esc(shortId(l))}</span>`}<span class="t">${esc(l === "none" ? "No epic" : e ? e.t : l)}</span></div>`
    + body + `</div>`;
}

function optsTab(): string {
  return `<button class="tab" data-act="boardopts" style="color:var(--muted)">⚙ ${S.board.group}${S.board.lanes !== "off" ? " · lanes " + S.board.lanes : ""}</button>`;
}

/**
 * railCell is a lane's cell in one column of the epic rail. A cell that must
 * not set its lane's height holds its cards in a box out of the flow, which
 * clips them: every cell of a Compact lane, whose height the epic cell sets,
 * and in an Open lane every cell but the open and in progress ones, which
 * set it. Only the box that holds the cursor's card scrolls (cur), so the
 * wheel over any other cell scrolls the board. A folded column stays an
 * empty rail.
 */
function railCell(k: string, lane: string, h: LaneHeight, arr: Item[]): string {
  if (S.board.folded.has(k)) return `<div class="wcell rail lt" data-dcol="${esc(k)}"></div>`;
  const boxed = arr.length > 0 && (h === "compact" || (h === "open" && k !== "open" && k !== "in_progress"));
  const cur = boxed && arr.some(i => i.id === S.cursor) ? " cur" : "";
  return boxed
    ? `<div class="wcell lt wsc" data-dcol="${esc(k)}"><div class="wbox${cur}" data-box="${esc(lane + " " + k)}">${cellCards(arr)}</div></div>`
    : `<div class="wcell lt" data-dcol="${esc(k)}">${cellCards(arr)}</div>`;
}

/** cellEdges watches the cell boxes of the board drawn last, so each marks its edges once laid out, and again when its size changes. */
let cellEdges: ResizeObserver | null = null;

/**
 * markCellEdges says which edges of a cell's box have cards past them, and
 * app.css fades those edges: only the box that holds the cursor's card has a
 * scroll bar, and macOS hides even that one until a scroll starts, so
 * nothing else shows that a cell holds more.
 */
export function markCellEdges(box: HTMLElement): void {
  const y = box.scrollTop;
  box.classList.toggle("up", y >= 1);
  box.classList.toggle("down", y < box.scrollHeight - box.clientHeight - 1);
}

/** revealBoardCursor scrolls the cursor's card or epic cell into view, inside its cell's box as well. */
export function revealBoardCursor(): void {
  if (!S.cursor) return;
  const id = CSS.escape(S.cursor);
  document.querySelector(`#vBoard [data-card="${id}"], #vBoard [data-epic="${id}"]`)?.scrollIntoView({ block: "nearest", inline: "nearest" });
}

/**
 * renderWideBoard lays every column out side by side in one grid, so an
 * epic lane is a row across all of them and the columns scroll together.
 */
function renderWideBoard(v: HTMLElement): void {
  const cols = boardCols().filter(([k]) => !S.board.hideEmpty || boardItems(k).length > 0);
  const per = new Map(cols.map(([k]) => [k, boardItems(k)]));
  const lanes = S.board.lanes !== "off" && S.board.group !== "type";
  const folded = (k: string) => S.board.folded.has(k);
  const tmpl = (S.board.lanes === "rail" && lanes ? "200px " : "") + cols.map(([k]) => (folded(k) ? "34px" : "minmax(240px, 1fr)")).join(" ");
  const head = cols.map(([k, l]) => {
    const n = per.get(k)!.length;
    return folded(k)
      ? `<button class="whead rail" data-tab="${esc(k)}" data-dcol="${esc(k)}" aria-label="Unfold ${esc(l)}" title="Unfold ${esc(l)}"><span>${esc(l)}</span><b>${n}</b></button>`
      : `<button class="whead" data-tab="${esc(k)}" data-dcol="${esc(k)}" style="--hc:var(--line)" aria-label="${esc(l)}, ${n} cards. Fold" title="Fold ${esc(l)} (z)"><span>${esc(l)}</span><b>${n}</b></button>`;
  }).join("");
  const cell = (k: string, arr: Item[]) => folded(k)
    ? `<div class="wcell rail" data-dcol="${esc(k)}"></div>`
    : `<div class="wcell" data-dcol="${esc(k)}">${cellCards(arr)}</div>`;
  const rail = S.board.lanes === "rail" && lanes;
  let rows = "";
  if (rail) {
    for (const [l, arr] of laneKeys(cols.filter(([k]) => !folded(k)).flatMap(([k]) => per.get(k)!))) {
      const h = laneHeight(l);
      rows += epicCellHTML(l, arr.length) + cols.map(([k]) => railCell(k, l, h, per.get(k)!.filter(i => (laneOf(i) || "none") === l))).join("");
    }
  } else if (S.board.lanes === "rows" && lanes) {
    for (const [l, arr] of laneKeys(cols.filter(([k]) => !folded(k)).flatMap(([k]) => per.get(k)!))) {
      rows += laneHTML(l, arr.length, true);
      if (!S.board.laneFold.has(l)) rows += cols.map(([k]) => cell(k, per.get(k)!.filter(i => (laneOf(i) || "none") === l))).join("");
    }
  } else rows = cols.map(([k]) => cell(k, per.get(k)!)).join("");
  const total = cols.reduce((n, [k]) => n + (folded(k) ? 0 : per.get(k)!.length), 0);
  if (!total) rows += `<div class="empty wempty">No cards match <b>${esc(queryString() || "the filter")}</b>.</div>`;
  const tools = optsTab()
    + (S.board.folded.size ? `<button class="tab unf" data-act="unfoldall">Z unfold</button>` : "")
    + `<span class="whint">drag cards between columns · h j k l move · z folds · Tab ${rail ? "steps a lane's height" : "folds a lane"} · v ${S.board.lanes === "rail" ? "epic rows" : "epic rail"}</span>`;
  const old = document.getElementById("wboard");
  const top = old ? old.scrollTop : 0, left = old ? old.scrollLeft : 0;
  // A cell keeps its place through a redraw, as the board does: a cursor move, a live update and a drop all redraw it.
  // A box that clips its cards still takes a scrollTop, so a cell the cursor has left shows the cards it was scrolled to.
  const cellTops = new Map<string, number>();
  old?.querySelectorAll<HTMLElement>(".wbox").forEach(b => { if (b.scrollTop) cellTops.set(b.dataset.box!, b.scrollTop); });
  v.innerHTML = `<div class="tabs" id="tabs">${tools}</div><div class="bbody" id="bbody"><div class="wboard" id="wboard" style="grid-template-columns:${tmpl}">${rail ? `<div class="whead epich"><span>EPIC</span><b>${new Set(cols.flatMap(([k]) => per.get(k)!).map(i => laneOf(i) || "none")).size}</b></div>` : ""}${head}${rows}</div></div>`;
  const nb = document.getElementById("wboard")!;
  nb.scrollTop = top; nb.scrollLeft = left;
  const boxes = nb.querySelectorAll<HTMLElement>(".wbox");
  boxes.forEach(b => { const t = cellTops.get(b.dataset.box!); if (t) b.scrollTop = t; });
  if (boxes.length) {
    cellEdges ??= new ResizeObserver(entries => entries.forEach(e => markCellEdges(e.target as HTMLElement)));
    boxes.forEach(b => cellEdges!.observe(b));
  }
}

/**
 * renderPhoneBoard stacks every column as a box, one below the other under
 * the chip row, as mockup A's phone board does (docs/adr/0052): a header
 * with the column's dot, name and card count, and the cards inside, grouped
 * under their epic lane headers. The header folds the box to itself and
 * S.board.folded keeps that across a reload. Each box is the drop target of a
 * dragged card (data-dcol), as a wide column's cells are.
 */
function renderPhoneBoard(v: HTMLElement): void {
  const cols = phoneBoardCols().filter(([k]) => !S.board.hideEmpty || boardItems(k).length > 0);
  if (cols.some(([k]) => k === "closed") && !store.get("phoneClosedFolded", false)) {
    S.board.folded.add("closed");
    store.set("phoneClosedFolded", true);
    store.set("boardFolded", [...S.board.folded]);
  }
  const boxes = cols.map(([k, l]) => {
    const items = phoneBoardItems(k), f = S.board.folded.has(k);
    const shown = k === "closed" ? items.slice(0, S.board.closedLimit) : items;
    const remaining = k === "closed" ? items.length - shown.length : 0;
    const head = `<button class="bhead" data-tab="${esc(k)}" aria-expanded="${!f}"><span class="st" style="--sc:${colColor(k)}" aria-hidden="true"></span><span class="t">${esc(l)}</span><b>${items.length}</b><span class="ch" aria-hidden="true">${f ? "▸" : "▾"}</span></button>`;
    if (f) return `<div class="bbox folded" data-dcol="${esc(k)}">${head}</div>`;
    let inner = "";
    const lanes = lanesShown() ? laneKeys(shown) : [];
    if (!items.length) inner = `<div class="empty">No cards in ${esc(l)}.</div>`;
    else if (lanes.length) inner = lanes.map(([ln, arr]) => `<div class="lgrp" data-lgrp="${esc(ln)}">${laneHTML(ln, arr.length)}${S.board.laneFold.has(ln) ? "" : cellCards(arr)}</div>`).join("");
    else inner = cellCards(shown);
    if (remaining > 0) inner += `<button class="bmore" data-act="closedmore">show ${Math.min(CLOSED_BATCH, remaining)} more</button>`;
    return `<div class="bbox" data-dcol="${esc(k)}">${head}<div class="bcards">${inner}</div></div>`;
  }).join("");
  const old = document.getElementById("bcol");
  const top = old ? old.scrollTop : 0;
  v.innerHTML = `<div class="bbody" id="bbody"><div class="bcol" id="bcol">${boxes || `<div class="empty">No column matches <b>${esc(queryString() || "the filter")}</b>.</div>`}</div></div>`;
  document.getElementById("bcol")!.scrollTop = top;
}

export function renderBoard(): void {
  const v = $("#vBoard");
  // The redraw replaces every cell box, so the old ones need no more watching.
  cellEdges?.disconnect();
  if (split()) renderWideBoard(v); else renderPhoneBoard(v);
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
    $("#sList").innerHTML = (rec.length ? `<div class="sec">Recent queries</div>` + rec.map(r => `<button class="item" data-recent="${esc(r)}"><span class="g" aria-hidden="true">↺</span><span class="x">${esc(r)}</span></button>`).join("") : "")
      + `<div class="sec">Syntax</div><div class="md syntax"><p><code>status:open</code> <code>type:bug</code> <code>priority:1</code> <code>assignee:alice</code> <code>label:mobile</code> <code>project:b9s</code> <code>id:foit</code> <code>title:graph</code>. The same field twice means either value. <code>!</code> negates, and a bare <code>!closed</code> means not that status. Plain words match ID and title, fuzzily. It is the TUI's query language, evaluated by b9s.</p></div>`;
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

/** buildLabel is the running build in one line: "v1.3.3 · 0704a347". */
export function buildLabel(): string {
  const b = D.build;
  return [b.version || "unknown", b.commit.slice(0, 8)].filter(Boolean).join(" · ") + (b.modified ? " · modified" : "");
}

function renderMore(): void {
  const it = (a: string, g: string, x: string, sub: string, v?: string | null, on?: boolean) =>
    `<button class="item" data-act="${a}"><span class="g" aria-hidden="true">${g}</span><span class="x">${x}${sub ? `<small>${sub}</small>` : ""}</span>${v != null ? `<span class="v ${on ? "on" : ""}">${esc(v)}</span>` : ""}</button>`;
  const h = D.health;
  const chipState = S.chips ? "shown" : "hidden";
  $("#moreList").innerHTML =
    `<div class="sec">Project</div>`
    + (D.public ? "" : it("projects", "▤", "Projects", "slots 1-9 · all projects", D.project.all ? "all" : D.project.name))
    + it("health", "●", "Data source health", "source · watcher · fallbacks (D)", h ? (h.ok ? "ok" : "problem") : "…", !!h && h.ok)
    + it("identity", "@", "Acting as", "author of your issues and comments", D.actor || "unknown")
    + `<div class="sec">Views</div>`
    + it("graph", "⋔", "Dependency graph", "blocked-by and blocks from the cursor", null)
    + it("treeopts", "≣", "Tree options", "sort, columns, wrap, text size, expand and collapse all", null)
    + it("boardopts", "▥", "Board options", "group, epic lanes, folded columns", S.board.group)
    + it("follow", "F", "Follow mode", "jump to each issue a live change touches", S.follow ? "on" : "off", S.follow)
    + it("togglechips", "⌄", "Filter chip row", "also: swipe down on the header", chipState, chipRowVisible())
    + `<div class="sec">Connection</div>`
    + it("pairing", "⌁", "Pairing", "b9s web · tailscale serve · pairing link", null)
    + it("log", "$", "Write log", "every bd command this browser asked for", String(S.log.length))
    + it("reload", "↻", "Reload", "also: pull down on the tree", null)
    + it("help", "?", "Gestures", "every gesture and its TUI key", null)
    + `<div class="sec">About</div>`
    + it("about", "ⓘ", "b9s version", "the release and the commit it was built from", buildLabel());
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
  const id = S.graph[S.graph.length - 1], i = get(id)!;
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
  v.innerHTML = `<div class="crumbs">${S.graph.map((g, n) => `<button data-crumb="${n}">${esc(shortId(g))}</button>`).join("<span class='msep'>›</span>")}</div>
    <div class="scroll" id="gList" role="list"><button class="gnode" data-open="${esc(id)}" style="display:block;width:calc(100% - 24px);text-align:left"><div class="l1">${statusDotHTML(eff(i))}<span class="id">${esc(i.id)}</span></div><div class="t2">${esc(i.t)}</div>${l2HTML(i)}<div class="pager" style="margin-top:4px">tap for detail · tap a row below to go there</div></button>
    ${sec("Blocked by", bb)}${sec("Blocks", bl)}${sec("Parent", flat(up))}${sec("Children", flat(kids(id)))}${sec("Discovered from", flat(i.discovered_from))}
    ${!bb.length && !bl.length ? `<div class="empty">No blocking relations. Parent and children are still listed.</div>` : ""}</div>`;
}

/* ================= detail sheet ================= */

export const appH = () => $("#app").clientHeight;
export const appW = () => $("#app").clientWidth;
export const halfH = () => Math.round(appH() * 0.52);
export const fullH = () => appH();
/** detailHeightPx is what the sheet covers of a list; the side panel of a split layout covers none. */
export const detailHeightPx = () => (split() ? 0 : S.detail && S.detail.size === "full" ? fullH() : halfH());

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

/** detailKids orders an issue's children as its detail lists them, so 1-9 pick the child shown with that number. */
export function detailKids(id: string): string[] {
  return kids(id).map(k => get(k)!).sort(SORTS.status).map(k => k.id);
}

/** relHTML is one related issue in the detail; num is the digit key that opens a child. */
function relHTML(x: string, num = 0): string {
  const j = get(x);
  if (!j) return "";
  return `<button class="rel" data-nav="${esc(x)}">${num ? `<span class="kn">${num}</span>` : ""}${statusDotHTML(eff(j))}<span class="id">${esc(shortId(x))}</span><span class="t">${esc(j.t)}</span></button>`;
}

function textSec(label: string, text: string | undefined, loading: boolean): string {
  if (text) return `<div class="dsec">${label}</div><div class="md">${md(text)}</div>`;
  if (label === "Description") return `<div class="dsec">${label}</div><div class="md"><p style="color:var(--muted)">${loading ? "Loading…" : "No description."}</p></div>`;
  return "";
}

function memoryLinks(issue: Issue | null): string {
  if (issue?.memory_loading) return '<div class="dsec">Memory Links</div><p style="color:var(--muted)">Loading Memory Links…</p>';
  if (!issue?.memory_available) return "";
  const links = issue.memory_links || [];
  return `<div class="dsec">Memory Links · ${links.length}</div>${links.length ? links.map(({ link, memory, outgoing }) => `<details class="issueMemoryLink"><summary><span>${outgoing ? `This Issue → ${esc(link.kind)} → Memory` : `Memory → ${esc(link.kind)} → this Issue`}</span><br><b>${esc(memory.title)}</b></summary><p>${esc(memory.id)}</p><small>${esc(link.type)}</small>${link.note ? `<p>${esc(link.note)}</p>` : ""}<div class="md">${md(memory.body || "No body.")}</div></details>`).join("") : '<p>No Memory Links recorded.</p>'}`;
}

/**
 * A place in the detail's text: the block at the top of the scrollport and
 * that block's offset from it. The scroll offset in px alone does not survive
 * a change of size class. The browser lays the old pane out at the new size
 * before any render and holds the block at the top in place through that
 * reflow, so the px value the old pane ends with belongs to a pane that no
 * longer exists, while the block it kept at the top is still the right one.
 * The place is read from that layout, before the class moves: once it has,
 * the old pane lays out for the new class as well, and that reflow does not
 * keep the block.
 */
interface TextPlace { block: number; offset: number }

/** textBlocks are the parts of the text in document order: the fields, headings, relations, and each paragraph of the Markdown. */
const textBlocks = (body: Element) => [...body.querySelectorAll<HTMLElement>(":scope > *:not(.md), .md > *")];

function textPlace(body: HTMLElement): TextPlace {
  const top = body.getBoundingClientRect().top;
  const blocks = textBlocks(body);
  const block = blocks.findIndex(b => b.getBoundingClientRect().bottom > top);
  return block < 0 ? { block, offset: body.scrollTop } : { block, offset: blocks[block].getBoundingClientRect().top - top };
}

function scrollToPlace(body: HTMLElement, p: TextPlace): void {
  const b = textBlocks(body)[p.block];
  if (!b) { body.scrollTop = p.block < 0 ? p.offset : 0; return; }
  body.scrollTop += b.getBoundingClientRect().top - body.getBoundingClientRect().top - p.offset;
}

/** detailPlace is the issue the detail shows and the place in its text, or null while no detail is on screen. */
export function detailPlace(): { id: string; place: TextPlace } | null {
  const sec = $opt<HTMLElement>("#detailHost .detail"), body = sec?.querySelector<HTMLElement>(".dbody");
  return sec && body && sec.dataset.id ? { id: sec.dataset.id, place: textPlace(body) } : null;
}

export function renderDetail(was = detailPlace()): void {
  if (S.view === "memory") { renderMemoryDetail(); return; }
  const host = $("#detailHost");
  $("#app").classList.toggle("hasdetail", !!S.detail);
  if (!S.detail) { host.innerHTML = ""; cachedDetailPlace = null; return; }
  const id = S.detail.stack[S.detail.stack.length - 1], i = get(id);
  if (!i) { S.detail = null; host.innerHTML = ""; cachedDetailPlace = null; $("#app").classList.remove("hasdetail"); return; }
  const f = fullIssue(id);
  const st = stOf(eff(i));
  const sib = siblings(id), pos = sib.indexOf(id);
  const pr = progress(id);
  const ob = i.blocked_by.filter(x => get(x)), bl = blocksOf(i).map(x => x.id);
  const existing = host.querySelector<HTMLElement>(".detail");
  const kept = was && was.id === id ? was.place : null;
  const cms = [...(f?.comments || [])].sort((a, b) => {
    const delta = (Date.parse(a.created_at) || 0) - (Date.parse(b.created_at) || 0);
    return S.commentsOldestFirst ? delta : -delta;
  });
  const ro = D.project.read_only;
  const full = S.detail.size === "full";
  const isSplit = split();
  // On a phone the size is the sheet's height, in a split layout the panel's width; the glyph says which.
  const size = isSplit ? (full ? ["⤡", "Side panel"] : ["⤢", "Full width"]) : (full ? ["⌄", "Half height"] : ["⌃", "Full height"]);
  const back = S.detail.stack.length > 1
    ? `<button class="back" data-act="dback" aria-label="Back to previous issue">‹ ${esc(shortId(S.detail.stack[S.detail.stack.length - 2]))}</button>`
    : (!isSplit ? `<button class="back" data-act="dclose" aria-label="Back to issue list">‹ Tree</button>` : "");
  // A finger swipes to the next sibling; a mouse has buttons for it.
  const pager = desktop()
    ? `<button data-act="dprev" aria-label="Previous sibling" title="Previous sibling (p)"${pos > 0 ? "" : " disabled"}>‹</button><span class="pager">${pos + 1}/${sib.length}</span><button data-act="dnext" aria-label="Next sibling" title="Next sibling (n)"${pos < sib.length - 1 ? "" : " disabled"}>›</button>`
    : `<span class="pager">${pos + 1}/${sib.length} ‹ swipe ›</span>`;
  host.innerHTML = `<section class="detail ${existing ? "" : "enter"} ${full ? "full" : ""}" data-id="${esc(id)}" style="--h:${detailHeightPx()}px" aria-label="Issue ${esc(shortId(id))}" role="dialog">
    <div class="dhead" id="dhead"><div class="grab"></div>
      <div class="dtop">${back}<span class="id">${esc(i.id)}</span><span class="sp"></span>${pager}<button data-act="dsize" aria-label="${size[1]}" title="${size[1]} (\\)">${size[0]}</button><button data-act="dclose" aria-label="Close">✕</button></div>
      <div class="dtitle">${esc(i.t)}</div>
      <div class="pills"><span class="pill status" style="--sc:${st.c}"><span class="sdot" aria-hidden="true"></span>${esc(i.status === eff(i) ? st.w : i.status + " - " + st.w)}</span><span class="pill">P${i.priority}</span><span class="pill">${esc(i.type)}</span>${i.labels.map(l => `<span class="pill">#${esc(l)}</span>`).join("")}</div>
    </div>
    <div class="dbody" id="dbody">
      <dl class="meta"><dt>assignee</dt><dd>${i.assignee ? "@" + esc(i.assignee) : "—"}</dd><dt>creator</dt><dd>${esc(i.created_by || "—")}</dd><dt>created</dt><dd>${fmtDate(i.crt)}${i.crt ? " · " + age(i.crt) : ""}</dd><dt>updated</dt><dd>${fmtDate(i.upd)}${i.upd ? " · " + age(i.upd) : ""}</dd>${i.cls ? `<dt>closed</dt><dd>${fmtDate(i.cls)}</dd>` : ""}${i.defer_until ? `<dt>deferred</dt><dd>until ${esc(i.defer_until.slice(0, 10))}</dd>` : ""}${D.project.all ? `<dt>project</dt><dd>${esc(projectOf(i))}</dd>` : ""}</dl>
      ${i.parent && get(i.parent) ? `<div class="dsec">Parent</div>${relHTML(i.parent)}` : ""}
      ${pr ? `<div class="dsec"><span>Children ${pr.done}/${pr.all}</span><button data-act="dfocus">⌖ subtree</button></div><div class="bar2"><i style="width:${Math.round(100 * pr.done / pr.all)}%"></i></div>${detailKids(id).map((k, n) => relHTML(k, n < 9 ? n + 1 : 0)).join("")}` : ""}
      ${ob.length ? `<div class="dsec"><span>Blocked by ${ob.length}</span><button data-act="dgraph">⋔ graph</button></div>${ob.map(relHTML).join("")}` : ""}
      ${bl.length ? `<div class="dsec"><span>Blocks ${bl.length}</span><button data-act="dgraph">⋔ graph</button></div>${bl.map(relHTML).join("")}` : ""}
      ${i.discovered_from.some(x => get(x)) ? `<div class="dsec">Discovered from</div>${i.discovered_from.map(relHTML).join("")}` : ""}
      ${i.related.some(x => get(x)) ? `<div class="dsec">Related</div>${i.related.map(relHTML).join("")}` : ""}
      ${memoryLinks(f)}
      ${textSec("Description", f?.description, !f)}${textSec("Design", f?.design, !f)}${textSec("Acceptance", f?.acceptance, !f)}${textSec("Notes", f?.notes, !f)}
      <div class="dsec"><span>Comments ${f ? cms.length : i.comment_count}</span><button data-act="commentsort" title="Reverse comment order">${S.commentsOldestFirst ? "Oldest first" : "Newest first"}</button>${ro ? "" : `<button data-act="comment">+ add</button>`}</div>
      ${cms.map(c => `<div class="cm"><div class="who"><b>${esc(c.author || "unknown")}</b> · ${age(Date.parse(c.created_at) || 0)}</div><div class="md">${md(c.text)}</div></div>`).join("") || `<div class="md"><p style="color:var(--muted)">${f || !i.comment_count ? "No comments." : "Loading…"}</p></div>`}
    </div>
    <div class="dacts">${ro ? "" : `<button data-act="status"><span class="g" aria-hidden="true">◐</span>Status</button><button data-act="edit"><span class="g" aria-hidden="true">✎</span>Edit</button><button data-act="comment"><span class="g" aria-hidden="true">✉</span>Comment</button>`}<button data-act="dbranch"><span class="g" aria-hidden="true">⎇</span>Branch</button><button data-act="actions"><span class="g" aria-hidden="true">⋯</span>More</button></div>
  </section>`;
  const sec = host.querySelector<HTMLElement>(".detail")!;
  const body = sec.querySelector<HTMLElement>(".dbody")!;
  bindDetailResizeGuard();
  const rememberDetailPlace = () => {
    if (performance.now() < suppressDetailScrollCacheUntil) return;
    const place = detailPlace();
    if (place) cachedDetailPlace = place;
  };
  body.addEventListener("scroll", rememberDetailPlace, { passive: true });
  if (kept) {
    scrollToPlace(body, kept);
    // The second pass settles the place once the new text has laid out. A page key, the wheel or a
    // finger may have scrolled the body before the frame comes, under load a few frames late, and
    // that scroll must stand: the pass applies only while the body still sits where the first left it.
    const placed = body.scrollTop;
    requestAnimationFrame(() => { if (body.scrollTop === placed) scrollToPlace(body, kept); rememberDetailPlace(); });
  }
  rememberDetailPlace();
  renderMermaidBlocks(sec);
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
