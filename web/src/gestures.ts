// Pointer gestures. Every handler is bound once, on a host element that
// survives re-renders, and finds its target by delegation.

import { D, eff, get } from "./data";
import {
  act, applySuggestion, chipTap, closeDetail, closeSheet, foldSubtree, moveCard, nextStatus, openDetail,
  openSheet, reload, setStatus, sheetAction, swipeRightLabel, tabTap, toast, toggleMark, markRange, hideToast,
} from "./actions";
import { WIDE, appW, boardCols, colOf, ensureVisible, fullH, halfH, openCols, render, renderBoard, renderSearch, siblings, wide } from "./render";
import { S } from "./state";
import { $, haptic } from "./util";

export const SHORT = 0.2, LONG = 0.55, LP_MS = 450, EDGE = 24;

interface RowGesture {
  row: HTMLElement; id: string; x0: number; y0: number; pid: number; w: number;
  mode: null | "swipe" | "scroll" | "press" | "none" | "pull"; chev: boolean; edge: boolean; type: string;
  container: HTMLElement; lp: number;
}

let G: RowGesture | null = null;
let DRAG = false;

/** busy is true while a finger is on a row, a card or a sheet, when a re-render would pull it away. */
export const busy = () => !!G || DRAG || !!B || !!detailDrag || !!bodySwipe;

const localX = (clientX: number) => clientX - $("#app").getBoundingClientRect().left;
const inDeadZone = (clientX: number) => { const x = localX(clientX); return x < EDGE || x > appW() - EDGE; };

let lastChev = { id: "", t: 0 };

function bindRows(container: HTMLElement): void {
  container.addEventListener("pointerdown", e => {
    const row = (e.target as HTMLElement).closest<HTMLElement>(".row");
    if (!row || e.button > 0 || !row.dataset.id) return;
    const g: RowGesture = {
      row, id: row.dataset.id, x0: e.clientX, y0: e.clientY, mode: null, pid: e.pointerId,
      chev: !!(e.target as HTMLElement).closest("[data-chev]"), w: row.getBoundingClientRect().width,
      edge: inDeadZone(e.clientX), type: e.pointerType, container, lp: 0,
    };
    g.lp = window.setTimeout(() => {
      if (G === g && !g.mode) {
        g.mode = "press";
        haptic(12);
        if (S.marking && S.marks.size) markRange(g.id); else toggleMark(g.id);
      }
    }, LP_MS);
    G = g;
  });
  container.addEventListener("pointermove", e => {
    if (!G || e.pointerId !== G.pid || G.container !== container) return;
    const dx = e.clientX - G.x0, dy = e.clientY - G.y0;
    if (!G.mode) {
      if (Math.abs(dx) > 10 && Math.abs(dx) > Math.abs(dy) * 1.3) {
        clearTimeout(G.lp);
        if (G.edge || S.marking || S.view === "graph") { G.mode = "none"; return; }
        G.mode = "swipe";
        try { G.row.setPointerCapture(G.pid); } catch { /* capture is best effort */ }
        G.row.classList.remove("snap");
      } else if (Math.abs(dy) > 8) {
        clearTimeout(G.lp);
        const tl = $("#treeList");
        G.mode = G.type === "mouse" && dy > 0 && tl.scrollTop <= 0 && container.id === "treeRows" ? "pull" : "scroll";
      }
    }
    if (G.mode === "swipe") swipeMove(G, dx);
    if (G.mode === "pull") pullMove(dy);
  });
  const end = (e: PointerEvent) => {
    if (!G || e.pointerId !== G.pid || G.container !== container) return;
    clearTimeout(G.lp);
    const g = G;
    G = null;
    if (e.type === "pointercancel") { if (g.mode === "swipe") snapBack(g.row); if (g.mode === "pull") pullEnd(false); return; }
    if (g.mode === "swipe") { swipeEnd(g, e.clientX - g.x0); return; }
    if (g.mode === "pull") { pullEnd(true); return; }
    if (g.mode) return;
    if (g.chev && !S.marking) {
      const now = performance.now(), dbl = lastChev.id === g.id && now - lastChev.t < 320;
      lastChev = { id: g.id, t: now };
      if (dbl) foldSubtree(g.id);
      else { if (S.folded.has(g.id)) S.folded.delete(g.id); else S.folded.add(g.id); render(); }
      return;
    }
    if (S.marking) { toggleMark(g.id); return; }
    if (S.view === "graph") { S.graph.push(g.id); S.cursor = g.id; render(); return; }
    openDetail(g.id);
  };
  container.addEventListener("pointerup", end);
  container.addEventListener("pointercancel", end);
}

