// The fleet view: the coding agents that run on the machine b9s web serves,
// drawn six ways. Board, Agent page, Constellation, Context matrix, Timeline
// and Projects share one view (#vFleet) and one sub-navigation. The server
// scans the transcripts, so this module only fetches and draws; it polls
// while the view is on screen and stops when another view takes over.

import type {
  AgentsResponse,
  AgentContext, AgentEvent, AgentLogResponse, AgentSession, FleetCard, FleetProject, FleetResponse, MatrixResponse, MatrixRow, TimelineResponse,
} from "./api.gen";
import { ApiError, getJSON } from "./api";
import { get } from "./data";
import { render } from "./render";
import { S } from "./state";
import { $opt, esc, parseTime } from "./util";

export type FleetPage = "board" | "agent" | "constellation" | "context" | "timeline" | "projects";

const PAGES: { id: Exclude<FleetPage, "agent">; label: string }[] = [
  { id: "board", label: "Fleet" },
  { id: "context", label: "Context" },
  { id: "constellation", label: "Constellation" },
  { id: "timeline", label: "Timeline" },
  { id: "projects", label: "Projects" },
];

const POLL_MS = 3000;
const KINDS = ["TASK", "USER", "AI", "TOOL", "RESULT", "BD", "CTX", "ERR"] as const;

interface AgentPage {
  session: AgentSession | null;
  log: AgentLogResponse | null;
  ctx: AgentContext | null;
  error: string;
}

const F = {
  page: "board" as FleetPage,
  /** the page the agent page returns to */
  from: "board" as Exclude<FleetPage, "agent">,
  agentId: "",
  fleet: null as FleetResponse | null,
  matrix: null as MatrixResponse | null,
  timeline: null as TimelineResponse | null,
  hours: 2,
  agent: { session: null, log: null, ctx: null, error: "" } as AgentPage,
  kinds: new Set<string>(KINDS),
  error: "",
  loading: false,
};

let timer = 0;
/** a response older than the latest request is dropped, so a slow poll never overwrites a newer page */
let seq = 0;
let lastSig = "";

/* ================= data ================= */

async function load(): Promise<void> {
  const mine = ++seq;
  const page = F.page;
  try {
    if (page === "board" || page === "projects" || page === "constellation") {
      const fleet = await getJSON<FleetResponse>("api/fleet");
      if (mine !== seq) return;
      F.fleet = fleet;
    }
    if (page === "context" || page === "constellation") {
      const matrix = await getJSON<MatrixResponse>("api/context-matrix");
      if (mine !== seq) return;
      F.matrix = matrix;
    }
    if (page === "timeline") {
      const timeline = await getJSON<TimelineResponse>("api/timeline?hours=" + F.hours);
      if (mine !== seq) return;
      F.timeline = timeline;
    }
    if (page === "agent") {
      const id = encodeURIComponent(F.agentId);
      const [session, log, ctx] = await Promise.all([
        getJSON<AgentSession>("api/agent?id=" + id),
        getJSON<AgentLogResponse>("api/agent/log?id=" + id),
        getJSON<AgentContext>("api/agent/context?id=" + id),
      ]);
      if (mine !== seq) return;
      F.agent = { session, log, ctx, error: "" };
    }
    F.error = "";
  } catch (e) {
    if (mine !== seq) return;
    if (page === "agent" && e instanceof ApiError && e.status === 404) F.agent = { session: null, log: null, ctx: null, error: "This session is gone: its transcript is older than the scan window." };
    else F.error = e instanceof Error ? e.message : String(e);
  }
  F.loading = false;
  draw();
}

function ensurePolling(): void {
  if (timer) return;
  timer = window.setInterval(() => {
    if (S.view !== "fleet" || document.hidden) {
      if (S.view !== "fleet") { clearInterval(timer); timer = 0; }
      return;
    }
    void load();
  }, POLL_MS);
}

function go(page: FleetPage): void {
  if (page !== "agent") F.from = page;
  F.page = page;
  F.error = "";
  F.loading = true;
  lastSig = "";
  draw();
  void load();
}

/** openAgent shows one session's page, as a click on an agent chip does. */
export function openAgent(id: string): void {
  if (F.page !== "agent") F.from = F.page;
  F.agentId = id;
  F.agent = { session: null, log: null, ctx: null, error: "" };
  F.page = "agent";
  F.loading = true;
  lastSig = "";
  draw();
  void load();
}

/** fleetBack steps the agent page back to the page it came from. It reports whether it used the key. */
export function fleetBack(): boolean {
  if (S.view !== "fleet" || F.page !== "agent") return false;
  go(F.from);
  return true;
}

