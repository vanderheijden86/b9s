// The TUI's regular keys for a browser with a keyboard. Each key calls the
// same action a tap or a gesture does, so a key and its gesture can never
// disagree. README.md's key tables are the reference; keys with no meaning
// in a browser (Ctrl-C quits, < > resize panes) are left out.
//
//   keydown ──▶ sheet open? ── only Esc
//               │
//               ├─ \ sizes the detail when it shows
//               ├─ 1-9 open a numbered child when the detail shows
//               ├─ global keys (views, projects, reload, help)
//               ├─ board keys          when the board shows
//               ├─ detail keys (n p c) when the detail shows
//               └─ tree keys           otherwise

import {
  act, chipTap, closeDetail, closeSheet, copyIssueMarkdown, markRange, openDetail, openSheet, readOnly,
  setFocus, setStatus, switchProject, toast, toggleMark,
} from "./actions";
import * as api from "./api";
import { D, eff, get, kids, laneOf, pool } from "./data";
import { detailKids, ensureVisible, render, renderBoard, siblings, wide } from "./render";
import { S, saveChips, treeRows } from "./state";
import { $, copyText, store } from "./util";

/**
 * keyName spells a key the way the README does: "j", "C-a", "S-Tab", or
 * returns "" for a Cmd or Alt chord, which stays with the browser. A Mac
 * keyboard has no Delete key, so Cmd-Backspace stands in for it, as in the Finder.
 */
function keyName(e: KeyboardEvent): string {
  if (e.metaKey && e.key === "Backspace" && !e.altKey && !e.ctrlKey && !e.shiftKey) return "Delete";
  if (e.metaKey || e.altKey) return "";
  if (e.key === "Tab") return e.shiftKey ? "S-Tab" : "Tab";
  return (e.ctrlKey ? "C-" : "") + (e.ctrlKey && e.key.length === 1 ? e.key.toLowerCase() : e.key);
}

/**
 * onKey handles one keydown and reports whether it used it. Keys typed into
 * a form never reach it, and Cmd or Alt chords stay with the browser.
 */
export function onKey(e: KeyboardEvent): boolean {
  const k = keyName(e);
  if (!k) return false;
  // Enter and Space on a focused button press the button.
  if ((k === "Enter" || k === " ") && (e.target as HTMLElement).closest("button, a")) return false;
  if (S.sheet) {
    if (k === "Escape") { closeSheet(); return true; }
    return false;
  }
  if (k === "Escape") return escape();
  // The TUI's \ stacks the detail full width; here it sizes the sheet or the side panel.
  if (k === "\\" && S.detail) { void act("dsize"); return true; }
  // With the detail open the digits open its numbered children, as in the TUI's detail pane.
  if (S.detail && /^[1-9]$/.test(k)) { detailChild(+k); return true; }
  if (globalKey(k)) return true;
  if (S.view === "board") return boardKey(k);
  if (S.detail && detailKey(k)) return true;
  if (S.view === "tree") return treeKey(k);
  return false;
}

/** Tree keys that put the cursor on the first row themselves when it has none. */
const TREE_STEPS = new Set(["j", "k", "ArrowDown", "ArrowUp", "C-f", "C-b", "C-d", "C-u", "Home", "End", "n", "N"]);
/** Board keys that act on the card under the cursor. */
const BOARD_CURSOR = new Set(["z", "Enter", "Tab", "y", "f", "S", "K"]);

/**
 * adoptCursor puts the cursor on the first row or card when it is on none
 * and the key acts on it. The TUI's cursor sits on a row from the first
 * frame on; a fresh page has none, and in the tree a filter can hide its row.
 */
export function adoptCursor(e: KeyboardEvent): void {
  const k = keyName(e);
  if (S.sheet || !k) return;
  if (S.view === "tree" && !TREE_STEPS.has(k)) {
    const ids = rowIds();
    if (ids.length && !(S.cursor && ids.includes(S.cursor))) S.cursor = ids[0];
  } else if (S.view === "board" && BOARD_CURSOR.has(k)) {
    // A card in a folded lane is off the page but still the cursor, and "none" is the No epic cell.
    if (S.cursor && (S.cursor === "none" || get(S.cursor))) return;
    const first = document.querySelector<HTMLElement>("#vBoard [data-card]");
    if (first) { S.cursor = first.dataset.card!; renderBoard(); }
  }
}