function swipeMove(g: RowGesture, dx: number): void {
  const i = get(g.id);
  if (!i) return;
  const lim = g.w * 0.85, x = Math.abs(dx) > lim ? Math.sign(dx) * (lim + (Math.abs(dx) - lim) * 0.25) : dx;
  g.row.querySelector<HTMLElement>(".rc")!.style.transform = `translateX(${x}px)`;
  const ul = g.row.querySelector<HTMLElement>(".ul")!, ur = g.row.querySelector<HTMLElement>(".ur")!;
  ul.style.visibility = dx > 0 ? "visible" : "hidden";
  ur.style.visibility = dx < 0 ? "visible" : "hidden";
  const f = Math.abs(dx) / g.w;
  if (dx > 0) { ul.className = "ul" + (f >= LONG ? " long" : f < SHORT ? " weak" : ""); ul.textContent = f >= LONG ? "◉ status…" : swipeRightLabel(i); }
  else {
    const closed = eff(i) === "closed";
    ur.className = "ur" + (f >= LONG ? " long" : f < SHORT || closed ? " weak" : "");
    ur.textContent = f >= LONG ? "actions ⋯" : closed ? "already closed" : "close ✓";
  }
}

function snapBack(row: HTMLElement): void {
  row.classList.add("snap");
  const rc = row.querySelector<HTMLElement>(".rc");
  if (rc) rc.style.transform = "";
}

function swipeEnd(g: RowGesture, dx: number): void {
  const f = Math.abs(dx) / g.w, i = get(g.id);
  snapBack(g.row);
  if (f < SHORT || !i) return;
  setTimeout(() => {
    if (dx > 0) { if (f >= LONG) openSheet("status", { id: g.id }); else void setStatus([g.id], nextStatus(i)); }
    else if (f >= LONG) openSheet("actions", { id: g.id });
    else if (eff(i) !== "closed") void setStatus([g.id], "closed");
  }, 160);
}

/* ================= pull to reload ================= */

let pullD = 0;
function pullMove(dy: number): void {
  const ptr = $("#ptr");
  pullD = dy;
  const h = Math.min(dy * 0.5, 64);
  ptr.style.transition = "none";
  ptr.style.height = h + "px";
  ptr.classList.toggle("ready", h >= 48);
  ptr.textContent = h >= 48 ? "↻ release to reload" : "↓ pull to reload";
}
function pullEnd(commit: boolean): void {
  const ptr = $("#ptr");
  ptr.style.transition = "";
  if (commit && pullD * 0.5 >= 48) void reload(); else ptr.style.height = "0";
  pullD = 0;
}

function bindPull(): void {
  const tl = $("#treeList");
  let P: { y0: number; d: number } | null = null;
  tl.addEventListener("touchstart", e => { if (tl.scrollTop <= 0 && !S.marking) P = { y0: e.touches[0].clientY, d: 0 }; }, { passive: true });
  tl.addEventListener("touchmove", e => {
    if (!P || (G && G.mode === "swipe")) return;
    const d = e.touches[0].clientY - P.y0;
    if (d < 0) { P = null; pullMove(0); return; }
    P.d = d;
    pullMove(d);
  }, { passive: true });
  tl.addEventListener("touchend", () => { if (!P) return; P = null; pullEnd(true); });
}

/* ================= header ================= */

function bindHeader(): void {
  const hd = $("#hd");
  let h: { y0: number } | null = null;
  hd.addEventListener("pointerdown", e => { h = { y0: e.clientY }; });
  hd.addEventListener("pointermove", e => {
    if (h && e.clientY - h.y0 > 26) {
      h = null;
      void act("togglechips").then(() => toast(S.chips ? "Chips shown" : "Chips hidden · swipe down again"));
    }
  });
  hd.addEventListener("pointerup", () => { h = null; });
  hd.addEventListener("pointercancel", () => { h = null; });
}

