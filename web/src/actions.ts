// Taps, sheets and writes. Every write goes to POST api/write, which runs
// one bd command on the server; the client only shows the change early and
// refetches the snapshot when the server answers.

import * as api from "./api";
import type { ProjectEntry, WriteRequest, WriteResult } from "./api.gen";
import { D, descendants, eff, get, kids, load, openBlockers, pool, shortId, type Item } from "./data";
import { boardCols, ensureVisible, fullIssue, openCols, render, renderBoard, renderSearch, revealInTree, treeList } from "./render";
import { PC, S, SORTS, SORT_HELP, ST, TY, queryString, saveChips, stOf, type Sheet, type SortKey } from "./state";
import { $, $opt, copyText, esc, fmtDate, haptic, store } from "./util";

/* ================= toast ================= */

let toastTimer = 0;
let undoFn: (() => void) | null = null;

export function toast(msg: string, cmd?: string, undo?: (() => void) | null, ms?: number): void {
  const el = $("#toast");
  clearTimeout(toastTimer);
  undoFn = undo || null;
  const dur = ms || (undo ? 5000 : 2600);
  el.innerHTML = `<span class="m">${esc(msg)}</span>${cmd ? `<span class="c">$ ${esc(cmd)}</span>` : ""}${undo ? `<button data-act="undo">UNDO</button><i class="tm"></i>` : ""}`;
  el.classList.add("on");
  const tm = el.querySelector<HTMLElement>(".tm");
  if (tm && tm.animate) tm.animate([{ transform: "scaleX(1)" }, { transform: "scaleX(0)" }], { duration: dur, easing: "linear", fill: "forwards" });
  toastTimer = window.setTimeout(() => { el.classList.remove("on"); undoFn = null; }, dur);
}

export const hideToast = () => $("#toast").classList.remove("on");

/* ================= data refresh ================= */

let loading: Promise<Set<string>> | null = null;

/** refresh fetches the snapshot and re-renders; the IDs it changed flash. */
export function refresh(): Promise<Set<string>> {
  if (loading) return loading;
  loading = api.snapshot().then(snap => {
    const first = !D.loaded;
    const changed = load(snap);
    if (!first) changed.forEach(flash);
    pruneState();
    render();
    return first ? new Set<string>() : changed;
  }).finally(() => { loading = null; });
  return loading;
}

/** pruneState drops references to issues the snapshot no longer has. */
function pruneState(): void {
  for (const id of [...S.marks]) if (!get(id)) S.marks.delete(id);
  if (!S.marks.size) S.marking = false;
  if (S.cursor && !get(S.cursor)) S.cursor = null;
  if (S.focus && !get(S.focus.id)) S.focus = null;
  if (S.detail) {
    S.detail.stack = S.detail.stack.filter(x => get(x));
    if (!S.detail.stack.length) S.detail = null;
  }
}

export function flash(id: string): void {
  S.flash.add(id);
  setTimeout(() => { S.flash.delete(id); }, 2500);
}

/* ================= writes ================= */

/** describe renders the bd command a request asks for, for the toast and the write log. */
export function describe(r: WriteRequest): string {
  const ids = (r.ids || []).join(" ");
  const q = (s: string) => JSON.stringify(s.length > 40 ? s.slice(0, 40) + "…" : s);
  const flags = (f: Record<string, string> = {}) => Object.entries(f).map(([k, v]) => `--${k}=${v.includes(" ") || !v ? q(v) : v}`).join(" ");
  switch (r.op) {
    case "status": return r.status === "closed" ? `bd close ${ids}${r.reason ? ` --reason=${q(r.reason)}` : ""}` : `bd update ${ids} --status=${r.status}`;
    case "close": return `bd close ${ids}${r.reason ? ` --reason=${q(r.reason)}` : ""}`;
    case "delete": return `bd delete ${ids} --force`;
    case "update": return `bd update ${ids} ${flags(r.fields)}`;
    case "create": return `bd create ${flags(r.fields)}`;
    case "comment": return `bd comments add ${ids} ${q(r.text || "")}`;
    case "defer": return `bd defer ${ids}${r.until ? ` --until=${r.until}` : ""}`;
  }
  return "bd";
}

export function readOnly(): boolean {
  if (!D.project.read_only) return false;
  toast("This project is read-only in b9s web");
  return true;
}

/**
 * send runs a write, logs it and refetches. optimistic changes the local
 * copy first so the row moves at once; any failure refetches, which puts
 * the row back where the server says it is.
 */
export async function send(r: WriteRequest, optimistic?: () => void): Promise<WriteResult> {
  const cmd = describe(r);
  if (optimistic) { optimistic(); render(); }
  const res = await api.write(r);
  S.log.unshift({ at: Date.now(), cmd, ok: res.ok });
  if (S.log.length > 100) S.log.length = 100;
  if (!res.ok) toast("Failed: " + (res.error || "bd returned an error"), cmd, null, 6000);
  await refresh().catch(e => toast("Reload failed: " + (e instanceof Error ? e.message : String(e))));
  return res;
}

type StatusSnap = { id: string; status: string }[];
const snapStatus = (ids: string[]): StatusSnap => ids.map(id => ({ id, status: get(id)?.status || "open" }));

function localStatus(ids: string[], status: string): void {
  ids.forEach(id => {
    const i = get(id);
    if (!i) return;
    i.status = status;
    i.closed_like = status === "closed";
    if (status !== "blocked") i.blocked = i.blocked && status !== "closed" && openBlockers(i).length > 0;
    i.upd = Date.now();
    flash(id);
  });
}

