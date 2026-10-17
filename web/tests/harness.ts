// One disposable project per test: a JSONL fixture, a fake bd on PATH and a
// real `b9s web --no-token` on a free loopback port. Nothing here can reach
// a Beads database: the fixture has no metadata.json, so b9s reads JSONL.

import { test as base, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Resolved now: the server starts in a temp project directory.
export const BIN = path.resolve(process.env.B9S_WEB_BIN || path.resolve(__dirname, "../../.b9s-e2e/b9s"));
const FAKE_BD = path.resolve(__dirname, "fake-bd.mjs");

export interface FixtureIssue {
  id: string; title: string; status?: string; priority?: number; type?: string; assignee?: string;
  parent?: string; blockedBy?: string[]; labels?: string[]; description?: string; comments?: string[];
  updatedMinutesAgo?: number;
}

export function toJSONL(list: FixtureIssue[]): string {
  const base = Date.now();
  return list.map((f, n) => {
    const t = new Date(base - (f.updatedMinutesAgo ?? n) * 60000).toISOString();
    const deps = [
      ...(f.parent ? [{ issue_id: f.id, depends_on_id: f.parent, type: "parent-child", created_at: t, created_by: "fixture" }] : []),
      ...(f.blockedBy || []).map(b => ({ issue_id: f.id, depends_on_id: b, type: "blocks", created_at: t, created_by: "fixture" })),
    ];
    return JSON.stringify({
      id: f.id, title: f.title, description: f.description || "", status: f.status || "open", priority: f.priority ?? 2,
      issue_type: f.type || "task", assignee: f.assignee || "", created_at: t, updated_at: t,
      ...(f.status === "closed" ? { closed_at: t } : {}), labels: f.labels || [], dependencies: deps,
      comments: (f.comments || []).map((c, k) => ({ id: `${f.id}-c${k}`, issue_id: f.id, author: "alice", text: c, created_at: t })),
    });
  }).join("\n") + "\n";
}

export const SAMPLE: FixtureIssue[] = [
  { id: "t-epic", title: "[epic] Epic: Mobile web", type: "epic", priority: 1 },
  { id: "t-1", title: "[1] Write the API", parent: "t-epic", status: "in_progress", assignee: "alice", priority: 1, description: "Serve **JSON** to the phone.", comments: ["Started on the handlers."] },
  { id: "t-2", title: "[2] Draw the tree", parent: "t-epic", priority: 2, labels: ["ui"] },
  { id: "t-3", title: "[3] Board drag", parent: "t-epic", blockedBy: ["t-1"], priority: 2 },
  { id: "t-4", title: "[4] Old bug", type: "bug", status: "closed", priority: 3 },
  { id: "t-5", title: "[5] Loose chore", type: "chore", priority: 3 },
];

export interface Project {
  dir: string; url: string; proc: ChildProcess; log: () => string;
  restart(renewSecret?: boolean): Promise<void>;
  /** pairLink is the "This machine" link from the banner; empty with --no-token. */
  pairLink(): string;
  issues(): Record<string, unknown>[];
  calls(): string[][];
  writeIssues(list: FixtureIssue[]): void;
}

export async function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const s = net.createServer();
    s.listen(0, "127.0.0.1", () => { const p = (s.address() as net.AddressInfo).port; s.close(() => resolve(p)); });
    s.on("error", reject);
  });
}

/**
 * waitFor reports whether proc is serving url, or false when proc exited
 * first. A health answer alone proves nothing: another test's server may hold
 * the port. b9s web prints its banner only once it has bound the port.
 */
async function waitFor(proc: ChildProcess, url: string, deadlineMs: number, log: () => string): Promise<boolean> {
  const deadline = Date.now() + deadlineMs;
  while (Date.now() < deadline) {
    if (proc.exitCode !== null || proc.signalCode !== null) return false;
    if (log().includes("b9s web is serving")) {
      try { const r = await fetch(url + "api/health", { signal: AbortSignal.timeout(1000) }); if (r.ok || r.status === 401) return true; } catch { /* not up yet */ }
    }
    await new Promise(r => setTimeout(r, 100));
  }
  proc.kill("SIGTERM");
  throw new Error(`b9s web did not start within ${deadlineMs} ms:\n${log()}`);
}