/**
 * fleetKey opens the fleet screen the TUI binds to function key n (ADR 0068).
 * F1 is the board, F2 a session's log, F3 the context matrix, F4 the project
 * treemap, F5 the constellation and F6 the timeline. F7 and F8 are the TUI's
 * commit screens, which the web has no page for, so those keys stay free.
 */
export function fleetKey(n: number, public_: boolean): boolean {
  const pages: Record<number, Exclude<FleetPage, "agent">> = { 1: "board", 3: "context", 4: "projects", 5: "constellation", 6: "timeline" };
  if (public_ || (n !== 2 && !pages[n])) return false;
  S.view = "fleet";
  S.detail = null;
  if (n !== 2) { render(); go(pages[n]); return true; }
  render();
  if (F.page === "agent" || F.agentId) { if (F.page !== "agent") openAgent(F.agentId); return true; }
  // No session was opened yet: take the first live one, else the first one listed.
  void getJSON<AgentsResponse>("api/agents").then(r => {
    const pick = r.sessions.find(x => x.live) ?? r.sessions[0];
    if (pick) openAgent(pick.id); else go("board");
  }).catch(() => go("board"));
  return true;
}

/** renderFleet draws the view; render() calls it when the view is on screen. */
export function renderFleet(): void {
  ensurePolling();
  if (!F.fleet && !F.matrix && !F.timeline && !F.agent.session && !F.loading) { F.loading = true; void load(); }
  draw();
}

/* ================= shared pieces ================= */

const STATE_CLASS: Record<string, string> = {
  Running: "run", Waiting: "wait", Idle: "idle", Failed: "fail", Pending: "pend", Done: "done",
};
const stateClass = (s: string): string => STATE_CLASS[s] || "idle";

const k = (n: number): string => (n >= 100000 ? Math.round(n / 1000) + "k" : n >= 1000 ? (n / 1000).toFixed(1) + "k" : String(n));
const pct = (a: number, b: number): number => (b > 0 ? Math.max(0, Math.min(100, Math.round((a * 100) / b))) : 0);
const clock = (iso: string, seconds = false): string => {
  const t = parseTime(iso);
  if (!t) return "";
  return new Date(t).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", ...(seconds ? { second: "2-digit" } : {}), hour12: false });
};
const sessionsById = (): Map<string, AgentSession> => new Map((F.fleet?.sessions ?? []).map(s => [s.id, s]));

function bar(percent: number): string {
  return `<span class="fl-bar" role="img" aria-label="context ${percent}% used"><i class="${percent >= 85 ? "hot" : ""}" style="width:${percent}%"></i></span>`;
}

function chip(s: AgentSession): string {
  return `<button class="fl-agent" data-fl-agent="${esc(s.id)}" title="${esc(s.name)} · ${esc(s.state)} · ${esc(s.model)}">`
    + `<span class="fl-dot s-${stateClass(s.state)}" aria-hidden="true"></span><span class="fl-an">${esc(s.name)}</span>`
    + `<small>${esc(s.tool)} ${s.context_percent}%</small></button>`;
}

const empty = (text: string): string => `<div class="fl-empty">${esc(text)}</div>`;

/* ================= W1 fleet board ================= */

const STAGES: { id: string; label: string }[] = [
  { id: "ready", label: "Ready" }, { id: "in_progress", label: "In progress" }, { id: "review", label: "Review" },
  { id: "test", label: "Test / deploy" }, { id: "done", label: "Done today" },
];

function cardHTML(c: FleetCard, by: Map<string, AgentSession>): string {
  const agents = c.agents.map(id => by.get(id)).filter((s): s is AgentSession => !!s);
  const id = c.id ? (get(c.id) ? `<button class="fl-id" data-nav="${esc(c.id)}">${esc(c.id)}</button>` : `<span class="fl-id">${esc(c.id)}</span>`) : "";
  return `<div class="fl-card ${agents.length ? "live" : ""} ${agents.some(a => a.state === "Failed") ? "bad" : ""}">${id}<span class="fl-ct">${esc(c.title)}</span>`
    + agents.map(a => `<div class="fl-agentrow">${chip(a)}${bar(a.context_percent)}</div>`).join("") + `</div>`;
}

