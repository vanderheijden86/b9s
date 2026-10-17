import type { Page } from "@playwright/test";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { expect, open, row, startProject, stopProject, swipe, test, toJSONL } from "./harness";
import { measureFixture } from "./measure-fixture";

const BIG = measureFixture(1000, "p");

const BUDGET = {
  loadMs: 1500,
  tapFeedbackMs: 100,
  projectSwitchMs: 500,
  writeRoundTripMs: 800,
  liveUpdateMs: 1000,
  fullRenderMs: 250,
  longTaskMs: 0,
};

const SAMPLE_COUNT = 5;
const CI_BUDGET_FACTOR = 1.5;

test.use({ fixture: { issues: BIG } });
test.describe.configure({ mode: "serial" });

const attr = (s: string) => s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
const rowStatusPred = (id: string, status: string) => `document.querySelector("#treeRows .row[data-id=\\"${attr(id)}\\"] .stw")?.textContent === ${JSON.stringify(status)}`;

async function samples(label: string, run: (sample: number) => Promise<number>, count = SAMPLE_COUNT): Promise<number[]> {
  const values: number[] = [];
  for (let n = 0; n < count; n++) {
    const value = await run(n);
    if (!Number.isFinite(value)) throw new Error(`${label} sample ${n + 1} is not finite: ${value}`);
    values.push(value);
  }
  return values;
}

function median(values: number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

function rounded(values: number[]): string {
  return values.map(v => String(Math.round(v))).join(", ");
}

function expectMedianWithinBudget(label: string, values: number[], budget: number, browserName: string): number {
  const value = median(values);
  // CI WebKit is not a real iPhone; the operator decided on 2026-10-12 to record it without enforcing.
  const skipsCIWebKitEnforcement = !!process.env.CI && browserName === "webkit";
  const usesCIAllowance = !skipsCIWebKitEnforcement && value > budget && !!process.env.CI;
  // The raw budgets stay fixed. CI gets a narrow allowance only after the
  // median misses the local budget, which filters slow-runner outliers.
  const limit = usesCIAllowance ? budget * CI_BUDGET_FACTOR : budget;
  const allowance = usesCIAllowance ? `, CI allowance ${CI_BUDGET_FACTOR}x` : "";
  const enforcement = skipsCIWebKitEnforcement ? ", not enforced on WebKit in CI" : "";
  test.info().annotations.push({ type: "perf", description: `${label} median ${Math.round(value)} ms from [${rounded(values)}]${allowance}${enforcement}` });
  if (!skipsCIWebKitEnforcement) {
    expect(value, `${label} median ${Math.round(value)} ms from samples [${rounded(values)}]`).toBeLessThanOrEqual(limit);
  }
  return value;
}

async function feedbackMs(page: Page, predicate: string, action: () => Promise<void>, timeoutMs = 5000, inputMode: "first" | "last" = "first"): Promise<number> {
  await page.evaluate(([predicate, timeoutMs, inputMode]) => {
    type PerfWindow = Window & { __b9sPerfWait?: Promise<number> };
    const w = window as PerfWindow;
    w.__b9sPerfWait = new Promise((resolve, reject) => {
      const test = new Function(`return (${predicate});`) as () => boolean;
      let input = NaN, done = false;
      let mo: MutationObserver | null = null;
      const types = ["pointerup", "touchend", "click", "keydown"];
      const cleanup = () => {
        window.clearTimeout(timer);
        mo?.disconnect();
        types.forEach(t => window.removeEventListener(t, onInput, true));
      };
      const finish = () => {
        if (done || !Number.isFinite(input)) return;
        done = true;
        cleanup();
        resolve(performance.now() - input);
      };
      const check = () => {
        if (!Number.isFinite(input)) return;
        try { if (test()) finish(); } catch { /* not ready */ }
      };
      const onInput = (e: Event) => {
        if (inputMode === "first" && Number.isFinite(input)) return;
        input = e.timeStamp || performance.now();
        setTimeout(check, 0);
      };
      const timer = window.setTimeout(() => { cleanup(); reject(new Error(`feedback budget predicate did not match within ${timeoutMs} ms`)); }, timeoutMs);
      types.forEach(t => window.addEventListener(t, onInput, true));
      mo = new MutationObserver(check);
      mo.observe(document.documentElement, { subtree: true, childList: true, attributes: true, characterData: true });
      check();
    });
  }, [predicate, timeoutMs, inputMode] as const);
  await action();
  return page.evaluate(() => (window as Window & { __b9sPerfWait: Promise<number> }).__b9sPerfWait);
}

async function firstOpenLeaf(page: Page, exclude: string[] = []): Promise<string> {
  const deadline = Date.now() + 5000;
  let lastID = "";
  while (Date.now() < deadline) {
    const id = await page.evaluate(exclude => {
      const skip = new Set(exclude);
      for (const r of document.querySelectorAll<HTMLElement>("#treeRows .row")) {
        const id = r.dataset.id || "";
        if (skip.has(id)) continue;
        if (r.querySelector(".stw")?.textContent === "open" && r.querySelector(".chev")?.textContent === "·") return id;
      }
      return "";
    }, exclude);
    lastID = id;
    if (!id) {
      await page.waitForTimeout(20);
      continue;
    }
    try {
      // The row's intrinsic placeholder can precede layout of its contents.
      const content = row(page, id).locator(".rc");
      await content.evaluate(el => el.scrollIntoView({ block: "nearest", inline: "nearest" }));
      await expect(content).toBeInViewport({ ratio: 1, timeout: 1000 });
      return id;
    } catch {
      await page.waitForTimeout(20);
    }
  }
  expect(lastID).toBeTruthy();
  throw new Error(`open leaf ${lastID} did not become fully visible`);
}

async function waitForFixtureStatus(project: { issues(): Record<string, unknown>[] }, id: string, status: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (project.issues().find(i => i.id === id)?.status === status) return;
    await new Promise(r => setTimeout(r, 20));
  }
  throw new Error(`${id} did not become ${status} within ${timeoutMs} ms`);
}

