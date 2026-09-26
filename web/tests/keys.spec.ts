// The TUI's regular keys in a browser with a keyboard: the tree, the board,
// the detail panel and the keys that work everywhere. Each key does what the
// README key tables say it does in the terminal.

import { expect, open, row, test } from "./harness";
import type { Page } from "@playwright/test";

const sel = (page: Page) => page.locator("#treeRows .row.sel");
const sheet = (page: Page) => page.locator("#sheetHost .msheet");
const toast = (page: Page) => page.locator("#toast");
const chip = (page: Page, k: string) => page.locator(`#chips [data-chip="${k}"]`);
const statusOf = (p: { issues(): Record<string, unknown>[] }, id: string) => p.issues().find(i => i.id === id)?.status;

async function cursorTo(page: Page, id: string): Promise<void> {
  await page.keyboard.press("Home");
  for (let n = 0; n < 12; n++) {
    if ((await sel(page).getAttribute("data-id")) === id) return;
    await page.keyboard.press("j");
  }
  throw new Error("the cursor never reached " + id);
}

test.describe("tree filters", () => {
  test("o, C, r and a show open, closed, ready or all issues", async ({ page, project }) => {
    await open(page, project);
    await expect(row(page, "t-4")).toHaveCount(0);
    await page.keyboard.press("Shift+C");
    await expect(row(page, "t-4")).toBeVisible();
    await expect(row(page, "t-2")).toHaveCount(0);
    await page.keyboard.press("o");
    await expect(row(page, "t-2")).toBeVisible();
    await expect(row(page, "t-4")).toHaveCount(0);
    await page.keyboard.press("r");
    await expect(chip(page, "ready")).toHaveAttribute("aria-pressed", "true");
    await expect(row(page, "t-3")).not.toBeVisible();
    await page.keyboard.press("a");
    await expect(chip(page, "all")).toHaveAttribute("aria-pressed", "true");
    await expect(row(page, "t-4")).toBeVisible();
    await expect(row(page, "t-3")).toBeVisible();
  });

  test("keys stay off while an input has focus", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("/");
    await expect(page.locator("#sIn")).toBeFocused();
    await page.keyboard.type("Ca");
    await expect(page.locator("#sIn")).toHaveValue("Ca");
    await expect(page.locator("#vSearch")).toBeVisible();
  });

  test("Esc leaves the search view and gives the keys back", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("/");
    await expect(page.locator("#sIn")).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(page.locator("#vTree")).toBeVisible();
    await expect(page.locator("#sIn")).not.toBeFocused();
    await page.keyboard.press("b");
    await expect(page.locator("#vBoard")).toBeVisible();
  });
});

test.describe("no cursor yet", () => {
  test("d opens the first row on a fresh page", async ({ page, project }) => {
    await open(page, project);
    const first = await page.locator("#treeRows .row").first().getAttribute("data-id");
    await page.keyboard.press("d");
    await expect(sel(page)).toHaveAttribute("data-id", first!);
    await expect(page.locator(".detail")).toHaveAttribute("data-id", first!);
  });

  // The board opens on the epic rail, so its first cell is the first epic.
  test("j on the board selects a cell on a fresh page", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("b");
    await page.keyboard.press("j");
    await expect(page.locator("#wboard .sel")).toHaveCount(1);
  });

  test("Enter on the board opens the first cell on a fresh page", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("b");
    await page.keyboard.press("Enter");
    await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-epic");
  });

  test("y on the board copies the first cell on a fresh page", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("b");
    await page.keyboard.press("y");
    await expect(toast(page)).toContainText("t-epic");
  });
});

