// The desktop and wide classes (docs/adr/0052), laid out for a mouse and a
// keyboard after the direction A mockup (docs/mockups/responsive-web/shots/a-desktop.png):
// a sidebar in place of the bottom bar, the detail as a pane beside the
// views, hover on rows and cards, a context menu with the swipe actions,
// sheets as dialogs, and a visible cursor and focus ring. The board's mouse
// drag between columns is in wide.spec.ts.

import type { Page } from "@playwright/test";
import { sizeClassOf } from "../src/util";
import { expect, open, row, SAMPLE, test } from "./harness";

const box = async (page: Page, sel: string) => (await page.locator(sel).boundingBox())!;
const menu = (page: Page) => page.locator("#menuHost .cmenu");

/** tokenBackground is a token's colour the way a computed background-color reports it. */
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

test.beforeEach(({ page }) => {
  test.skip(!["desktop", "wide"].includes(sizeClassOf(page.viewportSize()!.width)), "desktop and wide only");
});

test("the sidebar spans the height with the project, the views and the connection, and the header sits beside it", async ({ page, project }) => {
  await open(page, project);
  const app = await box(page, "#app"), bar = await box(page, "#bar"), hd = await box(page, "#hd"), chips = await box(page, "#chips"), tree = await box(page, "#treeList");
  expect(bar.x).toBeLessThanOrEqual(app.x + 1);
  expect(bar.height).toBeGreaterThanOrEqual(app.height - 1);
  expect(bar.width).toBeGreaterThanOrEqual(200);
  expect(bar.width).toBeLessThanOrEqual(240);
  for (const b of [hd, chips, tree]) expect(b.x).toBeGreaterThanOrEqual(bar.x + bar.width - 1);
  expect(hd.y).toBeLessThanOrEqual(app.y + 1);
  await expect(page.locator("#bar .sbrand b")).toHaveText("b9s");
  await expect(page.locator("#bar .sbrand small")).not.toBeEmpty();
  await expect(page.locator("#bar [data-view]")).toHaveCount(6);
  await expect(page.locator('#bar [data-view="tree"]')).toHaveAttribute("aria-current", "page");
  await expect(page.locator("#bar .sfoot .slive")).toHaveText("connected");
  // The glyph sits beside its label in one row of a mouse's height, not a stacked finger target.
  const g = await box(page, '#bar [data-view="board"] .g'), b = await box(page, '#bar [data-view="board"]');
  expect(g.x).toBeLessThan(b.x + b.width / 3);
  expect(b.height).toBeLessThan(44);
  // Rows keep the mockup's density. A row is content-visibility: auto with a 45 px placeholder, so a
  // read in the frame the rows were drawn still sees the placeholder: the real height needs a poll.
  const rowHeight = () => page.locator('#treeRows .row[data-id="t-2"]').evaluate(el => el.getBoundingClientRect().height);
  await expect.poll(rowHeight).toBeLessThanOrEqual(42);
  expect(await rowHeight()).toBeGreaterThanOrEqual(38);
  expect(await page.locator("#app").evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true);
});

test("the detail is a pane beside the tree and the board, below the chips, and \\ spreads it over the views", async ({ page, project }) => {
  await open(page, project);
  await row(page, "t-1").click();
  const d = page.locator(".detail");
  await expect(d).toHaveAttribute("data-id", "t-1");
  const app = await box(page, "#app"), bar = await box(page, "#bar"), chips = await box(page, "#chips");
  const cls = sizeClassOf(page.viewportSize()!.width);
  const minPane = cls === "wide" ? 520 : 400;
  const maxPane = cls === "wide" ? 700 : 460;
  const beside = async (view: string) => {
    const v = await box(page, view), p = await box(page, ".detail");
    return Math.abs(p.x - (v.x + v.width)) <= 1 && p.x + p.width >= app.x + app.width - 1
      && p.y >= chips.y + chips.height - 1 && p.height >= app.height * 0.8 && p.width >= minPane && p.width <= maxPane;
  };
  await expect.poll(() => beside("#treeList")).toBe(true);
  await expect(row(page, "t-2")).toBeInViewport();
  await page.locator('#bar [data-view="board"]').click();
  await expect.poll(() => beside("#wboard")).toBe(true);
  await page.keyboard.press("\\");
  await expect.poll(async () => { const p = await box(page, ".detail"); return Math.abs(p.x - (bar.x + bar.width)) <= 1 && p.x + p.width >= app.x + app.width - 1; }).toBe(true);
  await page.keyboard.press("\\");
  await expect.poll(() => beside("#wboard")).toBe(true);
  await page.keyboard.press("Escape");
  await expect(d).toHaveCount(0);
  const w = await box(page, "#wboard");
  expect(w.x + w.width).toBeGreaterThanOrEqual(app.x + app.width - 1);
});

