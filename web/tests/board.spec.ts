import { expect, open, test } from "./harness";

test.beforeEach(async ({ page }) => {
  page.on("pageerror", error => { throw error; });
});

// The phone board stacks every column as a box under the chip row, as mockup
// A's phone board does (docs/adr/0052). shell.spec.ts measures the stack and
// the fold at 360 and 412 px; these specs cover what happens inside a box.
test("the board stacks the columns as boxes and moves a card by drag onto another box", async ({ page, project }) => {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator("#bcol .bbox")).toHaveCount(4);
  await expect.poll(() => page.locator("#bcol .bbox").evaluateAll(els => els.map(el => (el as HTMLElement).dataset.dcol))).toEqual(["open", "in_progress", "blocked", "closed"]);
  const card = page.locator('#bcol .bbox[data-dcol="open"] [data-card="t-2"]');
  await expect(card).toBeVisible();
  const cb = (await card.boundingBox())!;
  const tb = (await page.locator('#bcol .bbox[data-dcol="in_progress"] .bhead').boundingBox())!;
  await page.mouse.move(cb.x + cb.width / 2, cb.y + cb.height / 2);
  await page.mouse.down();
  await page.waitForTimeout(600);
  for (let s = 1; s <= 10; s++) await page.mouse.move(cb.x + cb.width / 2 + ((tb.x + tb.width / 2 - cb.x - cb.width / 2) * s) / 10, cb.y + cb.height / 2 + ((tb.y + tb.height / 2 - cb.y - cb.height / 2) * s) / 10);
  await expect(page.locator('#bcol .bbox[data-dcol="in_progress"]')).toHaveClass(/drop/);
  await page.mouse.up();
  await expect.poll(() => project.issues().find(i => i.id === "t-2")?.status).toBe("in_progress");
  await expect(page.locator('#bcol .bbox[data-dcol="in_progress"] [data-card="t-2"]')).toBeVisible();
  await expect(page.locator('#bcol .bbox[data-dcol="open"] [data-card="t-2"]')).toHaveCount(0);
});

test("the chips choose which boxes show, and the ⚙ chip opens the board options", async ({ page, project }) => {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator("#chips")).toBeVisible();
  await expect(page.locator("#tabs")).toHaveCount(0);
  await expect(page.locator('#bcol .bbox[data-dcol="blocked"] [data-card="t-3"]')).toBeVisible();
  await page.locator('#chips [data-chip="blocked"]').click();
  await expect(page.locator('#bcol .bbox[data-dcol="blocked"]')).toHaveCount(0);
  await expect(page.locator('[data-card="t-3"]')).toHaveCount(0);
  await expect(page.locator('#bcol .bbox[data-dcol="in_progress"] [data-card="t-1"]')).toBeVisible();
  await page.locator('#chips [data-act="boardopts"]').click();
  await expect(page.locator("#sheetHost")).toContainText("Group by");
});

// Inside a box every lane of the column sits under a sticky epic header, as
// option 1 of docs/mockups/mobile-board-hierarchy-options.html keeps them;
// a phone has no epic rail, so the rail setting draws the same rows.
test.describe("epic lanes inside a box", () => {
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

  test("every lane of a column shows under its epic header inside the column's box, with no rail", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    const open_ = page.locator('#bcol .bbox[data-dcol="open"]');
    for (const l of ["t-epic", "t-beta", "none"]) await expect(open_.locator(`.lane[data-lane="${l}"]`)).toHaveCount(1);
    await expect(open_.locator(".card")).toHaveCount(17);
    await expect(open_.locator('.lane[data-lane="t-epic"] .bar')).toHaveCount(1);
    // The in-progress card of the same epic sits under that epic's lane header in its own box.
    await expect(page.locator('#bcol .bbox[data-dcol="in_progress"] .lane[data-lane="t-epic"]')).toHaveCount(1);
    await expect(page.locator('#bcol .bbox[data-dcol="in_progress"] [data-card="t-mid"]')).toBeVisible();
    await expect(page.locator(".brail")).toHaveCount(0);
    // Cards take the whole width of the box.
    const bx = (await open_.boundingBox())!, cd = (await open_.locator('[data-card="t-a0"]').boundingBox())!;
    expect(cd.width).toBeGreaterThanOrEqual(bx.width - 24);
  });

  test("a lane header stays at the top of the board while its lane scrolls through", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    // Halfway down its lane, the epic header is pinned to the top of the board.
    await page.locator("#bcol").evaluate(el => { el.scrollTop = 300; });
    await expect.poll(async () => {
      const col = (await page.locator("#bcol").boundingBox())!;
      const h = (await page.locator('#bcol .bbox[data-dcol="open"] .lane[data-lane="t-epic"]').boundingBox())!;
      return Math.abs(h.y - col.y) < 4;
    }).toBe(true);
  });

  test("a tap on the lane title folds the lane", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await page.locator('#bcol .lane[data-lane="t-beta"] .t').click();
    await expect(page.locator('#bcol [data-card="t-b1"]')).toHaveCount(0);
    await expect(page.locator('#bcol [data-card="t-a0"]')).toHaveCount(1);
    await expect(page.locator(".detail")).toHaveCount(0);
  });

  for (const part of [".ch", ".ei"]) {
    test(`a tap on the lane's ${part === ".ch" ? "chevron" : "epic ID"} opens the epic`, async ({ page, project }) => {
      await open(page, project);
      await page.locator('#bar [data-view="board"]').click();
      await page.locator(`#bcol .lane[data-lane="t-beta"] ${part}`).click();
      await expect(page.locator(".detail")).toContainText("Beta epic");
      await expect(page.locator('#bcol [data-card="t-b1"]')).toHaveCount(1);
    });
  }

  test("a nested card keeps its parent in details without a breadcrumb line", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    await expect(page.locator('#bcol [data-card="t-deep"] .l3')).toHaveCount(0);
    await expect(page.locator('#bcol [data-card="t-b1"] .l3')).toHaveCount(0);
    await page.locator('#bcol [data-card="t-deep"]').click();
    await expect(page.locator('.detail [data-nav="t-mid"]')).toContainText("Gestures for status");
  });
});
