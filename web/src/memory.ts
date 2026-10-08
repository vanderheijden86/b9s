// Snapshot identity keeps navigation from starting another graph read.
import { memoryGraph } from "./api";
import type { MemoryGraphResponse as MemoryGraph, MemoryGraphEdge as Edge } from "./api.gen";
import { D, get } from "./data";
import { openDetail, refresh, toast } from "./actions";
import { renderMermaidBlocks } from "./mermaid";
import { fullH, halfH } from "./render";
import { S } from "./state";
import { copyText, esc, md, wide } from "./util";

let key = "", project = "", selected = "", filter = "all";
let graph: MemoryGraph | null = null;
const positions = new Map<string, { x: number; y: number }>();
let scale = 1, panX = 0, panY = 0;
type Layout = "network" | "radial" | "columns";
let layout: Layout = "network", layoutKey = "", fitPending = false;
// Percent opacity of records outside the selection. Links fade further than
// cards (CSS scales them), so a faded card stays legible over its wires.
let fade = 75, line = 2, labels = true;
// The open detail mirrors the tree's: a history of records that Backspace walks
// back, or one Link chosen by its label. selected is the record on top.
let stack: string[] = [], openEdge: Edge | null = null, size: "half" | "full" = "half";
// The detail last closed, which d shows again.
let closed: { stack: string[]; edge: Edge | null } | null = null;
const kinds = ["follows", "cites", "related", "blocks"];
const label = (kind: string) => kind === "blocks" ? "depends on" : kind;
/**
 * uml draws each Link kind as the UML connector with the same meaning, so the
 * line itself reads without its colour: work realizes the decision it follows,
 * depends on is a dependency, a citation navigates one way, and related has no
 * direction. Unrecognized kinds keep a plain arrow in their stored direction.
 */
const uml = (kind: string): { name: string; dash: boolean; head: "hollow" | "open" | "" } => ({
  follows: { name: "realization", dash: true, head: "hollow" as const },
  blocks: { name: "dependency", dash: true, head: "open" as const },
  cites: { name: "directed association", dash: false, head: "open" as const },
  related: { name: "association", dash: false, head: "" as const },
})[kind] || { name: "link", dash: false, head: "open" };
const headMarker = (kind: string) => uml(kind).head ? `url(#memory-head-${uml(kind).head}-${kind})` : "";
const color = (kind: string) => ({ follows: "var(--cyan)", cites: "var(--accent)", related: "var(--green)", blocks: "var(--orange)" })[kind] || "var(--dim)";
const cardWidth = 250, cardHeight = 102;

// canvasSize measures the canvas, which flexes to fill the view below the
// toolbar; before the first draw it estimates from the window. The viewBox
// matches it, so one SVG unit is one CSS pixel at scale 1.
function canvasSize(host: HTMLElement): { width: number; height: number } {
  const canvas = host.querySelector<HTMLElement>(".memoryCanvas");
  return { width: Math.max(320, canvas?.clientWidth || host.clientWidth), height: Math.max(260, canvas?.clientHeight || Math.round(window.innerHeight * .6)) };
}
function fitViewBox(host: HTMLElement): void {
  const { width, height } = canvasSize(host);
  host.querySelector(".memoryCanvas svg")?.setAttribute("viewBox", `0 0 ${width} ${height}`);
}

window.addEventListener("resize", () => {
  const host = document.getElementById("vMemory");
  if (graph && host && !host.hidden) fitViewBox(host);
});

export function renderMemory(): void {
  const host = document.getElementById("vMemory")!;
  const want = `${D.project.key}:${D.version}`, version = D.version;
  if (key === want) return;
  if (project !== D.project.key) {
    project = D.project.key;
    positions.clear(); selected = ""; stack = []; openEdge = null; filter = "all"; scale = 1; panX = panY = 0;
    layoutKey = "";
  }
  key = want;
  graph = null;
  host.innerHTML = '<div class="empty" role="status">Loading Memory graph…</div>';
  const failed = (message: string, refreshSnapshot = false) => {
    if (key !== want) return;
    host.innerHTML = `<div class="empty" role="alert">Could not load Memory graph: ${esc(message)}<br><button id="memoryRetry">Retry</button></div>`;
    host.querySelector("button")!.addEventListener("click", async e => {
      (e.currentTarget as HTMLButtonElement).disabled = true;
      if (refreshSnapshot) {
        try { await refresh(); }
        catch (error) { failed(error instanceof Error ? error.message : String(error), true); return; }
      }
      if (key === want) { key = ""; renderMemory(); }
    });
  };
  // The server reads the graph on the first request and reports its progress
  // until it is read (ADR 0051). The finished read is a new version, so the
  // snapshot is fetched first.
  const poll = (): void => void memoryGraph().then(g => {
    if (key !== want) return;
    if (g.loading) {
      host.innerHTML = memoryProgress(g.done, g.total);
      setTimeout(() => { if (key === want) poll(); }, memoryPollMs);
      return;
    }
    if (g.version !== version) {
      refresh().then(() => { if (key === want) failed("Project changed while loading. Retry to refresh the project.", true); },
        error => failed(error instanceof Error ? error.message : String(error), true));
      return;
    }
    graph = g;
    draw(host);
  }, e => failed(e.message));
  poll();
}