test("a right click selects the row and opens a menu with the swipe actions; Close closes the issue with an undo", async ({ page, project }) => {
  await open(page, project);
  await row(page, "t-2").click({ button: "right" });
  const m = menu(page);
  await expect(m).toBeVisible();
  await expect(m).toHaveAttribute("data-id", "t-2");
  await expect(m).toBeFocused();
  await expect(page.locator("#treeRows .row.sel")).toHaveAttribute("data-id", "t-2");
  await expect(page.locator(".detail")).toHaveCount(0);
  await expect(m.locator("[data-cm]")).toHaveText([/Open/, /Start/, /Set status/, /Close/, /Edit/, /Comment/, /Copy ID/, /Mark/, /More/, /Delete/]);
  // By the right edge the menu moves left rather than off the app.
  const vp = page.viewportSize()!;
  const b = (await row(page, "t-3").boundingBox())!;
  await page.mouse.click(vp.width - 6, b.y + b.height / 2, { button: "right" });
  await expect(m).toHaveAttribute("data-id", "t-3");
  const mb = (await m.boundingBox())!;
  expect(mb.x + mb.width).toBeLessThanOrEqual(vp.width);
  await m.locator('[data-cm="close"]').click();
  await expect(m).toHaveCount(0);
  await expect.poll(() => project.issues().find(i => i.id === "t-3")?.status).toBe("closed");
  await expect(page.locator("#toast")).toContainText("UNDO");
});

test("the keyboard works the menu: arrows and j k walk it, Enter picks, Escape closes, Shift-F10 opens it for the cursor", async ({ page, project }) => {
  await open(page, project);
  await row(page, "t-3").click({ button: "right" });
  const m = menu(page);
  await expect(m).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(m.locator('[data-cm="open"]')).toBeFocused();
  await page.keyboard.press("j");
  await expect(m.locator('[data-cm="toggle"]')).toBeFocused();
  await page.keyboard.press("k");
  await expect(m.locator('[data-cm="open"]')).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(m).toHaveCount(0);
  await expect(page.locator("#treeRows .row.sel")).toHaveAttribute("data-id", "t-3");
  await expect(page.locator(".detail")).toHaveCount(0);
  await page.keyboard.press("Shift+F10");
  await expect(m).toHaveAttribute("data-id", "t-3");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(m).toHaveCount(0);
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-3");
  // A press anywhere else closes the menu without picking from it, and the open detail follows the right-clicked row.
  await row(page, "t-2").click({ button: "right" });
  await expect(m).toHaveAttribute("data-id", "t-2");
  await page.locator("#hd .cnt").click();
  await expect(m).toHaveCount(0);
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-2");
});

test("a right click on a card offers the same menu, Start moves the card, and a sheet opens as a dialog", async ({ page, project }) => {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  const card = page.locator('#wboard [data-card="t-2"]');
  await card.click({ button: "right" });
  const m = menu(page);
  await expect(m).toHaveAttribute("data-id", "t-2");
  await expect(card).toHaveClass(/sel/);
  await m.locator('[data-cm="toggle"]').click();
  await expect.poll(() => project.issues().find(i => i.id === "t-2")?.status).toBe("in_progress");
  await expect(page.locator('#wboard .wcell[data-dcol="in_progress"] [data-card="t-2"]')).toBeVisible();
  await card.click({ button: "right" });
  await m.locator('[data-cm="status"]').click();
  const sheet = page.locator("#sheetHost .msheet");
  await expect(sheet).toContainText("Status");
  const sb = await box(page, "#sheetHost .msheet"), app = await box(page, "#app");
  expect(sb.width).toBeLessThanOrEqual(560);
  expect(Math.abs(sb.x - app.x - (app.x + app.width - sb.x - sb.width))).toBeLessThanOrEqual(2);
  expect(sb.y + sb.height).toBeLessThan(app.y + app.height - 40);
  await page.keyboard.press("Escape");
  await expect(sheet).toHaveCount(0);
});