function boardHTML(): string {
  const fleet = F.fleet;
  if (!fleet) return "";
  if (!fleet.projects.length) return empty("No projects and no agent sessions yet.");
  const by = sessionsById();
  const head = `<div class="fl-grid fl-gh" aria-hidden="true"><span>Project</span>${STAGES.map(s => `<span>${s.label}</span>`).join("")}</div>`;
  const rows = fleet.projects.map(p => {
    const cells = STAGES.map(def => {
      const cell = p.stages.find(s => s.stage === def.id);
      const cards = cell?.cards ?? [];
      const shown = cards.filter(c => c.id).length;
      const more = cell && cell.count > shown ? `<span class="fl-more">${p.current || cards.length === 0 ? "" : "+"}${cell.count - shown} ${def.id === "done" ? "closed" : "more"}</span>` : "";
      const body = cards.map(c => cardHTML(c, by)).join("") + more;
      return `<div class="fl-cell ${body ? "has" : ""}"><span class="fl-cl">${def.label}${cell && cell.count ? ` · ${cell.count}` : ""}</span>${body}</div>`;
    }).join("");
    return `<div class="fl-grid fl-prow ${p.current ? "cur" : ""}"><div class="fl-pj"><b>${esc(p.name)}</b><small>${p.open} open${p.live_agents ? ` · ${p.live_agents} agent${p.live_agents === 1 ? "" : "s"}` : ""}${p.blocked ? ` · ${p.blocked} blocked` : ""}${p.reach && p.reach !== "ok" && p.reach !== "unknown" ? ` · ${esc(p.reach)}` : ""}</small></div>${cells}</div>`;
  }).join("");
  return `<div class="fl-scroll"><div class="fl-board">${head}${rows}</div></div>`;
}

/* ================= W2 agent page ================= */

const LAYER_CLASS: Record<string, string> = {
  Instructions: "l0", Memories: "l1", Skills: "l2", Task: "l3", "Files read": "l4", "Tool output": "l5", Conversation: "l6", "System and tools": "l7",
};

function logLine(e: AgentEvent): string {
  const text = `<span class="fl-ts">${esc(clock(e.time, true))}</span><span class="fl-tag t-${esc(e.kind.toLowerCase())}">${esc(e.kind)}</span><span class="fl-lt">${e.tool ? `<b>${esc(e.tool)}</b> ` : ""}${esc(e.text)}</span>`;
  return e.detail ? `<details class="fl-ln"><summary>${text}</summary><pre>${esc(e.detail)}</pre></details>` : `<div class="fl-ln">${text}</div>`;
}

function contextHTML(ctx: AgentContext | null): string {
  if (!ctx) return "";
  const limit = ctx.limit || ctx.window;
  const total = ctx.layers.reduce((n, l) => n + l.tokens, 0);
  const free = Math.max(0, limit - total);
  const seg = ctx.layers.filter(l => l.tokens > 0).map(l => `<i class="${LAYER_CLASS[l.layer] || "l7"}" style="flex:${l.tokens}" title="${esc(l.layer)} ${k(l.tokens)}"></i>`).join("") + (free ? `<i class="free" style="flex:${free}"></i>` : "");
  const legend = ctx.layers.filter(l => l.tokens > 0).map(l => `<span class="fl-sw ${LAYER_CLASS[l.layer] || "l7"}"></span><span>${esc(l.layer)}${countOf(ctx, l.layer)}</span><span class="r">${k(l.tokens)}</span>`).join("");
  const stale = ctx.sources.filter(s => s.stale);
  const warn = stale.map(s => `<div class="fl-warn"><b>Stale:</b> ${esc(s.name)} changed at ${esc(clock(s.changed_at))}. This session still runs on the ${esc(clock(s.loaded_at))} copy.</div>`).join("");
  const groups = ["Instructions", "Memories", "Skills", "Files read"].map(layer => {
    const items = ctx.sources.filter(s => s.layer === layer);
    if (!items.length) return "";
    return `<details class="fl-src"><summary>${esc(layer)} · ${items.length}</summary>${items.map(s => `<div class="fl-srow ${s.stale ? "fl-stale" : ""}"><span class="fl-sn" title="${esc(s.path)}">${esc(s.name)}</span><span class="fl-sc">${esc(s.scope)}</span><span class="r">${k(s.tokens)}</span></div>`).join("")}</details>`;
  }).join("");
  return `<div class="fl-eyebrow">Context · ${k(ctx.window)} of ${k(limit)}</div><div class="fl-stack" role="img" aria-label="Context by layer">${seg}</div><div class="fl-legend">${legend}</div>${warn}${groups}`;
}

const countOf = (ctx: AgentContext, layer: string): string => {
  const n = ctx.sources.filter(s => s.layer === layer).length;
  return n ? ` (${n})` : "";
};

