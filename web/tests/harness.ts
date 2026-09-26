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

function toJSONL(list: FixtureIssue[]): string {
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
  /** pairLink is the "This machine" link from the banner; empty with --no-token. */
  pairLink(): string;
  issues(): Record<string, unknown>[];
  calls(): string[][];
  writeIssues(list: FixtureIssue[]): void;
}

async function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const s = net.createServer();
    s.listen(0, "127.0.0.1", () => { const p = (s.address() as net.AddressInfo).port; s.close(() => resolve(p)); });
    s.on("error", reject);
  });
}

async function waitFor(url: string, deadlineMs: number, log: () => string): Promise<void> {
  const deadline = Date.now() + deadlineMs;
  while (Date.now() < deadline) {
    try { const r = await fetch(url + "api/health"); if (r.ok || r.status === 401) return; } catch { /* not up yet */ }
    await new Promise(r => setTimeout(r, 100));
  }
  throw new Error(`b9s web did not start within ${deadlineMs} ms:\n${log()}`);
}

export async function startProject(list: FixtureIssue[], env: Record<string, string> = {}, paired = false): Promise<Project> {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "b9s-web-e2e-"));
  const beads = path.join(dir, ".beads");
  fs.mkdirSync(beads);
  fs.writeFileSync(path.join(beads, "issues.jsonl"), toJSONL(list));
  const bin = path.join(dir, "bin");
  fs.mkdirSync(bin);
  fs.writeFileSync(path.join(bin, "bd"), `#!/bin/sh\nexec "${process.execPath}" "${FAKE_BD}" "$@"\n`, { mode: 0o755 });
  const port = await freePort();
  let out = "";
  const proc = spawn(BIN, ["web", ...(paired ? [] : ["--no-token"]), "--listen", `127.0.0.1:${port}`], {
    cwd: dir,
    env: {
      ...process.env, ...env,
      PATH: `${bin}:${process.env.PATH}`, XDG_CONFIG_HOME: path.join(dir, "config"), B9S_TEST_MODE: "1",
      BEADS_DIR: "", BEADS_DOLT_PASSWORD: "",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  proc.stdout!.on("data", d => { out += d; });
  proc.stderr!.on("data", d => { out += d; });
  const url = `http://127.0.0.1:${port}/`;
  const log = () => out;
  await waitFor(url, 15000, log);
  return {
    dir, url, proc, log,
    pairLink: () => (out.match(/This machine\s+(\S+\/pair\?t=\S+)/) || ["", ""])[1],
    issues: () => fs.readFileSync(path.join(beads, "issues.jsonl"), "utf8").split("\n").filter(Boolean).map(l => JSON.parse(l)),
    calls: () => { try { return fs.readFileSync(path.join(beads, "bd-calls.log"), "utf8").split("\n").filter(Boolean).map(l => JSON.parse(l)); } catch { return []; } },
    writeIssues: l => { const f = path.join(beads, "issues.jsonl"); fs.writeFileSync(f + ".tmp", toJSONL(l)); fs.renameSync(f + ".tmp", f); },
  };
}

export function stopProject(p: Project): void {
  p.proc.kill("SIGTERM");
  try { fs.rmSync(p.dir, { recursive: true, force: true }); } catch { /* temp dir */ }
}

// The fixture list is wrapped in an object: Playwright reads a bare array option as a [value, options] tuple.
export const test = base.extend<{ project: Project; fixture: { issues: FixtureIssue[] }; bdEnv: Record<string, string>; paired: boolean }>({
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

/** open loads the app and waits until the first snapshot has rendered. */
export async function open(page: Page, p: Project): Promise<void> {
  await page.goto(p.url);
  await expect(page.locator("#boot")).toBeHidden({ timeout: 10000 });
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
