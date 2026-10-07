// The board on a laptop, desktop browser or iPad: every column side by side,
// epic lanes as swimlanes, mouse drag between columns, TUI keys, and the
// detail as a side panel that leaves the board in view.

import { expect, open, SAMPLE, test, type Project } from "./harness";
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
  const heads = page.locator('#wboard .whead[data-tab]');
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
  await page.keyboard.press("v");
  const lane = page.locator('#wboard [data-lane="t-epic"]');
  await expect(lane).toContainText("Mobile web");
  const lb = (await lane.boundingBox())!, wb = (await page.locator("#wboard").boundingBox())!;
  expect(lb.width).toBeGreaterThan(wb.width * 0.8);
  // No epic lane holds the loose chore, below the epic lane.
  const none = page.locator('#wboard [data-lane="none"]');
  expect((await none.boundingBox())!.y).toBeGreaterThan(lb.y);
  expect((await card(page, "t-5").boundingBox())!.y).toBeGreaterThan((await none.boundingBox())!.y);
  await lane.locator(".t").click();
  await expect(card(page, "t-2")).toHaveCount(0);
  await expect(card(page, "t-5")).toBeVisible();
  await lane.locator(".ei").click();
  await expect(page.locator(".detail")).toContainText("Mobile web");
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

test("Back closes the side detail and Backspace steps back too", async ({ page, project }) => {
  await board(page, project);
  await page.keyboard.press("j");
  await page.keyboard.press("l");
  await page.keyboard.press("Enter");
  const d = page.locator(".detail");
  await expect(d).toHaveAttribute("data-id", "t-1");
  // Moving the cursor with the panel open replaces the entry: one Back closes it.
  await page.keyboard.press("l");
  await expect(d).toHaveAttribute("data-id", "t-3");
  await expect(page).toHaveURL(/#\/board\/t-3$/);
  await page.goBack();
  await expect(d).toHaveCount(0);
  await expect(page.locator("#wboard")).toBeVisible();
  await page.keyboard.press("Backspace");
  await expect(page.locator("#vTree")).toBeVisible();
  // At the first entry of the app, Backspace stays in the app.
  await page.keyboard.press("Backspace");
  await expect(page.locator("#vTree")).toBeVisible();
  expect(page.url()).toContain(project.url);
});

const epic = (page: Page, id: string) => page.locator(`#wboard .wepic[data-epic="${id}"]`);

test("the epic rail is the default: epics form the first column, level with their lane", async ({ page, project }) => {
  await board(page, project);
  const head = page.locator("#wboard .whead.epich");
  await expect(head).toContainText("EPIC");
  const e = epic(page, "t-epic");
  await expect(e).toContainText("Mobile web");
  await expect(e).toContainText("0/3");
  // The terminal's epic cell facts (docs/adr/0030): what moves, what waits, what is urgent.
  await expect(e.locator(".ec")).toHaveText("wip 1·waiting 1·ready 1");
  await expect(e.locator(".en")).toContainText("P1 1");
  const eb = (await e.boundingBox())!, ob = (await page.locator('#wboard .whead[data-tab="open"]').boundingBox())!;
  expect(eb.x + eb.width).toBeLessThanOrEqual(ob.x);
  // The epic's cards start on the epic's row; the loose chore sits in the No epic row below it.
  const c2 = (await card(page, "t-2").boundingBox())!;
  expect(Math.abs(c2.y - eb.y)).toBeLessThan(20);
  const none = (await epic(page, "none").boundingBox())!;
  expect(none.y).toBeGreaterThanOrEqual(eb.y + eb.height - 1);
  expect(Math.abs((await card(page, "t-5").boundingBox())!.y - none.y)).toBeLessThan(20);
  await expect(page.locator("#wboard .lane")).toHaveCount(0);
  await expect(page.locator(".brail")).toHaveCount(0);
});

test("v switches between the epic rail and epic rows, and the choice stays", async ({ page, project }) => {
  await board(page, project);
  await expect(epic(page, "t-epic")).toBeVisible();
  await page.keyboard.press("v");
  await expect(page.locator('#wboard .lane[data-lane="t-epic"]')).toBeVisible();
  await expect(page.locator("#wboard .wepic")).toHaveCount(0);
  await page.reload();
  await expect(page.locator("#boot")).toBeHidden({ timeout: 10000 });
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator('#wboard .lane[data-lane="t-epic"]')).toBeVisible();
  await page.keyboard.press("v");
  await expect(epic(page, "t-epic")).toBeVisible();
});

test("an epic cell opens the epic, and its arrow folds the lane", async ({ page, project }) => {
  await board(page, project);
  await epic(page, "t-epic").locator(".t").click();
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-epic");
  await page.keyboard.press("Escape");
  await expect(page.locator(".detail")).toHaveCount(0);
  await epic(page, "t-epic").locator("[data-lane]").click();
  await expect(card(page, "t-2")).toHaveCount(0);
  await expect(epic(page, "t-epic")).toContainText("▸");
  await expect(card(page, "t-5")).toBeVisible();
});