function taskHTML(s: AgentSession): string {
  const issue = s.task_id ? get(s.task_id) : undefined;
  const row = (label: string, value: string) => value ? `<div class="fl-kv"><span>${label}</span><span>${value}</span></div>` : "";
  const task = s.task_id
    ? `<div class="fl-card"><span class="fl-id">${issue ? `<button class="fl-id" data-nav="${esc(s.task_id)}">${esc(s.task_id)}</button>` : esc(s.task_id)}</span><span class="fl-ct">${esc(issue ? issue.t : "Not in the open project")}</span>${issue ? `<small class="fl-mut">${esc(issue.status)} · P${issue.priority} · ${esc(issue.type)}</small>` : ""}</div>`
    : `<div class="fl-mut">No task named in this session.</div>`;
  return `<div class="fl-eyebrow">Task</div>${task}<div class="fl-eyebrow">Session</div>`
    + row("Role", esc(s.role || "interactive")) + row("Branch", esc(s.branch)) + row("Where", esc(s.where_label || s.where_kind || "no process"))
    + row("Folder", `<span class="fl-path">${esc(s.cwd)}</span>`) + row("Model", esc(s.model))
    + row("Tokens", `${k(s.context_tokens)} window · ${k(s.total_tokens)} total · ${s.tokens_per_min}/min`)
    + row("Tool calls", `${s.tool_calls}${s.errors ? ` · <span class="fl-bad">${s.errors} errors</span>` : ""}`) + row("Compactions", String(s.compactions))
    + row("Started", esc(clock(s.started))) + row("Last activity", esc(clock(s.last_activity, true)));
}

function agentHTML(): string {
  const a = F.agent;
  if (a.error) return `<div class="fl-bar2"><button data-fl="back">‹ Back</button></div>${empty(a.error)}`;
  if (!a.session) return `<div class="fl-bar2"><button data-fl="back">‹ Back</button></div>${empty("Loading the session…")}`;
  const s = a.session;
  const events = (a.log?.events ?? []).filter(e => F.kinds.has(e.kind));
  const filters = KINDS.map(kind => `<button class="fl-f ${F.kinds.has(kind) ? "on" : ""}" data-fl-kind="${kind}" aria-pressed="${F.kinds.has(kind)}">${kind}</button>`).join("");
  return `<div class="fl-bar2"><button data-fl="back">‹ ${esc(PAGES.find(p => p.id === F.from)?.label ?? "Back")}</button>`
    + `<span class="fl-title"><span class="fl-dot s-${stateClass(s.state)}" aria-hidden="true"></span><b>${esc(s.name)}</b><small>${esc(s.project)} · ${esc(s.tool)} · ${esc(s.state)}</small></span></div>`
    + `<div class="fl-agentgrid"><section class="fl-col fl-logcol"><div class="fl-eyebrow">Log · ${events.length} of ${a.log?.events.length ?? 0}</div><div class="fl-filters">${filters}</div>`
    + `<div class="fl-log" id="flLog">${events.length ? events.map(logLine).join("") : `<div class="fl-mut">No events match.</div>`}</div></section>`
    + `<section class="fl-col fl-ctxcol">${contextHTML(a.ctx)}</section><section class="fl-col fl-taskcol">${taskHTML(s)}</section></div>`;
}

/* ================= W3 constellation ================= */

const W = 900, H = 520, CX = W / 2, CY = H / 2 - 6;

