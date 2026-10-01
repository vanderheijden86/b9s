import { expect, open, row, test } from "./harness";

test.describe("reading", () => {
  // The live stream re-renders the tree when it connects. A test that acts
  // before then can hold a row that the render replaces.
  test("open returns once the live stream has connected", async ({ page, project }) => {
    await open(page, project);
    expect(await page.locator("body").getAttribute("data-live")).toBe("live");
  });

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
    await expect(page.locator(".msheet .gtable tr")).toHaveCount(25);
  });

  test("theme chooser switches and saves the browser appearance", async ({ page, project }) => {
    const browserErrors: string[] = [];
    page.on("pageerror", error => browserErrors.push(error.message));
    page.on("console", message => { if (message.type() === "error") browserErrors.push(message.text()); });
    await page.emulateMedia({ colorScheme: "dark" });
    await open(page, project);
    await page.locator('[data-act="theme"]').click();
    await page.locator('[data-sa="settheme"][data-val="light"]').click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await expect.poll(() => page.locator("body").evaluate(el => getComputedStyle(el).backgroundColor)).toBe("rgb(233, 223, 203)");
    await page.locator('#bar [data-view="board"]').click();
    await expect(page.locator("#vBoard")).toBeVisible();
    await page.locator('#bar [data-view="tree"]').click();
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    await page.locator('[data-act="theme"]').click();
    await page.locator('[data-sa="settheme"][data-val="dark"]').click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await page.locator('[data-act="theme"]').click();
    await page.locator('[data-sa="settheme"][data-val="auto"]').click();
    await page.emulateMedia({ colorScheme: "light" });
    await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
    expect(browserErrors).toEqual([]);
  });
});