const memoryPollMs = 250;

/** memoryProgress shows how far the graph read has come; it has no count while it reads the inventory. */
function memoryProgress(done: number, total: number): string {
  if (!total) return '<div class="empty memoryLoading" role="status"><span>Reading the Memory inventory…</span><progress role="progressbar" aria-label="Reading the Memory inventory"></progress></div>';
  return `<div class="empty memoryLoading" role="status"><span>Reading Memory Links ${done}/${total}</span><progress role="progressbar" aria-label="Reading Memory Links" max="${total}" value="${done}" aria-valuemin="0" aria-valuemax="${total}" aria-valuenow="${done}"></progress></div>`;
}

function titleLines(title: string): string[] {
  const words = title.split(/\s+/), lines: string[] = [];
  let line = "";
  for (const word of words) {
    if (line && (line + " " + word).length > 29) { lines.push(line); line = ""; }
    line += (line ? " " : "") + word;
  }
  if (line) lines.push(line);
  return lines.slice(0, 2).map((s, i) => s.slice(0, 29) + (s.length > 29 || (i === 1 && lines.length > 2) ? "…" : ""));
}

function arrange(width: number, height: number): void {
  if (!graph || layoutKey === `${key}:${layout}`) return;
  layoutKey = `${key}:${layout}`;
  positions.clear();
  const nodes = [...graph.nodes].sort((a, b) => a.id.localeCompare(b.id));
  const aspect = Math.max(.6, width / height / 1.9);
  if (layout === "columns") {
    for (const kind of ["issue", "memory", "other"]) {
      nodes.filter(n => n.kind === kind).forEach((n, i) => positions.set(n.id, { x: kind === "issue" ? 30 : kind === "memory" ? 510 : 990, y: 45 + i * 154 }));
    }
  } else if (layout === "radial") {
    let at = 0, ring = 0;
    while (at < nodes.length) {
      const count = Math.min(nodes.length - at, ring === 0 ? 1 : ring * 8);
      for (let i = 0; i < count; i++) {
        const angle = 2 * Math.PI * i / count - Math.PI / 2;
        positions.set(nodes[at++].id, { x: Math.cos(angle) * ring * 340, y: Math.sin(angle) * ring * 185 });
      }
      ring++;
    }
  } else {
    const cols = Math.max(1, Math.ceil(Math.sqrt(nodes.length * aspect)));
    const points = nodes.map((n, i) => ({ id: n.id, x: (i % cols) * 1.5, y: Math.floor(i / cols) * 1.5 }));
    const byID = new Map(points.map(p => [p.id, p]));
    // A bounded, deterministic relaxation settles before the first frame.
    // Coordinates stay fixed during selection and filtering.
    for (let step = 0; step < Math.min(240, Math.max(40, Math.floor(12000 / Math.max(1, nodes.length)))); step++) {
      const forces = new Map(points.map(p => [p.id, { x: -p.x * .015, y: -p.y * .015 }]));
      for (let i = 0; i < points.length; i++) for (let j = i + 1; j < points.length; j++) {
        const a = points[i], b = points[j], dx = a.x - b.x || .001, dy = a.y - b.y || .001;
        const distance = Math.max(.1, Math.hypot(dx, dy)), force = .8 / (distance * distance);
        forces.get(a.id)!.x += dx / distance * force; forces.get(a.id)!.y += dy / distance * force;
        forces.get(b.id)!.x -= dx / distance * force; forces.get(b.id)!.y -= dy / distance * force;
      }
      for (const edge of graph.edges) {
        const a = byID.get(edge.source), b = byID.get(edge.target);
        if (!a || !b || a === b) continue;
        const dx = b.x - a.x, dy = b.y - a.y, distance = Math.max(.01, Math.hypot(dx, dy));
        const force = (distance - 1.4) * .25;
        forces.get(a.id)!.x += dx / distance * force; forces.get(a.id)!.y += dy / distance * force;
        forces.get(b.id)!.x -= dx / distance * force; forces.get(b.id)!.y -= dy / distance * force;
      }
      for (const p of points) {
        const f = forces.get(p.id)!;
        p.x += Math.max(-.2, Math.min(.2, f.x * .12)); p.y += Math.max(-.2, Math.min(.2, f.y * .12));
      }
      for (let i = 0; i < points.length; i++) for (let j = i + 1; j < points.length; j++) {
        const a = points[i], b = points[j], dx = a.x - b.x, dy = a.y - b.y;
        const ox = 1.04 - Math.abs(dx), oy = .92 - Math.abs(dy);
        if (ox > 0 && oy > 0) {
          if (ox < oy) { const shift = (dx < 0 ? -1 : 1) * ox / 2; a.x += shift; b.x -= shift; }
          else { const shift = (dy < 0 ? -1 : 1) * oy / 2; a.y += shift; b.y -= shift; }
        }
      }
    }
    const minX = Math.min(...points.map(p => p.x)), minY = Math.min(...points.map(p => p.y));
    const spanX = Math.max(1, ...points.map(p => p.x - minX)), spanY = Math.max(1, ...points.map(p => p.y - minY));
    for (const p of points) {
      p.x = (p.x - minX) / spanX * Math.max(1, cols - 1) * 1.15;
      p.y = (p.y - minY) / spanY * Math.max(1, Math.ceil(nodes.length / cols) - 1) * 1.15;
    }
    // Pack disconnected records into the same overview without overlapping cards.
    for (let pass = 0; pass < 100; pass++) {
      let overlaps = false;
      for (let i = 0; i < points.length; i++) for (let j = i + 1; j < points.length; j++) {
        const a = points[i], b = points[j], dx = a.x - b.x, dy = a.y - b.y;
        const ox = 1.04 - Math.abs(dx), oy = .92 - Math.abs(dy);
        if (ox > 0 && oy > 0) {
          overlaps = true;
          if (ox < oy) { const shift = (dx < 0 ? -1 : 1) * (ox + .002) / 2; a.x += shift; b.x -= shift; }
          else { const shift = (dy < 0 ? -1 : 1) * (oy + .002) / 2; a.y += shift; b.y -= shift; }
        }
      }
      if (!overlaps) break;
    }
    for (const p of points) positions.set(p.id, { x: p.x * 310, y: p.y * 165 });
  }
  fitPending = true;
}