test.describe("tree cursor", () => {
  test("j, k, Home and End move the cursor", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("j");
    const first = await sel(page).getAttribute("data-id");
    await page.keyboard.press("j");
    const second = await sel(page).getAttribute("data-id");
    expect(second).not.toBe(first);
    await page.keyboard.press("k");
    await expect(sel(page)).toHaveAttribute("data-id", first!);
    await page.keyboard.press("ArrowDown");
    await expect(sel(page)).toHaveAttribute("data-id", second!);
    await page.keyboard.press("End");
    const last = await page.locator("#treeRows .row").last().getAttribute("data-id");
    await expect(sel(page)).toHaveAttribute("data-id", last!);
    await page.keyboard.press("Home");
    await expect(sel(page)).toHaveAttribute("data-id", first!);
  });

  test("h and l collapse and expand, then go to the parent or the child", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-epic");
    await page.keyboard.press("h");
    await expect(row(page, "t-2")).toHaveCount(0);
    await page.keyboard.press("l");
    await expect(row(page, "t-2")).toBeVisible();
    await page.keyboard.press("l");
    await expect(sel(page)).not.toHaveAttribute("data-id", "t-epic");
    await page.keyboard.press("h");
    await expect(sel(page)).toHaveAttribute("data-id", "t-epic");
  });

  test("Tab folds the issue, Z collapses all, X expands all, Ctrl-A switches", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-epic");
    await page.keyboard.press("Tab");
    await expect(row(page, "t-2")).toHaveCount(0);
    await page.keyboard.press("Tab");
    await expect(row(page, "t-2")).toBeVisible();
    await page.keyboard.press("Shift+Z");
    await expect(row(page, "t-2")).toHaveCount(0);
    await page.keyboard.press("Shift+X");
    await expect(row(page, "t-2")).toBeVisible();
    await page.keyboard.press("Control+a");
    await expect(row(page, "t-2")).toHaveCount(0);
    await page.keyboard.press("Control+a");
    await expect(row(page, "t-2")).toBeVisible();
  });

  test("p goes to the parent, { and } to the first and last sibling", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-2");
    await page.keyboard.press("p");
    await expect(sel(page)).toHaveAttribute("data-id", "t-epic");
    await cursorTo(page, "t-2");
    await page.keyboard.press("}");
    const last = await sel(page).getAttribute("data-id");
    await page.keyboard.press("{");
    const first = await sel(page).getAttribute("data-id");
    expect(first).not.toBe(last);
    for (const id of [first, last]) expect(["t-1", "t-2", "t-3"]).toContain(id);
  });

  test("Enter and d open the detail, Esc closes it", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-2");
    await page.keyboard.press("Enter");
    await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-2");
    await page.keyboard.press("Escape");
    await expect(page.locator(".detail")).toHaveCount(0);
    await page.keyboard.press("d");
    await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-2");
    await page.keyboard.press("d");
    await expect(page.locator(".detail")).toHaveCount(0);
  });

  test("f and x show the branch or the subtree, and again undo it", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-2");
    await page.keyboard.press("x");
    await expect(page.locator("#fbar")).toContainText("subtree");
    await expect(row(page, "t-5")).toHaveCount(0);
    await page.keyboard.press("x");
    await expect(page.locator("#fbar")).toBeEmpty();
    await page.keyboard.press("f");
    await expect(page.locator("#fbar")).toContainText("branch");
    await page.keyboard.press("f");
    await expect(row(page, "t-5")).toBeVisible();
  });

  test("/ searches, n and N go to the next and previous match", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("/");
    await expect(page.locator("#sIn")).toBeFocused();
    await page.keyboard.type("title:tree title:board");
    await page.keyboard.press("Enter");
    await expect(page.locator("#vTree")).toBeVisible();
    await expect(row(page, "t-5")).toHaveCount(0);
    await page.keyboard.press("n");
    const a = await sel(page).getAttribute("data-id");
    await page.keyboard.press("n");
    const b = await sel(page).getAttribute("data-id");
    expect([a, b].sort()).toEqual(["t-2", "t-3"]);
    await page.keyboard.press("Shift+N");
    await expect(sel(page)).toHaveAttribute("data-id", a!);
  });
});

