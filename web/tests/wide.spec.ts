// The board on a laptop, desktop browser or iPad: every column side by side,
// epic lanes as swimlanes, mouse drag between columns, TUI keys, and the
// detail as a side panel that leaves the board in view.

import { expect, open, test, type Project } from "./harness";
import type { Page } from "@playwright/test";

const col = (page: Page, k: string) => page.locator(`#wboard .wcell[data-dcol="${k}"]`);
const card = (page: Page, id: string) => page.locator(`#wboard [data-card="${id}"]`);

async function board(page: Page, project: Project): Promise<void> {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator("#wboard")).toBeVisible();
}

async function drag(page: Page, from: { x: number; y: number }, to: { x: number; y: number }): Promise<void> {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  for (let s = 1; s <= 12; s++) await page.mouse.move(from.x + ((to.x - from.x) * s) / 12, from.y + ((to.y - from.y) * s) / 12);
  await page.mouse.up();
}

const mid = (b: { x: number; y: number; width: number; height: number }) => ({ x: b.x + b.width / 2, y: b.y + b.height / 2 });

test("every column shows side by side", async ({ page, project }) => {
  await board(page, project);
  const heads = page.locator('#wboard .whead');
  await expect(heads).toHaveCount(3);
  const boxes = await Promise.all(["open", "in_progress", "blocked"].map(k => page.locator(`#wboard .whead[data-tab="${k}"]`).boundingBox()));
  expect(boxes[0]!.y).toBe(boxes[1]!.y);
  expect(boxes[0]!.x).toBeLessThan(boxes[1]!.x);
  expect(boxes[1]!.x).toBeLessThan(boxes[2]!.x);
  for (const [id, k] of [["t-2", "open"], ["t-1", "in_progress"], ["t-3", "blocked"]]) {
    await expect(col(page, k).locator(`[data-card="${id}"]`)).toBeInViewport();
  }
  await expect(page.locator("#bcol")).toHaveCount(0);
});

test("epic lanes are swimlanes across the columns", async ({ page, project }) => {
  await board(page, project);
  const lane = page.locator('#wboard [data-lane="t-epic"]');
  await expect(lane).toContainText("Mobile web");
  const lb = (await lane.boundingBox())!, wb = (await page.locator("#wboard").boundingBox())!;
  expect(lb.width).toBeGreaterThan(wb.width * 0.8);
  // No epic lane holds the loose chore, below the epic lane.
  const none = page.locator('#wboard [data-lane="none"]');
  expect((await none.boundingBox())!.y).toBeGreaterThan(lb.y);
  expect((await card(page, "t-5").boundingBox())!.y).toBeGreaterThan((await none.boundingBox())!.y);
  await lane.click();
  await expect(card(page, "t-2")).toHaveCount(0);
  await expect(card(page, "t-5")).toBeVisible();
});

test("a mouse drags a card into another column", async ({ page, project }) => {
  await board(page, project);
  const from = mid((await card(page, "t-2").boundingBox())!);
  const to = mid((await col(page, "in_progress").first().boundingBox())!);
  await drag(page, from, to);
  await expect.poll(() => project.issues().find(i => i.id === "t-2")?.status).toBe("in_progress");
  await expect(col(page, "in_progress").locator('[data-card="t-2"]')).toBeVisible();
  expect(project.calls()).toContainEqual(["update", "t-2", "--status=in_progress"]);
});

test("a header click folds its column into a rail and a rail click unfolds it", async ({ page, project }) => {
  await board(page, project);
  const head = page.locator('#wboard .whead[data-tab="blocked"]');
  const w0 = (await head.boundingBox())!.width;
  await head.click();
  await expect(head).toHaveClass(/rail/);
  expect((await head.boundingBox())!.width).toBeLessThan(w0 / 3);
  await expect(card(page, "t-3")).toHaveCount(0);
  await page.locator('#tabs [data-act="unfoldall"]').click();
  await expect(head).not.toHaveClass(/rail/);
  await expect(card(page, "t-3")).toBeVisible();
});

test("the options button in the board toolbar opens board options", async ({ page, project }) => {
  await board(page, project);
  await page.locator('#tabs [data-act="boardopts"]').click();
  await expect(page.locator("#sheetHost")).toContainText("Group by");
});

test("keys move the cursor across columns and Enter opens the side detail", async ({ page, project }) => {
  await board(page, project);
  await page.keyboard.press("j");
  await expect(page.locator("#wboard .card.sel")).toHaveCount(1);
  await page.keyboard.press("l");
  await expect(card(page, "t-1")).toHaveClass(/sel/);
  await page.keyboard.press("ArrowRight");
  await expect(card(page, "t-3")).toHaveClass(/sel/);
  await page.keyboard.press("h");
  await expect(card(page, "t-1")).toHaveClass(/sel/);
  await page.keyboard.press("Enter");
  const d = page.locator(".detail");
  await expect(d).toContainText("Write the API");
  // Poll every box: the panel slides in, and the full issue arriving for it
  // re-renders both the panel and the board under a held element.
  const vp = page.viewportSize()!;
  const box = async (sel: string) => page.locator(sel).boundingBox();
  await expect.poll(async () => {
    const b = await box(".detail");
    return !!b && Math.round(b.x + b.width) >= vp.width - 1 && b.width < vp.width * 0.6 && b.height > vp.height * 0.7;
  }).toBe(true);
  // The board stays usable beside the panel: it ends where the panel starts
  // and scrolls sideways when the columns no longer fit, as on an iPad upright.
  await expect.poll(async () => {
    const [w, b] = [await box("#wboard"), await box(".detail")];
    return !!w && !!b && Math.abs(w.x + w.width - b.x) <= 1;
  }).toBe(true);
  await page.keyboard.press("l");
  await expect(d).toContainText("Board drag");
  await page.keyboard.press("Escape");
  await expect(d).toHaveCount(0);
});

test("z folds the cursor column and Z unfolds every column", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-3").click();
  await page.keyboard.press("Escape");
  await page.keyboard.press("z");
  await expect(page.locator('#wboard .whead[data-tab="blocked"]')).toHaveClass(/rail/);
  await page.keyboard.press("Shift+Z");
  await expect(page.locator('#wboard .whead[data-tab="blocked"]')).not.toHaveClass(/rail/);
});
