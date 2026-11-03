import type { Locator, Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, open, row, test } from "./harness";

const here = dirname(fileURLToPath(import.meta.url));
const appCSS = join(here, "..", "src", "app.css");

async function tokenColor(page: Page, token: string): Promise<string> {
  return page.evaluate(t => {
    const el = document.createElement("span");
    el.style.color = `var(${t})`;
    document.body.append(el);
    const color = getComputedStyle(el).color;
    el.remove();
    return color;
  }, token);
}

async function expectColor(locator: Locator, color: string): Promise<void> {
  await expect(locator.first()).toBeVisible();
  await expect(locator.first()).toHaveCSS("color", color);
}

test.describe("tokens", () => {
  test("app.css gets colors from tokens.css", () => {
    const css = readFileSync(appCSS, "utf8");
    expect(css.trimStart()).toMatch(/^@import\s+url\("tokens\.css"\);/);

    const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, "");
    const literals = withoutComments.match(/#[0-9a-fA-F]{3,8}\b|(?:rgb|hsl)a?\(/g) ?? [];
    expect(literals).toEqual([]);
  });

  test("issue metadata keeps text neutral and reserves color for status dots", async ({ page, project }) => {
    await page.setViewportSize({ width: 412, height: 780 });
    await open(page, project);

    const neutral = await tokenColor(page, "--muted");
    const line = await tokenColor(page, "--line");
    const openColor = await tokenColor(page, "--open");

    await expectColor(row(page, "t-1").locator(".stw"), neutral);
    await expectColor(row(page, "t-1").locator(".l2 > span").filter({ hasText: /^P1$/ }), neutral);
    await expectColor(row(page, "t-1").locator(".l2 > span").filter({ hasText: /task/ }), neutral);
    await expectColor(row(page, "t-epic").locator(".l2 > span").filter({ hasText: "0/3" }), neutral);
    await expectColor(row(page, "t-3").locator(".l2 > span").filter({ hasText: "blocked-by 1" }), neutral);

    const openChip = page.locator('[data-chip="open"]');
    await expectColor(openChip, neutral);
    await expect(openChip).toHaveCSS("border-color", line);
    await expect.poll(() => openChip.evaluate(el => getComputedStyle(el, "::before").backgroundColor)).toBe(openColor);

    await page.locator('#bar [data-view="board"]').click();
    const card = page.locator('[data-card="t-2"]');
    await expect(card.locator(".ty")).toHaveCount(0);
    await expect.poll(() => card.locator(".st").evaluate(el => getComputedStyle(el).backgroundColor)).toBe(openColor);
    await card.click();
    await expect(card.locator(".l2, .l3")).toHaveCount(0);
    await expectColor(page.locator(".detail .pill").filter({ hasText: /^P2$/ }), neutral);
    await expectColor(page.locator(".detail .pill").filter({ hasText: /^task$/ }), neutral);
    await page.locator('.detail [data-act="dclose"]').last().click();
    await expect(page.locator(".detail")).toHaveCount(0);

    await page.locator('#bar [data-view="tree"]').click();
    await row(page, "t-3").click();
    const detail = page.locator(".detail");
    await expect(detail).toHaveAttribute("data-id", "t-3");
    await expectColor(detail.locator(".pill").filter({ hasText: /blocked/ }), neutral);
    await expectColor(detail.locator(".pill").filter({ hasText: /^P2$/ }), neutral);
    await expectColor(detail.locator(".pill").filter({ hasText: /task/ }), neutral);
    await expectColor(detail.locator(".dsec span").filter({ hasText: "Blocked by 1" }), neutral);
  });
});