/* ================= detail sheet ================= */

let detailDrag: { y0: number; h0: number; moved: boolean; sec: HTMLElement } | null = null;
let bodySwipe: { x0: number; y0: number; pid: number; mode: null | "h" | "v"; edge: boolean; body: HTMLElement } | null = null;

function bindDetail(): void {
  const host = $("#detailHost");
  host.addEventListener("pointerdown", e => {
    const t = e.target as HTMLElement;
    const head = t.closest<HTMLElement>("#dhead");
    if (head && !t.closest("button") && !wide()) {
      const sec = head.closest<HTMLElement>(".detail")!;
      detailDrag = { y0: e.clientY, h0: sec.getBoundingClientRect().height, moved: false, sec };
      try { head.setPointerCapture(e.pointerId); } catch { /* best effort */ }
      sec.classList.add("drag");
      return;
    }
    const body = t.closest<HTMLElement>("#dbody");
    if (body && e.button === 0) bodySwipe = { x0: e.clientX, y0: e.clientY, pid: e.pointerId, mode: null, edge: inDeadZone(e.clientX), body };
  });
  host.addEventListener("pointermove", e => {
    if (detailDrag) {
      const dy = e.clientY - detailDrag.y0;
      if (Math.abs(dy) > 4) detailDrag.moved = true;
      detailDrag.sec.style.height = Math.max(80, detailDrag.h0 - dy) + "px";
      return;
    }
    const s = bodySwipe;
    if (!s || e.pointerId !== s.pid) return;
    const dx = e.clientX - s.x0, dy = e.clientY - s.y0;
    if (!s.mode) {
      if (Math.abs(dx) > 12 && Math.abs(dx) > Math.abs(dy) * 1.3 && !s.edge) {
        s.mode = "h";
        try { s.body.setPointerCapture(s.pid); } catch { /* best effort */ }
        s.body.classList.remove("snap");
      } else if (Math.abs(dy) > 10) s.mode = "v";
    }
    if (s.mode === "h") { s.body.style.transform = `translateX(${dx}px)`; s.body.style.opacity = String(1 - Math.min(Math.abs(dx) / 400, 0.5)); }
  });
  const up = (e: PointerEvent) => {
    if (detailDrag) {
      const d = detailDrag;
      detailDrag = null;
      d.sec.classList.remove("drag");
      if (!S.detail) return;
      const h = d.h0 - (e.clientY - d.y0);
      if (!d.moved) { S.detail.size = S.detail.size === "full" ? "half" : "full"; d.sec.style.height = ""; render(); return; }
      if (h < halfH() * 0.62) { closeDetail(); return; }
      S.detail.size = h > (halfH() + fullH()) / 2 ? "full" : "half";
      d.sec.style.height = "";
      render();
      return;
    }
    const g = bodySwipe;
    bodySwipe = null;
    if (!g || g.mode !== "h" || !S.detail) return;
    const dx = e.clientX - g.x0, body = g.body;
    body.classList.add("snap");
    const id = S.detail.stack[S.detail.stack.length - 1], sib = siblings(id), pos = sib.indexOf(id);
    const next = dx < 0 ? sib[pos + 1] : sib[pos - 1];
    if (Math.abs(dx) < appW() * 0.22 || !next) {
      body.style.transform = ""; body.style.opacity = "";
      if (!next && Math.abs(dx) > 40) toast(dx < 0 ? "Last sibling" : "First sibling");
      return;
    }
    body.style.transform = `translateX(${dx < 0 ? -appW() : appW()}px)`;
    body.style.opacity = "0";
    setTimeout(() => {
      if (!S.detail) return;
      S.detail.stack[S.detail.stack.length - 1] = next;
      S.cursor = next;
      render();
      ensureVisible(next);
      const nb = document.getElementById("dbody");
      if (!nb) return;
      nb.style.transform = `translateX(${dx < 0 ? appW() : -appW()}px)`;
      nb.style.opacity = "0";
      requestAnimationFrame(() => { nb.classList.add("snap"); nb.style.transform = ""; nb.style.opacity = ""; });
    }, 170);
  };
  host.addEventListener("pointerup", up);
  host.addEventListener("pointercancel", up);
}

