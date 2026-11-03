import type { Page } from "@playwright/test";
import { expect, open, test } from "./harness";

test.beforeEach(async ({ page }) => {
  page.on("pageerror", error => { throw error; });
});

async function tokenColor(page: Page, token: string): Promise<string> {
  return page.evaluate(t => {
    const el = document.createElement("span");
    el.style.color = `var(${t})`;
    document.body.append(el);
    const color = getComputedStyle(el).color;
    el.remove();
    return color;
  }, token);
}

async function tokenBackground(page: Page, token: string): Promise<string> {
  return page.evaluate(t => {
    const el = document.createElement("span");
    el.style.background = `var(${t})`;
    document.body.append(el);
    const color = getComputedStyle(el).backgroundColor;
    el.remove();
    return color;
  }, token);
}

const memoryGraph = {
  version: 1,
  available: true,
  nodes: [
    { id: "mem-1", kind: "memory", title: "Responsive search notes", status: "", type: "s/types/preview-memory-v2", record_version: "v1", body: "Search and projects stay neutral." },
    { id: "d-task", kind: "issue", title: "Render Markdown detail", status: "open", type: "s/types/preview-issue-v2", record_version: "v2", body: "Work description." },
    { id: "d-blocker", kind: "issue", title: "Resolve relation", status: "in_progress", type: "s/types/preview-issue-v2", record_version: "v3", body: "Blocking work." },
  ],
  edges: [
    { id: "follow", source: "d-task", target: "mem-1", kind: "follows", type: "s/types/example-follows", note: "" },
    { id: "block", source: "d-task", target: "d-blocker", kind: "blocks", type: "s/types/preview-blocks-v1", note: "" },
  ],
};

test.use({
  fixture: {
    issues: [
      { id: "d-epic", title: "[d] Epic: Calm board detail", type: "epic", priority: 1, description: "The board groups work by epic lanes." },
      { id: "d-feature", title: "[d.1] Feature shell", type: "feature", parent: "d-epic", status: "in_progress", priority: 1 },
      { id: "d-blocker", title: "[d.2] Resolve relation", parent: "d-epic", status: "in_progress", priority: 1 },
      {
        id: "d-task",
        title: "[d.1.1] Render Markdown detail",
        parent: "d-feature",
        blockedBy: ["d-blocker"],
        priority: 1,
        labels: ["ui"],
        description: "### Markdown\n\nServe **Markdown** with `code` in the calm detail pane.\n\n```mermaid\ngraph LR\nA[Detail] --> B[Calm]\n```",
        comments: ["Comment text keeps the same neutral detail treatment."],
      },
    ],
  },
});

test("board cards, epic lanes and rich detail use the calm look in this size class", async ({ page, project }) => {
  await open(page, project);
  const muted = await tokenColor(page, "--muted");
  const text = await tokenColor(page, "--text");
  const textSoft = await tokenColor(page, "--text-soft");
  const dangerColor = await tokenColor(page, "--danger");

  await page.locator('#bar [data-view="board"]').click();
  const card = page.locator('[data-card="d-task"]');
  await expect(card).toBeVisible();
  await expect(card).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await expect(card).toHaveCSS("border-top-width", "0px");
  await expect(card.locator(".tags .tp")).toHaveCSS("color", muted);
  await expect.poll(() => card.locator(".st").evaluate(el => getComputedStyle(el).backgroundColor)).toBe(dangerColor);

  const lane = page.locator('.lane[data-lane="d-epic"], .wepic[data-epic="d-epic"]').first();
  await expect(lane).toBeVisible();
  await expect(lane).toHaveCSS("color", muted);

  await card.click();
  const detail = page.locator(".detail");
  await expect(detail).toHaveAttribute("data-id", "d-task");
  await expect(detail.locator(".md h3").first()).toHaveCSS("color", text);
  await expect(detail.locator(".md code").first()).toHaveCSS("color", textSoft);
  await expect(detail.locator(".mermaid-diagram svg")).toBeVisible();
  await expect(detail.locator('.rel[data-nav="d-feature"]')).toContainText("Feature shell");
  await expect(detail.locator('.rel[data-nav="d-blocker"]')).toContainText("Resolve relation");
  await expect(detail.locator(".cm", { hasText: "Comment text" })).toBeVisible();
  await expect(detail.locator(".cm .who b").first()).toHaveCSS("color", textSoft);
  await expect(detail.locator(".pill").filter({ hasText: /open/ })).toHaveCSS("color", muted);
  await expect.poll(() => detail.locator(".pill.status .sdot").evaluate(el => getComputedStyle(el).backgroundColor)).toBe(dangerColor);
});

