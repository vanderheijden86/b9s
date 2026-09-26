import { expect, open, row, SAMPLE, test } from "./harness";

test("an outside write shows up without a reload", async ({ page, project }) => {
  await open(page, project);
  await page.evaluate(() => { (window as unknown as { marker: number }).marker = 1; });
  project.writeIssues([...SAMPLE, { id: "t-9", title: "[9] Written by someone else", priority: 1 }]);
  await expect(row(page, "t-9")).toBeVisible({ timeout: 10000 });
  expect(await page.evaluate(() => (window as unknown as { marker?: number }).marker)).toBe(1);
});

test("the projects sheet lists the current project", async ({ page, project }) => {
  await open(page, project);
  await page.locator('[data-act="projects"]').click();
  await expect(page.locator('.msheet [data-sa="proj"]').first()).toBeVisible();
});