/* ================= modal sheet ================= */

function bindSheet(): void {
  const host = $("#sheetHost");
  let d: { y0: number; sh: HTMLElement } | null = null;
  host.addEventListener("pointerdown", e => {
    const mh = (e.target as HTMLElement).closest<HTMLElement>("#mh");
    if (!mh) return;
    const sh = mh.closest<HTMLElement>(".msheet")!;
    d = { y0: e.clientY, sh };
    try { mh.setPointerCapture(e.pointerId); } catch { /* best effort */ }
    sh.style.transition = "none";
  });
  host.addEventListener("pointermove", e => { if (d) d.sh.style.transform = `translateY(${Math.max(0, e.clientY - d.y0)}px)`; });
  const up = (e: PointerEvent) => {
    if (!d) return;
    const dy = e.clientY - d.y0, sh = d.sh;
    d = null;
    sh.style.transition = ""; sh.style.transform = "";
    if (dy > 70) closeSheet();
  };
  host.addEventListener("pointerup", up);
  host.addEventListener("pointercancel", up);
}

/* ================= board ================= */

interface BoardGesture {
  tab?: string; col?: HTMLElement; card?: HTMLElement | null; id?: string; lane?: HTMLElement | null;
  x0: number; y0: number; pid: number; mode: null | "swipe" | "scroll" | "drag"; edge?: boolean; lp?: number;
  /** wide is the side-by-side board, where a mouse drags at once and nothing swipes */
  wide?: boolean; mouse?: boolean;
  ghost?: HTMLElement; ox?: number; oy?: number; target?: string | null; edgeDir?: number; edgeT?: number;
}
let B: BoardGesture | null = null;

function bindBoard(): void {
  const v = $("#vBoard"), app = $("#app");
  document.addEventListener("touchmove", e => { if (DRAG) e.preventDefault(); }, { passive: false });
  v.addEventListener("pointerdown", e => {
    if (e.button > 0) return;
    const t = e.target as HTMLElement;
    const tab = t.closest<HTMLElement>("[data-tab]");
    if (tab) { B = { tab: tab.dataset.tab, pid: e.pointerId, x0: e.clientX, y0: e.clientY, mode: null }; return; }
    const col = t.closest<HTMLElement>("#bcol, #wboard");
    if (!col) return;
    const card = t.closest<HTMLElement>("[data-card]");
    const b: BoardGesture = {
      col, card, id: card?.dataset.card, x0: e.clientX, y0: e.clientY, pid: e.pointerId, mode: null, edge: inDeadZone(e.clientX),
      lane: t.closest<HTMLElement>("[data-lane]"), wide: col.id === "wboard", mouse: e.pointerType === "mouse",
    };
    if (card && !D.project.read_only && !(b.wide && b.mouse)) b.lp = window.setTimeout(() => { if (B === b && !b.mode) startDrag(b, app); }, LP_MS);
    B = b;
  });
  v.addEventListener("pointermove", e => {
    if (!B || e.pointerId !== B.pid) return;
    if (B.mode === "drag") { dragMove(B, e, app); return; }
    if (B.tab) return;
    const dx = e.clientX - B.x0, dy = e.clientY - B.y0;
    if (B.wide) {
      if (B.mode) return;
      if (B.mouse && B.card && !D.project.read_only && Math.hypot(dx, dy) > 5) { startDrag(B, app); dragMove(B, e, app); return; }
      if (Math.abs(dx) > 8 || Math.abs(dy) > 8) { clearTimeout(B.lp); B.mode = "scroll"; }
      return;
    }
    if (!B.mode) {
      if (Math.abs(dx) > 12 && Math.abs(dx) > Math.abs(dy) * 1.3 && !B.edge) {
        clearTimeout(B.lp); B.mode = "swipe";
        try { B.col!.setPointerCapture(B.pid); } catch { /* best effort */ }
        B.col!.classList.remove("snap");
      } else if (Math.abs(dy) > 8 || Math.abs(dx) > 8) { clearTimeout(B.lp); B.mode = "scroll"; }
    }
    if (B.mode === "swipe") B.col!.style.transform = `translateX(${dx}px)`;
  });
  const end = (e: PointerEvent) => {
    if (!B || e.pointerId !== B.pid) return;
    clearTimeout(B.lp);
    const b = B;
    B = null;
    if (b.mode === "drag") { finishDrag(b, e.type === "pointercancel"); return; }
    if (e.type === "pointercancel") { if (b.col) { b.col.classList.add("snap"); b.col.style.transform = ""; } return; }
    if (b.tab) {
      if (Math.abs(e.clientX - b.x0) >= 10) return;
      // A wide header folds its own column; a phone tab first selects it.
      if (wide() && !S.board.folded.has(b.tab)) S.board.col = b.tab;
      tabTap(b.tab);
      return;
    }
    if (b.mode === "swipe") { swipeColumn(b, e.clientX - b.x0); return; }
    if (b.mode) return;
    if (b.lane) {
      const l = b.lane.dataset.lane!;
      if (S.board.laneFold.has(l)) S.board.laneFold.delete(l); else S.board.laneFold.add(l);
      S.board.keep = true;
      renderBoard();
      return;
    }
    if (b.card && b.id) openDetail(b.id);
  };
  v.addEventListener("pointerup", end);
  v.addEventListener("pointercancel", end);
  v.addEventListener("click", e => {
    const rl = (e.target as HTMLElement).closest<HTMLElement>("[data-rlane]");
    if (rl) { S.board.lane = rl.dataset.rlane || null; renderBoard(); }
  });
}