async function lastWriteFetchMs(page: Page): Promise<number> {
  return page.evaluate(() => {
    const writes = performance.getEntriesByType("resource")
      .filter((e): e is PerformanceResourceTiming => e.name.endsWith("api/write"));
    const rec = writes[writes.length - 1];
    return rec ? rec.responseEnd - rec.startTime : NaN;
  });
}

async function smoothSample(page: Page, action: () => Promise<void>): Promise<{ frames: number; frameP95: number; slowFrames: number; longTaskMs: number }> {
  await page.evaluate(() => {
    const w = window as Window & {
      __b9sFrames?: number[];
      __b9sFrameReq?: number;
      __b9sLongTasks?: number[];
      __b9sLongTaskSampleStart?: number;
      __b9sLongTaskObserver?: PerformanceObserver;
    };
    const sampleStart = performance.now();
    w.__b9sFrames = [];
    w.__b9sLongTasks = [];
    w.__b9sLongTaskSampleStart = sampleStart;
    try {
      w.__b9sLongTaskObserver?.disconnect();
      w.__b9sLongTaskObserver = new PerformanceObserver(list => {
        for (const e of list.getEntries()) {
          if (e.startTime >= sampleStart) w.__b9sLongTasks?.push(e.duration);
        }
      });
      w.__b9sLongTaskObserver.observe({ type: "longtask", buffered: true });
    } catch { /* WebKit has no longtask observer */ }
    const loop = (t: number) => {
      if (!w.__b9sFrames) return;
      w.__b9sFrames.push(t);
      w.__b9sFrameReq = requestAnimationFrame(loop);
    };
    w.__b9sFrameReq = requestAnimationFrame(loop);
  });
  await action();
  await page.waitForTimeout(120);
  return page.evaluate(() => {
    const w = window as Window & {
      __b9sFrames?: number[];
      __b9sFrameReq?: number;
      __b9sLongTasks?: number[];
      __b9sLongTaskSampleStart?: number;
      __b9sLongTaskObserver?: PerformanceObserver;
    };
    if (w.__b9sFrameReq) cancelAnimationFrame(w.__b9sFrameReq);
    const sampleStart = w.__b9sLongTaskSampleStart ?? 0;
    const observer = w.__b9sLongTaskObserver;
    if (observer) {
      for (const e of observer.takeRecords()) {
        if (e.startTime >= sampleStart) w.__b9sLongTasks?.push(e.duration);
      }
      observer.disconnect();
    }
    const frames = w.__b9sFrames || [];
    const longTaskMs = (w.__b9sLongTasks || []).reduce((a, b) => a + b, 0);
    w.__b9sFrames = undefined;
    w.__b9sLongTasks = undefined;
    w.__b9sLongTaskSampleStart = undefined;
    const intervals: number[] = [];
    for (let n = 1; n < frames.length; n++) intervals.push(frames[n] - frames[n - 1]);
    intervals.sort((a, b) => a - b);
    const frameP95 = intervals.length ? intervals[Math.min(intervals.length - 1, Math.ceil(intervals.length * 0.95) - 1)] : 0;
    return { frames: frames.length, frameP95, slowFrames: intervals.filter(x => x > 33.4).length, longTaskMs };
  });
}

