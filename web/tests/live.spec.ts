import { expect, open, reloadAndSettle, row, SAMPLE, test } from "./harness";

test("an outside write shows up without a reload", async ({ page, project }) => {
  await open(page, project);
  await page.evaluate(() => { (window as unknown as { marker: number }).marker = 1; });
  project.writeIssues([...SAMPLE, { id: "t-9", title: "[9] Written by someone else", priority: 1 }]);
  await expect(row(page, "t-9")).toBeVisible({ timeout: 10000 });
  expect(await page.evaluate(() => (window as unknown as { marker?: number }).marker)).toBe(1);
});

test("a failed live snapshot keeps the last snapshot visible until reconnect", async ({ page, project }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await open(page, project);
  await page.route("**/api/snapshot", route => route.abort("failed"));

  project.writeIssues([...SAMPLE, { id: "t-9", title: "[9] Written while disconnected", priority: 1 }]);

  await expect(page.locator("#stale")).toBeVisible();
  await expect(page.locator("#toast")).toContainText("Reload failed:");
  await expect(row(page, "t-1")).toBeVisible();
  await page.unroute("**/api/snapshot");
  await expect(row(page, "t-9")).toBeVisible({ timeout: 10000 });
  await expect(page.locator("body")).toHaveAttribute("data-live", "live");
  await expect(page.locator("#stale")).toBeHidden();
  expect(errors).toEqual([]);
});

test("the fixture server outlives the browser context", async ({ page, project, context }) => {
  await open(page, project);
  context.on("close", () => {
    expect.soft(project.proc.killed, "the browser context must close before its server stops").toBe(false);
  });
});

test("leaving the page cancels an in-flight live snapshot", async ({ page, project }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await open(page, project);
  await page.route("**/api/snapshot", () => {});
  const snapshot = page.waitForRequest("**/api/snapshot", { timeout: 5000 });
  project.writeIssues([...SAMPLE, { id: "t-9", title: "[9] Written during navigation", priority: 1 }]);
  await snapshot;

  const cancelled = page.waitForEvent("requestfailed", { predicate: request => request.url().endsWith("/api/snapshot"), timeout: 5000 });
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true })));
  await cancelled;

  await page.unroute("**/api/snapshot");
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true })));
  await expect(row(page, "t-9")).toBeVisible({ timeout: 10000 });
  await expect(page.locator("body")).toHaveAttribute("data-live", "live");
  expect(errors).toEqual([]);
});

test("the projects sheet lists the current project", async ({ page, project }) => {
  await open(page, project);
  await page.locator('[data-act="projects"]').click();
  await expect(page.locator('.msheet [data-sa="proj"]').first()).toBeVisible();
});

test("a reload settles on the selected issue", async ({ page, project }) => {
  await open(page, project);
  await row(page, "t-1").click();
  await expect(page).toHaveURL(/#\/.+\/t-1$/);
  const url = page.url();
  await page.evaluate(() => { (window as unknown as { marker: number }).marker = 1; });

  await reloadAndSettle(page, () => page.reload({ waitUntil: "commit" }));

  expect(await page.evaluate(() => document.readyState)).toBe("complete");
  expect(await page.evaluate(() => (window as unknown as { marker?: number }).marker)).toBeUndefined();
  await expect(page).toHaveURL(url);
  await expect(page.locator("body")).toHaveAttribute("data-live", "live");
  await expect(page.locator("#boot")).toBeHidden();
});