function swipeColumn(b: BoardGesture, dx: number): void {
  const cols = boardCols().filter(([k]) => !S.board.folded.has(k));
  const idx = cols.findIndex(([k]) => k === S.board.col);
  const next = cols[idx + (dx < 0 ? 1 : -1)];
  const col = b.col!;
  col.classList.add("snap");
  if (Math.abs(dx) < appW() * 0.22 || !next) {
    col.style.transform = "";
    if (!next && Math.abs(dx) > 40) toast(dx < 0 ? "Last column" : "First column");
    return;
  }
  col.style.transform = `translateX(${dx < 0 ? -appW() : appW()}px)`;
  setTimeout(() => {
    S.board.col = next[0];
    renderBoard();
    const nc = document.getElementById("bcol");
    if (!nc) return;
    nc.style.transform = `translateX(${dx < 0 ? appW() : -appW()}px)`;
    requestAnimationFrame(() => { nc.classList.add("snap"); nc.style.transform = ""; });
  }, 160);
}

function startDrag(b: BoardGesture, app: HTMLElement): void {
  b.mode = "drag";
  DRAG = true;
  haptic(15);
  const card = b.card!, r = card.getBoundingClientRect(), ar = app.getBoundingClientRect();
  const ghost = card.cloneNode(true) as HTMLElement;
  ghost.classList.add("ghost");
  ghost.style.width = r.width + "px";
  b.ox = b.x0 - r.left; b.oy = b.y0 - r.top;
  ghost.style.left = r.left - ar.left + "px";
  ghost.style.top = r.top - ar.top + "px";
  app.appendChild(ghost);
  b.ghost = ghost;
  card.classList.add("lifted");
  try { b.col!.setPointerCapture(b.pid); } catch { /* best effort */ }
  document.getElementById("bbody")?.classList.add("dragging");
  toast(b.wide ? "Drop on a column" : "Drop on a column tab, or hold at an edge");
}

function dragMove(b: BoardGesture, e: PointerEvent, app: HTMLElement): void {
  const ar = app.getBoundingClientRect();
  const x = e.clientX - ar.left, y = e.clientY - ar.top;
  b.ghost!.style.left = x - b.ox! + "px";
  b.ghost!.style.top = y - b.oy! + "px";
  if (b.wide) { dragMoveWide(b, e); return; }
  document.querySelectorAll(".tab.drop").forEach(t => t.classList.remove("drop"));
  const el = document.elementFromPoint(e.clientX, e.clientY) as HTMLElement | null;
  const tab = el && el.closest<HTMLElement>("[data-tab]");
  let target: string | null = tab ? tab.dataset.tab! : null;
  const cols = boardCols();
  const idx = cols.findIndex(([k]) => k === S.board.col);
  const edge = x < 36 ? -1 : x > appW() - 36 ? 1 : 0;
  if (!tab && edge) {
    if (b.edgeDir !== edge) { b.edgeDir = edge; b.edgeT = performance.now(); }
    if (performance.now() - b.edgeT! > LP_MS && cols[idx + edge]) target = cols[idx + edge][0];
  } else b.edgeDir = 0;
  b.target = target;
  if (target != null) document.querySelector(`[data-tab="${CSS.escape(target)}"]`)?.classList.add("drop");
}