test("rows and cards light up under the mouse", async ({ page, project }) => {
  await open(page, project);
  const hover = await tokenBackground(page, "--hover");
  const r = row(page, "t-2");
  await r.hover();
  // The wash is a layer over the row's opaque base, so the swipe labels beneath it stay hidden.
  await expect.poll(() => r.locator(".rc").evaluate(el => getComputedStyle(el).backgroundImage)).toContain(hover);
  await page.mouse.move(100, 600);
  await expect.poll(() => r.locator(".rc").evaluate(el => getComputedStyle(el).backgroundImage)).toBe("none");
  await page.locator('#bar [data-view="board"]').click();
  const card = page.locator('#wboard [data-card="t-2"]');
  await card.hover();
  await expect(card).toHaveCSS("background-color", hover);
  await expect(card).toHaveCSS("cursor", "pointer");
});

test("the cursor row and keyboard focus are visible", async ({ page, project }) => {
  await open(page, project);
  await page.keyboard.press("j");
  const sel = page.locator("#treeRows .row.sel .rc");
  await expect(sel).toHaveCount(1);
  await expect.poll(() => sel.evaluate(el => getComputedStyle(el).boxShadow)).toContain("inset");
  // Tab folds in the tree and the board, as in the TUI; from More it walks the controls, and each shows a ring.
  await page.locator('#bar [data-view="more"]').click();
  await page.keyboard.press("Tab");
  const focused = page.locator(":focus-visible");
  await expect(focused).toHaveCount(1);
  await expect(focused).toHaveCSS("outline-style", "solid");
  await expect(focused).toHaveCSS("outline-width", "2px");
});

const WIDE_DESCRIPTION = Array.from({ length: 8 }, (_, n) =>
  `Paragraph ${n + 1} keeps the detail text readable on a very wide monitor while the surrounding panes use the extra room.`,
).join("\n\n");

test.describe("wide monitor layout", () => {
  test.use({ fixture: { issues: [...SAMPLE, { id: "w-long", title: "[wide] Readable description", priority: 1, description: WIDE_DESCRIPTION }] } });

  test("uses the extra width while prose keeps a readable measure", async ({ page, project }) => {
    test.skip(sizeClassOf(page.viewportSize()!.width) !== "wide", "wide only");
    await open(page, project);
    await row(page, "w-long").click();
    const d = page.locator(".detail");
    await expect(d).toHaveAttribute("data-id", "w-long");
    await expect(d.locator(".md p").first()).toContainText("Paragraph 1");

    const app = await box(page, "#app");
    const pane = await box(page, ".detail");
    const prose = await box(page, '#dbody > .md:has-text("Paragraph 1")');
    expect(pane.width).toBeGreaterThanOrEqual(520);
    expect(pane.width).toBeLessThan(app.width * 0.35);
    expect(prose.width).toBeLessThanOrEqual(720);

    await page.keyboard.press("\\");
    const fullPane = await box(page, ".detail");
    const fullProse = await box(page, '#dbody > .md:has-text("Paragraph 1")');
    expect(fullPane.width).toBeGreaterThan(app.width * 0.85);
    expect(fullProse.width).toBeLessThanOrEqual(720);

    await page.keyboard.press("\\");
    await page.locator('#bar [data-view="board"]').click();
    const board = await box(page, "#wboard");
    const openHead = await box(page, '#wboard .whead[data-tab="open"]');
    const inProgressHead = await box(page, '#wboard .whead[data-tab="in_progress"]');
    expect(board.width).toBeGreaterThan(app.width * 0.6);
    expect(openHead.width).toBeGreaterThan(360);
    expect(inProgressHead.x).toBeGreaterThan(openHead.x + openHead.width - 1);
  });
});