function constellationHTML(): string {
  const m = F.matrix;
  if (!m) return "";
  if (!m.sessions.length) return empty("No live sessions.");
  const R = 210, n = m.sessions.length;
  const angle = (i: number) => (-Math.PI / 2) + (2 * Math.PI * i) / n;
  const pos = m.sessions.map((_, i) => ({ x: CX + Math.cos(angle(i)) * R * 1.55, y: CY + Math.sin(angle(i)) * R }));
  const shared = m.rows.filter(r => !r.aggregate && r.sessions >= 2);
  const sources = (shared.length ? shared : m.rows.filter(r => !r.aggregate).slice(0, 8)).slice(0, 24);
  const placed: { row: MatrixRow; x: number; y: number }[] = [];
  const taken = new Map<string, number>();
  sources.forEach((row, k2) => {
    const idx = row.cells.flatMap((c, i) => (c.loaded ? [i] : []));
    let sx = 0, sy = 0;
    for (const i of idx) { sx += Math.cos(angle(i)); sy += Math.sin(angle(i)); }
    // A source that every session loads has no direction to lean in: its
    // centroid sits on the middle and the labels would pile up there, so such
    // sources spread evenly around two alternating rings. A source that only
    // some sessions load sits toward them.
    const lean = idx.length ? Math.hypot(sx, sy) / idx.length : 0;
    const ring = k2 % 2 ? 0.72 : 0.42;
    let a = lean > 0.3 ? Math.atan2(sy, sx) : (2 * Math.PI * k2) / sources.length + Math.PI / 7;
    const slot = Math.round(a * 10) + ":" + ring;
    const n2 = taken.get(slot) ?? 0;
    taken.set(slot, n2 + 1);
    a += n2 * 0.3;
    placed.push({ row, x: CX + Math.cos(a) * R * 1.55 * ring, y: CY + Math.sin(a) * R * ring });
  });
  const max = Math.max(1, ...sources.map(r => r.tokens));
  const edges = placed.flatMap(({ row, x, y }) => row.cells.flatMap((c, i) => c.loaded
    ? [`<path class="fl-edge ${LAYER_CLASS[row.layer] || "l7"} ${c.stale ? "fl-stale" : ""}" d="M${x.toFixed(1)} ${y.toFixed(1)} L${pos[i].x.toFixed(1)} ${pos[i].y.toFixed(1)}" stroke-width="${(1 + (c.tokens / max) * 5).toFixed(1)}"/>`] : [])).join("");
  const nodes = placed.map(({ row, x, y }) => {
    const r = 7 + Math.sqrt(row.tokens / max) * 13;
    return `<g class="fl-src-node ${LAYER_CLASS[row.layer] || "l7"} ${row.stale ? "fl-stale" : ""}"><title>${esc(row.name)} · ${row.sessions} sessions · ${k(row.tokens)} tokens${row.stale ? " · " + row.stale + " stale" : ""}</title>`
      + `<circle cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="${r.toFixed(1)}"/><text x="${x.toFixed(1)}" y="${(y + r + 12).toFixed(1)}" text-anchor="middle">${esc(trim(row.name, 26))}</text></g>`;
  }).join("");
  const labels = m.sessions.map((s, i) => {
    const p = pos[i], right = p.x > CX + 10, left = p.x < CX - 10;
    const anchor = right ? "start" : left ? "end" : "middle";
    const dx = right ? 15 : left ? -15 : 0, dy = right || left ? 4 : p.y > CY ? 24 : -14;
    return `<g class="fl-sess" data-fl-agent="${esc(s.id)}" tabindex="0" role="button" aria-label="${esc(s.name)}"><title>${esc(s.name)} · ${esc(s.state)}</title>`
      + `<circle class="s-${stateClass(s.state)}" cx="${p.x.toFixed(1)}" cy="${p.y.toFixed(1)}" r="9"/><text x="${(p.x + dx).toFixed(1)}" y="${(p.y + dy).toFixed(1)}" text-anchor="${anchor}">${esc(trim(s.name, 22))}</text></g>`;
  }).join("");
  const note = shared.length ? `${sources.length} shared sources` : "No source is shared yet; showing the largest";
  return `<div class="fl-scroll"><div class="fl-note">${m.sessions.length} sessions · ${note} · line width = tokens loaded · outlined = stale</div>`
    + `<svg class="fl-svg" viewBox="0 0 ${W} ${H}" role="img" aria-label="Context constellation: sessions on the outer ring, shared context sources inside">`
    + `<ellipse class="fl-ring" cx="${CX}" cy="${CY}" rx="${R * 1.55 * 0.42}" ry="${R * 0.42}"/><ellipse class="fl-ring" cx="${CX}" cy="${CY}" rx="${R * 1.55 * 0.72}" ry="${R * 0.72}"/>`
    + `<g>${edges}</g>${nodes}${labels}</svg></div>`;
}

const trim = (s: string, n: number): string => (s.length > n ? s.slice(0, n - 1) + "…" : s);

/* ================= W4 context matrix ================= */