/** dragMoveWide targets whichever column is under the pointer, and scrolls the board near its edges. */
function dragMoveWide(b: BoardGesture, e: PointerEvent): void {
  document.querySelectorAll("#wboard .drop").forEach(x => x.classList.remove("drop"));
  const el = document.elementFromPoint(e.clientX, e.clientY) as HTMLElement | null;
  const c = el && el.closest<HTMLElement>("#wboard [data-dcol]");
  b.target = c ? c.dataset.dcol! : null;
  if (b.target != null) document.querySelectorAll(`#wboard [data-dcol="${CSS.escape(b.target)}"]`).forEach(x => x.classList.add("drop"));
  const wb = b.col!, r = wb.getBoundingClientRect(), M = 40, STEP = 16;
  if (e.clientX > r.right - M) wb.scrollLeft += STEP; else if (e.clientX < r.left + M) wb.scrollLeft -= STEP;
  if (e.clientY > r.bottom - M) wb.scrollTop += STEP; else if (e.clientY < r.top + M) wb.scrollTop -= STEP;
}

function finishDrag(b: BoardGesture, cancel: boolean): void {
  DRAG = false;
  b.ghost?.remove();
  b.card?.classList.remove("lifted");
  document.getElementById("bbody")?.classList.remove("dragging");
  document.querySelectorAll(".tab.drop, #wboard .drop").forEach(t => t.classList.remove("drop"));
  const target = b.target;
  const i = b.id ? get(b.id) : undefined;
  if (cancel || target == null || !i || target === colOf(i) || !b.id) { hideToast(); return; }
  void moveCard(b.id, target);
}

/* ================= taps and keys ================= */

function bindClicks(): void {
  const app = $("#app");
  app.addEventListener("click", e => {
    const t = e.target as HTMLElement;
    const sa = t.closest<HTMLElement>("[data-sa]");
    if (sa) { e.preventDefault(); void sheetAction(sa.dataset.sa!, sa.dataset.val); return; }
    const sg = t.closest<HTMLElement>("[data-seg] button");
    if (sg) { sg.parentElement!.querySelectorAll("button").forEach(b => b.classList.toggle("on", b === sg)); return; }
    const nav = t.closest<HTMLElement>("[data-nav]");
    if (nav) { openDetail(nav.dataset.nav!, true); return; }
    // Rows, cards and column tabs act on pointerup; toolbar buttons in #tabs are plain actions.
    if (t.closest(".row, .card, .bcol, .wboard") || (t.closest("#tabs") && !t.closest("[data-act]"))) return;
    const a = t.closest<HTMLElement>("[data-act]");
    if (a) { void act(a.dataset.act!); return; }
    const v = t.closest<HTMLElement>("[data-view]");
    if (v) {
      S.view = v.dataset.view as typeof S.view;
      if (S.view === "search") $<HTMLInputElement>("#sIn").value = S.text;
      render();
      return;
    }
    const c = t.closest<HTMLElement>("[data-chip]");
    if (c) { chipTap(c.dataset.chip!); return; }
    const hdr = t.closest<HTMLElement>("[data-hdr]");
    if (hdr) { const k = hdr.dataset.hdr!; if (S.folded.has(k)) S.folded.delete(k); else S.folded.add(k); render(); return; }
    const cr = t.closest<HTMLElement>("[data-crumb]");
    if (cr) { S.graph = S.graph.slice(0, +cr.dataset.crumb! + 1); render(); return; }
    const op = t.closest<HTMLElement>("[data-open]");
    if (op) { openDetail(op.dataset.open!); return; }
    const rc = t.closest<HTMLElement>("[data-recent]");
    if (rc) { $<HTMLInputElement>("#sIn").value = rc.dataset.recent!; renderSearch(); return; }
    const sgg = t.closest<HTMLElement>("[data-sugg]");
    if (sgg) applySuggestion(sgg.dataset.sugg!);
  });
  app.addEventListener("contextmenu", e => { if (!(e.target as HTMLElement).closest("input, textarea, .md")) e.preventDefault(); });
  const sIn = $<HTMLInputElement>("#sIn");
  sIn.addEventListener("input", renderSearch);
  sIn.addEventListener("keydown", e => { if (e.key === "Enter") { e.preventDefault(); void act("applysearch"); } });
  document.addEventListener("keydown", e => {
    const t = e.target as HTMLElement;
    if (e.key === "Escape") {
      if (S.sheet) { closeSheet(); e.preventDefault(); return; }
      if (S.detail) { closeDetail(); e.preventDefault(); }
      return;
    }
    if (t.closest("input, textarea, select")) return;
    if (S.view === "board" && !S.sheet && wide() && !e.metaKey && !e.ctrlKey && !e.altKey && boardKey(e.key)) { e.preventDefault(); return; }
    if (e.key === "Enter" && !e.metaKey && !e.ctrlKey) {
      const row = t.closest<HTMLElement>("[data-id], [data-card]");
      const id = row && (row.dataset.id || row.dataset.card);
      if (id && get(id)) { e.preventDefault(); openDetail(id); }
    }
  });
}

