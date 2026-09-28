// The phone detail sheet has two detents, half and full, and opens at half.
// Scrolling the half sheet's text up raises it to full; pulling the text down
// from its top lowers full to half and closes half. A tap on the head, the
// handle and title above the text, switches the detents, and taps on the text
// change nothing, so a relation acts at once.

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

/**
 * drag sends a vertical touch drag over the sheet's text. WebKit cannot
 * construct a Touch, so each event is a plain Event that carries the one
 * touch list the sheet reads.
 */
async function drag(page: Page, dy: number): Promise<void> {
  const b = await boxOf(page, "#dbody");
  await page.evaluate(({ x, y, dy }) => {
    const el = document.querySelector("#dbody .meta") || document.getElementById("dbody")!;
    const fire = (type: string, cy: number) => {
      const ev = new Event(type, { bubbles: true });
      Object.defineProperty(ev, "touches", { value: type === "touchend" ? [] : [{ clientX: x, clientY: cy }] });
      el.dispatchEvent(ev);
    };
    fire("touchstart", y);
    for (let s = 1; s <= 8; s++) fire("touchmove", y + (dy * s) / 8);
    fire("touchend", y + dy);
  }, { x: b.x + b.width / 2, y: b.y + 20, dy });
}

async function openT3(page: Page) {
  const d = page.locator(".detail");
  await row(page, "t-3").click();
  await expect(d).toHaveAttribute("data-id", "t-3");
  return d;
}

test("the detail opens at half height", async ({ page, project }) => {
  await open(page, project);
  const d = await openT3(page);
  await expect(d).not.toHaveClass(/\bfull\b/);
  await expect.poll(() => sheetShare(page)).toBeLessThan(0.6);
});

test("scrolling the half sheet up raises it, and pulling down lowers it and then closes it", async ({ page, project }) => {
  await open(page, project);
  const d = await openT3(page);

  await drag(page, -40);
  await expect(d).toHaveClass(/\bfull\b/);
  await expect.poll(() => sheetShare(page)).toBeGreaterThan(0.97);

  await drag(page, 100);
  await expect(d).not.toHaveClass(/\bfull\b/);
  await expect.poll(() => sheetShare(page)).toBeLessThan(0.6);

  await drag(page, 100);
  await expect(d).toHaveCount(0);
});

test("a tap on the head switches the detents", async ({ page, project }) => {
  await open(page, project);
  const d = await openT3(page);

  const grab = await boxOf(page, ".detail .grab");
  await page.touchscreen.tap(grab.x + grab.width / 2, grab.y + grab.height / 2);
  await expect(d).toHaveClass(/\bfull\b/);
  const g2 = await boxOf(page, ".detail .grab");
  await page.touchscreen.tap(g2.x + g2.width / 2, g2.y + g2.height / 2);
  await expect(d).not.toHaveClass(/\bfull\b/);
});

test("taps on the text keep the sheet's size, and a relation tap navigates at once", async ({ page, project }) => {
  await open(page, project);
  const d = await openT3(page);

  const b = await boxOf(page, ".detail .meta");
  await page.touchscreen.tap(b.x + 20, b.y + 8);
  await page.touchscreen.tap(b.x + 20, b.y + 8);
  await page.waitForTimeout(450);
  await expect(d).not.toHaveClass(/\bfull\b/);
  await expect(d).toHaveCount(1);

  await d.locator('.rel[data-nav="t-1"]').tap();
  await expect(d).toHaveAttribute("data-id", "t-1", { timeout: 250 });
});