/** restoreStatus writes back each issue's previous raw status, one request per status. */
async function restoreStatus(snap: StatusSnap): Promise<void> {
  const groups = new Map<string, string[]>();
  snap.forEach(s => { const g = groups.get(s.status); if (g) g.push(s.id); else groups.set(s.status, [s.id]); });
  let ok = true;
  for (const [status, ids] of groups) {
    const res = await send({ op: "status", ids, status }, () => localStatus(ids, status));
    ok = ok && res.ok;
  }
  if (ok) toast("Undone", [...groups].map(([s, ids]) => describe({ op: "status", ids, status: s })).join("; "));
}

export async function setStatus(ids: string[], status: string, reason?: string): Promise<void> {
  if (readOnly() || !ids.length) return;
  const snap = snapStatus(ids);
  const req: WriteRequest = { op: "status", ids, status, reason: reason || undefined };
  const res = await send(req, () => localStatus(ids, status));
  if (!res.ok) return;
  const who = ids.length > 1 ? ids.length + " issues" : shortId(ids[0]);
  toast(`${who} → ${stOf(status).w}`, describe(req), () => { void restoreStatus(snap); });
}

export async function deleteIssues(ids: string[]): Promise<void> {
  if (readOnly()) return;
  const req: WriteRequest = { op: "delete", ids };
  const res = await send(req);
  if (res.ok) toast(`Deleted ${ids.length} issue${ids.length > 1 ? "s" : ""}`, describe(req));
}

export function nextStatus(i: Item): string {
  const e = eff(i);
  if (e === "in_progress" || e === "closed") return "open";
  return "in_progress";
}
export const swipeRightLabel = (i: Item) => { const n = nextStatus(i); return eff(i) === "closed" ? "○ reopen" : n === "open" ? "○ stop" : "◐ start"; };

/** moveCard is a board drop: a status column sets the status, the others update the field. */
export async function moveCard(id: string, target: string): Promise<void> {
  const i = get(id);
  if (!i || readOnly()) return;
  const g = S.board.group;
  if (g === "status") {
    if (target === "open" && openBlockers(i).length) toast(`${shortId(id)} stays in Blocked: ${openBlockers(i).length} open blocker(s)`);
    await setStatus([id], target);
    return;
  }
  const before = g === "priority" ? String(i.priority) : i.type;
  const req: WriteRequest = { op: "update", ids: [id], fields: { [g]: target } };
  const apply = (v: string) => () => { if (g === "priority") i.priority = +v; else i.type = v; flash(id); };
  const res = await send(req, apply(target));
  if (res.ok) toast(`${shortId(id)} → ${g === "priority" ? "P" + target : target}`, describe(req), () => {
    void send({ op: "update", ids: [id], fields: { [g]: before } }, apply(before)).then(r => { if (r.ok) toast("Undone"); });
  });
}

/* ================= detail ================= */

export function openDetail(id: string, push = false): void {
  S.cursor = id;
  if (S.detail && push) S.detail.stack.push(id);
  else if (S.detail) S.detail.stack = [id];
  else S.detail = { stack: [id], size: "half" };
  revealInTree(id);
  render();
  ensureVisible(id);
}

export function closeDetail(): void {
  const sec = $opt(".detail");
  if (sec) { sec.style.transform = "translateY(100%)"; setTimeout(() => { S.detail = null; render(); }, 220); }
  else { S.detail = null; render(); }
}

const detailId = () => (S.detail ? S.detail.stack[S.detail.stack.length - 1] : S.cursor) || "";

/* ================= marking and focus ================= */

export function toggleMark(id: string): void {
  S.marking = true;
  if (S.marks.has(id)) S.marks.delete(id); else S.marks.add(id);
  S.lastMark = id;
  if (!S.marks.size) S.marking = false;
  render();
}

const visibleRowIds = () => [...document.querySelectorAll<HTMLElement>("#treeRows .row, #sRows .row")].filter(r => r.offsetParent !== null).map(r => r.dataset.id!);

export function markRange(id: string): void {
  const ids = visibleRowIds();
  const a = ids.indexOf(S.lastMark || ""), b = ids.indexOf(id);
  if (a < 0 || b < 0) { toggleMark(id); return; }
  ids.slice(Math.min(a, b), Math.max(a, b) + 1).forEach(x => S.marks.add(x));
  S.lastMark = id;
  render();
  toast(`Marked ${Math.abs(a - b) + 1} rows`);
}

function endMarking(): void { S.marks.clear(); S.marking = false; }

export function setFocus(id: string, mode: string): void {
  S.focus = { id, mode: mode === "branch" ? "branch" : "subtree" };
  S.folded.delete(id);
  revealInTree(id);
  S.view = S.view === "board" ? "board" : "tree";
  render();
  toast((mode === "branch" ? "Branch " : "Subtree ") + shortId(id) + " · ✕ in the bar clears it");
}

/* ================= query ================= */

/** setText makes a query the tree's filter; a status predicate in it replaces the chips. */
export function setText(v: string): void {
  S.text = v.trim();
  if (/(^|\s)!?status:|(^|\s)![a-z_]+(\s|$)/i.test(S.text)) { S.st.clear(); S.ready = false; saveChips(); }
  store.set("text", S.text);
}

function remember(v: string): void {
  if (!v.trim()) return;
  const rec = store.get<string[]>("recent", []).filter(x => x !== v);
  rec.unshift(v);
  store.set("recent", rec.slice(0, 8));
}