type SmoothSample = Awaited<ReturnType<typeof smoothSample>>;

test("1000 issue reads meet load and tap feedback budgets", async ({ page, project, browserName }) => {
  const loadSamples = await samples("load", async () => {
    const t0 = Date.now();
    await open(page, project);
    return Date.now() - t0;
  });
  const loadMs = expectMedianWithinBudget("load", loadSamples, BUDGET.loadMs, browserName);
  await expect.poll(() => page.locator("#treeRows .row").count()).toBeGreaterThan(600);

  const detailId = await firstOpenLeaf(page);
  const detailSamples = await samples("detail feedback", async () => {
    const value = await feedbackMs(page, `!!document.querySelector(".detail[data-id=\\"${attr(detailId)}\\"]")`, () => row(page, detailId).click());
    await page.locator('.detail [aria-label="Close"]').click();
    await expect(page.locator(".detail")).toHaveCount(0);
    return value;
  });
  const detailMs = expectMedianWithinBudget("detail feedback", detailSamples, BUDGET.tapFeedbackMs, browserName);

  const renderSamples = await samples("full render", () => page.evaluate(async () => {
    const chip = document.querySelector<HTMLElement>('[data-chip="closed"]')!;
    const t = performance.now();
    chip.click();
    await new Promise(r => requestAnimationFrame(() => setTimeout(r, 0)));
    return performance.now() - t;
  }));
  expectMedianWithinBudget("full render", renderSamples, BUDGET.fullRenderMs, browserName);
  const chipSamples = await samples("chip feedback", async () => {
    const count = await page.locator("#treeRows .row").count();
    return feedbackMs(page, `document.querySelectorAll("#treeRows .row").length !== ${count}`, () => page.locator('#chips [data-chip="closed"]').click());
  });
  const chipMs = expectMedianWithinBudget("chip feedback", chipSamples, BUDGET.tapFeedbackMs, browserName);

  const boardSamples = await samples("board feedback", async () => {
    await page.locator('#bar [data-view="tree"]').click();
    await expect(page.locator("#vTree")).toBeVisible();
    return feedbackMs(page, `!document.getElementById("vBoard")?.hidden && !!document.querySelector("#vBoard [data-card]")`, () => page.locator('#bar [data-view="board"]').click());
  });
  const boardMs = expectMedianWithinBudget("board feedback", boardSamples, BUDGET.tapFeedbackMs, browserName);
  test.info().annotations.push({ type: "perf", description: `summary load ${Math.round(loadMs)} ms, detail ${Math.round(detailMs)} ms, chip ${Math.round(chipMs)} ms, board ${Math.round(boardMs)} ms` });
});

test("project switch meets the measured budget", async ({ page, browserName }) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "b9s-perf-projects-"));
  const first = path.join(root, "first");
  const second = path.join(root, "second", ".beads");
  fs.mkdirSync(second, { recursive: true });
  fs.writeFileSync(path.join(second, "issues.jsonl"), toJSONL(measureFixture(1000, "q")));
  const project = await startProject(BIG, {}, false, undefined, ["--projects-root", root], first);
  try {
    await open(page, project);
    const switchSamples = await samples("project switch", async () => {
      await page.locator('#hd [data-act="projects"]').click();
      const target = page.locator('.msheet [data-sa="proj"]:not(.cur):not([data-val="*"])').first();
      await expect(target).toBeVisible();
      const name = await target.locator(".x").evaluate(el => el.firstChild?.textContent || "");
      return feedbackMs(
        page,
        `document.querySelector("#hd .proj")?.textContent?.includes(${JSON.stringify(name)}) && !!document.querySelector("#treeRows .row")`,
        () => target.click(),
        5000,
        "last",
      );
    });
    const switchMs = expectMedianWithinBudget("project switch", switchSamples, BUDGET.projectSwitchMs, browserName);
    test.info().annotations.push({ type: "perf", description: `summary project switch ${Math.round(switchMs)} ms` });
  } finally {
    stopProject(project);
  }
});