/** Esc closes the innermost thing: the detail, the marks, then the view. */
function escape(): boolean {
  if (S.detail) { closeDetail(); return true; }
  if (S.marking) { void act("unmark"); return true; }
  if (S.view !== "tree") { S.view = "tree"; render(); return true; }
  return false;
}

/* ================= everywhere ================= */

type Slots = "projects" | "labels" | "assignees";
let slots: Slots = "projects";

function globalKey(k: string): boolean {
  switch (k) {
    case "b": S.view = "board"; S.detail = null; render(); return true;
    case "g": {
      const c = S.cursor && get(S.cursor);
      if (c) { S.graph = [c.id]; S.detail = null; S.view = "graph"; render(); } else void act("graph");
      return true;
    }
    case "?": openSheet("keys"); return true;
    case "D": void act("health"); return true;
    case "C-e": case "H": void act("togglechips"); return true;
    case "C-r": case "F5": void act("reload"); return true;
    case "/": void act("search"); return true;
    case "F": void act("follow"); return true;
    case "C-n": void act("create"); return true;
    case "L": slots = "labels"; toast("1-9 toggle " + slotHint()); return true;
    case "A": slots = "assignees"; toast("1-9 toggle " + slotHint()); return true;
    case "P": slots = "projects"; toast("1-9 open a project, 0 shows all"); return true;
    case "0": void switchProject("*"); return true;
  }
  if (/^[1-9]$/.test(k)) { void slot(+k); return true; }
  return false;
}

/** slotValues are what 1-9 stand for when they put labels or assignees in the query. */
function slotValues(): string[] {
  const n = new Map<string, number>();
  for (const i of pool()) {
    const vals = slots === "labels" ? i.labels : i.assignee ? [i.assignee] : [];
    vals.forEach(v => n.set(v, (n.get(v) || 0) + 1));
  }
  return [...n].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).slice(0, 9).map(([v]) => v);
}

const slotHint = () => slotValues().map((v, n) => `${n + 1} ${v}`).join(" · ") || "nothing: no " + slots;

async function slot(n: number): Promise<void> {
  if (slots === "projects") {
    if (D.public) { toast("This demo shows one project"); return; }
    try {
      const p = (await api.projects()).projects.find(x => x.slot === n);
      if (p) await switchProject(p.key); else toast("No project on " + n);
    } catch (e) { toast("Could not list projects: " + (e instanceof Error ? e.message : String(e))); }
    return;
  }
  const v = slotValues()[n - 1];
  if (!v) return;
  const term = (slots === "labels" ? "label:" : "assignee:") + v;
  const words = S.text.split(/\s+/).filter(Boolean);
  const next = words.includes(term) ? words.filter(w => w !== term) : [...words, term];
  S.text = next.join(" ");
  store.set("text", S.text);
  render();
}

/* ================= tree ================= */

const rowIds = () => treeRows().rows.filter(r => r.i).map(r => r.i!.id);

function moveTo(id: string | undefined): void {
  if (!id) return;
  S.cursor = id;
  if (S.detail) S.detail.stack = [id];
  render();
  ensureVisible(id);
}

function step(ids: string[], by: number): void {
  const at = S.cursor ? ids.indexOf(S.cursor) : -1;
  if (at < 0) { moveTo(ids[0]); return; }
  moveTo(ids[Math.max(0, Math.min(ids.length - 1, at + by))]);
}

/** page is how many rows fit in the tree's scroll area. */
function page(): number {
  const r = $("#treeRows").querySelector<HTMLElement>(".row");
  return Math.max(1, Math.floor($("#treeList").clientHeight / (r ? r.offsetHeight : 40)) - 1);
}

function setFolded(all: boolean): void {
  S.folded.clear();
  if (all) pool().forEach(i => { if (kids(i.id).length) S.folded.add(i.id); });
  render();
}

const anyFolded = () => pool().some(i => S.folded.has(i.id));