export function chipTap(k: string): void {
  if (k === "all") { S.st.clear(); S.ready = false; }
  else if (k === "ready") S.ready = !S.ready;
  else if (S.st.has(k)) S.st.delete(k); else S.st.add(k);
  saveChips();
  render();
}

export function applySuggestion(s: string): void {
  const inp = $<HTMLInputElement>("#sIn");
  const parts = inp.value.split(/(\s+)/);
  let last = parts.pop() || "";
  const m = last.match(/^(!?\w+:)(.*)$/);
  last = m && !s.includes(":") ? m[1] + s : s;
  inp.value = parts.join("") + last + (s.endsWith(":") ? "" : " ");
  inp.focus();
  renderSearch();
}

/* ================= projects and reload ================= */

export async function switchProject(key: string): Promise<void> {
  try {
    await api.openProject(key);
  } catch (e) {
    toast("Could not open project: " + (e instanceof Error ? e.message : String(e)));
    return;
  }
  Object.assign(S, { focus: null, detail: null, cursor: null, marking: false, graph: [], qres: null });
  S.marks.clear(); S.folded.clear();
  S.board.lane = null;
  await refresh().catch(e => toast("Reload failed: " + (e instanceof Error ? e.message : String(e))));
  treeList().scrollTop = 0;
  toast(D.project.all ? "All projects" : "Project " + D.project.name);
}

export async function reload(): Promise<void> {
  const ptr = $("#ptr");
  ptr.style.height = "36px"; ptr.textContent = "↻ reloading…"; ptr.classList.add("ready");
  try {
    const h = await api.reload();
    D.health = h;
    await refresh();
    toast(`Reloaded ${D.all.size} issues`, h.ok ? h.source : h.error);
  } catch (e) {
    toast("Reload failed: " + (e instanceof Error ? e.message : String(e)));
  } finally {
    ptr.style.height = "0"; ptr.classList.remove("ready"); ptr.textContent = "↓ pull to reload";
  }
}

/* ================= sheets ================= */

export function openSheet(kind: string, data: Partial<Sheet> = {}): void {
  S.sheet = { ...data, kind };
  renderSheet();
}

export function closeSheet(): void {
  $opt(".msheet")?.classList.remove("on");
  $opt(".backdrop")?.classList.remove("on");
  S.sheet = null;
  setTimeout(() => { if (!S.sheet) $("#sheetHost").innerHTML = ""; }, 230);
}

const opt = (a: string, g: string, x: string, sub: string, extra: { val?: string | number; cls?: string; color?: string; key?: string } = {}) =>
  `<button class="opt ${extra.cls || ""}" data-sa="${a}"${extra.val != null ? ` data-val="${esc(extra.val)}"` : ""}><span class="g" ${extra.color ? `style="color:${extra.color}"` : ""}>${g}</span><span class="x">${x}${sub ? `<small>${sub}</small>` : ""}</span>${extra.key ? `<span class="key">${esc(extra.key)}</span>` : ""}</button>`;

const SETTABLE = ["open", "in_progress", "blocked", "deferred", "review", "closed"];

export const GESTURES: [string, string, string][] = [
  ["Tap a row", "open detail sheet", "Enter"], ["Tap ▾ / ▸", "fold", "Tab"], ["Double-tap ▾", "fold whole subtree", "h / l"],
  ["Short swipe →", "start / stop", "S"], ["Long swipe →", "status picker", "S"], ["Short swipe ←", "close, 5 s undo", "K"], ["Long swipe ←", "action sheet", "e c y f x"],
  ["Long-press a row", "mark; tap more rows", "Space"], ["Long-press while marking", "mark the range", "Ctrl-Space"], ["Pull down on tree", "reload", "Ctrl-R"],
  ["Swipe ← → on detail", "next / previous sibling", "n p"], ["Drag handle up / down", "full height / close", "Esc"], ["Tap a relation", "go there; ‹ goes back", ""],
  ["Swipe ← → on board", "next column", "h l"], ["Tap the active column tab", "fold it into a rail", "z"], ["Tap Z unfold", "unfold all", "Z"],
  ["Long-press a card, drag", "move to a column", ""], ["Tap project name", "project sheet", "1-9 0"], ["Tap the ● dot", "source health", "D"],
  ["Swipe down on header", "hide or show chips", "Ctrl-E"], ["Tap F", "follow live changes", "F"],
];

const gestureTable = () => `<table class="gtable">${GESTURES.map(([g, a, k]) => `<tr><td>${g}</td><td>${a}${k ? ` · <kbd>${k}</kbd>` : ""}</td></tr>`).join("")}</table>`;

let projectList: ProjectEntry[] | null = null;
let projectError = "";

