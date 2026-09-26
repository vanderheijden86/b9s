// The browser's Back and Forward step through the app: every view switch,
// opened issue and followed relation is a history entry with its own URL,
// so a reload or a shared link lands on the same view and issue.

import { expect, open, row, test } from "./harness";

test("Back and Forward step through views, issues and relations", async ({ page, project }) => {
  await open(page, project);
  const d = page.locator(".detail");
  await row(page, "t-3").click();
  await expect(d).toHaveAttribute("data-id", "t-3");
  await expect(page).toHaveURL(/#\/tree\/t-3$/);
  await d.locator('.rel[data-nav="t-1"]').click();
  await expect(d).toHaveAttribute("data-id", "t-1");
  await expect(page).toHaveURL(/#\/tree\/t-1$/);

  await page.goBack();
  await expect(d).toHaveAttribute("data-id", "t-3");
  await page.goBack();
  await expect(d).toHaveCount(0);
  await expect(page.locator("#vTree")).toBeVisible();
  await expect(page).toHaveURL(/#\/tree$/);

  await page.goForward();
  await expect(d).toHaveAttribute("data-id", "t-3");
  await page.goBack();
  await expect(d).toHaveCount(0);

  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator("#vBoard")).toBeVisible();
  await expect(page).toHaveURL(/#\/board$/);
  await page.goBack();
  await expect(page.locator("#vTree")).toBeVisible();
  await page.goForward();
  await expect(page.locator("#vBoard")).toBeVisible();
});

test("the in-app back and close buttons use the same history as the browser", async ({ page, project }) => {
  await open(page, project);
  const d = page.locator(".detail");
  await row(page, "t-3").click();
  await d.locator('.rel[data-nav="t-1"]').click();
  await expect(d).toHaveAttribute("data-id", "t-1");
  await d.locator('[data-act="dback"]').click();
  await expect(d).toHaveAttribute("data-id", "t-3");
  // ‹ was a step back, so Back now leaves the issue rather than reopening t-1.
  await page.goBack();
  await expect(d).toHaveCount(0);
  await expect(page.locator("#vTree")).toBeVisible();

  await row(page, "t-2").click();
  await expect(d).toHaveAttribute("data-id", "t-2");
  await d.locator('[data-act="dclose"]').click();
  await expect(d).toHaveCount(0);
  await page.goForward();
  await expect(d).toHaveAttribute("data-id", "t-2");
});

test("a link with a view and an issue opens them", async ({ page, project }) => {
  await page.goto(project.url + "#/board/t-2");
  await expect(page.locator("#boot")).toBeHidden({ timeout: 10000 });
  await expect(page.locator("#vBoard")).toBeVisible();
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-2");
  await page.goto(project.url + "#/tree/no-such-issue");
  await page.reload();
  await expect(page.locator("#boot")).toBeHidden({ timeout: 10000 });
  await expect(page.locator("#vTree")).toBeVisible();
  await expect(page.locator(".detail")).toHaveCount(0);
});