function fit(host: HTMLElement, ids?: Set<string>): void {
  const points = [...positions].filter(([id]) => !ids || ids.has(id)).map(([, p]) => p);
  if (!points.length) return;
  const left = Math.min(...points.map(p => p.x)) - 35, top = Math.min(...points.map(p => p.y)) - 35;
  const w = Math.max(...points.map(p => p.x + cardWidth)) + 35 - left;
  const h = Math.max(...points.map(p => p.y + cardHeight + 42)) + 35 - top;
  const { width, height } = canvasSize(host);
  scale = Math.min(1, width / w, height / h);
  panX = (width - w * scale) / 2 - left * scale;
  panY = (height - h * scale) / 2 - top * scale;
}

/**
 * edgeShape bends a Link from the rim of one record to the rim of the other;
 * i staggers the bend so parallel Links stay apart. A dragged record redraws
 * its Links through the same geometry.
 */
function edgeShape(e: Edge, i: number): { d: string; lx: number; ly: number } | null {
  const s = positions.get(e.source), t = positions.get(e.target);
  if (!s || !t) return null;
  const dx = t.x - s.x, dy = t.y - s.y, length = Math.hypot(dx, dy) || 1;
  const boundary = Math.min(cardWidth / 2 / Math.max(.001, Math.abs(dx)), cardHeight / 2 / Math.max(.001, Math.abs(dy)));
  const sourceBoundary = graph!.nodes.find(n => n.id === e.source)?.kind === "issue" ? cardHeight / 2 / length : boundary;
  const targetBoundary = graph!.nodes.find(n => n.id === e.target)?.kind === "issue" ? cardHeight / 2 / length : boundary;
  const x1 = s.x + cardWidth / 2 + dx * sourceBoundary, y1 = s.y + cardHeight / 2 + dy * sourceBoundary;
  const x2 = t.x + cardWidth / 2 - dx * targetBoundary, y2 = t.y + cardHeight / 2 - dy * targetBoundary;
  const bendX = (x1 + x2) / 2 - dy / length * (18 + i % 3 * 10), bendY = (y1 + y2) / 2 + dx / length * (18 + i % 3 * 10);
  const lx = (x1 + 2 * bendX + x2) / 4, ly = (y1 + 2 * bendY + y2) / 4 - 8;
  return { d: `M${x1},${y1} Q${bendX},${bendY} ${x2},${y2}`, lx, ly };
}