function formHTML(sh: Sheet): string {
  const edit = sh.kind === "edit";
  const f = edit ? (sh.full as import("./api.gen").Issue) : null;
  const e = f
    ? { title: f.title, status: f.status, priority: f.priority, type: f.type, assignee: f.assignee, labels: f.labels.join(", "), description: f.description, notes: f.notes }
    : { title: "", status: "open", priority: 2, type: String(sh.type || "task"), assignee: "", labels: "", description: "", notes: "" };
  const seg = (name: string, vals: [string | number, string][], cur: string | number, color?: (v: string) => string) =>
    `<div class="segs" data-seg="${name}">${vals.map(([v, l]) => `<button type="button" class="${String(v) === String(cur) ? "on" : ""}" data-v="${esc(v)}" style="${color ? "color:" + color(String(v)) : ""}">${esc(l)}</button>`).join("")}</div>`;
  const types = [...new Set([...Object.keys(TY), e.type])];
  const statuses = [...new Set([...SETTABLE, e.status])];
  const people = [...new Set([D.actor, ...D.people.map(p => p.name)].filter(Boolean))];
  const epics = pool().filter(x => x.type === "epic" && !x.closed_like);
  return `<form class="form" id="fEdit"><label>Title<input id="fTitle" value="${esc(e.title)}" placeholder="Short imperative title" required></label>
    ${edit ? `<label>Status${seg("status", statuses.map(s => [s, stOf(s).g + " " + (s === "in_progress" ? "prog" : s)]), e.status, s => stOf(s).c)}</label>` : ""}
    <label>Priority${seg("priority", [0, 1, 2, 3, 4].map(p => [p, "P" + p]), e.priority, p => PC[+p])}</label>
    <label>Type${seg("type", types.map(t => [t, (TY[t] ? TY[t][0] + " " : "") + t]), e.type, t => (TY[t] ? TY[t][1] : "var(--muted)"))}</label>
    <label>Assignee<input id="fAssignee" value="${esc(e.assignee)}" placeholder="none" list="dlWho" autocapitalize="off"><datalist id="dlWho">${people.map(a => `<option value="${esc(a)}">`).join("")}</datalist></label>
    <label>Labels<input id="fLabels" value="${esc(e.labels)}" placeholder="comma separated" autocapitalize="off"></label>
    ${!edit ? `<label>Parent<select id="fParent"><option value="">none</option>${epics.map(x => `<option value="${esc(x.id)}" ${x.id === sh.parent ? "selected" : ""}>${esc(shortId(x.id) + " " + x.t)}</option>`).join("")}${sh.parent && !epics.some(x => x.id === sh.parent) ? `<option value="${esc(sh.parent)}" selected>${esc(shortId(String(sh.parent)))}</option>` : ""}</select></label>` : ""}
    <label>Description<textarea id="fDesc" placeholder="Why this exists and what done looks like">${esc(e.description)}</textarea></label>
    ${edit ? `<label>Notes<textarea id="fNotes">${esc(e.notes)}</textarea></label>` : ""}</form>
    <div class="fbtns"><button data-sa="cancel">Cancel</button><button class="pri" data-sa="${edit ? "save" : "docreate"}">${edit ? "Save" : "Create"}</button></div>`;
}

