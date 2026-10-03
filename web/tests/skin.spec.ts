import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, open, test } from "./harness";

const background = (page: import("@playwright/test").Page) =>
  page.evaluate(() => getComputedStyle(document.body).backgroundColor);

test("appearance sheet names the built-in skins and paints with them", async ({ page, project }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await open(page, project);
  await expect.poll(() => background(page)).toBe("rgb(40, 42, 54)"); // dracula #282A36
  await page.locator('[data-act="theme"]').click();
  await expect(page.locator('[data-sa="settheme"][data-val="light"]')).toContainText("Sepia");
  await expect(page.locator('[data-sa="settheme"][data-val="dark"]')).toContainText("Dracula");
  await page.locator('[data-sa="settheme"][data-val="light"]').click();
  await expect.poll(() => background(page)).toBe("rgb(233, 223, 203)"); // sepia #E9DFCB
});

test.describe("with a k9s skin in ui.skin", () => {
  test.use({ configYAML: `ui:\n  skin: ${path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../pkg/skin/testdata/gruvbox-light.yaml")}\n` });

  test("the light theme takes the skin's name and colours", async ({ page, project }) => {
    await page.emulateMedia({ colorScheme: "light" });
    await open(page, project);
    await expect.poll(() => background(page)).toBe("rgb(251, 241, 199)"); // gruvbox #FBF1C7
    await page.locator('[data-act="theme"]').click();
    await expect(page.locator('[data-sa="settheme"][data-val="light"]')).toContainText("Gruvbox-light");
    await expect(page.locator('[data-sa="settheme"][data-val="dark"]')).toContainText("Dracula");
  });
});