function draw(host: HTMLElement): void {
  if (!graph) return;
  if (!graph.available) {
    host.innerHTML = `<div class="empty">Memories unavailable: ${esc(graph.reason || "this source has no Memory graph")}</div>`;
    return;
  }
  const { width, height } = canvasSize(host);
  arrange(width, height);
  const edges = graph.edges.filter(e => filter === "all" || e.kind === filter);
  host.innerHTML = `<div class="memoryTools"><b>Memory relationships</b><label>Link Type <select id="memoryType"><option value="all">All Types</option>${kinds.map(k => `<option value="${k}" ${filter === k ? "selected" : ""}>${label(k)}</option>`).join("")}</select></label><button id="memoryFit">Fit</button></div>
    <div class="memoryLegend">${kinds.map(k => `<span style="color:${color(k)}" title="UML ${uml(k).name}"><svg width="40" height="10" aria-hidden="true"><line x1="1" y1="5" x2="34" y2="5" stroke="currentColor" stroke-width="1.5" ${uml(k).dash ? 'stroke-dasharray="5 3"' : ""} ${headMarker(k) ? `marker-end="${headMarker(k)}"` : ""}/></svg> ${label(k)} <small>${uml(k).name}</small></span>`).join(" · ")}<br>Select a bead to explore its connections. Drag a record to move it, or the background to pan.</div>
    <div class="memoryCanvas">${graph.nodes.length ? `<svg role="group" aria-label="Memory relationship graph" viewBox="0 0 ${width} ${height}"><defs>${kinds.filter(k => uml(k).head).map(k => `<marker id="memory-head-${uml(k).head}-${k}" viewBox="-1 -6 13 12" refX="10" markerWidth="5" markerHeight="5" markerUnits="strokeWidth" orient="auto">${uml(k).head === "hollow" ? `<path d="M0,-5L10,0L0,5Z" style="fill:var(--bg);stroke:${color(k)}" stroke-width="1.3" stroke-linejoin="round"/>` : `<path d="M0,-5L10,0L0,5" style="fill:none;stroke:${color(k)}" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round"/>`}</marker>`).join("")}</defs><g class="memoryViewport">
    ${layout === "columns" ? '<text class="memoryLane" x="30" y="25">ISSUES · WORK</text><text class="memoryLane" x="510" y="25">MEMORIES · KNOWLEDGE</text>' : ''}
    ${edges.map((e, i) => {
      const shape = edgeShape(e, i);
      if (!shape) return "";
      const { d, lx, ly } = shape, labelWidth = Math.max(64, label(e.kind).length * 8 + 16);
      return `<g data-edge="${esc(e.id)}" data-edge-kind="${esc(e.kind)}" data-uml="${uml(e.kind).name}" data-source="${esc(e.source)}" data-target="${esc(e.target)}" style="color:${color(e.kind)}"><path d="${d}" fill="none" stroke="currentColor" ${uml(e.kind).dash ? 'stroke-dasharray="7 5"' : ""} ${headMarker(e.kind) ? `marker-end="${esc(headMarker(e.kind))}"` : ""}/><g class="memoryEdgeLabel" transform="translate(${lx},${ly})" role="button" tabindex="0" aria-label="${esc(e.source + ' ' + label(e.kind) + ' ' + e.target)}"><rect x="${-labelWidth / 2}" y="-17" width="${labelWidth}" height="26" rx="5"/><text text-anchor="middle">${esc(label(e.kind))}</text></g><title>${esc(e.type)}${e.note ? ': ' + esc(e.note) : ''}</title></g>`;
    }).join("")}
    ${graph.nodes.map(n => {
      const p = positions.get(n.id)!;
      if (n.kind === "issue") {
        const shortID = n.id.length > 15 ? n.id.slice(0, 14) + "…" : n.id;
        return `<g data-node="${esc(n.id)}" data-kind="issue" role="button" tabindex="0" aria-label="${esc(n.title + ', issue')}" transform="translate(${p.x},${p.y})"><circle class="memoryCard" cx="125" cy="51" r="51"/><text class="memoryCardKind" x="125" y="32" text-anchor="middle">${esc(n.status || "ISSUE")}</text><text class="memoryCardID" x="125" y="58" text-anchor="middle">${esc(shortID)}</text>${titleLines(n.title).map((line, i) => `<text class="memoryCardTitle" x="125" y="${122 + i * 18}" text-anchor="middle">${esc(line)}</text>`).join("")}<title>${esc('issue: ' + n.title + ' [' + n.id + ']')}</title></g>`;
      }
      return `<g data-node="${esc(n.id)}" data-kind="${esc(n.kind)}" role="button" tabindex="0" aria-label="${esc(n.title + ', ' + n.kind)}" transform="translate(${p.x},${p.y})"><rect class="memoryCard" width="${cardWidth}" height="${cardHeight}" rx="13"/><circle class="memoryBead" cx="18" cy="16" r="5" fill="currentColor"/><text class="memoryCardKind" x="30" y="20">${esc(n.kind.toUpperCase())}${n.kind === "issue" && n.status ? " · " + esc(n.status) : ""}</text>${titleLines(n.title).map((line, i) => `<text class="memoryCardTitle" x="14" y="${43 + i * 18}">${esc(line)}</text>`).join("")}<text class="memoryCardID" x="14" y="86">${esc(n.id.length > 31 ? n.id.slice(0, 30) + "…" : n.id)}</text><title>${esc(n.kind + ': ' + n.title)}</title></g>`;
    }).join("")}</g></svg><div class="memoryZoom" role="group" aria-label="Zoom"><button id="memoryZoomOut" aria-label="Zoom out (-)" title="Zoom out (-)">−</button><output id="memoryZoomLevel" title="Zoom level · f fits">100%</output><button id="memoryZoomIn" aria-label="Zoom in (+)" title="Zoom in (+)">+</button></div>` : '<div class="empty">No records.</div>'}</div>`;
  const recordLabel = document.createElement("label");
  recordLabel.innerHTML = `Record <select id="memoryRecord" aria-label="Find record"><option value="">Find a record…</option>${graph.nodes.map(n => `<option value="${esc(n.id)}" ${n.id === selected ? "selected" : ""}>${esc(n.kind + ': ' + n.title)}</option>`).join("")}</select>`;
  host.querySelector(".memoryTools")!.append(recordLabel);
  const layoutLabel = document.createElement("label");
  layoutLabel.innerHTML = `Layout <select id="memoryLayout">${(["network", "radial", "columns"] as const).map(l => `<option value="${l}" ${l === layout ? "selected" : ""}>${l[0].toUpperCase() + l.slice(1)}</option>`).join("")}</select>`;
  host.querySelector(".memoryTools b")!.after(layoutLabel);
  const fadeLabel = document.createElement("label");
  fadeLabel.title = "How visible records outside the selection stay";
  fadeLabel.innerHTML = `Unselected opacity <input id="memoryFade" type="range" min="0" max="100" step="5" value="${fade}" aria-label="Opacity of records outside the selection"><output for="memoryFade">${fade}%</output>`;
  layoutLabel.after(fadeLabel);
  const lineLabel = document.createElement("span");
  lineLabel.className = "memoryStep"; lineLabel.title = "Link line thickness";
  lineLabel.innerHTML = `Line <button id="memoryLineDown" aria-label="Thinner Links">−</button><output id="memoryLine">${line}px</output><button id="memoryLineUp" aria-label="Thicker Links">+</button>`;
  fadeLabel.after(lineLabel);
  const setLine = (to: number) => {
    line = Math.min(6, Math.max(1, to));
    host.style.setProperty("--memory-line", `${line}px`);
    lineLabel.querySelector("output")!.textContent = `${line}px`;
    lineLabel.querySelector<HTMLButtonElement>("#memoryLineDown")!.disabled = line <= 1;
    lineLabel.querySelector<HTMLButtonElement>("#memoryLineUp")!.disabled = line >= 6;
  };
  setLine(line);
  lineLabel.querySelector("#memoryLineDown")!.addEventListener("click", () => setLine(line - 1));
  lineLabel.querySelector("#memoryLineUp")!.addEventListener("click", () => setLine(line + 1));
  host.style.setProperty("--memory-fade", String(fade / 100));
  fadeLabel.querySelector("input")!.addEventListener("input", e => {
    fade = Number((e.target as HTMLInputElement).value);
    fadeLabel.querySelector("output")!.textContent = `${fade}%`;
    host.style.setProperty("--memory-fade", String(fade / 100));
  });
  layoutLabel.querySelector("select")!.addEventListener("change", e => { layout = (e.target as HTMLSelectElement).value as Layout; draw(host); });
  const labelsButton = document.createElement("button");
  labelsButton.id = "memoryLabels"; labelsButton.textContent = "Labels"; labelsButton.title = "Show or hide Link labels (l)";
  lineLabel.after(labelsButton);
  const setLabels = (on: boolean) => {
    labels = on;
    host.classList.toggle("memoryNoLabels", !on);
    labelsButton.setAttribute("aria-pressed", String(on));
  };
  setLabels(labels);
  labelsButton.addEventListener("click", () => setLabels(!labels));
  toggleLabels = () => setLabels(!labels);
  const fitSelection = document.createElement("button");
  fitSelection.id = "memoryFitSelection"; fitSelection.textContent = "Fit neighbours";
  host.querySelector("#memoryFit")!.after(fitSelection);
  const viewport = host.querySelector(".memoryViewport");
  const transform = () => {
    viewport?.setAttribute("transform", `translate(${panX},${panY}) scale(${scale})`);
    host.classList.toggle("memoryOverview", scale < .65);
    host.style.setProperty("--memory-scale", String(scale));
    const level = host.querySelector("#memoryZoomLevel");
    if (level) level.textContent = `${Math.round(scale * 100)}%`;
  };
  fitViewBox(host);
  if (fitPending) { fit(host); fitPending = false; }
  transform();
  host.querySelector("#memoryType")!.addEventListener("change", e => { filter = (e.target as HTMLSelectElement).value; draw(host); });
  // zoom keeps the canvas point (cx, cy) still, so the record under the pointer
  // or between the fingers stays put; buttons and keys use the canvas centre.
  const zoom = (factor: number, cx = canvasSize(host).width / 2, cy = canvasSize(host).height / 2) => {
    const next = Math.max(.08, Math.min(3, scale * factor)), ratio = next / scale;
    panX = cx - (cx - panX) * ratio; panY = cy - (cy - panY) * ratio;
    scale = next; transform();
  };
  const fitAll = () => { fit(host); transform(); };
  zoomBy = zoom; fitGraph = fitAll;
  host.querySelector("#memoryZoomIn")?.addEventListener("click", () => zoom(1.25));
  host.querySelector("#memoryZoomOut")?.addEventListener("click", () => zoom(.8));
  host.querySelector("#memoryFit")!.addEventListener("click", fitAll);
  fitSelection.addEventListener("click", () => {
    if (!selected) { fit(host); transform(); return; }
    const ids = new Set([selected]);
    for (const edge of edges) if (edge.source === selected || edge.target === selected) { ids.add(edge.source); ids.add(edge.target); }
    fit(host, ids); transform();
  });
  const svg = host.querySelector<SVGSVGElement>(".memoryCanvas svg");
  let drag: { x: number; y: number; px: number; py: number } | null = null;
  // A record follows the pointer once it moves past a few pixels, so a tap
  // still opens it; its Links redraw from the new position.
  let carry: { id: string; x: number; y: number; px: number; py: number; moved: boolean } | null = null, carried = false;
  const moveRecord = (id: string, x: number, y: number) => {
    positions.set(id, { x, y });
    host.querySelector(`[data-node="${CSS.escape(id)}"]`)?.setAttribute("transform", `translate(${x},${y})`);
    edges.forEach((e, i) => {
      if (e.source !== id && e.target !== id) return;
      const shape = edgeShape(e, i), el = host.querySelector(`[data-edge="${CSS.escape(e.id)}"]`);
      if (!shape || !el) return;
      el.querySelector(":scope > path")!.setAttribute("d", shape.d);
      el.querySelector(".memoryEdgeLabel")!.setAttribute("transform", `translate(${shape.lx},${shape.ly})`);
    });
  };
  // Two touching pointers pinch: the zoom follows their distance around their midpoint.
  const touches = new Map<number, { x: number; y: number }>();
  let pinch = 0;
  const local = (e: { clientX: number; clientY: number }) => {
    const r = svg!.getBoundingClientRect();
    return { x: e.clientX - r.left, y: e.clientY - r.top };
  };
  const spread = () => { const [a, b] = [...touches.values()]; return Math.hypot(a.x - b.x, a.y - b.y); };
  svg?.addEventListener("pointerdown", e => {
    touches.set(e.pointerId, local(e));
    if (touches.size === 2) { drag = null; carry = null; pinch = spread(); return; }
    const record = (e.target as Element).closest<SVGGElement>("[data-node]");
    if (record) {
      const p = positions.get(record.dataset.node!)!;
      carry = { id: record.dataset.node!, x: e.clientX, y: e.clientY, px: p.x, py: p.y, moved: false };
      return;
    }
    if ((e.target as Element).closest(".memoryEdgeLabel")) return;
    drag = { x: e.clientX, y: e.clientY, px: panX, py: panY };
    try { svg.setPointerCapture(e.pointerId); } catch { /* the pointer may already be gone */ }
  });
  svg?.addEventListener("pointermove", e => {
    if (touches.has(e.pointerId)) touches.set(e.pointerId, local(e));
    if (touches.size === 2 && pinch) {
      const now = spread(), [a, b] = [...touches.values()];
      zoom(now / pinch, (a.x + b.x) / 2, (a.y + b.y) / 2); pinch = now;
      return;
    }
    if (carry) {
      const dx = e.clientX - carry.x, dy = e.clientY - carry.y;
      if (!carry.moved && Math.hypot(dx, dy) < 5) return;
      if (!carry.moved) try { svg!.setPointerCapture(e.pointerId); } catch { /* the pointer may already be gone */ }
      carry.moved = true;
      moveRecord(carry.id, carry.px + dx / scale, carry.py + dy / scale);
      return;
    }
    if (drag) { panX = drag.px + e.clientX - drag.x; panY = drag.py + e.clientY - drag.y; transform(); }
  });
  const lift = (e: PointerEvent) => {
    touches.delete(e.pointerId); if (touches.size < 2) pinch = 0; drag = null;
    if (carry?.moved) { carried = true; setTimeout(() => { carried = false; }); }
    carry = null;
  };
  svg?.addEventListener("pointerup", lift);
  svg?.addEventListener("pointercancel", lift);
  // A wheel or a trackpad pinch (which arrives as a wheel with ctrlKey) zooms toward the pointer.
  svg?.addEventListener("wheel", e => {
    e.preventDefault();
    const p = local(e);
    zoom(Math.exp(-e.deltaY * (e.ctrlKey ? .01 : .0015)), p.x, p.y);
  }, { passive: false });
  const highlight = (id: string) => {
    const neighbours = new Set([id]);
    for (const e of edges) if (e.source === id || e.target === id) { neighbours.add(e.source); neighbours.add(e.target); }
    host.querySelectorAll<SVGElement>("[data-node]").forEach(el => el.classList.toggle("memoryDim", !!id && !neighbours.has(el.dataset.node!)));
    host.querySelectorAll<SVGElement>("[data-edge-kind]").forEach(el => el.classList.toggle("memoryDim", !!id && el.dataset.source !== id && el.dataset.target !== id));
  };
  const choose = (id: string) => openRecord(id, false);
  host.querySelector("#memoryRecord")!.addEventListener("change", e => {
    const id = (e.target as HTMLSelectElement).value, point = positions.get(id);
    if (!point) return;
    scale = 1; panX = (host.clientWidth - cardWidth) / 2 - point.x; panY = 35 - point.y;
    transform(); choose(id);
  });
  host.querySelectorAll<SVGGElement>("[data-node]").forEach(el => {
    const id = el.dataset.node!;
    el.addEventListener("pointerenter", () => highlight(id));
    el.addEventListener("pointerleave", () => highlight(selected));
    el.addEventListener("focus", () => highlight(id));
    el.addEventListener("blur", () => highlight(selected));
    el.addEventListener("click", () => { if (!carried) choose(id); });
    el.addEventListener("keydown", e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); e.stopPropagation(); choose(id); } });
  });
  host.querySelectorAll<SVGElement>(".memoryEdgeLabel").forEach(el => {
    const edge = graph!.edges.find(e => e.id === el.parentElement!.dataset.edge)!;
    const chooseEdge = () => { selected = ""; stack = []; openEdge = edge; highlight(""); renderMemoryDetail(); };
    el.addEventListener("click", chooseEdge);
    el.addEventListener("keydown", e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); e.stopPropagation(); chooseEdge(); } });
  });
  highlightSelection = () => {
    highlight(selected);
    host.querySelectorAll<SVGElement>("[data-node]").forEach(el => el.setAttribute("aria-pressed", String(el.dataset.node === selected)));
  };
  renderMemoryDetail(); highlightSelection();
}