function sheetBody(sh: Sheet): [string, string, string] {
  const i = sh.id ? get(sh.id) : undefined;
  switch (sh.kind) {
    case "status": {
      const ids = sh.ids || [sh.id!];
      const cur = ids.length === 1 && get(ids[0]) ? get(ids[0])!.status : null;
      return [ids.length > 1 ? ids.length + " issues" : "Status", ids.length > 1 ? "bd update --status on every marked issue" : `${esc(shortId(ids[0]))} · ${esc(get(ids[0])?.t || "")}`,
        SETTABLE.map(s => opt("setstatus", ST[s].g, ST[s].w, s === "blocked" ? "stored as blocked; b9s also derives it from open blockers" : s === "closed" ? "asks for a reason" : "", { val: s, color: ST[s].c, cls: cur === s ? "cur" : "" })).join("")
        + opt("defer", "◌", "Defer until…", "bd defer, with an optional date", { color: "var(--orange)" })];
    }
    case "closereason": return ["Close " + (sh.ids!.length > 1 ? sh.ids!.length + " issues" : esc(shortId(sh.ids![0]))), "The reason goes to bd close --reason",
      `<div class="form"><label>Reason<textarea id="fReason" placeholder="Done: what shipped and where"></textarea></label></div><div class="fbtns"><button data-sa="cancel">Cancel</button><button class="pri" data-sa="doclose">Close</button></div>`];
    case "defer": return ["Defer " + esc(shortId(sh.id!)), "bd defer; leave the date empty to defer without one",
      `<div class="form"><label>Until<input id="fUntil" type="date"></label></div><div class="fbtns"><button data-sa="cancel">Cancel</button><button class="pri" data-sa="dodefer">Defer</button></div>`];
    case "actions": {
      if (!i) return ["", "", ""];
      const ro = D.project.read_only;
      return [esc(shortId(i.id)), esc(i.t), [
        ro ? "" : opt("edit", "✎", "Edit", "title, status, priority, type, assignee, labels, text", { key: "e" }),
        ro ? "" : opt("comment", "✉", "Comment", "", { key: "c" }),
        ro ? "" : opt("status", "◐", "Set status…", "", { key: "S" }),
        ro ? "" : opt("defer", "◌", "Defer…", "bd defer", {}),
        opt("copy", "⧉", "Copy ID", esc(i.id), { key: "y" }),
        opt("copymd", "⧉", "Copy Markdown", "title, fields and description", {}),
        opt("branch", "⎇", "Focus branch", "ancestors and subtree only", { key: "f" }),
        opt("subtree", "⌖", "Focus subtree", "this issue and its descendants", { key: "x" }),
        opt("graph", "⋔", "Dependency graph", "", { key: "g" }),
        opt("mark", "▪", S.marks.has(i.id) ? "Unmark" : "Mark", "start bulk selection", { key: "Space" }),
        ro ? "" : opt("child", "+", "Create child", "parent " + esc(shortId(i.id)), { key: "N" }),
        ro ? "" : opt("delete", "⌫", "Delete", "asks first", { cls: "red", key: "Del" })].join("")];
    }
    case "confirmdelete": return [`Delete ${sh.ids!.length > 1 ? sh.ids!.length + " issues" : esc(shortId(sh.ids![0]))}?`, "This cannot be undone. Dependencies on it are removed too.",
      `<div class="cmd">bd delete ${esc(sh.ids!.join(" "))} --force</div><div class="fbtns"><button data-sa="cancel">Cancel</button><button class="danger" data-sa="dodelete">Delete</button></div>`];
    case "edit": return ["Edit " + esc(shortId(sh.id!)), "Saved with bd update; only changed fields are sent", formHTML(sh)];
    case "create": return ["New issue", "Created with bd create" + (sh.parent ? " --parent=" + esc(shortId(String(sh.parent))) : ""), formHTML(sh)];
    case "comment": return ["Comment on " + esc(shortId(sh.id!)), "as " + esc(D.actor || "the server's bd actor") + " · bd comments add",
      `<div class="form"><label>Comment<textarea id="fComment" placeholder="Markdown works"></textarea></label></div><div class="fbtns"><button data-sa="cancel">Cancel</button><button class="pri" data-sa="docomment">Send</button></div>`];
    case "projects": {
      if (projectError) return ["Projects", "", `<div class="empty err">${esc(projectError)}</div>`];
      if (!projectList) return ["Projects", "", `<div class="empty">Loading…</div>`];
      const rows = projectList.map(p => opt("proj", p.slot ? String(p.slot) : "·", esc(p.name),
        p.reach && p.reach !== "ok" ? esc(p.reach) : `${p.open} open · ${p.in_progress} in progress · ${p.ready} ready${p.blocked ? " · " + p.blocked + " blocked" : ""}`,
        { val: p.key, key: p.slot ? String(p.slot) : "", cls: p.active && !D.project.all ? "cur" : "", color: "var(--accent)" })).join("");
      return ["Projects", "What b9s can read from here", rows + opt("proj", "0", "All projects", "one tree, grouped by project", { val: "*", key: "0", cls: D.project.all ? "cur" : "", color: "var(--accent)" })];
    }
    case "health": {
      const h = D.health;
      if (!h) return ["Data source health", "", `<div class="empty">Loading…</div>`];
      const row = (k: string, v: string) => (v ? `<tr><td>${k}</td><td>${v}</td></tr>` : "");
      return ["Data source health", "What b9s web reads, and how it notices changes", `<table class="gtable">
        ${row("Status", h.ok ? `<span class="ok">● ok</span>` : `<span class="err">● problem</span>`)}
        ${row("Source", esc(h.kind + (h.source ? " · " + h.source : "")))}
        ${row("Watcher", esc(h.watching))}
        ${row("Loaded", h.loaded_at ? esc(fmtDate(Date.parse(h.loaded_at))) : "")}
        ${row("Issues", String(h.issues))}
        ${row("Fallback", esc(h.fallback))}
        ${row("Error", h.error ? `<span class="err">${esc(h.error)}</span>` : "")}
        ${row("bd CLI", h.bd_found ? "found on the server" : `<span class="err">not found: writes fail</span>`)}
        ${row("Live push", S.live === "live" ? "SSE connected" : S.live === "stale" ? `<span class="err">disconnected, retrying</span>` : "connecting")}
        ${row("Credentials", "The database password stays on the server. This browser holds a session cookie only.")}
        </table><div class="fbtns"><button data-sa="cancel">Close</button><button class="pri" data-sa="reload">Reload</button></div>`];
    }
    case "identity": return ["Acting as", "bd on the server writes as this actor. Change it where b9s web runs (BD_ACTOR or git config).",
      `<table class="gtable"><tr><td>Actor</td><td>${esc(D.actor || "unknown")}</td></tr>${D.people.length ? `<tr><td>People</td><td>${D.people.map(p => esc(p.name) + (p.kind ? ` <small>(${esc(p.kind)})</small>` : "")).join(", ")}</td></tr>` : ""}</table>`];
    case "sort": return ["Sort", "Same fields as the TUI sort menu",
      (Object.keys(SORTS) as SortKey[]).map(k => opt("setsort", "⇅", k, SORT_HELP[k], { val: k, cls: S.sort === k ? "cur" : "" })).join("")];
    case "treeopts": return ["Tree options", "", opt("sortsheet", "⇅", "Sort", S.sort)
      + opt("wrap", "↩", S.wrap ? "Truncate titles" : "Wrap titles", "TUI v or :wrap", { key: "v" })
      + opt("textsize", "A", "Text size", [100, 115, 130].map(n => (n === S.textSize ? `<b>${n}%</b>` : n + "%")).join(" · "))
      + opt("expandall", "▾", "Expand all", "", { key: "L" }) + opt("collapseall", "▸", "Collapse all", "", { key: "H" })
      + `<div class="sec">Second line shows</div>` + [["prio", "Priority"], ["type", "Type"], ["who", "Assignee"], ["age", "Updated age"], ["kids", "Children progress"], ["deps", "Blocked by and blocks"], ["notes", "Comment count"], ["labels", "Labels"]].map(([k, l]) => opt("col", S.cols.has(k) ? "☑" : "☐", l, "", { val: k })).join("")];
    case "boardopts": return ["Board options", "", `<div class="sec">Group by</div>` + ["status", "priority", "type"].map(g => opt("group", g === S.board.group ? "◉" : "○", g, "", { val: g })).join("")
      + `<div class="sec">Epic lanes</div>` + [["off", "No lanes", "epics show as cards"], ["rows", "Rows", "cards grouped under each epic"], ["rail", "Rail", "a side rail filters one epic"]].map(([v, l, s]) => opt("lanes", v === S.board.lanes ? "◉" : "○", l, s, { val: v })).join("")
      + `<div class="sec">Columns</div>` + opt("closedcol", S.st.has("closed") ? "☑" : "☐", "Show closed", "same as the C chip") + opt("unfoldall", "▭", "Unfold all columns", S.board.folded.size + " folded", { key: "Z" })];
    case "pairing": return ["Pairing", "b9s web listens on loopback. Tailscale serve publishes it to your tailnet.",
      `<table class="gtable"><tr><td>This browser</td><td>paired: it holds a session cookie</td></tr><tr><td>Pair another</td><td>open the link <b>b9s web</b> printed at start on the other device</td></tr><tr><td>Revoke</td><td>restart with <b>b9s web --new-token</b> to unpair every device</td></tr></table>`];
    case "log": return ["Write log", "Commands this browser asked the server to run through bd",
      S.log.length ? S.log.map(l => `<div class="cmd ${l.ok ? "" : "err"}">${esc(fmtDate(l.at).slice(11))} ${l.ok ? "" : "failed "}\n$ ${esc(l.cmd)}</div>`).join("") : `<div class="empty">Nothing written yet. Swipe a row.</div>`];
    case "help": return ["Gestures", "No gesture starts in the outer 24 px: both OSes use the edges for back and home.", gestureTable()];
  }
  return ["", "", ""];
}