export async function startProject(list: FixtureIssue[], env: Record<string, string> = {}, paired = false, nextPort: () => Promise<number> = freePort, args: string[] = [], projectDir = ""): Promise<Project> {
  const dir = projectDir || fs.mkdtempSync(path.join(os.tmpdir(), "b9s-web-e2e-"));
  fs.mkdirSync(dir, { recursive: true });
  const beads = path.join(dir, ".beads");
  fs.mkdirSync(beads);
  fs.writeFileSync(path.join(beads, "issues.jsonl"), toJSONL(list));
  const bin = path.join(dir, "bin");
  fs.mkdirSync(bin);
  fs.writeFileSync(path.join(bin, "bd"), `#!/bin/sh\nexec "${process.execPath}" "${FAKE_BD}" "$@"\n`, { mode: 0o755 });
  let out = "", url = "", proc: ChildProcess | undefined;
  const log = () => out;
  const launch = (port: number, renewSecret = false): ChildProcess => {
    const p = spawn(BIN, ["web", ...(paired ? [] : ["--no-token"]), "--listen", `127.0.0.1:${port}`, ...args, ...(renewSecret ? ["--new-token"] : [])], {
      cwd: dir,
      env: {
        ...process.env, ...env,
        PATH: `${bin}:${process.env.PATH}`, XDG_CONFIG_HOME: path.join(dir, "config"), B9S_TEST_MODE: "1",
        BEADS_DIR: "", BEADS_DOLT_PASSWORD: "",
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    p.stdout!.on("data", d => { out += d; });
    p.stderr!.on("data", d => { out += d; });
    return p;
  };
  // The probed port is free only until the probe closes, so a parallel test
  // can bind it first. Each attempt takes a fresh port.
  for (let attempt = 0; attempt < 5 && !proc; attempt++) {
    const port = await nextPort();
    out = "";
    const p = launch(port);
    url = `http://127.0.0.1:${port}/`;
    if (await waitFor(p, url, 15000, log)) proc = p;
  }
  if (!proc) throw new Error(`b9s web could not bind a port in 5 attempts:\n${out}`);
  return {
    dir, url, get proc() { return proc!; }, log,
    restart: async (renewSecret = false) => {
      const previous = proc!;
      await new Promise<void>((resolve, reject) => {
        const timer = setTimeout(() => {
          previous.kill("SIGKILL");
          reject(new Error("b9s web did not stop within 5000 ms"));
        }, 5000);
        previous.once("exit", () => { clearTimeout(timer); resolve(); });
        previous.kill("SIGTERM");
      });
      out = "";
      proc = launch(Number(new URL(url).port), renewSecret);
      if (!await waitFor(proc, url, 15000, log)) throw new Error(`b9s web exited during restart:\n${out}`);
    },
    pairLink: () => (out.match(/This machine\s+(\S+\/pair\?t=\S+)/) || ["", ""])[1],
    issues: () => fs.readFileSync(path.join(beads, "issues.jsonl"), "utf8").split("\n").filter(Boolean).map(l => JSON.parse(l)),
    calls: () => { try { return fs.readFileSync(path.join(beads, "bd-calls.log"), "utf8").split("\n").filter(Boolean).map(l => JSON.parse(l)); } catch { return []; } },
    writeIssues: l => { const f = path.join(beads, "issues.jsonl"); fs.writeFileSync(f + ".tmp", toJSONL(l)); fs.renameSync(f + ".tmp", f); },
  };
}

export function stopProject(p: Project): void {
  p.proc.kill("SIGTERM");
  if (!process.env.B9S_KEEP_TEST_FIXTURES) {
    try { fs.rmSync(p.dir, { recursive: true, force: true }); } catch { /* temp dir */ }
  }
}

/**
 * cpuThrottle is B9S_E2E_CPU_THROTTLE, the factor by which a Chromium page
 * runs slower than this machine (Emulation.setCPUThrottlingRate). CI runners
 * are slower than a laptop, so a race between the app's timers and the keys a
 * test presses can fail there only; docs/testing.md says how to bring it here.
 */
const cpuThrottle = Number(process.env.B9S_E2E_CPU_THROTTLE) || 0;

// The fixture list is wrapped in an object: Playwright reads a bare array option as a [value, options] tuple.
export const test = base.extend<{ project: Project; fixture: { issues: FixtureIssue[] }; bdEnv: Record<string, string>; paired: boolean }>({
  page: async ({ page, browserName }, use) => {
    if (cpuThrottle > 1 && browserName === "chromium") {
      await (await page.context().newCDPSession(page)).send("Emulation.setCPUThrottlingRate", { rate: cpuThrottle });
    }
    await use(page);
  },
  fixture: [{ issues: SAMPLE }, { option: true }],
  bdEnv: [{}, { option: true }],
  paired: [false, { option: true }],
  project: async ({ fixture, bdEnv, paired }, use, info) => {
    const p = await startProject(fixture.issues, bdEnv, paired);
    await use(p);
    if (info.status !== info.expectedStatus) {
      await info.attach("bd-calls", { body: p.calls().map(c => c.join(" ")).join("\n") || "(none)", contentType: "text/plain" });
      await info.attach("b9s-web-log", { body: p.log() || "(empty)", contentType: "text/plain" });
    }
    stopProject(p);
  },
});
export { expect };

/**
 * settled resolves once the first snapshot has rendered (#boot is gone) and
 * the live stream has connected (body carries data-live="live"). Call it
 * after every page.goto and page.reload. It reads two scalars in the page
 * through one waitForFunction rather than through locator assertions: an
 * assertion whose element is not on the page yet snapshots the whole ARIA
 * tree on every poll, and on a loaded runner that costs the app the CPU its
 * own timers need between the test's steps (bd-zoom.1.4).
 */
export async function settled(page: Page): Promise<void> {
  await page.waitForFunction(() => {
    const boot = document.getElementById("boot");
    const bootGone = !boot || boot.hidden || boot.getClientRects().length === 0;
    return bootGone && document.body.dataset.live === "live";
  }, undefined, { timeout: 10000 });
}

/**
 * open loads the app and waits until the first snapshot has rendered and the
 * live stream has connected. The connect renders the tree again, so a row
 * found before it may be detached by the time a test measures it.
 */
export async function open(page: Page, p: Project): Promise<void> {
  await page.goto(p.url);
  await settled(page);
}

/** The load waiter must be armed before the action that can replace the document. */
export async function reloadAndSettle(page: Page, trigger: () => unknown | Promise<unknown>): Promise<void> {
  await Promise.all([
    page.waitForEvent("load", { timeout: 10000 }),
    Promise.resolve().then(trigger),
  ]);
  await settled(page);
}

export const row = (page: Page, id: string) => page.locator(`#treeRows .row[data-id="${id}"]`);

/** swipe drags a row horizontally by a fraction of its width, with real pointer events. */
export async function swipe(page: Page, id: string, fraction: number): Promise<void> {
  const box = (await row(page, id).boundingBox())!;
  const y = box.y + box.height / 2, x0 = box.x + box.width / 2 - (fraction > 0 ? box.width * 0.3 : -box.width * 0.3);
  const x1 = x0 + fraction * box.width;
  await page.mouse.move(x0, y);
  await page.mouse.down();
  const steps = 12;
  for (let s = 1; s <= steps; s++) await page.mouse.move(x0 + ((x1 - x0) * s) / steps, y);
  await page.mouse.up();
}

export async function longPress(page: Page, sel: string): Promise<void> {
  const box = (await page.locator(sel).first().boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.waitForTimeout(600);
  await page.mouse.up();
}
