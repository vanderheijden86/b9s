import { expect, open, test } from "./harness";

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