export function renderSheet(): void {
  const sh = S.sheet, host = $("#sheetHost");
  if (!sh) { host.innerHTML = ""; return; }
  const [title, sub, body] = sheetBody(sh);
  const fresh = !host.querySelector(".msheet");
  host.innerHTML = `<div class="backdrop ${fresh ? "" : "on"}" data-sa="cancel"></div><section class="msheet ${fresh ? "" : "on"}" role="dialog" aria-label="${esc(title.replace(/<[^>]+>/g, ""))}"><div class="mh" id="mh"><div class="grab"></div><h3>${title}</h3>${sub ? `<p>${sub}</p>` : ""}</div><div class="mb">${body}</div></section>`;
  if (fresh) requestAnimationFrame(() => requestAnimationFrame(() => { host.querySelector(".backdrop")?.classList.add("on"); host.querySelector(".msheet")?.classList.add("on"); }));
}

function readForm() {
  const seg = (n: string) => { const b = document.querySelector<HTMLElement>(`[data-seg="${n}"] .on`); return b ? b.dataset.v! : null; };
  const val = (s: string) => $opt<HTMLInputElement>(s)?.value ?? "";
  return {
    title: val("#fTitle").trim(), status: seg("status"), priority: seg("priority") || "2", type: seg("type") || "task",
    assignee: val("#fAssignee").trim(), labels: val("#fLabels").split(",").map(s => s.trim()).filter(Boolean),
    description: val("#fDesc"), notes: val("#fNotes"), parent: val("#fParent"),
  };
}

async function save(sh: Sheet): Promise<void> {
  const f = sh.full as import("./api.gen").Issue;
  const v = readForm();
  if (!v.title) { $("#fTitle").focus(); return; }
  const fields: Record<string, string> = {};
  if (v.title !== f.title) fields.title = v.title;
  if (v.status && v.status !== f.status) fields.status = v.status;
  if (v.priority !== String(f.priority)) fields.priority = v.priority;
  if (v.type !== f.type) fields.type = v.type;
  if (v.assignee !== (f.assignee || "")) fields.assignee = v.assignee;
  if (v.description !== (f.description || "")) fields.description = v.description;
  if (v.notes !== (f.notes || "")) fields.notes = v.notes;
  const add = v.labels.filter(l => !f.labels.includes(l)), rm = f.labels.filter(l => !v.labels.includes(l));
  if (add.length) fields["add-label"] = add.join(",");
  if (rm.length) fields["remove-label"] = rm.join(",");
  closeSheet();
  if (!Object.keys(fields).length) { toast("No changes"); return; }
  const req: WriteRequest = { op: "update", ids: [f.id], fields };
  const res = await send(req, () => {
    const i = get(f.id);
    if (!i) return;
    if (fields.title) { i.title = fields.title; i.t = fields.title.replace(/^\[[^\]]+\]\s*/, ""); }
    if (fields.priority) i.priority = +fields.priority;
    if (fields.type) i.type = fields.type;
    if ("assignee" in fields) i.assignee = fields.assignee;
    i.labels = v.labels;
    flash(i.id);
  });
  if (res.ok) toast("Saved " + shortId(f.id), describe(req));
}

async function create(sh: Sheet): Promise<void> {
  const v = readForm();
  if (!v.title) { $("#fTitle").focus(); return; }
  const fields: Record<string, string> = { title: v.title, priority: v.priority, type: v.type };
  if (v.assignee) fields.assignee = v.assignee;
  if (v.labels.length) fields.labels = v.labels.join(",");
  if (v.description) fields.description = v.description;
  if (v.parent) fields.parent = v.parent;
  closeSheet();
  const req: WriteRequest = { op: "create", fields };
  const res = await send(req);
  if (!res.ok) return;
  const id = res.created;
  if (id && get(id)) {
    S.cursor = id;
    revealInTree(id);
    flash(id);
    if (S.st.size && !S.st.has("open")) { S.st.add("open"); saveChips(); }
    render();
    ensureVisible(id);
  }
  toast("Created " + (id ? shortId(id) : "issue"), describe(req));
  void sh;
}

