import { expect, longPress, open, test } from "./harness";

test("board lists columns and moves a card by drag onto a tab", async ({ page, project }) => {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator('[data-tab="open"]')).toHaveClass(/on/);
  const card = page.locator('[data-card="t-2"]');
  await expect(card).toBeVisible();
  const cb = (await card.boundingBox())!;
  const tb = (await page.locator('[data-tab="in_progress"]').boundingBox())!;
  await page.mouse.move(cb.x + cb.width / 2, cb.y + cb.height / 2);
  await page.mouse.down();
  await page.waitForTimeout(600);
  for (let s = 1; s <= 10; s++) await page.mouse.move(cb.x + cb.width / 2 + ((tb.x + tb.width / 2 - cb.x - cb.width / 2) * s) / 10, cb.y + cb.height / 2 + ((tb.y + tb.height / 2 - cb.y - cb.height / 2) * s) / 10);
  await page.mouse.up();
  await expect.poll(() => project.issues().find(i => i.id === "t-2")?.status).toBe("in_progress");
  await expect(page.locator('[data-card="t-2"]')).toHaveCount(0);
});

test("swiping the column moves to the next one", async ({ page, project }) => {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  const b = (await page.locator("#bcol").boundingBox())!;
  const y = b.y + 40;
  await page.mouse.move(b.x + b.width * 0.8, y);
  await page.mouse.down();
  for (let s = 1; s <= 10; s++) await page.mouse.move(b.x + b.width * (0.8 - 0.06 * s), y);
  await page.mouse.up();
  await expect(page.locator('[data-tab="in_progress"]')).toHaveClass(/on/);
  await expect(page.locator('[data-card="t-1"]')).toBeVisible();
});

test("the options button in the board toolbar opens board options", async ({ page, project }) => {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  await page.locator('#tabs [data-act="boardopts"]').click();
  await expect(page.locator("#sheetHost")).toContainText("Group by");
});

// Option 1 of docs/mockups/mobile-board-hierarchy-options.html: in rail mode
// the column keeps every lane under a sticky epic header, and the rail is an
// index into it rather than a filter.
test.describe("epic lanes with the rail as an index", () => {
  const many = Array.from({ length: 14 }, (_, n) => ({ id: `t-a${n}`, title: `Alpha task ${n}`, parent: "t-epic", priority: 2 }));
  test.use({
    fixture: {
      issues: [
        { id: "t-epic", title: "Alpha epic", type: "epic" },
        ...many,
        { id: "t-mid", title: "Gestures for status", parent: "t-epic", status: "in_progress" },
        { id: "t-deep", title: "Swipe threshold", parent: "t-mid", type: "bug", priority: 3 },
        { id: "t-beta", title: "Beta epic", type: "epic" },
        { id: "t-b1", title: "Beta task", parent: "t-beta", priority: 3 },
        { id: "t-loose", title: "Loose chore", type: "chore", priority: 4 },
      ],
    },
  });
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem("b9s.boardLanes", JSON.stringify("rail")));
  });

  test("every lane shows under its epic header, not a filtered subset", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    for (const l of ["t-epic", "t-beta", "none"]) await expect(page.locator(`#bcol .lane[data-lane="${l}"]`)).toHaveCount(1);
    await expect(page.locator("#bcol .card")).toHaveCount(17);
    await expect(page.locator('#bcol .lane[data-lane="t-epic"] .bar')).toHaveCount(1);
  });

  test("a rail tap scrolls to the lane and lights its entry, the column stays whole", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await page.locator('.brail [data-rlane="t-beta"]').click();
    await expect(page.locator("#bcol .card")).toHaveCount(17);
    await expect(page.locator('.brail [data-rlane="t-beta"]')).toHaveClass(/on/);
    await expect.poll(async () => {
      const col = (await page.locator("#bcol").boundingBox())!;
      const h = (await page.locator('#bcol .lane[data-lane="t-beta"]').boundingBox())!;
      return Math.abs(h.y - col.y) < 12;
    }).toBe(true);
    await expect(page.locator('#bcol [data-card="t-b1"]')).toBeInViewport();
  });

  test("scrolling the column lights the rail entry of the lane in view", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await expect(page.locator('.brail [data-rlane="t-epic"]')).toHaveClass(/on/);
    // Halfway down its lane, the epic header is still pinned to the top.
    await page.locator("#bcol").evaluate(el => { el.scrollTop = 300; });
    await expect.poll(async () => {
      const col = (await page.locator("#bcol").boundingBox())!;
      const h = (await page.locator('#bcol .lane[data-lane="t-epic"]').boundingBox())!;
      return Math.abs(h.y - col.y) < 4;
    }).toBe(true);
    await page.locator("#bcol").evaluate(el => { el.scrollTop = el.scrollHeight; });
    await expect(page.locator('.brail [data-rlane="none"]')).toHaveClass(/on/);
    await expect(page.locator('.brail [data-rlane="t-epic"]')).not.toHaveClass(/on/);
  });

  test("a long press on a rail entry folds that lane", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await longPress(page, '.brail [data-rlane="t-epic"]');
    await expect(page.locator('#bcol .lane[data-lane="t-epic"]')).toHaveAttribute("aria-expanded", "false");
    await expect(page.locator('#bcol [data-card="t-a0"]')).toHaveCount(0);
    await expect(page.locator('#bcol [data-card="t-b1"]')).toBeVisible();
  });

  test("the lane chevron folds the lane", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await page.locator('#bcol .lane[data-lane="t-beta"]').click();
    await expect(page.locator('#bcol [data-card="t-b1"]')).toHaveCount(0);
    await expect(page.locator('#bcol [data-card="t-a0"]')).toHaveCount(1);
  });

  test("a card nested below a task names its parent on a third line", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await expect(page.locator('#bcol [data-card="t-deep"] .l3')).toContainText("↳ mid Gestures for status");
    await expect(page.locator('#bcol [data-card="t-b1"] .l3')).toHaveCount(0);
  });
});