function matrixHTML(): string {
  const m = F.matrix;
  if (!m) return "";
  if (!m.sessions.length) return empty("No live sessions.");
  const max = Math.max(1, ...m.rows.filter(r => !r.aggregate).flatMap(r => r.cells.map(c => c.tokens)));
  const maxAgg = Math.max(1, ...m.rows.filter(r => r.aggregate).flatMap(r => r.cells.map(c => c.tokens)));
  const heat = (tokens: number, ceiling: number) => 1 + Math.round(Math.sqrt(tokens / ceiling) * 3);
  const head = `<tr><th class="fl-corner">Source</th>${m.sessions.map(s => `<th class="fl-colh"><button data-fl-agent="${esc(s.id)}" title="${esc(s.name)} · ${esc(s.state)}"><span class="fl-dot s-${stateClass(s.state)}"></span><span>${esc(trim(s.name, 18))}</span></button></th>`).join("")}<th class="fl-colh">Sessions</th></tr>`;
  const body = m.rows.map(r => {
    const cells = r.cells.map(c => c.loaded
      ? `<td class="h${heat(c.tokens, r.aggregate ? maxAgg : max)} ${c.stale ? "fl-stale" : ""}" title="${k(c.tokens)} tokens${c.stale ? " · stale copy" : ""}">${k(c.tokens)}</td>` : `<td class="h0">·</td>`).join("");
    return `<tr class="${r.aggregate ? "agg" : ""}"><th class="fl-rowh" title="${esc(r.path || r.name)}"><span>${esc(trim(r.name, 44))}</span><small>${esc(r.layer)}${r.scope ? " · " + esc(r.scope) : ""}</small></th>${cells}<td class="fl-tot">${r.sessions}</td></tr>`;
  }).join("");
  const insights = m.insights.map(i => `<div class="fl-ins"><strong>${esc(i.title)}</strong>${esc(i.text)}</div>`).join("");
  return `<div class="fl-scroll"><div class="fl-note">cells = tokens loaded · <span class="fl-stalekey">outlined</span> = changed after the session loaded it</div>`
    + `<div class="fl-tablewrap"><table class="fl-heat"><thead>${head}</thead><tbody>${body}</tbody></table></div><div class="fl-insights">${insights}</div></div>`;
}

/* ================= W5 timeline ================= */

const HOUR_CHOICES = [1, 2, 6, 12, 24];
const PHASES: { id: string; label: string }[] = [
  { id: "plan", label: "plan" }, { id: "implement", label: "implement" }, { id: "track", label: "bd" }, { id: "fail", label: "fail" }, { id: "idle", label: "idle" },
];

function timelineHTML(): string {
  const t = F.timeline;
  const picker = `<div class="fl-filters">${HOUR_CHOICES.map(h => `<button class="fl-f ${F.hours === h ? "on" : ""}" data-fl-hours="${h}" aria-pressed="${F.hours === h}">${h}h</button>`).join("")}</div>`;
  if (!t) return `<div class="fl-scroll">${picker}</div>`;
  const from = parseTime(t.from), to = parseTime(t.to), span = Math.max(1, to - from);
  const at = (iso: string) => Math.max(0, Math.min(100, ((parseTime(iso) - from) / span) * 100));
  const stepMin = ({ 1: 10, 2: 30, 6: 60, 12: 120, 24: 240 } as Record<number, number>)[t.hours] || 60;
  const ticks: string[] = [];
  const stepMs = stepMin * 60000;
  for (let ms = Math.ceil(from / stepMs) * stepMs; ms <= to; ms += stepMs) {
    ticks.push(`<span style="left:${(((ms - from) / span) * 100).toFixed(2)}%">${esc(new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false }))}</span>`);
  }
  const lanes = t.lanes.map(l => {
    const segs = l.segments.map(s => {
      const left = at(s.start), width = Math.max(0.4, at(s.end) - left);
      return `<i class="ph-${esc(s.phase)}" style="left:${left.toFixed(2)}%;width:${width.toFixed(2)}%" title="${esc(s.phase)} ${esc(clock(s.start))} to ${esc(clock(s.end))}${s.label ? " · " + esc(s.label) : ""}">${width > 9 ? esc(s.label || s.phase) : ""}</i>`;
    }).join("");
    const marks = l.markers.map(m => `<em class="m-${esc(m.kind)}" style="left:${at(m.time).toFixed(2)}%" title="${esc(m.kind)} ${esc(clock(m.time, true))} · ${esc(m.label)}"></em>`).join("");
    return `<button class="fl-who" data-fl-agent="${esc(l.session_id)}"><span class="fl-dot s-${stateClass(l.state)}"></span><b>${esc(trim(l.name, 22))}</b><small>${esc(l.project)}</small></button><div class="fl-lane">${segs}${marks}</div>`;
  }).join("");
  const legend = PHASES.map(p => `<span><i class="ph-${p.id}"></i> ${p.label}</span>`).join("") + `<span><em class="m-commit"></em> commit</span><span><em class="m-bd"></em> bd call</span><span><em class="m-fail"></em> error</span><span><em class="m-ctx"></em> compaction</span>`;
  return `<div class="fl-scroll">${picker}${t.lanes.length ? `<div class="fl-tl"><div></div><div class="fl-axis">${ticks.join("")}</div>${lanes}</div>` : empty("No agent was active in this window.")}<div class="fl-legend2">${legend}</div></div>`;
}