test("h from the first column selects the epic; Enter opens it and Tab folds its lane", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-2").click();
  await page.keyboard.press("Escape");
  await page.keyboard.press("h");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await page.keyboard.press("l");
  await expect(card(page, "t-2")).toHaveClass(/sel/);
  // Each key acts on the selection the previous key left, so wait for it:
  // Enter on the "none" lane opens nothing.
  await page.keyboard.press("h");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await page.keyboard.press("j");
  await expect(epic(page, "none")).toHaveClass(/sel/);
  await page.keyboard.press("k");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await page.keyboard.press("Enter");
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-epic");
  await page.keyboard.press("Escape");
  await expect(page.locator(".detail")).toHaveCount(0);
  await page.keyboard.press("Tab");
  await expect(card(page, "t-2")).toHaveCount(0);
  await page.keyboard.press("Tab");
  await expect(card(page, "t-2")).toBeVisible();
});

test("\\ or the panel's button switches the detail between a side panel and the full width", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-1").click();
  const d = page.locator(".detail"), vp = page.viewportSize()!;
  // The panel slides in, so its box settles a moment after it appears.
  const box = async () => (await d.boundingBox()) || { x: -1, width: -1 };
  const panel = async () => { const b = await box(); return b.width > 0 && b.width < vp.width * 0.6; };
  const full = async () => { const b = await box(); return b.x <= 1 && b.width >= vp.width - 2; };
  await expect.poll(panel).toBe(true);
  await page.keyboard.press("\\");
  await expect.poll(full).toBe(true);
  await expect(d).toHaveAttribute("data-id", "t-1");
  await expect(d.locator('[data-act="dsize"]')).toHaveAttribute("aria-label", "Side panel");
  await page.keyboard.press("\\");
  await expect.poll(panel).toBe(true);
  await d.locator('[data-act="dsize"]').click();
  await expect.poll(full).toBe(true);
  // The board's keys still move the cursor behind the full panel, and the panel follows.
  await page.keyboard.press("l");
  await expect(d).toHaveAttribute("data-id", "t-3");
  await expect.poll(full).toBe(true);
  await page.keyboard.press("Escape");
  await expect(d).toHaveCount(0);
});

test.describe("a lane taller than its epic cell", () => {
  test.use({ fixture: { issues: [...SAMPLE, ...[6, 7, 8, 9].map(n => ({ id: `t-${n}`, title: `[${n}] Task ${n}`, parent: "t-epic", priority: 3 }))] } });

  test("the epic cell spans the whole lane", async ({ page, project }) => {
    await board(page, project);
    const eb = (await epic(page, "t-epic").boundingBox())!;
    const last = (await card(page, "t-9").boundingBox())!;
    expect(last.y + last.height).toBeGreaterThan(eb.y + 120);
    expect(eb.y + eb.height).toBeGreaterThanOrEqual(last.y + last.height - 2);
    // The next lane's epic starts below it, so the cell fills its own row only.
    const none = (await epic(page, "none").boundingBox())!;
    expect(none.y).toBeGreaterThanOrEqual(eb.y + eb.height - 1);
  });
});

test("a card is one line with its tags on the right, and opens under the cursor", async ({ page, project }) => {
  await board(page, project);
  await expect(card(page, "t-3").locator(".tags")).toHaveText("blocked 1");
  await expect(card(page, "t-1").locator(".tags")).toHaveText("blocks 1P1");
  await expect(card(page, "t-2").locator(".tags")).toHaveCount(0);
  await expect(card(page, "t-2").locator(".l2")).toHaveCount(0);
  const h = (await card(page, "t-2").boundingBox())!.height;
  expect(h).toBeLessThan(32);
  await card(page, "t-2").click();
  await expect(card(page, "t-2").locator(".l2")).toHaveCount(1);
});

test.describe("hierarchy in a cell", () => {
  test.use({
    fixture: {
      issues: [
        { id: "h-epic", title: "[epic] Epic: Checkout", type: "epic", priority: 2, description: "# Goal\nShip a faster checkout. Then more." },
        { id: "h-task", title: "[t] Card form", parent: "h-feat", priority: 1 },
        { id: "h-feat", title: "[f] Payments", type: "feature", parent: "h-epic", priority: 2 },
        { id: "h-other", title: "[o] Receipts", parent: "h-epic", priority: 3 },
      ],
    },
  });

  test("a child sits straight below its parent, indented, and the epic cell carries its first sentence", async ({ page, project }) => {
    await board(page, project);
    const ids = await col(page, "open").first().locator("[data-card]").evaluateAll(els => els.map(e => (e as HTMLElement).dataset.card));
    expect(ids).toEqual(["h-feat", "h-task", "h-other"]);
    const fx = (await card(page, "h-feat").locator(".id").boundingBox())!.x;
    const tx = (await card(page, "h-task").locator(".id").boundingBox())!.x;
    expect(tx).toBeGreaterThan(fx + 8);
    await expect(card(page, "h-task").locator(".l3")).toHaveCount(0);
    await expect(epic(page, "h-epic").locator(".es")).toHaveText("Ship a faster checkout.");
  });
});