let highlightSelection = () => {};
let zoomBy = (_factor: number) => {}, fitGraph = () => {}, toggleLabels = () => {};
const nodeOf = (id: string) => graph?.nodes.find(n => n.id === id);
const titleOf = (id: string) => nodeOf(id)?.title || id;
/** links are the selected record's Links in stored order, which the detail numbers 1-9. */
const linksOf = (id: string) => graph ? graph.edges.filter(e => e.source === id || e.target === id) : [];

function openRecord(id: string, push: boolean): void {
  if (!nodeOf(id)) return;
  stack = push && stack.length ? [...stack, id] : [id];
  openEdge = null; selected = id;
  renderMemoryDetail(); highlightSelection();
}

function closeMemoryDetail(): void {
  if (selected || openEdge) closed = { stack, edge: openEdge };
  stack = []; openEdge = null; selected = "";
  renderMemoryDetail(); highlightSelection();
}

/** reopenMemoryDetail shows the detail last closed, if its record or Link is still in the graph. */
function reopenMemoryDetail(): void {
  const edge = closed?.edge && graph?.edges.find(e => e.id === closed!.edge!.id);
  const ids = closed?.stack.filter(id => nodeOf(id)) || [];
  if (edge) { openEdge = edge; renderMemoryDetail(); highlightSelection(); return; }
  if (!ids.length) { toast("Select a record or a Link to open its detail"); return; }
  stack = ids; selected = ids[ids.length - 1]; openEdge = null;
  renderMemoryDetail(); highlightSelection();
}