/* ================= W6 project treemap ================= */

interface Rect { x: number; y: number; w: number; h: number }

/** squarify lays weights out as rectangles of near-square shape (Bruls, Huizing and van Wijk). Weights must be sorted descending. */
function squarify(weights: number[], box: Rect): Rect[] {
  const total = weights.reduce((a, b) => a + b, 0) || 1;
  const out: Rect[] = [];
  let area = weights.map(w => (w / total) * box.w * box.h);
  let free = { ...box };
  const worst = (row: number[], side: number) => {
    const sum = row.reduce((a, b) => a + b, 0);
    return Math.max((side * side * Math.max(...row)) / (sum * sum), (sum * sum) / (side * side * Math.min(...row)));
  };
  while (area.length) {
    const side = Math.min(free.w, free.h);
    let n = 1;
    while (n < area.length && worst(area.slice(0, n + 1), side) <= worst(area.slice(0, n), side)) n++;
    const row = area.slice(0, n), sum = row.reduce((a, b) => a + b, 0);
    if (free.w >= free.h) {
      const w = sum / free.h;
      let y = free.y;
      for (const a of row) { out.push({ x: free.x, y, w, h: a / w }); y += a / w; }
      free = { x: free.x + w, y: free.y, w: free.w - w, h: free.h };
    } else {
      const h = sum / free.w;
      let x = free.x;
      for (const a of row) { out.push({ x, y: free.y, w: a / h, h }); x += a / h; }
      free = { x: free.x, y: free.y + h, w: free.w, h: free.h - h };
    }
    area = area.slice(n);
  }
  return out;
}

function projectsHTML(): string {
  const fleet = F.fleet;
  if (!fleet) return "";
  const projects = [...fleet.projects].sort((a, b) => b.open - a.open || a.name.localeCompare(b.name));
  if (!projects.length) return empty("No projects.");
  const shown = projects.slice(0, 24), quiet = projects.slice(24);
  const rects = squarify(shown.map(p => Math.max(p.open, 1)), { x: 0, y: 0, w: 100, h: 100 });
  const tiles = shown.map((p, i) => {
    const r = rects[i];
    const agents = Math.min(p.live_agents, 3);
    const mostlyBlocked = p.open > 0 && p.blocked * 2 >= p.open;
    return `<div class="fl-tile a${agents} ${mostlyBlocked ? "blk" : ""} ${p.current ? "cur" : ""}" style="left:${r.x.toFixed(2)}%;top:${r.y.toFixed(2)}%;width:${r.w.toFixed(2)}%;height:${r.h.toFixed(2)}%" title="${esc(p.name)}: ${p.open} open, ${p.ready} ready, ${p.blocked} blocked, ${p.live_agents} agents">`
      + `<b>${esc(p.name)}</b><span>${p.open} open${p.live_agents ? ` · ${p.live_agents} agent${p.live_agents === 1 ? "" : "s"}` : ""}</span></div>`;
  }).join("");
  return `<div class="fl-scroll"><div class="fl-note">area = open issues · colour = live agents · red outline = at least half blocked${quiet.length ? ` · +${quiet.length} smaller projects` : ""}</div>`
    + `<div class="fl-tm">${tiles}</div><div class="fl-insights">${projectInsights(projects).map(i => `<div class="fl-ins"><strong>${esc(i.title)}</strong>${esc(i.text)}</div>`).join("")}</div></div>`;
}

function projectInsights(ps: FleetProject[]): { title: string; text: string }[] {
  const out: { title: string; text: string }[] = [];
  const biggest = ps[0];
  const totalOpen = ps.reduce((n, p) => n + p.open, 0);
  if (biggest && biggest.open > 0) out.push({ title: `${biggest.name} holds the most open work`, text: `${biggest.open} of ${totalOpen} open issues (${pct(biggest.open, totalOpen)}%), ${biggest.ready} ready.` });
  const stuck = ps.filter(p => p.open > 0 && p.blocked * 2 >= p.open);
  if (stuck.length) out.push({ title: `${stuck.map(p => p.name).slice(0, 2).join(" and ")} ${stuck.length === 1 ? "is" : "are"} mostly blocked`, text: `${stuck.reduce((n, p) => n + p.blocked, 0)} blocked issues with no ready work to start.` });
  const idle = ps.filter(p => p.open > 0 && p.live_agents === 0 && p.ready > 0);
  if (idle.length) out.push({ title: `${idle.length} project${idle.length === 1 ? " has" : "s have"} ready work and no agent`, text: idle.slice(0, 4).map(p => `${p.name} (${p.ready})`).join(", ") + "." });
  const live = ps.reduce((n, p) => n + p.live_agents, 0), closed = ps.reduce((n, p) => n + p.closed_today, 0);
  out.push({ title: `${live} live agent${live === 1 ? "" : "s"}, ${closed} closed today`, text: closed ? "Closed counts cover the project on screen only." : "Open a project to count what it closed today." });
  return out;
}

