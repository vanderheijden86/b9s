import { expect, open, row, test } from "./harness";

test.describe("reading", () => {
  test("tree nests children under their epic and hides closed issues", async ({ page, project }) => {
    await open(page, project);
    await expect(row(page, "t-epic")).toBeVisible();
    await expect(row(page, "t-1")).toBeVisible();
    await expect(row(page, "t-4")).toHaveCount(0);
    const ids = await page.locator("#treeRows .row").evaluateAll(els => els.map(e => (e as HTMLElement).dataset.id));
    expect(ids.indexOf("t-epic")).toBeLessThan(ids.indexOf("t-1"));
  });

  test("tapping the chevron folds the epic", async ({ page, project }) => {
    await open(page, project);
    await row(page, "t-epic").locator("[data-chev]").click();
    await expect(row(page, "t-1")).toHaveCount(0);
    await row(page, "t-epic").locator("[data-chev]").click();
    await expect(row(page, "t-1")).toBeVisible();
  });

  test("the Closed chip shows closed issues", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-chip="closed"]').click();
    await expect(row(page, "t-4")).toBeVisible();
  });

  test("a query narrows the tree through the server", async ({ page, project }) => {
    await open(page, project);
    await page.locator('#bar [data-view="search"]').click();
    await page.locator("#sIn").fill("type:chore");
    await expect(page.locator('#sList .row[data-id="t-5"]')).toBeVisible();
    await expect(page.locator('#sList .row[data-id="t-1"]')).toHaveCount(0);
    await page.locator("#sApply").click();
    await expect(page.locator("#vTree")).toBeVisible();
    await expect(row(page, "t-5")).toBeVisible();
    await expect(row(page, "t-2")).toHaveCount(0);
  });

  test("detail shows markdown, comments and relations, and a relation navigates", async ({ page, project }) => {
    await open(page, project);
    await row(page, "t-3").click();
    const d = page.locator(".detail");
    await expect(d).toHaveAttribute("data-id", "t-3");
    await expect(d.locator(".dsec", { hasText: "Blocked by 1" })).toBeVisible();
    await d.locator('.rel[data-nav="t-1"]').click();
    await expect(d).toHaveAttribute("data-id", "t-1");
    await expect(d.locator(".md b", { hasText: "JSON" })).toBeVisible();
    await expect(d.locator(".cm", { hasText: "Started on the handlers." })).toBeVisible();
    await d.locator('[data-act="dback"]').click();
    await expect(d).toHaveAttribute("data-id", "t-3");
    await d.locator('[data-act="dclose"]').click();
    await expect(page.locator(".detail")).toHaveCount(0);
  });

  test("graph view shows the blocker chain", async ({ page, project }) => {
    await open(page, project);
    await row(page, "t-3").click();
    await page.locator('.detail [data-act="dgraph"]').first().click();
    await expect(page.locator("#vGraph")).toBeVisible();
    await expect(page.locator("#vGraph")).toContainText("Blocked by");
    await expect(page.locator('#vGraph .row[data-id="t-1"]')).toBeVisible();
  });

  test("health sheet reports the JSONL source and bd", async ({ page, project }) => {
    await open(page, project);
    await page.locator("#hdot").click();
    const sheet = page.locator(".msheet");
    await expect(sheet).toContainText("Data source health");
    await expect(sheet).toContainText("found on the server");
    await expect(sheet).toContainText(/jsonl/i);
  });

  test("gesture help lists every gesture", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-act="help"]').click();
    await expect(page.locator(".msheet .gtable tr")).toHaveCount(21);
  });
});
