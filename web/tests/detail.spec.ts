// The phone detail sheet opens at full height, and a double tap on the sheet
// switches it between half and full height. A single tap on the sheet's text
// changes nothing, and a tap on a relation still acts at once.

import type { Page } from "@playwright/test";
import { expect, open, row, test } from "./harness";

const sheetShare = (page: Page) => page.evaluate(() => {
  const d = document.querySelector(".detail")!.getBoundingClientRect(), a = document.querySelector("#app")!.getBoundingClientRect();
  return d.height / a.height;
});

/** boxOf waits for a box, because the sheet re-renders once the full issue arrives and a read in between finds no element. */
async function boxOf(page: Page, selector: string) {
  let b: Awaited<ReturnType<ReturnType<Page["locator"]>["boundingBox"]>> = null;
  await expect.poll(async () => (b = await page.locator(selector).boundingBox())).not.toBeNull();
  return b!;
}

/** doubleTap taps twice at the centre of a locator, fast enough to count as a double tap. */
async function doubleTap(page: Page, selector: string): Promise<void> {
  const b = await boxOf(page, selector);
  const x = b.x + b.width / 2, y = b.y + Math.min(b.height / 2, 10);
  await page.touchscreen.tap(x, y);
  await page.touchscreen.tap(x, y);
}

test("the detail opens full screen and a double tap toggles half and full", async ({ page, project }) => {
  await open(page, project);
  const d = page.locator(".detail");
  await row(page, "t-3").click();
  await expect(d).toHaveAttribute("data-id", "t-3");
  await expect(d).toHaveClass(/\bfull\b/);
  await expect.poll(() => sheetShare(page)).toBeGreaterThan(0.97);

  await doubleTap(page, ".detail .meta");
  await expect(d).not.toHaveClass(/\bfull\b/);
  await expect.poll(() => sheetShare(page)).toBeLessThan(0.6);

  await doubleTap(page, ".detail .dtitle");
  await expect(d).toHaveClass(/\bfull\b/);
  await expect.poll(() => sheetShare(page)).toBeGreaterThan(0.97);
});

test("single taps on the sheet keep its size, and a relation tap navigates at once", async ({ page, project }) => {
  await open(page, project);
  const d = page.locator(".detail");
  await row(page, "t-3").click();
  await expect(d).toHaveClass(/\bfull\b/);

  const b = await boxOf(page, ".detail .dtitle");
  await page.touchscreen.tap(b.x + 20, b.y + 8);
  await page.waitForTimeout(450);
  await page.touchscreen.tap(b.x + 20, b.y + 8);
  await page.waitForTimeout(450);
  await expect(d).toHaveClass(/\bfull\b/);

  const grab = await boxOf(page, ".detail .grab");
  await page.touchscreen.tap(grab.x + grab.width / 2, grab.y + grab.height / 2);
  await page.waitForTimeout(450);
  await expect(d).toHaveClass(/\bfull\b/);

  await d.locator('.rel[data-nav="t-1"]').tap();
  await expect(d).toHaveAttribute("data-id", "t-1", { timeout: 250 });
  await expect(d).toHaveClass(/\bfull\b/);
});