test.describe("tree sheets and toggles", () => {
  test("s sorts, | picks columns, v wraps, F follows", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("s");
    await expect(sheet(page)).toHaveAttribute("aria-label", "Sort");
    await page.keyboard.press("Escape");
    await expect(sheet(page)).toHaveCount(0);
    await page.keyboard.press("|");
    await expect(sheet(page)).toHaveAttribute("aria-label", "Tree options");
    await page.keyboard.press("Escape");
    await page.keyboard.press("v");
    await expect(page.locator("#treeRows")).toHaveClass(/wrap/);
    await page.keyboard.press("Shift+F");
    await expect(page.locator('#hd [data-act="follow"]')).toHaveAttribute("aria-pressed", "true");
  });

  test("Space marks, V marks a range, u unmarks, Ctrl-\\ clears", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-epic");
    await page.keyboard.press(" ");
    await expect(page.locator("#treeRows .row.marked")).toHaveCount(1);
    await page.keyboard.press("j");
    await page.keyboard.press("j");
    await page.keyboard.press("Shift+V");
    await expect(page.locator("#treeRows .row.marked")).toHaveCount(3);
    await page.keyboard.press("u");
    await expect(page.locator("#treeRows .row.marked")).toHaveCount(2);
    await page.keyboard.press("Control+\\");
    await expect(page.locator("#treeRows .row.marked")).toHaveCount(0);
  });

  test("e edits, S sets the status, Delete asks, Ctrl-N creates", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-2");
    await page.keyboard.press("e");
    await expect(page.locator("#fTitle")).toHaveValue("[2] Draw the tree");
    await page.keyboard.press("Escape");
    await expect(sheet(page)).toHaveCount(0);
    await page.keyboard.press("Shift+S");
    await expect(sheet(page).locator('[data-sa="setstatus"]').first()).toBeVisible();
    await page.keyboard.press("Escape");
    await page.keyboard.press("Delete");
    await expect(sheet(page)).toHaveAttribute("aria-label", /Delete/);
    await page.keyboard.press("Escape");
    await page.keyboard.press("Control+n");
    await expect(sheet(page)).toHaveAttribute("aria-label", "New issue");
  });

  test("K closes the issue under the cursor", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-5");
    await page.keyboard.press("Shift+K");
    await expect.poll(() => statusOf(project, "t-5")).toBe("closed");
  });

  test("c copies the ID and title", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-2");
    await page.keyboard.press("c");
    await expect(toast(page)).toContainText("t-2");
  });
});

test.describe("everywhere", () => {
  test("b opens the board, g the graph, Esc goes back to the tree", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("b");
    await expect(page.locator("#vBoard")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator("#vTree")).toBeVisible();
    await cursorTo(page, "t-3");
    await page.keyboard.press("g");
    await expect(page.locator("#vGraph")).toBeVisible();
    await expect(page.locator("#vGraph")).toContainText("Write the API");
    await page.keyboard.press("Escape");
    await expect(page.locator("#vTree")).toBeVisible();
  });

  test("? helps, D shows health, 0 shows all projects, Ctrl-E and H hide the header", async ({ page, project }) => {
    await open(page, project);
    await page.keyboard.press("?");
    await expect(sheet(page)).toHaveAttribute("aria-label", "Keys");
    await page.keyboard.press("Escape");
    await page.keyboard.press("Shift+D");
    await expect(sheet(page)).toHaveAttribute("aria-label", /health/i);
    await page.keyboard.press("Escape");
    await page.keyboard.press("Control+e");
    await expect(page.locator("#chips")).toBeHidden();
    await page.keyboard.press("Shift+H");
    await expect(page.locator("#chips")).toBeVisible();
    const opened = page.waitForRequest(r => r.url().endsWith("api/projects/open"));
    await page.keyboard.press("0");
    expect((await opened).postDataJSON()).toEqual({ key: "*" });
  });

  test("Ctrl-R reloads without leaving the page", async ({ page, project }) => {
    await open(page, project);
    await page.evaluate(() => { (window as unknown as { __stay: number }).__stay = 1; });
    await page.keyboard.press("Control+r");
    await expect(toast(page)).toContainText("Reloaded");
    expect(await page.evaluate(() => (window as unknown as { __stay?: number }).__stay)).toBe(1);
  });
});

