// The fleet views: the agent board, a session's page, the context matrix, the
// constellation, the timeline and the project treemap. The sessions come from
// a throwaway HOME built from internal/agents/testdata, so nothing reads the
// real ~/.claude and no browser, editor or Dolt server is touched.

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, freePort, open, settled, startProject, stopProject, SAMPLE, test } from "./harness";
import type { Page } from "@playwright/test";

const here = path.dirname(fileURLToPath(import.meta.url));
const transcript = fs.readFileSync(path.resolve(here, "../../internal/agents/testdata/claude.jsonl"), "utf8");

/**
 * fixtureHome writes two Claude transcripts shifted so that their last line is
 * a few minutes old, which is what makes a session live, and a context file
 * that changed after the sessions loaded it.
 */
function fixtureHome(): string {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), "b9s-fleet-e2e-"));
  const base = Date.parse("2026-11-15T09:00:00Z");
  const now = Date.now();
  const sessions: [string, string, number][] = [["s-fleet-1", path.join(home, "work", "alpha"), 12], ["s-fleet-2", path.join(home, "work", "beta"), 30]];
  for (const [id, cwd, minutesAgo] of sessions) {
    const body = transcript
      .replaceAll("/work/b9s/.worktrees/feat-x", cwd)
      .replaceAll("s-claude-1", id)
      .replace(/2026-11-15T\d\d:\d\d:\d\d/g, m => new Date(now - minutesAgo * 60000 + (Date.parse(m + "Z") - base)).toISOString().slice(0, 19));
    const dir = path.join(home, ".claude", "projects", cwd.replace(/[/.]/g, "-"));
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, id + ".jsonl"), body);
    fs.mkdirSync(cwd, { recursive: true });
  }
  fs.writeFileSync(path.join(home, ".claude", "CLAUDE.md"), "global rules\n@~/.config/agents/AGENTS.md\n");
  fs.mkdirSync(path.join(home, ".config", "agents"), { recursive: true });
  fs.writeFileSync(path.join(home, ".config", "agents", "AGENTS.md"), "baseline ".repeat(200));
  return home;
}

const home = fixtureHome();
test.afterAll(() => { fs.rmSync(home, { recursive: true, force: true }); });
test.use({ bdEnv: { HOME: home } });

const fleet = (page: Page) => page.locator("#vFleet");
const on = (page: Page) => page.locator("#vFleet .fl-nav button.on");

async function key(page: Page, k: string): Promise<void> {
  await page.keyboard.press(k);
}

/** show opens the Fleet tab and, when a page is named, that page of its navigation. */
async function show(page: Page, label?: string): Promise<void> {
  await page.locator('[data-view="fleet"]').click();
  await expect(fleet(page)).toBeVisible();
  if (label) await fleet(page).locator(".fl-nav button", { hasText: label }).click();
}