export async function sheetAction(a: string, val: string | undefined): Promise<void> {
  const sh = S.sheet || ({ kind: "" } as Sheet);
  switch (a) {
    case "cancel": closeSheet(); break;
    case "setstatus": {
      const ids = sh.ids || [sh.id!];
      if (val === "closed") { openSheet("closereason", { ids }); break; }
      closeSheet();
      if (sh.ids) endMarking();
      await setStatus(ids, val!);
      break;
    }
    case "doclose": {
      const r = $<HTMLTextAreaElement>("#fReason").value.trim(), ids = sh.ids!;
      closeSheet();
      if (ids.length > 1 || S.marking) endMarking();
      await setStatus(ids, "closed", r);
      break;
    }
    case "defer": openSheet("defer", { id: sh.id || (sh.ids && sh.ids[0]) }); break;
    case "dodefer": {
      const until = $<HTMLInputElement>("#fUntil").value, id = sh.id!;
      closeSheet();
      if (readOnly()) break;
      const req: WriteRequest = { op: "defer", ids: [id], until: until || undefined };
      const res = await send(req);
      if (res.ok) toast(`Deferred ${shortId(id)}${until ? " until " + until : ""}`, describe(req));
      break;
    }
    case "edit": await openEdit(sh.id!); break;
    case "comment": openSheet("comment", { id: sh.id }); break;
    case "status": openSheet("status", { id: sh.id }); break;
    case "copy": { const id = sh.id!; closeSheet(); toast(await copyText(id) ? "Copied " + id : "Clipboard refused. ID: " + id); break; }
    case "copymd": { const id = sh.id!; closeSheet(); toast(await copyText(await markdownOf(id)) ? "Copied " + shortId(id) + " as Markdown" : "Clipboard refused"); break; }
    case "branch": case "subtree": closeSheet(); setFocus(sh.id!, a); break;
    case "graph": closeSheet(); S.graph = [sh.id!]; S.detail = null; S.view = "graph"; render(); break;
    case "mark": closeSheet(); toggleMark(sh.id!); break;
    case "child": openSheet("create", { parent: sh.id }); break;
    case "delete": openSheet("confirmdelete", { ids: [sh.id!] }); break;
    case "dodelete": { const ids = sh.ids!; closeSheet(); if (ids.length > 1 || S.marking) endMarking(); await deleteIssues(ids); break; }
    case "save": await save(sh); break;
    case "docreate": await create(sh); break;
    case "docomment": {
      const t = $<HTMLTextAreaElement>("#fComment").value.trim();
      if (!t) { $("#fComment").focus(); break; }
      const id = sh.id!;
      closeSheet();
      if (readOnly()) break;
      const req: WriteRequest = { op: "comment", ids: [id], text: t };
      const res = await send(req);
      if (res.ok) {
        toast("Comment added", describe(req));
        requestAnimationFrame(() => { const b = $opt("#dbody"); if (b) b.scrollTop = b.scrollHeight; });
      }
      break;
    }
    case "proj": closeSheet(); await switchProject(val!); break;
    case "reload": closeSheet(); await reload(); break;
    case "setsort": S.sort = val as SortKey; store.set("sort", S.sort); closeSheet(); render(); break;
    case "sortsheet": openSheet("sort"); break;
    case "wrap": S.wrap = !S.wrap; store.set("wrap", S.wrap); closeSheet(); render(); toast(S.wrap ? "Titles wrap" : "Titles truncate"); break;
    case "textsize": S.textSize = S.textSize === 100 ? 115 : S.textSize === 115 ? 130 : 100; store.set("textSize", S.textSize); applyTextSize(); renderSheet(); render(); break;
    case "expandall": S.folded.clear(); closeSheet(); render(); break;
    case "collapseall": pool().forEach(i => { if (kids(i.id).length) S.folded.add(i.id); }); closeSheet(); render(); break;
    case "col": if (S.cols.has(val!)) S.cols.delete(val!); else S.cols.add(val!); store.set("cols", [...S.cols]); renderSheet(); render(); break;
    case "group": S.board.group = val as typeof S.board.group; store.set("boardGroup", val); S.board.col = null; S.board.folded.clear(); renderSheet(); render(); break;
    case "lanes": S.board.lanes = val as typeof S.board.lanes; store.set("boardLanes", val); S.board.lane = null; renderSheet(); render(); break;
    case "closedcol": if (S.st.has("closed")) S.st.delete("closed"); else S.st.add("closed"); saveChips(); renderSheet(); render(); break;
    case "unfoldall": S.board.folded.clear(); renderSheet(); render(); break;
  }
}

export function applyTextSize(): void {
  document.documentElement.style.fontSize = (13 * S.textSize) / 100 + "px";
}

async function openEdit(id: string): Promise<void> {
  if (readOnly()) return;
  const cached = fullIssue(id);
  if (cached) { openSheet("edit", { id, full: cached }); return; }
  try {
    openSheet("edit", { id, full: await api.issue(id) });
  } catch (e) {
    toast("Could not load " + shortId(id) + ": " + (e instanceof Error ? e.message : String(e)));
  }
}