/* ================= frame ================= */

function pageHTML(): string {
  if (F.error) return `<div class="fl-empty">${esc(F.error)}</div>`;
  switch (F.page) {
    case "board": return boardHTML();
    case "agent": return agentHTML();
    case "constellation": return constellationHTML();
    case "context": return matrixHTML();
    case "timeline": return timelineHTML();
    case "projects": return projectsHTML();
  }
}

function summary(): string {
  const sessions = F.fleet?.sessions ?? F.matrix?.sessions.map(s => ({ state: s.state, live: true })) ?? [];
  const live = sessions.filter(s => "live" in s ? s.live : true);
  if (!live.length) return "";
  const run = live.filter(s => s.state === "Running").length, wait = live.filter(s => s.state === "Waiting").length, fail = live.filter(s => s.state === "Failed").length;
  return `<span class="fl-sum">${live.length} live · ${run} running · ${wait} waiting${fail ? ` · <span class="fl-bad">${fail} failed</span>` : ""}</span>`;
}

function draw(): void {
  const host = $opt("#vFleet");
  if (!host || host.hidden) return;
  const tab = F.page === "agent" ? F.from : F.page;
  const nav = F.page === "agent" ? "" : `<nav class="fl-nav" aria-label="Fleet pages">${PAGES.map(p => `<button class="${p.id === tab ? "on" : ""}" data-fl="${p.id}" aria-current="${p.id === tab ? "page" : "false"}">${p.label}</button>`).join("")}${summary()}</nav>`;
  const html = nav + `<div class="fl-body" id="flBody">${F.loading && !F.fleet && !F.matrix && !F.timeline && F.page !== "agent" ? empty("Reading agent sessions…") : pageHTML()}</div>`;
  // A poll that changed nothing must not redraw: it would close open log details and reset scrolling.
  const sig = F.page + "|" + F.hours + "|" + [...F.kinds].join() + "|" + html;
  if (sig === lastSig) return;
  lastSig = sig;
  const logBox = $opt("#flLog");
  const stick = !logBox || logBox.scrollTop + logBox.clientHeight >= logBox.scrollHeight - 24;
  const logTop = logBox?.scrollTop ?? 0;
  const bodyTop = $opt("#flBody > .fl-scroll")?.scrollTop ?? 0;
  host.innerHTML = html;
  const next = $opt("#flLog");
  if (next) next.scrollTop = stick ? next.scrollHeight : logTop;
  const scroller = $opt("#flBody > .fl-scroll");
  if (scroller) scroller.scrollTop = bodyTop;
}

document.addEventListener("click", ev => {
  const host = (ev.target as HTMLElement).closest<HTMLElement>("#vFleet");
  if (!host) return;
  const t = ev.target as HTMLElement;
  const agent = t.closest<HTMLElement>("[data-fl-agent]");
  if (agent) { openAgent(agent.dataset.flAgent!); return; }
  const nav = t.closest<HTMLElement>("[data-fl]");
  if (nav) { const page = nav.dataset.fl!; if (page === "back") go(F.from); else go(page as FleetPage); return; }
  const kind = t.closest<HTMLElement>("[data-fl-kind]");
  if (kind) {
    const v = kind.dataset.flKind!;
    if (F.kinds.has(v)) F.kinds.delete(v); else F.kinds.add(v);
    draw();
    return;
  }
  const hours = t.closest<HTMLElement>("[data-fl-hours]");
  if (hours) { F.hours = Number(hours.dataset.flHours); F.timeline = null; F.loading = true; draw(); void load(); }
});

document.addEventListener("keydown", ev => {
  const t = ev.target as HTMLElement;
  if ((ev.key === "Enter" || ev.key === " ") && t.matches?.("#vFleet [data-fl-agent]:not(button)")) { ev.preventDefault(); openAgent(t.dataset.flAgent!); }
});