test.describe("title-focused board rows", () => {
  test.use({
    fixture: {
      issues: [
        { id: "c-epic", title: "Epic: Compact checkout", type: "epic", description: "Ship checkout. More context.", labels: ["checkout"], assignee: "alice" },
        { id: "c-parent", title: "Checkout service", type: "feature", parent: "c-epic", status: "in_progress", priority: 1 },
        ...Array.from({ length: 16 }, (_, n) => ({
          id: `c-${n}`, title: `Checkout task ${n}`, parent: "c-parent",
          status: n < 8 ? "open" : "in_progress", priority: 2,
          labels: ["ui", "lane-stage=tests"], assignee: "bob", description: "Complete issue context.",
        })),
      ],
    },
  });

  test("density, selected rows and epic context survive opening issue details", async ({ page, project, hasTouch }, info) => {
    await open(page, project);
    await page.locator('#bar [data-view="board"]').click();
    const row = page.locator('[data-card="c-0"]');
    await expect(row).toBeVisible();
    await expect(row.locator(".l2, .l3, .tags")).toHaveCount(0);
    await expect(row).toHaveCSS("border-top-width", "0px");
    await expect(row).toHaveCSS("height", hasTouch ? "44px" : "24px");
    const rowHeight = (await row.boundingBox())!.height;
    const phone = await page.locator("#app").evaluate(el => el.classList.contains("phone"));
    const epic = page.locator('[data-epic="c-epic"]');
    if (phone) {
      await expect(epic).toHaveCount(0);
      const lanes = page.locator('.lane[data-lane="c-epic"]');
      await expect(lanes).toHaveCount(2);
      await expect(lanes.first()).toBeVisible();
      await expect(lanes.first().locator(".t")).toContainText("Compact checkout");
    } else {
      await expect(epic).toBeVisible();
      await expect(epic.locator(".es")).toHaveText("Ship checkout.");
      await expect(epic.locator(".en")).toHaveText("0/17 · P1 1");
      await expect(epic.locator(".ec")).toHaveText("wip 9·ready 8");
      await expect(epic.locator(".em")).toContainText("#checkout");
      await expect(epic.locator(".em")).toContainText("@alice");
    }
    await info.attach("compact-board", { body: await page.screenshot(), contentType: "image/png" });
    await row.click();
    await expect(row).toHaveClass(/sel/);
    await expect(row).toHaveCSS("height", hasTouch ? "44px" : "24px");
    await expect(row.locator(".l2, .l3, .tags")).toHaveCount(0);
    const detail = page.locator('.detail[data-id="c-0"]');
    await expect(detail).toContainText("Complete issue context.");
    await expect(detail).toContainText("bob");
    await expect(detail).toContainText("lane-stage=tests");
    const parent = detail.locator('[data-nav="c-parent"]');
    await expect(parent).toContainText("Checkout service");
    await info.attach("selected-board", { body: await page.screenshot(), contentType: "image/png" });
    await parent.click();
    await expect(page.locator(".detail")).toHaveAttribute("data-id", "c-parent");
    await info.attach("row-density", {
      body: JSON.stringify({ rowHeight, rowsPer240px: Math.floor(240 / rowHeight), touchTarget: hasTouch, phoneLayout: phone }),
      contentType: "application/json",
    });
  });
});

test("search, sheets, projects and Memory use the calm look in this size class", async ({ page, project }) => {
  await page.route("**/api/memory-graph", route => route.fulfill({ json: memoryGraph }));
  await open(page, project);
  const text = await tokenColor(page, "--text");
  const textSoft = await tokenColor(page, "--text-soft");
  const muted = await tokenColor(page, "--muted");
  const danger = await tokenColor(page, "--danger");
  const surface1 = await tokenBackground(page, "--surface-1");
  const raised = await tokenBackground(page, "--surface-raised");

  await page.locator('#bar [data-view="search"]').click();
  await expect(page.locator("#sIn")).toHaveCSS("background-color", surface1);
  await expect(page.locator("#sSugg .chip").first()).toHaveCSS("color", textSoft);
  await page.locator("#sIn").fill("status:");
  await expect(page.locator("#sSugg .chip.v").first()).toHaveCSS("color", text);

  await page.locator('#hd [data-act="create"]').click();
  const sheet = page.locator("#sheetHost .msheet");
  await expect(sheet).toHaveAttribute("aria-label", "New issue");
  await expect(sheet.locator(".form input").first()).toHaveCSS("background-color", surface1);
  await expect(sheet.locator('[data-seg="priority"] .segDot')).toHaveCount(5);
  await expect(sheet.locator('[data-seg="type"] .segDot')).toHaveCount(0);
  await sheet.locator('[data-sa="cancel"]').click();
  await expect(sheet).toHaveCount(0);

  await page.locator('#hd [data-act="projects"]').click();
  await expect(sheet.locator('[data-sa="proj"]').first()).toBeVisible();
  await expect(sheet.locator('[data-sa="proj"] .g').first()).toHaveCSS("color", muted);
  await expect(sheet.locator('[data-sa="proj"].cur')).toHaveCSS("color", text);
  await page.keyboard.press("Escape");
  await expect(sheet).toHaveCount(0);

  await page.locator("#hdot").click();
  await expect(sheet).toHaveAttribute("aria-label", "Data source health");
  await expect(sheet.locator(".gtable .ok")).toHaveCSS("color", textSoft);
  await sheet.locator('[data-sa="cancel"]').click();
  await expect(sheet).toHaveCount(0);

  await page.locator('#bar [data-view="memory"]').click();
  await expect(page.locator("#vMemory [data-node]")).toHaveCount(3);
  await expect(page.locator("#vMemory .memoryTools")).toHaveCSS("background-color", surface1);
  await expect(page.locator("#vMemory .memoryLegend")).not.toContainText("Drag");
  await expect(page.locator('[data-node="mem-1"] .memoryCard')).toHaveCSS("fill", raised);
  await expect(page.locator('[data-edge="follow"] > path')).toHaveCSS("stroke", muted);
  await expect(page.locator('[data-edge="block"] > path')).toHaveCSS("stroke", danger);
});