test.describe("fleet views", () => {
  test("the Fleet tab shows the board with a card for each live session", async ({ page, project }) => {
    await open(page, project);
    await show(page);
    await expect(on(page)).toHaveText("Fleet");
    await expect(fleet(page)).toContainText("review-thing");
    await expect(fleet(page)).toContainText("bd-ab12.3");
    await expect(fleet(page).locator(".fl-prow")).toHaveCount(3);
  });

  test("an agent chip opens the session page with its log and context", async ({ page, project }) => {
    await open(page, project);
    await show(page);
    await fleet(page).locator("[data-fl-agent]").first().click();
    await expect(fleet(page).locator(".fl-title")).toContainText("review-thing");
    await expect(fleet(page).locator(".fl-log")).toContainText("Review bd-ab12.3 please");
    await expect(fleet(page).locator(".fl-log .fl-tag.t-err")).toBeVisible();
    await expect(fleet(page).locator(".fl-stack")).toBeVisible();
    await expect(fleet(page)).toContainText("Instructions");
    await expect(fleet(page)).toContainText("claude-opus-5-5");
  });

  test("a log filter switches one kind of line off and on", async ({ page, project }) => {
    await open(page, project);
    await show(page);
    await fleet(page).locator("[data-fl-agent]").first().click();
    const log = fleet(page).locator(".fl-log");
    await expect(log).toContainText("Review done.");
    const ai = fleet(page).locator('[data-fl-kind="AI"]');
    await ai.click();
    await expect(ai).toHaveAttribute("aria-pressed", "false");
    await expect(log).not.toContainText("Review done.");
    await expect(log).toContainText("loaded 2 memories");
    await ai.click();
    await expect(log).toContainText("Review done.");
  });

  test("the Context page shows the context matrix with a stale copy outlined", async ({ page, project }) => {
    await open(page, project);
    await show(page, "Context");
    await expect(on(page)).toHaveText("Context");
    await expect(fleet(page).locator(".fl-heat")).toContainText("no-shared-dolt");
    await expect(fleet(page).locator(".fl-heat td.fl-stale").first()).toBeVisible();
    await expect(fleet(page).locator(".fl-insights .fl-ins").first()).toBeVisible();
  });

  test("the Constellation page draws with a node for each session", async ({ page, project }) => {
    await open(page, project);
    await show(page, "Constellation");
    await expect(on(page)).toHaveText("Constellation");
    await expect(fleet(page).locator("svg.fl-svg")).toBeVisible();
    await expect(fleet(page).locator(".fl-sess")).toHaveCount(2);
    await expect(fleet(page).locator(".fl-src-node").first()).toBeAttached();
  });

  test("the Timeline page draws a lane for each session and the hours picker reloads it", async ({ page, project }) => {
    await open(page, project);
    await show(page, "Timeline");
    await expect(on(page)).toHaveText("Timeline");
    await expect(fleet(page).locator(".fl-lane")).toHaveCount(2);
    await fleet(page).locator('[data-fl-hours="6"]').click();
    await expect(fleet(page).locator('[data-fl-hours="6"]')).toHaveAttribute("aria-pressed", "true");
    await expect(fleet(page).locator(".fl-lane")).toHaveCount(2);
  });

  test("the Projects page shows a tile for each project", async ({ page, project }) => {
    await open(page, project);
    await show(page, "Projects");
    await expect(on(page)).toHaveText("Projects");
    await expect(fleet(page).locator(".fl-tile").first()).toBeVisible();
    await expect(fleet(page).locator(".fl-ins").first()).toBeVisible();
  });
});

test.describe("fleet keys", () => {
  test("function keys keep their issue meaning and leave the view alone", async ({ page, project }) => {
    await open(page, project);
    for (const k of ["F1", "F2", "F3", "F4", "F6", "F7", "F8"]) await key(page, k);
    await expect(page.locator("#vTree")).toBeVisible();
    await expect(fleet(page)).toBeHidden();
  });

  test("Esc leaves the session page for the page before it, then the fleet", async ({ page, project }) => {
    await open(page, project);
    await show(page, "Context");
    await fleet(page).locator("[data-fl-agent]").first().click();
    await expect(fleet(page).locator(".fl-title")).toBeVisible();
    await key(page, "Escape");
    await expect(on(page)).toHaveText("Context");
    await key(page, "Escape");
    await expect(page.locator("#vTree")).toBeVisible();
  });

  test("the keys sheet lists F5 as reload", async ({ page, project }) => {
    await open(page, project);
    await key(page, "?");
    await expect(page.locator("#sheetHost .msheet")).toContainText("Ctrl-R, F5");
  });
});

test.describe("a public server", () => {
  test("refuses the agent endpoints and hides the fleet", async ({ page }) => {
    const p = await startProject(SAMPLE, { HOME: home }, true, freePort, ["--public"]);
    try {
      for (const route of ["api/agents", "api/fleet", "api/context-matrix", "api/timeline", "api/agent?id=s-fleet-1"]) {
        const r = await fetch(p.url + route);
        expect(r.status, route).toBe(403);
      }
      await page.goto(p.url);
      await settled(page);
      await expect(page.locator('[data-view="fleet"]')).toHaveCount(0);
      await page.keyboard.press("F1");
      await expect(page.locator("#vTree")).toBeVisible();
    } finally {
      stopProject(p);
    }
  });
});
