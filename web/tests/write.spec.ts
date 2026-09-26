import { expect, longPress, open, row, swipe, test } from "./harness";

const statusOf = (p: { issues(): Record<string, unknown>[] }, id: string) => p.issues().find(i => i.id === id)?.status;

test.describe("writing through bd", () => {
  test("short swipe right starts an issue", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-2", 0.35);
    await expect.poll(() => statusOf(project, "t-2")).toBe("in_progress");
    expect(project.calls()).toContainEqual(["update", "t-2", "--status=in_progress"]);
  });

  test("short swipe left closes, and undo restores the status", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-5", -0.35);
    await expect.poll(() => statusOf(project, "t-5")).toBe("closed");
    await page.locator('#toast [data-act="undo"]').click();
    await expect.poll(() => statusOf(project, "t-5")).toBe("open");
  });

  test("long swipe right opens the status picker; closed asks for a reason", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-2", 0.65);
    await page.locator('.msheet [data-sa="setstatus"][data-val="closed"]').click();
    await page.locator("#fReason").fill("Shipped in the web UI");
    await page.locator('[data-sa="doclose"]').click();
    await expect.poll(() => project.issues().find(i => i.id === "t-2")?.close_reason).toBe("Shipped in the web UI");
  });

  test("edit sends only changed fields", async ({ page, project }) => {
    await open(page, project);
    await row(page, "t-2").click();
    await page.locator('.detail [data-act="edit"]').click();
    await expect(page.locator("#fTitle")).toHaveValue("[2] Draw the tree");
    await page.locator("#fTitle").fill("[2] Draw the tree fast");
    await page.locator('[data-seg="priority"] [data-v="0"]').click();
    await page.locator("#fLabels").fill("ui, perf");
    await page.locator('[data-sa="save"]').click();
    await expect.poll(() => project.issues().find(i => i.id === "t-2")?.title).toBe("[2] Draw the tree fast");
    const call = project.calls().find(c => c[0] === "update")!;
    expect(call).toContain("--priority=0");
    expect(call).toContain("--add-label=perf");
    expect(call.some(a => a.startsWith("--description"))).toBe(false);
    await expect(page.locator(".detail .dtitle")).toHaveText("Draw the tree fast");
  });

  test("comment is added and shown", async ({ page, project }) => {
    await open(page, project);
    await row(page, "t-2").click();
    await page.locator('.detail .dacts [data-act="comment"]').click();
    await page.locator("#fComment").fill("Looks good on the phone");
    await page.locator('[data-sa="docomment"]').click();
    await expect(page.locator(".detail .cm", { hasText: "Looks good on the phone" })).toBeVisible();
  });

  test("create a child from the action sheet", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-epic", -0.65);
    await page.locator('.msheet [data-sa="child"]').click();
    await page.locator("#fTitle").fill("[6] Offline banner");
    await page.locator('[data-sa="docreate"]').click();
    await expect(page.locator("#treeRows .row", { hasText: "Offline banner" })).toBeVisible();
    expect(project.calls().find(c => c[0] === "create")).toContain("--parent=t-epic");
  });

  test("defer with a date", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-5", 0.65);
    await page.locator('.msheet [data-sa="defer"]').click();
    await page.locator("#fUntil").fill("2030-01-15");
    await page.locator('[data-sa="dodefer"]').click();
    await expect.poll(() => statusOf(project, "t-5")).toBe("deferred");
  });

  test("long-press marks rows and the bulk bar closes them", async ({ page, project }) => {
    await open(page, project);
    await longPress(page, '#treeRows .row[data-id="t-2"]');
    await expect(page.locator("#bar.bulk")).toBeVisible();
    await row(page, "t-5").click();
    await expect(page.locator("#bar .cnt")).toContainText("2 marked");
    await page.locator('#bar [data-act="bulkclose"]').click();
    await page.locator('[data-sa="doclose"]').click();
    await expect.poll(() => [statusOf(project, "t-2"), statusOf(project, "t-5")]).toEqual(["closed", "closed"]);
  });

  test("delete asks first", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-5", -0.65);
    await page.locator('.msheet [data-sa="delete"]').click();
    await expect(page.locator(".msheet .cmd")).toHaveText("bd delete t-5 --force");
    await page.locator('[data-sa="dodelete"]').click();
    await expect(row(page, "t-5")).toHaveCount(0);
    expect(project.issues().some(i => i.id === "t-5")).toBe(false);
  });
});

test.describe("a failing bd", () => {
  test.use({ bdEnv: { FAKE_BD_FAIL: "update" } });
  test("shows the error and keeps the old status", async ({ page, project }) => {
    await open(page, project);
    await swipe(page, "t-2", 0.35);
    await expect(page.locator("#toast")).toContainText(/fail|refused/i);
    await expect(row(page, "t-2").locator(".st")).not.toHaveText("◐");
    expect(statusOf(project, "t-2")).toBe("open");
  });
});
