import { expect, open, row, test } from "./harness";

test.use({ paired: true });

test("an unpaired browser is told how to pair, and the printed link pairs it", async ({ page, project }) => {
  await page.goto(project.url);
  await expect(page.locator("#boot")).toContainText("This browser is not paired");
  await expect(page.locator("#boot")).toContainText("b9s web");
  await expect(page.locator("#boot")).not.toContainText("--pair");
  await expect.poll(() => project.pairLink()).toContain("/pair?t=");
  await page.goto(project.pairLink());
  await expect(page.locator("#boot")).toBeHidden({ timeout: 10000 });
  await expect(row(page, "t-1")).toBeVisible();
  await open(page, project);
  await expect(row(page, "t-1")).toBeVisible();
});