const MOVES: Record<string, [number, number]> = {
  h: [-1, 0], ArrowLeft: [-1, 0], l: [1, 0], ArrowRight: [1, 0], j: [0, 1], ArrowDown: [0, 1], k: [0, -1], ArrowUp: [0, -1],
};

/**
 * boardKey gives the wide board the TUI's keys. h and l land on the card
 * nearest in height in the next column that has cards, so a move keeps to
 * the same epic lane where it can. With the detail open, it follows the cursor.
 */
function boardKey(key: string): boolean {
  if (key === "Z") { void act("unfoldall"); return true; }
  const cur = S.cursor ? document.querySelector<HTMLElement>(`#wboard [data-card="${CSS.escape(S.cursor)}"]`) : null;
  if (key === "z") {
    const i = cur && get(S.cursor!);
    if (!i) return false;
    S.board.col = colOf(i);
    tabTap(S.board.col);
    return true;
  }
  if (key === "Enter") { if (!cur) return false; openDetail(S.cursor!); return true; }
  const d = MOVES[key];
  if (!d) return false;
  const grid = openCols()
    .map(([c]) => [...document.querySelectorAll<HTMLElement>(`#wboard .wcell[data-dcol="${CSS.escape(c)}"] [data-card]`)])
    .filter(a => a.length);
  if (!grid.length) return true;
  let next: HTMLElement;
  const ci = cur ? grid.findIndex(a => a.includes(cur)) : -1;
  if (!cur || ci < 0) next = grid[0][0];
  else if (d[0]) {
    const to = grid[Math.max(0, Math.min(grid.length - 1, ci + d[0]))], y = cur.getBoundingClientRect().top;
    next = to.reduce((a, c) => (Math.abs(c.getBoundingClientRect().top - y) < Math.abs(a.getBoundingClientRect().top - y) ? c : a));
  } else {
    const a = grid[ci];
    next = a[Math.max(0, Math.min(a.length - 1, a.indexOf(cur) + d[1]))];
  }
  const id = next.dataset.card!;
  S.cursor = id;
  if (S.detail) { S.detail.stack = [id]; render(); } else renderBoard();
  document.querySelector<HTMLElement>(`#wboard [data-card="${CSS.escape(id)}"]`)?.scrollIntoView({ block: "nearest", inline: "nearest" });
  return true;
}

export function bindAll(): void {
  window.matchMedia(WIDE).addEventListener("change", () => render());
  bindRows($("#treeRows"));
  bindRows($("#sList"));
  bindRows($("#vGraph"));
  bindPull();
  bindHeader();
  bindDetail();
  bindSheet();
  bindBoard();
  bindClicks();
}