function statusFilter(st: string[], ready: boolean): void {
  S.st = new Set(st);
  S.ready = ready;
  saveChips();
  render();
}

function toggleFocus(id: string, mode: "branch" | "subtree"): void {
  if (S.focus && S.focus.id === id && S.focus.mode === mode) { S.focus = null; render(); return; }
  setFocus(id, mode);
}

function treeKey(k: string): boolean {
  const ids = rowIds();
  const c = S.cursor ? get(S.cursor) : undefined;
  switch (k) {
    case "j": case "ArrowDown": step(ids, 1); return true;
    case "k": case "ArrowUp": step(ids, -1); return true;
    case "C-f": step(ids, page()); return true;
    case "C-b": step(ids, -page()); return true;
    case "C-d": step(ids, Math.ceil(page() / 2)); return true;
    case "C-u": step(ids, -Math.ceil(page() / 2)); return true;
    case "Home": moveTo(ids[0]); return true;
    case "End": moveTo(ids[ids.length - 1]); return true;
    case "o": statusFilter(["open", "in_progress", "blocked", "deferred"], false); return true;
    case "C": statusFilter(["closed"], false); return true;
    case "r": statusFilter([], true); return true;
    case "a": statusFilter([], false); return true;
    case "O": S.onlyMatches = !S.onlyMatches; render(); toast(S.onlyMatches ? "Only matches" : "Matches with their parents"); return true;
    case "X": setFolded(false); return true;
    case "Z": setFolded(true); return true;
    case "C-a": case "S-Tab": setFolded(!anyFolded()); return true;
    case "v": S.wrap = !S.wrap; store.set("wrap", S.wrap); render(); return true;
    case "s": void act("sort"); return true;
    case "|": void act("treeopts"); return true;
    case "C-\\": void act("unmark"); return true;
    case "n": case "N": {
      const m = treeRows().rows.filter(r => r.i && !r.dim).map(r => r.i!.id);
      if (!S.text.trim() || !m.length) { toast("No search matches"); return true; }
      const at = S.cursor ? m.indexOf(S.cursor) : -1;
      moveTo(m[(at + (k === "n" ? 1 : m.length - 1 + (at < 0 ? 1 : 0))) % m.length]);
      return true;
    }
  }
  if (!c) return false;
  const hasKids = treeRows().rows.some(r => r.i && r.i.parent === c.id) || (S.folded.has(c.id) && kids(c.id).length > 0);
  switch (k) {
    case "Enter": openDetail(c.id); return true;
    case "d": openDetail(c.id); return true;
    case "h":
      if (hasKids && !S.folded.has(c.id)) { S.folded.add(c.id); render(); } else if (c.parent && ids.includes(c.parent)) moveTo(c.parent);
      return true;
    case "l":
      if (S.folded.has(c.id)) { S.folded.delete(c.id); render(); } else if (hasKids) moveTo(ids[ids.indexOf(c.id) + 1]);
      return true;
    case "Tab":
      if (hasKids) { if (S.folded.has(c.id)) S.folded.delete(c.id); else S.folded.add(c.id); render(); }
      return true;
    case "p": if (c.parent && ids.includes(c.parent)) moveTo(c.parent); return true;
    case "{": case "}": {
      const sib = ids.filter(x => (get(x)!.parent || "") === (c.parent || ""));
      moveTo(k === "{" ? sib[0] : sib[sib.length - 1]);
      return true;
    }
    case "f": toggleFocus(c.id, "branch"); return true;
    case "x": toggleFocus(c.id, "subtree"); return true;
    case " ": toggleMark(c.id); return true;
    case "V": markRange(c.id); return true;
    case "u": if (S.marks.has(c.id)) toggleMark(c.id); return true;
    case "e": void act("edit"); return true;
    case "S": void act("status"); return true;
    case "K": if (!readOnly() && eff(c) !== "closed") void setStatus([c.id], "closed"); return true;
    case "Delete": if (!readOnly()) openSheet("confirmdelete", { ids: S.marks.size ? [...S.marks] : [c.id] }); return true;
    case "c": void copy(`${c.id} ${c.t}`, "Copied " + c.id); return true;
  }
  return false;
}

