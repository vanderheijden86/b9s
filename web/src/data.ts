// The issue snapshot and the indexes every view reads. The server owns the
// data; this module only indexes what the last snapshot said.

import type { Health, Issue, Person, ProjectInfo, Snapshot } from "./api.gen";
import { parseTime } from "./util";

export interface Item extends Issue {
  /** title without the "[short-id] " prefix Beads titles carry */
  t: string;
  upd: number;
  crt: number;
  cls: number;
}

export const D = {
  version: 0,
  project: { name: "", key: "", read_only: false, all: false } as ProjectInfo,
  actor: "",
  people: [] as Person[],
  health: null as Health | null,
  all: new Map<string, Item>(),
  kids: new Map<string, string[]>(),
  blocks: new Map<string, string[]>(),
  loaded: false,
};

/** load replaces the data with a snapshot and returns the IDs it changed. */
export function load(snap: Snapshot): Set<string> {
  const changed = new Set<string>();
  const prev = D.all;
  const all = new Map<string, Item>();
  for (const src of snap.issues) {
    const i = src as Item;
    i.t = String(i.title || "").replace(/^\[[^\]]+\]\s*/, "");
    i.upd = parseTime(i.updated_at);
    i.crt = parseTime(i.created_at);
    i.cls = parseTime(i.closed_at);
    const old = prev.get(i.id);
    if (!old || old.updated_at !== i.updated_at || old.status !== i.status || old.comment_count !== i.comment_count || old.blocked !== i.blocked) changed.add(i.id);
    all.set(i.id, i);
  }
  D.all = all;
  D.version = snap.version;
  D.project = snap.project;
  D.actor = snap.actor;
  D.people = snap.people || [];
  D.health = snap.health;
  D.loaded = true;
  reindex();
  return changed;
}

export function reindex(): void {
  const kids = new Map<string, string[]>(), blocks = new Map<string, string[]>();
  for (const i of D.all.values()) {
    if (i.parent && D.all.has(i.parent)) {
      const k = kids.get(i.parent);
      if (k) k.push(i.id); else kids.set(i.parent, [i.id]);
    }
    for (const b of i.blocked_by) {
      const k = blocks.get(b);
      if (k) k.push(i.id); else blocks.set(b, [i.id]);
    }
  }
  D.kids = kids;
  D.blocks = blocks;
}

export const get = (id: string): Item | undefined => D.all.get(id);
export const kids = (id: string): string[] => D.kids.get(id) || [];
export const pool = (): Item[] => [...D.all.values()];

export function descendants(id: string, out: string[] = [], seen = new Set<string>()): string[] {
  for (const k of kids(id)) {
    if (seen.has(k)) continue;
    seen.add(k); out.push(k); descendants(k, out, seen);
  }
  return out;
}

export function ancestors(id: string): string[] {
  const out: string[] = [];
  let p = get(id)?.parent;
  while (p && D.all.has(p) && !out.includes(p)) { out.push(p); p = get(p)?.parent; }
  return out;
}

/**
 * eff is the status a row shows: closed-like statuses read as closed, an
 * issue with open blockers reads as blocked, the rest show what bd stores.
 */
export function eff(i: Item): string {
  if (i.closed_like) return "closed";
  if (i.status === "deferred") return "deferred";
  if (i.blocked) return "blocked";
  return i.status;
}

export const isReady = (i: Item): boolean => i.status === "open" && i.ready;
export const blocksOf = (i: Item): Item[] => (D.blocks.get(i.id) || []).map(get).filter((x): x is Item => !!x);
export const openBlockers = (i: Item): string[] => i.blocked_by.filter(b => { const x = get(b); return !!x && !x.closed_like; });

/** shortId drops the repository prefix, as issue titles do: bd-foit.3 → foit.3. */
export function shortId(id: string): string {
  const base = id.split(".")[0];
  const k = base.lastIndexOf("-");
  return k < 0 ? id : id.slice(k + 1);
}

export function progress(id: string): { done: number; all: number } | null {
  const k = kids(id).map(get).filter((x): x is Item => !!x);
  if (!k.length) return null;
  return { done: k.filter(x => x.closed_like).length, all: k.length };
}

/** laneOf is the topmost epic above an issue, which names its board lane. */
export function laneOf(i: Item): string | null {
  const chain = [i.id, ...ancestors(i.id)];
  for (let n = chain.length - 1; n >= 0; n--) if (get(chain[n])?.type === "epic") return chain[n];
  return null;
}

export const projectOf = (i: Item): string => i.project || D.project.name;
