import http from "node:http";
import net from "node:net";
import { expect, freePort, open, row, SAMPLE, startProject, stopProject, test } from "./harness";

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

  test("each row names its status first on the second line", async ({ page, project }) => {
    await open(page, project);
    await expect(row(page, "t-1").locator(".l2 .stw")).toHaveText("in progress");
    await expect(row(page, "t-3").locator(".l2 .stw")).toHaveText("blocked");
    await expect(row(page, "t-2").locator(".l2 > span").first()).toHaveText("open");
  });

  test("a saved column choice from before the status column still shows status", async ({ page, project }) => {
    await page.addInitScript(() => localStorage.setItem("b9s.cols", JSON.stringify(["prio"])));
    await open(page, project);
    await expect(row(page, "t-2").locator(".l2 .stw")).toHaveText("open");
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

  // The harness builds b9s with go build in this checkout, so the binary
  // carries the commit it was built from.
  test("about sheet names the build and links its commit on GitHub", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-view="more"]').click();
    const item = page.locator('#moreList [data-act="about"]');
    await expect(item.locator(".v")).toHaveText(/ · [0-9a-f]{8}/);
    await item.click();
    const sheet = page.locator(".msheet");
    await expect(sheet).toContainText("b9s version");
    const link = sheet.locator("a");
    await expect(link).toHaveAttribute("href", /^https:\/\/github\.com\/vanderheijden86\/b9s\/commit\/[0-9a-f]{40}$/);
    await expect(link).toHaveAttribute("target", "_blank");
  });

  test("gesture help lists every gesture", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-act="help"]').click();
    await expect(page.locator(".msheet .gtable tr")).toHaveCount(25);
  });
});

test.describe("harness", () => {
  // A parallel test can take the port between the probe and the bind. The
  // server already on it answers health checks, so readiness must come from
  // the started process, or a test reads another test's issues.
  test("a project never reaches a server it did not start", async () => {
    const squatter = http.createServer((_, res) => { res.writeHead(200); res.end("{}"); });
    await new Promise<void>(r => squatter.listen(0, "127.0.0.1", () => r()));
    const taken = (squatter.address() as net.AddressInfo).port;
    const ports = [taken];
    const p = await startProject(SAMPLE, {}, false, async () => ports.shift() ?? freePort());
    try {
      expect(p.url).not.toContain(`:${taken}/`);
      expect(p.log()).toContain("b9s web is serving");
    } finally {
      stopProject(p);
      squatter.close();
    }
  });
});