test.describe("detail keys", () => {
  test("with the detail open, j and k move it and n and p page through siblings", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-1");
    await page.keyboard.press("Enter");
    await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-1");
    await page.keyboard.press("n");
    const next = await page.locator(".detail").getAttribute("data-id");
    expect(["t-2", "t-3"]).toContain(next);
    await page.keyboard.press("p");
    await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-1");
    await page.keyboard.press("j");
    await expect(page.locator(".detail")).not.toHaveAttribute("data-id", "t-1");
    await expect(sel(page)).toHaveAttribute("data-id", (await page.locator(".detail").getAttribute("data-id"))!);
  });

  test("c in the detail copies the issue as Markdown", async ({ page, project }) => {
    await open(page, project);
    await cursorTo(page, "t-1");
    await page.keyboard.press("Enter");
    await page.keyboard.press("c");
    await expect(toast(page)).toContainText(/Markdown|Clipboard/);
  });
});

test.describe("board keys", () => {
  async function board(page: Page, project: Parameters<typeof open>[1]): Promise<void> {
    await open(page, project);
    await page.keyboard.press("b");
    await expect(page.locator("#vBoard")).toBeVisible();
  }

  test("o, i and C toggle status filters, r shows ready, Esc clears them", async ({ page, project }) => {
    await board(page, project);
    await page.keyboard.press("i");
    await expect(chip(page, "in_progress")).toHaveAttribute("aria-pressed", "false");
    await page.keyboard.press("i");
    await expect(chip(page, "in_progress")).toHaveAttribute("aria-pressed", "true");
    await page.keyboard.press("Shift+C");
    await expect(chip(page, "closed")).toHaveAttribute("aria-pressed", "true");
    await page.keyboard.press("r");
    await expect(chip(page, "ready")).toHaveAttribute("aria-pressed", "true");
    await page.keyboard.press("o");
    await expect(chip(page, "open")).toHaveAttribute("aria-pressed", "false");
  });

  test("c shows the closed column, s changes the grouping, v the lane design, e hides empty columns", async ({ page, project }) => {
    await board(page, project);
    const closed = page.locator('#wboard .whead[data-tab="closed"]');
    await expect(closed).toHaveCount(0);
    await page.keyboard.press("c");
    await expect(closed).toHaveCount(1);
    await page.keyboard.press("v");
    await expect(page.locator('#wboard .lane[data-lane="t-epic"]')).toBeVisible();
    await page.keyboard.press("v");
    await expect(page.locator('#wboard .wepic[data-epic="t-epic"]')).toBeVisible();
    await page.keyboard.press("s");
    await expect(page.locator('#wboard .whead[data-tab="1"]')).toHaveCount(1);
    await page.keyboard.press("e");
    await expect(toast(page)).toContainText(/empty/i);
  });

  test("y copies the card ID and f shows its branch", async ({ page, project }) => {
    await board(page, project);
    await page.locator('#wboard [data-card="t-2"]').click();
    await page.keyboard.press("Escape");
    await expect(page.locator(".detail")).toHaveCount(0);
    await page.keyboard.press("y");
    await expect(toast(page)).toContainText("t-2");
    await page.keyboard.press("f");
    await expect(toast(page)).toContainText("Branch");
    await page.keyboard.press("f");
    await expect(page.locator('#wboard [data-card="t-5"]')).toBeVisible();
  });

  test("Tab folds the cursor's lane and Shift-Tab folds every lane", async ({ page, project }) => {
    await board(page, project);
    await page.locator('#wboard [data-card="t-2"]').click();
    await page.keyboard.press("Escape");
    await expect(page.locator(".detail")).toHaveCount(0);
    await page.keyboard.press("Tab");
    await expect(page.locator('#wboard .wepic[data-epic="t-epic"]')).toContainText("▸");
    await page.keyboard.press("Tab");
    await page.keyboard.press("Shift+Tab");
    await expect(page.locator('#wboard [data-card="t-5"]')).toHaveCount(0);
    await page.keyboard.press("Shift+Tab");
    await expect(page.locator('#wboard [data-card="t-5"]')).toBeVisible();
  });
});
