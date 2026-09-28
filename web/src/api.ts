// The only module that talks to b9s web. Every URL is relative, so the app
// works behind a path prefix such as the one Tailscale Serve adds.

import type { Health, Issue, ProjectInfo, ProjectList, QueryResult, Session, Snapshot, WriteRequest, WriteResult } from "./api.gen";
import { uid } from "./util";

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

let csrf = "";

async function parse<T>(res: Response): Promise<T> {
  const text = await res.text();
  let body: unknown = null;
  try { body = text ? JSON.parse(text) : null; } catch { /* plain text errors */ }
  if (!res.ok && !(res.status === 422 && body)) {
    const msg = body && typeof body === "object" && "error" in body ? String((body as { error: unknown }).error) : text.trim() || res.statusText;
    throw new ApiError(res.status, msg);
  }
  return body as T;
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin", cache: "no-store" });
  return parse<T>(res);
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST", credentials: "same-origin", cache: "no-store",
    headers: { "Content-Type": "application/json", "X-B9s-CSRF": csrf },
    body: JSON.stringify(body),
  });
  return parse<T>(res);
}

export async function session(): Promise<Session> {
  const s = await get<Session>("api/session");
  csrf = s.csrf;
  return s;
}

export const snapshot = () => get<Snapshot>("api/snapshot");
export const issue = (id: string) => get<Issue>("api/issue?id=" + encodeURIComponent(id));
export const query = (q: string) => get<QueryResult>("api/query?q=" + encodeURIComponent(q));
export const health = () => get<Health>("api/health");
export const projects = () => get<ProjectList>("api/projects");
export const openProject = (key: string) => post<ProjectInfo>("api/projects/open", { key });
export const reload = () => post<Health>("api/reload", {});

/**
 * write runs one bd command on the server. The idempotency key makes a retry
 * after a dropped response return the first result instead of running twice.
 */
export async function write(req: WriteRequest): Promise<WriteResult> {
  const body: WriteRequest = { ...req, key: req.key || uid() };
  try {
    return await post<WriteResult>("api/write", body);
  } catch (e) {
    if (e instanceof ApiError) return { ok: false, ids: req.ids || [], created: "", error: e.message, output: "", version: 0 };
    // A network error may hide a write that ran: retry once with the same key.
    try { return await post<WriteResult>("api/write", body); } catch (e2) {
      return { ok: false, ids: req.ids || [], created: "", error: e2 instanceof Error ? e2.message : String(e2), output: "", version: 0 };
    }
  }
}

export interface LiveHandlers {
  onEvent(type: string, version: number): void;
  onState(state: "live" | "stale", retryIn: number): void;
}

/**
 * live keeps one EventSource open. EventSource retries on its own, but with
 * no backoff and no signal the page can show, so this closes it on error and
 * reconnects after 1, 2, 4 … 30 seconds.
 */
export function live(h: LiveHandlers): { retryNow(): void } {
  let es: EventSource | null = null;
  let delay = 1000;
  let timer = 0;
  const connect = () => {
    clearTimeout(timer);
    if (es) es.close();
    es = new EventSource("api/events");
    const on = (type: string) => es!.addEventListener(type, ev => {
      let v = 0;
      try { v = JSON.parse((ev as MessageEvent).data).version || 0; } catch { /* keep 0 */ }
      if (type === "hello") { delay = 1000; h.onState("live", 0); }
      h.onEvent(type, v);
    });
    ["hello", "changed", "project", "health"].forEach(on);
    es.onerror = () => {
      es?.close(); es = null;
      h.onState("stale", delay);
      timer = window.setTimeout(connect, delay);
      delay = Math.min(delay * 2, 30000);
    };
  };
  connect();
  document.addEventListener("visibilitychange", () => { if (!document.hidden && !es) connect(); });
  return { retryNow: () => { delay = 1000; connect(); } };
}