/** memoryBack returns to the record shown before the last followed Link; false when there is none. */
export function memoryBack(): boolean {
  if (S.view !== "memory" || stack.length < 2) return false;
  stack.pop(); selected = stack[stack.length - 1];
  renderMemoryDetail(); highlightSelection();
  return true;
}

/** follow opens the far end of the n-th numbered row, as 1-9 open children in the tree detail. */
function follow(n: number): void {
  const targets = openEdge ? [openEdge.source, openEdge.target] : linksOf(selected).map(e => e.source === selected ? e.target : e.source);
  const to = targets[n - 1];
  if (to) openRecord(to, true); else toast(`No Link ${n}`);
}

/**
 * memoryKey zooms the graph with + and -, fits it with f, hides the Link labels
 * with l, and shows or hides the detail with d, which brings back the last one
 * closed. The open detail takes the tree detail's keys: \ sizes it, 1-9 follow
 * a numbered row, n and p step through records, c copies, Esc closes.
 */
export function memoryKey(k: string): boolean {
  if (!graph) return false;
  switch (k) {
    case "+": case "=": zoomBy(1.25); return true;
    case "-": zoomBy(.8); return true;
    case "f": fitGraph(); return true;
    case "l": toggleLabels(); return true;
    case "d":
      if (selected || openEdge) closeMemoryDetail();
      else reopenMemoryDetail();
      return true;
  }
  if (!(selected || openEdge)) return false;
  if (/^[1-9]$/.test(k)) { follow(+k); return true; }
  switch (k) {
    case "\\": size = size === "full" ? "half" : "full"; renderMemoryDetail(); return true;
    case "Escape": closeMemoryDetail(); return true;
    case "n": case "p": {
      const ids = graph.nodes.map(n => n.id), at = ids.indexOf(selected);
      const next = ids[at < 0 ? 0 : at + (k === "n" ? 1 : -1)];
      if (next) openRecord(next, false);
      return true;
    }
    case "c": {
      const text = openEdge ? `${openEdge.id} ${openEdge.type}` : `${selected} ${titleOf(selected)}`;
      void copyText(text).then(ok => toast(ok ? "Copied " + (openEdge ? openEdge.id : selected) : "Clipboard refused: " + text));
      return true;
    }
  }
  return false;
}