test("write and live updates meet feedback and round-trip budgets", async ({ page, project, browserName }) => {
  await open(page, project);
  const writeIds: string[] = [];
  const before = await page.locator("#treeList").evaluate(el => el.scrollTop);
  const feedbackSamples: number[] = [];
  const writeFetchSamples: number[] = [];
  for (let n = 0; n < SAMPLE_COUNT; n++) {
    const writeId = await firstOpenLeaf(page, writeIds);
    writeIds.push(writeId);
    feedbackSamples.push(await feedbackMs(page, rowStatusPred(writeId, "in progress"), async () => {
      await swipe(page, writeId, 0.35);
    }));
    await waitForFixtureStatus(project, writeId, "in_progress", 5000);
    await expect(row(page, writeId).locator(".stw")).toHaveText("in progress", { timeout: 5000 });
    const writeFetchMs = await lastWriteFetchMs(page);
    if (Number.isFinite(writeFetchMs)) writeFetchSamples.push(writeFetchMs);
  }
  const feedback = expectMedianWithinBudget("write feedback", feedbackSamples, BUDGET.tapFeedbackMs, browserName);
  if (writeFetchSamples.length) expectMedianWithinBudget("write round trip", writeFetchSamples, BUDGET.writeRoundTripMs, browserName);
  expect(Math.abs((await page.locator("#treeList").evaluate(el => el.scrollTop)) - before)).toBeLessThan(80);

  let current = BIG.map(i => writeIds.includes(i.id) ? { ...i, status: "in_progress" } : i);
  const liveIds: string[] = [];
  const liveSamples: number[] = [];
  for (let n = 0; n < SAMPLE_COUNT; n++) {
    const liveId = await firstOpenLeaf(page, [...writeIds, ...liveIds]);
    liveIds.push(liveId);
    const liveStarted = Date.now();
    current = current.map(i => i.id === liveId ? { ...i, status: "in_progress" } : i);
    project.writeIssues(current);
    await expect(row(page, liveId).locator(".stw")).toHaveText("in progress", { timeout: 5000 });
    liveSamples.push(Date.now() - liveStarted);
  }
  const liveMs = expectMedianWithinBudget("live update", liveSamples, BUDGET.liveUpdateMs, browserName);
  test.info().annotations.push({ type: "perf", description: `write feedback ${Math.round(feedback)} ms, live ${liveMs} ms` });
});

test("scroll and drag stay inside frame budgets", async ({ page, project, browserName }) => {
  await open(page, project);
  const list = page.locator("#treeList");
  await list.hover();
  const scrollSamples: SmoothSample[] = [];
  for (let sample = 0; sample < SAMPLE_COUNT; sample++) scrollSamples.push(await smoothSample(page, async () => {
    for (let n = 0; n < 8; n++) {
      try {
        await page.mouse.wheel(0, 280);
      } catch {
        await list.evaluate(el => { el.scrollTop += 280; });
      }
      await page.waitForTimeout(30);
    }
  }));
  scrollSamples.forEach(scroll => expect(scroll.frames).toBeGreaterThan(4));
  expectMedianWithinBudget("scroll long tasks", scrollSamples.map(s => s.longTaskMs), BUDGET.longTaskMs, browserName);

  const dragSamples: SmoothSample[] = [];
  const dragIds: string[] = [];
  for (let sample = 0; sample < SAMPLE_COUNT; sample++) {
    const dragId = await firstOpenLeaf(page, dragIds);
    dragIds.push(dragId);
    dragSamples.push(await smoothSample(page, () => swipe(page, dragId, 0.15)));
  }
  dragSamples.forEach(drag => expect(drag.frames).toBeGreaterThan(2));
  expectMedianWithinBudget("drag long tasks", dragSamples.map(s => s.longTaskMs), BUDGET.longTaskMs, browserName);
  test.info().annotations.push({ type: "perf", description: `summary scroll p95 ${Math.round(median(scrollSamples.map(s => s.frameP95)))} ms, drag p95 ${Math.round(median(dragSamples.map(s => s.frameP95)))} ms` });
});
