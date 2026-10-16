import http from "node:http";
import net from "node:net";
import { expect, freePort, open, row, SAMPLE, settled, startProject, stopProject, test } from "./harness";

test.describe("reading", () => {
  test("comments default to newest first and the order choice survives reload", async ({ page, project }, testInfo) => {
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.route("**/api/issue?id=t-1", async route => {
      const response = await route.fetch();
      const issue = await response.json();
      issue.comments = [
        { id: "1", author: "alice", text: "Old comment", created_at: "2026-01-01T00:00:00Z" },
        { id: "3", author: "alice", text: "New comment", created_at: "2026-01-03T00:00:00Z" },
        { id: "2", author: "alice", text: "Middle comment", created_at: "2026-01-02T00:00:00Z" },
      ];
      await route.fulfill({ response, json: issue });
    });
    await open(page, project);
    await row(page, "t-1").click();
    const comments = page.locator(".detail .cm .md");
    await expect(comments).toHaveText(["New comment", "Middle comment", "Old comment"]);
    await page.getByRole("button", { name: "Newest first", exact: true }).click();
    await expect(comments).toHaveText(["Old comment", "Middle comment", "New comment"]);
    await page.reload();
    await settled(page);
    await row(page, "t-1").click();
    await expect(comments).toHaveText(["Old comment", "Middle comment", "New comment"]);
    await page.getByRole("button", { name: "Oldest first", exact: true }).click();
    await expect(comments).toHaveText(["New comment", "Middle comment", "Old comment"]);
    expect(errors).toEqual([]);
    for (const [width, size] of [[390, "phone"], [820, "tablet"], [1440, "desktop"], [2560, "wide"]] as const) {
      await page.setViewportSize({ width, height: 900 });
      await expect(page.locator("#app")).toHaveClass(new RegExp(`\\b${size}\\b`));
      const order = page.getByRole("button", { name: "Newest first", exact: true });
      await order.scrollIntoViewIfNeeded();
      await expect(order).toBeVisible();
      await page.screenshot({ path: testInfo.outputPath(`comment-order-${width}.png`) });
    }
  });

  // The live stream re-renders the tree when it connects. A test that acts
  // before then can hold a row that the render replaces.
  test("open returns once the live stream has connected", async ({ page, project }) => {
    await open(page, project);
    await expect(page.locator("body")).toHaveAttribute("data-live", "live");
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
    await d.locator('[aria-label="Close"]').click();
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

test.describe("type chips", () => {
  test.use({ fixture: { issues: [
    ...SAMPLE,
    { id: "t-feature", title: "New feature", type: "feature" },
    { id: "t-bug", title: "Open bug", type: "bug" },
  ] } });

  for (const [type, ids] of [
    ["epic", ["t-epic"]],
    ["feature", ["t-feature"]],
    ["task", ["t-1", "t-2", "t-3"]],
    ["bug", ["t-bug"]],
    ["chore", ["t-5"]],
  ] as const) {
    test(`${type} toggles its query term and matching rows`, async ({ page, project }, testInfo) => {
      const errors: string[] = [];
      page.on("pageerror", error => errors.push(error.message));
      page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
      await open(page, project);
      const chip = page.locator(`[data-chip="type:${type}"]`);
      await chip.click();
      await expect(chip).toHaveAttribute("aria-pressed", "true");
      await expect(page.locator("#chips .q")).toHaveText(`⌕ type:${type}`);
      await expect.poll(() => page.locator("#treeRows .row:not(.dim)").evaluateAll(
        rows => rows.map(row => (row as HTMLElement).dataset.id).sort(),
      )).toEqual([...ids]);
      for (const id of ids) await expect(row(page, id).locator(".t")).toBeVisible();
      await expect(row(page, "t-4")).toHaveCount(0);
      if (type === "bug") await page.screenshot({ path: testInfo.outputPath("type-chip.png") });
      await chip.click();
      await expect(chip).toHaveAttribute("aria-pressed", "false");
      await expect(page.locator("#chips .q")).toHaveCount(0);
      await expect(row(page, "t-bug")).toBeVisible();
      await expect(row(page, "t-feature")).toBeVisible();
      expect(errors).toEqual([]);
    });
  }

  test("selecting another type replaces the previous type", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-chip="type:epic"]').click();
    await page.locator('[data-chip="type:feature"]').click();
    await expect(page.locator('[data-chip="type:epic"]')).toHaveAttribute("aria-pressed", "false");
    await expect(page.locator("#chips .q")).toHaveText("⌕ type:feature");
    await expect(row(page, "t-feature")).toBeVisible();
    await expect(row(page, "t-epic")).toHaveCount(0);
  });

  test("a type chip synchronizes an open search query", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-chip="type:epic"]').click();
    await page.locator('#bar [data-view="search"]').click();
    await page.locator('[data-chip="type:feature"]').click();
    await expect(page.locator("#sIn")).toHaveValue("type:feature");
    await expect(page.locator('#sList .row[data-id="t-feature"]')).toBeVisible();
    await expect(page.locator('#sList .row[data-id="t-epic"]')).toHaveCount(0);
    await page.locator("#sApply").click();
    await expect(row(page, "t-feature")).toBeVisible();
    await expect(row(page, "t-epic")).toHaveCount(0);
  });

  for (const query of [
    "status:open type:bug",
    "status:open TYPE:FEATURE",
    "status:open type:feature type:bug",
    "status:open !type:feature",
    "status:open type:feature !type:bug",
  ]) {
    test(`a chip replaces type predicates in ${query}`, async ({ page, project }) => {
      await open(page, project);
      await page.locator('#bar [data-view="search"]').click();
      await page.locator("#sIn").fill(query);
      await page.locator("#sApply").click();
      const chip = page.locator('[data-chip="type:feature"]');
      await expect(chip).toHaveAttribute("aria-pressed", String(/(^|\s)type:feature(\s|$)/i.test(query)));
      await chip.click();
      const expected = query === "status:open TYPE:FEATURE" ? "status:open" : "status:open type:feature";
      await expect(page.locator("#chips .q")).toHaveText(`⌕ ${expected}`);
      await expect(row(page, "t-feature")).toBeVisible();
      await expect(row(page, "t-1")).toHaveCount(0);
      await page.locator('#chips [data-act="search"]').click();
      await expect(page.locator("#sIn")).toHaveValue(expected);
    });
  }

  test("the query and selected type survive reload and clear together", async ({ page, project }) => {
    await open(page, project);
    await page.locator('[data-chip="closed"]').click();
    await page.locator('[data-chip="type:bug"]').click();
    await page.reload();
    await expect(page.locator("body")).toHaveAttribute("data-live", "live");
    await expect(page.locator('[data-chip="type:bug"]')).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator('[data-chip="closed"]')).toHaveAttribute("aria-pressed", "true");
    await expect(row(page, "t-4")).toBeVisible();
    await expect(row(page, "t-feature")).toHaveCount(0);
    await page.locator('[data-act="clearq"]').click();
    await expect(page.locator('[data-chip="type:bug"]')).toHaveAttribute("aria-pressed", "false");
    await expect(row(page, "t-feature")).toBeVisible();
    await page.reload();
    await expect(page.locator('[data-chip="type:bug"]')).toHaveAttribute("aria-pressed", "false");
    await expect(row(page, "t-feature")).toBeVisible();
  });

  test("the server startup query selects its type chip", async ({ page, project }) => {
    await page.route("**/api/session", async route => {
      const response = await route.fetch();
      await route.fulfill({ response, json: { ...await response.json(), query: "type:chore" } });
    });
    await open(page, project);
    await expect(page.locator('[data-chip="type:chore"]')).toHaveAttribute("aria-pressed", "true");
    await expect(row(page, "t-5")).toBeVisible();
    await expect(row(page, "t-bug")).toHaveCount(0);
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