function relRow(id: string, n: number, lead: string, edge?: Edge): string {
  const node = nodeOf(id);
  return `<button class="rel" data-mnav="${esc(id)}"${edge?.note ? ` title="${esc(edge.note)}"` : ""}>${n <= 9 ? `<span class="kn">${n}</span>` : `<span class="kn"></span>`}<span class="id"${edge ? ` style="color:${color(edge.kind)}"` : ""}>${lead}</span><span class="t">${esc(node?.title || id)}</span></button>`;
}

const kindPill = (kind: string) => kind === "issue" ? `<span class="pill" style="color:var(--orange)">○ issue</span>` : `<span class="pill" style="color:var(--cyan)">◇ ${esc(kind)}</span>`;

/** renderMemoryDetail draws the open record or Link in the tree detail's panel, so both read and size alike. */
export function renderMemoryDetail(): void {
  const host = document.getElementById("detailHost")!, app = document.getElementById("app")!;
  const node = selected ? nodeOf(selected) : undefined;
  const open = S.view === "memory" && !!graph && !!(node || openEdge);
  app.classList.toggle("hasdetail", open);
  if (!open) { host.innerHTML = ""; return; }
  const did = openEdge ? "link:" + openEdge.id : node!.id;
  const existing = host.querySelector<HTMLElement>("#memoryDetail");
  const scrollTop = existing && existing.dataset.id === did ? existing.querySelector(".dbody")!.scrollTop : 0;
  const full = size === "full";
  const glyph = wide() ? (full ? ["⤡", "Side panel"] : ["⤢", "Full width"]) : (full ? ["⌄", "Half height"] : ["⌃", "Full height"]);
  const ids = graph!.nodes.map(n => n.id), pos = ids.indexOf(selected);
  const back = stack.length > 1 ? `<button class="back" data-mact="back">‹ ${esc(stack[stack.length - 2])}</button>` : "";
  const pager = node ? `<button data-mact="prev" aria-label="Previous record">‹</button><span class="pager">${pos + 1}/${ids.length}</span><button data-mact="next" aria-label="Next record">›</button>` : "";
  let head: string, body: string;
  if (node) {
    const links = linksOf(node.id);
    head = `<div class="dtitle">${esc(node.title)}</div><div class="pills">${kindPill(node.kind)}${node.kind === "issue" && node.status ? `<span class="pill" style="color:var(--green)">${esc(node.status)}</span>` : ""}</div>`;
    body = `<dl class="meta"><dt>type</dt><dd>${esc(node.type)}</dd><dt>version</dt><dd>${esc(node.record_version || "—")}</dd></dl>
      <div class="dsec">Links · ${links.length}</div>${links.map((e, n) => {
        const out = e.source === node.id;
        return relRow(out ? e.target : e.source, n + 1, `${out ? "→" : "←"} ${esc(label(e.kind))}`, e);
      }).join("") || '<div class="md"><p style="color:var(--muted)">No Links recorded.</p></div>'}
      <div class="dsec">${node.kind === "issue" ? "Description" : "Body"}</div><div class="md">${md(node.body || "No body.")}</div>`;
  } else {
    const e = openEdge!;
    head = `<div class="dtitle">Link · ${esc(label(e.kind))}</div><div class="pills"><span class="pill" style="color:${color(e.kind)}">${esc(label(e.kind))}</span></div>`;
    body = `<dl class="meta"><dt>type</dt><dd>${esc(e.type)}</dd>${e.note ? `<dt>note</dt><dd>${esc(e.note)}</dd>` : ""}</dl>
      <div class="dsec">From</div>${relRow(e.source, 1, "", undefined)}<div class="dsec">To</div>${relRow(e.target, 2, "", undefined)}
      ${e.kind === "blocks" ? '<div class="md"><p>The source depends on the target: the target blocks the source.</p></div>' : ""}`;
  }
  const inTree = node && node.kind === "issue" && get(node.id) ? `<div class="dacts"><button data-mact="tree"><span class="g">≣</span>Show in tree</button></div>` : "";
  host.innerHTML = `<section class="detail ${existing ? "" : "enter"} ${full ? "full" : ""}" id="memoryDetail" data-id="${esc(did)}" style="--h:${wide() ? 0 : full ? fullH() : halfH()}px" aria-label="Memory detail" role="dialog">
    <div class="dhead"><div class="grab"></div>
      <div class="dtop">${back}<span class="id">${esc(openEdge ? openEdge.id : node!.id)}</span><span class="sp"></span>${pager}<button data-mact="size" aria-label="${glyph[1]}" title="${glyph[1]} (\\)">${glyph[0]}</button><button id="memoryClose" data-mact="close" aria-label="Close Memory details">✕</button></div>
      ${head}
    </div>
    <div class="dbody">${body}</div>${inTree}
  </section>`;
  const sec = host.querySelector<HTMLElement>("#memoryDetail")!;
  renderMermaidBlocks(sec);
  sec.querySelector(".dbody")!.scrollTop = scrollTop;
  sec.addEventListener("click", ev => {
    const t = ev.target as HTMLElement;
    const nav = t.closest<HTMLElement>("[data-mnav]");
    if (nav) { openRecord(nav.dataset.mnav!, true); return; }
    switch (t.closest<HTMLElement>("[data-mact]")?.dataset.mact) {
      case "back": stack.length > 1 && memoryBack(); break;
      case "close": closeMemoryDetail(); break;
      case "size": memoryKey("\\"); break;
      case "prev": memoryKey("p"); break;
      case "next": memoryKey("n"); break;
      case "tree": { const id = node!.id; S.view = "tree"; openDetail(id); break; }
    }
  });
  if (!existing) requestAnimationFrame(() => requestAnimationFrame(() => sec.classList.remove("enter")));
}