async function copy(text: string, done: string): Promise<void> {
  toast(await copyText(text) ? done : "Clipboard refused: " + text);
}

/* ================= detail ================= */

function detailKey(k: string): boolean {
  const id = S.detail!.stack[S.detail!.stack.length - 1];
  switch (k) {
    case "d": closeDetail(); return true;
    case "n": case "p": {
      const sib = siblings(id), at = sib.indexOf(id);
      const next = sib[at + (k === "n" ? 1 : -1)];
      if (next) moveTo(next);
      return true;
    }
    case "c": void copyIssueMarkdown(id); return true;
  }
  return false;
}

/** detailChild opens the n-th child of the issue the detail shows; Backspace comes back through history. */
function detailChild(n: number): void {
  const id = S.detail!.stack[S.detail!.stack.length - 1];
  const kid = detailKids(id)[n - 1];
  if (kid) openDetail(kid, true); else toast(`No child ${n}`);
}

/* ================= board ================= */

const GROUP_ORDER = ["status", "priority", "type"] as const;

function boardKey(k: string): boolean {
  const c = S.cursor ? get(S.cursor) : undefined;
  switch (k) {
    case "o": chipTap("open"); return true;
    case "i": chipTap("in_progress"); return true;
    case "C": chipTap("closed"); return true;
    case "r": chipTap("ready"); return true;
    case "c":
      if (S.board.group !== "status") { toast("c shows the closed column when the board groups by status"); return true; }
      if (S.st.has("closed")) S.st.delete("closed"); else S.st.add("closed");
      saveChips(); render();
      toast(S.st.has("closed") ? "Closed column shown" : "Closed column hidden");
      return true;
    case "s": {
      const g = GROUP_ORDER[(GROUP_ORDER.indexOf(S.board.group) + 1) % GROUP_ORDER.length];
      S.board.group = g; store.set("boardGroup", g); S.board.col = null; S.board.folded.clear();
      render(); toast("Board by " + g);
      return true;
    }
    case "e": S.board.hideEmpty = !S.board.hideEmpty; render(); toast(S.board.hideEmpty ? "Empty columns hidden" : "Empty columns shown"); return true;
    case "S-Tab": {
      const lanes = [...new Set(pool().map(i => laneOf(i) || "none"))];
      if (lanes.every(l => S.board.laneFold.has(l))) S.board.laneFold.clear(); else lanes.forEach(l => S.board.laneFold.add(l));
      renderBoard();
      return true;
    }
    case "{": case "}": {
      const cards = [...document.querySelectorAll<HTMLElement>("#wboard [data-card], #vBoard [data-card]")].map(x => x.dataset.card!);
      const lanes = [...new Set(cards.map(x => laneOf(get(x)!) || "none"))];
      const at = lanes.indexOf(c ? laneOf(c) || "none" : lanes[0]);
      const to = lanes[Math.max(0, Math.min(lanes.length - 1, at + (k === "{" ? -1 : 1)))];
      const first = cards.find(x => (laneOf(get(x)!) || "none") === to);
      if (first) { S.cursor = first; renderBoard(); }
      return true;
    }
  }
  if (!wide() && (k === "h" || k === "l")) {
    const cols = [...document.querySelectorAll<HTMLElement>("#tabs [data-tab]")].map(t => t.dataset.tab!);
    const at = cols.indexOf(S.board.col || "");
    const to = cols[at + (k === "l" ? 1 : -1)];
    if (to) { S.board.col = to; renderBoard(); }
    return true;
  }
  if (!c) return false;
  switch (k) {
    case "Tab": {
      const l = laneOf(c) || "none";
      if (S.board.laneFold.has(l)) S.board.laneFold.delete(l); else S.board.laneFold.add(l);
      renderBoard();
      return true;
    }
    case "y": void copy(c.id, "Copied " + c.id); return true;
    case "f": toggleFocus(c.id, "branch"); return true;
    case "S": void act("status"); return true;
    case "K": if (!readOnly() && eff(c) !== "closed") void setStatus([c.id], "closed"); return true;
  }
  return false;
}