async function markdownOf(id: string): Promise<string> {
  const f = fullIssue(id) || await api.issue(id).catch(() => null);
  const i = get(id)!;
  const lines = [`# ${id}: ${i.t}`, "", `- status: ${i.status}`, `- priority: P${i.priority}`, `- type: ${i.type}`];
  if (i.assignee) lines.push(`- assignee: ${i.assignee}`);
  if (i.labels.length) lines.push(`- labels: ${i.labels.join(", ")}`);
  if (i.parent) lines.push(`- parent: ${i.parent}`);
  if (i.blocked_by.length) lines.push(`- blocked by: ${i.blocked_by.join(", ")}`);
  if (f?.description) lines.push("", f.description);
  return lines.join("\n");
}

async function loadProjects(): Promise<void> {
  projectList = null; projectError = "";
  try { projectList = (await api.projects()).projects; } catch (e) { projectError = e instanceof Error ? e.message : String(e); }
  if (S.sheet && S.sheet.kind === "projects") renderSheet();
}

/* ================= taps ================= */

export async function act(a: string): Promise<void> {
  const cur = detailId();
  switch (a) {
    case "projects": openSheet("projects"); void loadProjects(); break;
    case "health": openSheet("health"); api.health().then(h => { D.health = h; if (S.sheet?.kind === "health") renderSheet(); render(); }, () => { /* the snapshot's health still shows */ }); break;
    case "identity": openSheet("identity"); break;
    case "help": openSheet("help"); break;
    case "create": {
      if (readOnly()) break;
      const d = S.detail ? get(cur) : undefined;
      openSheet("create", { parent: d ? (d.type === "epic" ? d.id : d.parent || null) : null });
      break;
    }
    case "follow": S.follow = !S.follow; store.set("follow", S.follow); render(); toast(S.follow ? "Follow on: live changes move the cursor" : "Follow off"); break;
    case "search": S.view = "search"; $<HTMLInputElement>("#sIn").value = S.text; render(); setTimeout(() => $("#sIn").focus(), 50); break;
    case "clearq": setText(""); render(); break;
    case "allq": S.st.clear(); S.ready = false; saveChips(); setText(""); S.focus = null; render(); break;
    case "sort": openSheet("sort"); break;
    case "unfocus": S.focus = null; render(); break;
    case "markall": {
      const ids = visibleRowIds();
      const all = ids.every(x => S.marks.has(x));
      ids.forEach(x => (all ? S.marks.delete(x) : S.marks.add(x)));
      if (!S.marks.size) S.marking = false;
      render();
      break;
    }
    case "bulkstatus": if (!readOnly()) openSheet("status", { ids: [...S.marks] }); break;
    case "bulkclose": if (!readOnly()) openSheet("closereason", { ids: [...S.marks] }); break;
    case "bulkdelete": if (!readOnly()) openSheet("confirmdelete", { ids: [...S.marks] }); break;
    case "unmark": endMarking(); render(); break;
    case "undo": if (undoFn) { const f = undoFn; undoFn = null; hideToast(); f(); } break;
    case "dback": if (S.detail) { S.detail.stack.pop(); S.cursor = S.detail.stack[S.detail.stack.length - 1]; render(); } break;
    case "dclose": closeDetail(); break;
    case "dsize": if (S.detail) { S.detail.size = S.detail.size === "full" ? "half" : "full"; render(); } break;
    case "dfocus": setFocus(cur, "subtree"); break;
    case "dbranch": setFocus(cur, "branch"); break;
    case "dgraph": S.graph = [cur]; S.detail = null; S.view = "graph"; render(); break;
    case "status": if (!readOnly()) openSheet("status", { id: cur }); break;
    case "edit": await openEdit(cur); break;
    case "comment": if (!readOnly()) openSheet("comment", { id: cur }); break;
    case "actions": openSheet("actions", { id: cur }); break;
    case "unfoldall": S.board.folded.clear(); render(); toast("Unfolded all columns"); break;
    case "boardopts": openSheet("boardopts"); break;
    case "treeopts": openSheet("treeopts"); break;
    case "graph": S.graph = []; S.view = "graph"; render(); break;
    case "togglechips": S.chips = !S.chips; store.set("chipRow", S.chips); render(); break;
    case "pairing": openSheet("pairing"); break;
    case "log": openSheet("log"); break;
    case "reload": await reload(); break;
    case "retrylive": retryLive(); break;
    case "applysearch": {
      const v = $<HTMLInputElement>("#sIn").value;
      setText(v); remember(v.trim());
      S.view = "tree";
      render();
      break;
    }
  }
}

let retryLive = () => { /* set by main once the live stream exists */ };
export function setRetryLive(f: () => void): void { retryLive = f; }

/* ================= board ================= */

export function tabTap(k: string): void {
  const cols = boardCols();
  const name = (cols.find(([c]) => c === k) || [k, k])[1];
  if (S.board.folded.has(k)) { S.board.folded.delete(k); S.board.col = k; renderBoard(); toast("Unfolded " + name); return; }
  if (S.board.col === k) {
    const open = openCols();
    if (open.length < 2) { toast("The last open column cannot fold"); return; }
    S.board.folded.add(k);
    const idx = open.findIndex(([c]) => c === k);
    const next = open[idx + 1] || open[idx - 1];
    S.board.col = next[0];
    renderBoard();
    toast("Folded " + name + " · Z unfolds");
    return;
  }
  S.board.col = k;
  renderBoard();
}

export function foldSubtree(id: string): void {
  const target = !S.folded.has(id);
  [id, ...descendants(id)].forEach(x => { if (kids(x).length) { if (target) S.folded.add(x); else S.folded.delete(x); } });
  toast(target ? "Folded subtree" : "Expanded subtree");
  render();
}

export { haptic, queryString };
