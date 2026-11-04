// The board on a laptop, desktop browser or iPad: every column side by side,
// epic lanes as swimlanes, mouse drag between columns, TUI keys, and the
// detail as a side panel that leaves the board in view.

import { expect, open, SAMPLE, settled, test, type Project } from "./harness";
import type { Locator, Page } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  page.on("pageerror", error => { throw error; });
});

const col = (page: Page, k: string) => page.locator(`#wboard .wcell[data-dcol="${k}"]`);
const card = (page: Page, id: string) => page.locator(`#wboard [data-card="${id}"]`);

async function board(page: Page, project: Project): Promise<void> {
  await open(page, project);
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator("#wboard")).toBeVisible();
}

async function drag(page: Page, from: { x: number; y: number }, to: { x: number; y: number }): Promise<void> {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  for (let s = 1; s <= 12; s++) await page.mouse.move(from.x + ((to.x - from.x) * s) / 12, from.y + ((to.y - from.y) * s) / 12);
  await page.mouse.up();
}

const mid = (b: { x: number; y: number; width: number; height: number }) => ({ x: b.x + b.width / 2, y: b.y + b.height / 2 });

test("every column shows side by side", async ({ page, project }) => {
  await board(page, project);
  const heads = page.locator('#wboard .whead[data-tab]');
  await expect(heads).toHaveCount(3);
  const boxes = await Promise.all(["open", "in_progress", "blocked"].map(k => page.locator(`#wboard .whead[data-tab="${k}"]`).boundingBox()));
  expect(boxes[0]!.y).toBe(boxes[1]!.y);
  expect(boxes[0]!.x).toBeLessThan(boxes[1]!.x);
  expect(boxes[1]!.x).toBeLessThan(boxes[2]!.x);
  for (const [id, k] of [["t-2", "open"], ["t-1", "in_progress"], ["t-3", "blocked"]]) {
    await expect(col(page, k).locator(`[data-card="${id}"]`)).toBeInViewport();
  }
  await expect(page.locator("#bcol")).toHaveCount(0);
});

test("epic lanes are swimlanes across the columns", async ({ page, project }) => {
  await board(page, project);
  await page.keyboard.press("v");
  const lane = page.locator('#wboard [data-lane="t-epic"]');
  await expect(lane).toContainText("Mobile web");
  const lb = (await lane.boundingBox())!, wb = (await page.locator("#wboard").boundingBox())!;
  expect(lb.width).toBeGreaterThan(wb.width * 0.8);
  // No epic lane holds the loose chore, below the epic lane.
  const none = page.locator('#wboard [data-lane="none"]');
  expect((await none.boundingBox())!.y).toBeGreaterThan(lb.y);
  expect((await card(page, "t-5").boundingBox())!.y).toBeGreaterThan((await none.boundingBox())!.y);
  await lane.locator(".t").click();
  await expect(card(page, "t-2")).toHaveCount(0);
  await expect(card(page, "t-5")).toBeVisible();
  await lane.locator(".ei").click();
  await expect(page.locator(".detail")).toContainText("Mobile web");
});

test("a mouse drags a card into another column", async ({ page, project }) => {
  await board(page, project);
  const from = mid((await card(page, "t-2").boundingBox())!);
  const to = mid((await col(page, "in_progress").first().boundingBox())!);
  await drag(page, from, to);
  await expect.poll(() => project.issues().find(i => i.id === "t-2")?.status).toBe("in_progress");
  await expect(col(page, "in_progress").locator('[data-card="t-2"]')).toBeVisible();
  expect(project.calls()).toContainEqual(["update", "t-2", "--status=in_progress"]);
});

test("a header click folds its column into a rail and a rail click unfolds it", async ({ page, project }) => {
  await board(page, project);
  const head = page.locator('#wboard .whead[data-tab="blocked"]');
  const w0 = (await head.boundingBox())!.width;
  await head.click();
  await expect(head).toHaveClass(/rail/);
  expect((await head.boundingBox())!.width).toBeLessThan(w0 / 3);
  await expect(card(page, "t-3")).toHaveCount(0);
  await page.locator('#tabs [data-act="unfoldall"]').click();
  await expect(head).not.toHaveClass(/rail/);
  await expect(card(page, "t-3")).toBeVisible();
});

test("the options button in the board toolbar opens board options", async ({ page, project }) => {
  await board(page, project);
  await page.locator('#tabs [data-act="boardopts"]').click();
  await expect(page.locator("#sheetHost")).toContainText("Group by");
});

test("keys move the cursor across columns and Enter opens the side detail", async ({ page, project }) => {
  await board(page, project);
  await page.keyboard.press("j");
  await expect(page.locator("#wboard .card.sel")).toHaveCount(1);
  await page.keyboard.press("l");
  await expect(card(page, "t-1")).toHaveClass(/sel/);
  await page.keyboard.press("ArrowRight");
  await expect(card(page, "t-3")).toHaveClass(/sel/);
  await page.keyboard.press("h");
  await expect(card(page, "t-1")).toHaveClass(/sel/);
  await page.keyboard.press("Enter");
  const d = page.locator(".detail");
  await expect(d).toContainText("Write the API");
  // Poll every box: the panel slides in, and the full issue arriving for it
  // re-renders both the panel and the board under a held element.
  const vp = page.viewportSize()!;
  const box = async (sel: string) => page.locator(sel).boundingBox();
  await expect.poll(async () => {
    const b = await box(".detail");
    return !!b && Math.round(b.x + b.width) >= vp.width - 1 && b.width < vp.width * 0.6 && b.height > vp.height * 0.7;
  }).toBe(true);
  // The board stays usable beside the panel: it ends where the panel starts
  // and scrolls sideways when the columns no longer fit, as on an iPad upright.
  await expect.poll(async () => {
    const [w, b] = [await box("#wboard"), await box(".detail")];
    return !!w && !!b && Math.abs(w.x + w.width - b.x) <= 1;
  }).toBe(true);
  await page.keyboard.press("l");
  await expect(d).toContainText("Board drag");
  await page.keyboard.press("Escape");
  await expect(d).toHaveCount(0);
});

test("z folds the cursor column and Z unfolds every column", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-3").click();
  await page.keyboard.press("Escape");
  await page.keyboard.press("z");
  await expect(page.locator('#wboard .whead[data-tab="blocked"]')).toHaveClass(/rail/);
  await page.keyboard.press("Shift+Z");
  await expect(page.locator('#wboard .whead[data-tab="blocked"]')).not.toHaveClass(/rail/);
});

test("Back closes the side detail and Backspace steps back too", async ({ page, project }) => {
  await board(page, project);
  await page.keyboard.press("j");
  await page.keyboard.press("l");
  await page.keyboard.press("Enter");
  const d = page.locator(".detail");
  await expect(d).toHaveAttribute("data-id", "t-1");
  // Moving the cursor with the panel open replaces the entry: one Back closes it.
  await page.keyboard.press("l");
  await expect(d).toHaveAttribute("data-id", "t-3");
  await expect(page).toHaveURL(/#\/board\/t-3$/);
  await page.goBack();
  await expect(d).toHaveCount(0);
  await expect(page.locator("#wboard")).toBeVisible();
  await page.keyboard.press("Backspace");
  await expect(page.locator("#vTree")).toBeVisible();
  // At the first entry of the app, Backspace stays in the app.
  await page.keyboard.press("Backspace");
  await expect(page.locator("#vTree")).toBeVisible();
  expect(page.url()).toContain(project.url);
});

const epic = (page: Page, id: string) => page.locator(`#wboard .wepic[data-epic="${id}"]`);

test("the epic rail is the default: epics form the first column, level with their lane", async ({ page, project }) => {
  await board(page, project);
  const head = page.locator("#wboard .whead.epich");
  await expect(head).toContainText("EPIC");
  const e = epic(page, "t-epic");
  await expect(e).toContainText("Mobile web");
  await expect(e).toContainText("0/3");
  // The terminal's epic cell facts (docs/adr/0030): what moves, what waits, what is urgent.
  await expect(e.locator(".ec")).toHaveText("wip 1·waiting 1·ready 1");
  await expect(e.locator(".en")).toContainText("P1 1");
  const eb = (await e.boundingBox())!, ob = (await page.locator('#wboard .whead[data-tab="open"]').boundingBox())!;
  expect(eb.x + eb.width).toBeLessThanOrEqual(ob.x);
  // The epic's cards start on the epic's row; the loose chore sits in the No epic row below it.
  const c2 = (await card(page, "t-2").boundingBox())!;
  expect(Math.abs(c2.y - eb.y)).toBeLessThan(20);
  const none = (await epic(page, "none").boundingBox())!;
  expect(none.y).toBeGreaterThanOrEqual(eb.y + eb.height - 1);
  expect(Math.abs((await card(page, "t-5").boundingBox())!.y - none.y)).toBeLessThan(20);
  await expect(page.locator("#wboard .lane")).toHaveCount(0);
  await expect(page.locator(".brail")).toHaveCount(0);
});

test("v switches between the epic rail and epic rows, and the choice stays", async ({ page, project }) => {
  await board(page, project);
  await expect(epic(page, "t-epic")).toBeVisible();
  await page.keyboard.press("v");
  await expect(page.locator('#wboard .lane[data-lane="t-epic"]')).toBeVisible();
  await expect(page.locator("#wboard .wepic")).toHaveCount(0);
  await page.reload();
  await settled(page);
  await page.locator('#bar [data-view="board"]').click();
  await expect(page.locator('#wboard .lane[data-lane="t-epic"]')).toBeVisible();
  await page.keyboard.press("v");
  await expect(epic(page, "t-epic")).toBeVisible();
});

test("an epic cell opens the epic, and its arrow steps the lane through Compact, Open and Full", async ({ page, project }) => {
  await board(page, project);
  await epic(page, "t-epic").locator(".t").click();
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-epic");
  await page.keyboard.press("Escape");
  await expect(page.locator(".detail")).toHaveCount(0);
  const step = epic(page, "t-epic").locator("[data-lane]");
  const boxed = (id: string) => page.locator(`#wboard .wbox [data-card="${id}"]`);
  // Compact, the default: each cell holds its cards in a box that scrolls, so the epic cell sets the lane's height.
  await expect(step).toHaveAttribute("aria-label", "Compact lane");
  await expect(step).toHaveAttribute("title", /^Open: /);
  await expect(step).toHaveText("▸");
  for (const id of ["t-1", "t-2", "t-3"]) await expect(boxed(id)).toHaveCount(1);
  // Open: the open and in progress cells show every card and set the height, and the blocked cell still scrolls.
  await step.click();
  await expect(step).toHaveAttribute("aria-label", "Open lane");
  await expect(step).toHaveAttribute("title", /^Full: /);
  await expect(step).toHaveText("▿");
  await expect(boxed("t-1")).toHaveCount(0);
  await expect(boxed("t-2")).toHaveCount(0);
  await expect(boxed("t-3")).toHaveCount(1);
  // Full: nothing scrolls.
  await step.click();
  await expect(step).toHaveAttribute("aria-label", "Full lane");
  await expect(step).toHaveAttribute("title", /^Compact: /);
  await expect(step).toHaveText("▾");
  await expect(boxed("t-3")).toHaveCount(0);
  await step.click();
  await expect(step).toHaveAttribute("aria-label", "Compact lane");
  await expect(boxed("t-2")).toHaveCount(1);
  // Every lane has a height of its own.
  await expect(epic(page, "none").locator("[data-lane]")).toHaveAttribute("aria-label", "Compact lane");
  await expect(card(page, "t-5")).toBeVisible();
});

test("h from the first column selects the epic; Enter opens it and Tab steps its lane", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-2").click();
  await page.keyboard.press("Escape");
  await page.keyboard.press("h");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await page.keyboard.press("l");
  await expect(card(page, "t-2")).toHaveClass(/sel/);
  // Each key acts on the selection the previous key left, so wait for it:
  // Enter on the "none" lane opens nothing.
  await page.keyboard.press("h");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await page.keyboard.press("j");
  await expect(epic(page, "none")).toHaveClass(/sel/);
  await page.keyboard.press("k");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await page.keyboard.press("Enter");
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-epic");
  await page.keyboard.press("Escape");
  await expect(page.locator(".detail")).toHaveCount(0);
  const step = epic(page, "t-epic").locator("[data-lane]");
  await page.keyboard.press("Tab");
  await expect(step).toHaveAttribute("aria-label", "Open lane");
  await page.keyboard.press("Tab");
  await expect(step).toHaveAttribute("aria-label", "Full lane");
  await expect(card(page, "t-2")).toBeVisible();
});

test("Enter opens the selected epic when the board DOM redraws under it", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-2").click();
  await page.keyboard.press("Escape");
  // The observer below holds the epic out of this #wboard only: the render
  // that ends the panel's slide out draws a new one, so it runs first.
  await expect(page.locator(".detail")).toHaveCount(0);
  await page.keyboard.press("h");
  await expect(epic(page, "t-epic")).toHaveClass(/sel/);
  await card(page, "t-2").evaluate((el: HTMLElement) => el.focus());
  await page.evaluate(() => {
    const remove = () => document.querySelector('#wboard .wepic[data-epic="t-epic"]')?.remove();
    new MutationObserver(remove).observe(document.querySelector("#wboard")!, { childList: true, subtree: true });
    remove();
  });
  await expect(epic(page, "t-epic")).toHaveCount(0);
  await page.keyboard.press("Enter");
  await expect(page.locator(".detail")).toHaveAttribute("data-id", "t-epic");
});

test("keys typed while the closed panel slides out act on a closed panel", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-2").click();
  const d = page.locator(".detail");
  await expect(d).toHaveAttribute("data-id", "t-2");
  // Esc, h and Enter land in one task, before the panel has slid out. A timer
  // started after them fires after the slide's own, so what follows sees the
  // board once the slide has ended.
  await page.evaluate(async () => {
    for (const key of ["Escape", "h", "Enter"]) {
      (document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  });
  await expect(d).toHaveAttribute("data-id", "t-epic");
  await expect(page).toHaveURL(/#\/board\/t-epic$/);
});

test("Enter while the closed panel's Back is under way keeps the epic open", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-2").click();
  const d = page.locator(".detail");
  await expect(d).toHaveAttribute("data-id", "t-2");
  // Closing the panel steps the browser Back, and the browser reports the step
  // a moment later. h and Enter land in that moment, as keys do on a slow runner.
  await page.evaluate(() => {
    const back = history.back;
    (window as Window & { __b9sBackLanded?: Promise<void> }).__b9sBackLanded = new Promise<void>((resolve, reject) => {
      history.back = () => {
        history.back = back;
        back.call(history);
        addEventListener("popstate", () => resolve(), { once: true });
        queueMicrotask(() => {
          for (const key of ["h", "Enter"]) {
            (document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
          }
        });
      };
      setTimeout(() => reject(new Error("closing the panel stepped no Back within 5 s")), 5000);
    });
  });
  await page.keyboard.press("Escape");
  await page.evaluate(() => (window as Window & { __b9sBackLanded?: Promise<void> }).__b9sBackLanded);
  await expect(d).toHaveAttribute("data-id", "t-epic");
  await expect(page).toHaveURL(/#\/board\/t-epic$/);
  // The epic is a step of its own: Back closes it and stays on the board.
  await page.goBack();
  await expect(d).toHaveCount(0);
  await expect(page).toHaveURL(/#\/board$/);
});

test("\\ or the panel's button switches the detail between a side panel and the full width", async ({ page, project }) => {
  await board(page, project);
  await card(page, "t-1").click();
  const d = page.locator(".detail"), vp = page.viewportSize()!;
  // The panel slides in, so its box settles a moment after it appears.
  const box = async () => (await d.boundingBox()) || { x: -1, width: -1 };
  const panel = async () => { const b = await box(); return b.width > 0 && b.width < vp.width * 0.6; };
  // The full panel covers the views from the rail's edge to the right edge; the rail stays in reach.
  const rail = (await page.locator("#bar").boundingBox())!;
  const full = async () => { const b = await box(); return Math.abs(b.x - (rail.x + rail.width)) <= 1 && b.x + b.width >= vp.width - 2; };
  await expect.poll(panel).toBe(true);
  await page.keyboard.press("\\");
  await expect.poll(full).toBe(true);
  await expect(d).toHaveAttribute("data-id", "t-1");
  await expect(d.locator('[data-act="dsize"]')).toHaveAttribute("aria-label", "Side panel");
  await page.keyboard.press("\\");
  await expect.poll(panel).toBe(true);
  await d.locator('[data-act="dsize"]').click();
  await expect.poll(full).toBe(true);
  // The board's keys still move the cursor behind the full panel, and the panel follows.
  await page.keyboard.press("l");
  await expect(d).toHaveAttribute("data-id", "t-3");
  await expect.poll(full).toBe(true);
  await page.keyboard.press("Escape");
  await expect(d).toHaveCount(0);
});

test.describe("a lane taller than its epic cell", () => {
  test.use({ fixture: { issues: [...SAMPLE, ...[6, 7, 8, 9].map(n => ({ id: `t-${n}`, title: `[${n}] Task ${n}`, parent: "t-epic", priority: 3 }))] } });

  test("in a Full lane the epic cell spans the whole lane", async ({ page, project }) => {
    await board(page, project);
    const step = epic(page, "t-epic").locator("[data-lane]");
    await step.click();
    await expect(step).toHaveAttribute("aria-label", "Open lane");
    await step.click();
    await expect(step).toHaveAttribute("aria-label", "Full lane");
    const eb = (await epic(page, "t-epic").boundingBox())!;
    const last = (await card(page, "t-9").boundingBox())!;
    expect(last.y + last.height).toBeGreaterThan(eb.y + 120);
    expect(eb.y + eb.height).toBeGreaterThanOrEqual(last.y + last.height - 2);
    // The next lane's epic starts below it, so the cell fills its own row only.
    const none = (await epic(page, "none").boundingBox())!;
    expect(none.y).toBeGreaterThanOrEqual(eb.y + eb.height - 1);
  });
});

test.describe("a lane with more blocked cards than open cards", () => {
  const opens = [1, 2, 3, 4, 5].map(n => ({ id: `t-o${n}`, title: `[o${n}] Open ${n}`, parent: "t-epic", priority: 3 }));
  const blocked = Array.from({ length: 12 }, (_, n) => ({ id: `t-b${n + 1}`, title: `[b${n + 1}] Blocked ${n + 1}`, parent: "t-epic", status: "blocked", priority: 3 }));
  test.use({ fixture: { issues: [...SAMPLE, ...opens, ...blocked] } });

  test("Compact holds the cells at the epic's height, Open shows every open card, Full every card, and only the cursor's cell scrolls", async ({ page, project }) => {
    await board(page, project);
    const step = epic(page, "t-epic").locator("[data-lane]");
    // The epic lane is the first row; the No epic lane holds only the loose chore.
    const cell = (k: string) => col(page, k).first();
    const box = (k: string) => cell(k).locator(".wbox");
    const bottom = async (l: Locator) => { const b = (await l.boundingBox())!; return b.y + b.height; };
    const overflows = (k: string) => box(k).evaluate(b => b.scrollHeight > b.clientHeight + 1);
    // A card is in view when its cell's box shows all of it.
    const inView = (id: string) => card(page, id).evaluate(c => {
      const r = c.getBoundingClientRect(), b = c.closest(".wbox")!.getBoundingClientRect();
      return r.top >= b.top - 1 && r.bottom <= b.bottom + 1;
    });

    // Compact: the epic cell sets the lane's height, and the open and blocked cells hold more cards than it shows.
    // Neither holds the cursor's card, so neither scrolls.
    const compact = (await epic(page, "t-epic").boundingBox())!;
    expect(Math.abs(await bottom(box("blocked")) - (compact.y + compact.height))).toBeLessThanOrEqual(1);
    expect(await overflows("open")).toBe(true);
    expect(await overflows("blocked")).toBe(true);
    await expect(box("open")).toHaveCSS("overflow-y", "hidden");
    await expect(box("blocked")).toHaveCSS("overflow-y", "hidden");
    // The edge with more cards past it fades.
    await expect(box("blocked")).toHaveClass(/\bdown\b/);
    await expect(box("blocked")).not.toHaveClass(/\bup\b/);

    // Open: every open card shows, and the blocked cell holds more at the open cell's height.
    await step.click();
    await expect(step).toHaveAttribute("aria-label", "Open lane");
    await expect(box("open")).toHaveCount(0);
    const lastOpen = await bottom(card(page, "t-o5"));
    expect(lastOpen).toBeLessThanOrEqual(await bottom(cell("open")));
    expect(await bottom(box("blocked"))).toBeGreaterThanOrEqual(lastOpen);
    expect(await overflows("blocked")).toBe(true);
    await expect(box("blocked")).toHaveCSS("overflow-y", "hidden");
    expect((await epic(page, "t-epic").boundingBox())!.height).toBeGreaterThan(compact.height);

    // Full: nothing in the lane scrolls, and the epic cell spans its last blocked card.
    await step.click();
    await expect(step).toHaveAttribute("aria-label", "Full lane");
    await expect(page.locator('#wboard .wbox[data-box^="t-epic "]')).toHaveCount(0);
    expect(await bottom(epic(page, "t-epic"))).toBeGreaterThanOrEqual(await bottom(card(page, "t-b12")) - 2);

    // Back in Compact, the cell that holds the cursor's card scrolls: j walks down the blocked cell, which keeps the card in view.
    await step.click();
    await expect(step).toHaveAttribute("aria-label", "Compact lane");
    await page.keyboard.press("j");
    await expect(card(page, "t-2")).toHaveClass(/sel/);
    await expect(box("open")).toHaveCSS("overflow-y", "auto");
    await page.keyboard.press("l");
    await expect(card(page, "t-1")).toHaveClass(/sel/);
    await page.keyboard.press("l");
    await expect(card(page, "t-3")).toHaveClass(/sel/);
    await expect(box("blocked")).toHaveCSS("overflow-y", "auto");
    await expect(box("open")).toHaveCSS("overflow-y", "hidden");
    for (const id of ["t-b1", "t-b2", "t-b3", "t-b4", "t-b5", "t-b6", "t-b7", "t-b8"]) {
      await page.keyboard.press("j");
      await expect(card(page, id)).toHaveClass(/sel/);
      await expect.poll(() => inView(id)).toBe(true);
    }
    const scrolled = await box("blocked").evaluate(b => b.scrollTop);
    expect(scrolled).toBeGreaterThan(0);
    await expect(box("blocked")).toHaveClass(/\bup\b/);
    // The cell the cursor leaves stops scrolling and keeps its place and its fades through the redraw.
    await page.keyboard.press("h");
    await expect(card(page, "t-1")).toHaveClass(/sel/);
    await expect(box("blocked")).toHaveCSS("overflow-y", "hidden");
    expect(await box("blocked").evaluate(b => b.scrollTop)).toBe(scrolled);
    await expect(box("blocked")).toHaveClass(/\bup\b/);
    await expect(box("blocked")).toHaveClass(/\bdown\b/);
  });
});

// The wheel and a trackpad move through the epics, so a cell scrolls only
// while it holds the cursor's card (b9m-vdp.4.26): over any other cell the
// wheel scrolls the board, and never catches in a cell on the way.
test.describe("a board taller than the window, with a lane that holds more cards than it shows", () => {
  const epics = Array.from({ length: 8 }, (_, n) => ({ id: `w-e${n + 1}`, title: `[e${n + 1}] Epic ${n + 1}`, type: "epic", priority: 2 }));
  const opens = epics.map((e, n) => ({ id: `w-${n + 1}`, title: `[${n + 1}] Open ${n + 1}`, parent: e.id, priority: 3 }));
  const blocked = Array.from({ length: 14 }, (_, n) => ({ id: `w-b${n + 1}`, title: `[b${n + 1}] Blocked ${n + 1}`, parent: "w-e1", status: "blocked", priority: 3 }));
  test.use({ fixture: { issues: [...epics, ...opens, ...blocked] } });

  test("the wheel over a cell without the cursor scrolls the board, and over the cursor's cell scrolls that cell", async ({ page, project, isMobile }) => {
    test.skip(isMobile, "Playwright has no wheel in mobile WebKit, and an iPad scrolls the board by touch");
    await board(page, project);
    const wboard = page.locator("#wboard");
    // The first lane is the biggest: w-e1, whose blocked cell holds fourteen cards.
    const blockedBox = page.locator('#wboard .wbox[data-box="w-e1 blocked"]');
    const scrollTop = (l: Locator) => l.evaluate(el => el.scrollTop);
    // Firefox runs a wheel scroll over several frames, so a place is read once two polls agree on it.
    const settled = async (l: Locator) => {
      let last = -1;
      await expect.poll(async () => { const now = await scrollTop(l); const still = now === last; last = now; return still; }).toBe(true);
      return last;
    };
    const wheelOver = async (l: Locator, dy: number) => {
      const b = (await l.boundingBox())!;
      await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2);
      await page.mouse.wheel(0, dy);
    };

    // No card holds the cursor yet, so no cell scrolls, and the wheel over a cell with cards past its edge scrolls the board.
    await expect(page.locator("#wboard .card.sel")).toHaveCount(0);
    for (const b of await page.locator("#wboard .wbox").all()) await expect(b).toHaveCSS("overflow-y", "hidden");
    expect(await blockedBox.evaluate(b => b.scrollHeight > b.clientHeight + 1)).toBe(true);
    expect(await wboard.evaluate(b => b.scrollHeight > b.clientHeight + 1)).toBe(true);
    await wheelOver(blockedBox, 200);
    await expect.poll(() => scrollTop(wboard)).toBeGreaterThan(0);
    await settled(wboard);
    expect(await scrollTop(blockedBox)).toBe(0);

    // A click puts the cursor in the cell, and the wheel over it then scrolls the cell while the board stays.
    await wboard.evaluate(b => { b.scrollTop = 0; });
    await card(page, "w-b1").click();
    await page.keyboard.press("Escape");
    await expect(page.locator(".detail")).toHaveCount(0);
    await expect(card(page, "w-b1")).toHaveClass(/sel/);
    await expect(blockedBox).toHaveCSS("overflow-y", "auto");
    const boardTop = await settled(wboard);
    await wheelOver(blockedBox, 100);
    await expect.poll(() => scrollTop(blockedBox)).toBeGreaterThan(0);
    const cellTop = await settled(blockedBox);
    expect(await scrollTop(wboard)).toBe(boardTop);

    // The cell the cursor leaves stops scrolling where the wheel left it.
    await page.keyboard.press("h");
    await expect(card(page, "w-1")).toHaveClass(/sel/);
    await expect(blockedBox).toHaveCSS("overflow-y", "hidden");
    expect(await scrollTop(blockedBox)).toBe(cellTop);
  });
});

test("ordinary cards keep only urgent priority beside the title under the cursor", async ({ page, project }) => {
  await board(page, project);
  await expect(card(page, "t-3").locator(".tags")).toHaveCount(0);
  await expect(card(page, "t-1").locator(".tags")).toHaveText("P1");
  await expect(card(page, "t-2").locator(".tags")).toHaveCount(0);
  await expect(card(page, "t-2").locator(".l2")).toHaveCount(0);
  const h = (await card(page, "t-2").boundingBox())!.height;
  expect(h).toBeLessThanOrEqual(44);
  await expect(card(page, "t-2")).toHaveCSS("border-top-width", "0px");
  await card(page, "t-2").click();
  await expect(card(page, "t-2")).toHaveClass(/sel/);
  await expect(card(page, "t-2").locator(".l2, .l3")).toHaveCount(0);
  expect((await card(page, "t-2").boundingBox())!.height).toBe(h);
  await expect(page.locator('.detail[data-id="t-2"]')).toContainText("ui");
});

test.describe("hierarchy in a cell", () => {
  test.use({
    fixture: {
      issues: [
        { id: "h-epic", title: "[epic] Epic: Checkout", type: "epic", priority: 2, description: "# Goal\nShip a faster checkout. Then more." },
        { id: "h-task", title: "[t] Card form", parent: "h-feat", priority: 1 },
        { id: "h-feat", title: "[f] Payments", type: "feature", parent: "h-epic", priority: 2 },
        { id: "h-other", title: "[o] Receipts", parent: "h-epic", priority: 3 },
      ],
    },
  });

  test("a child sits straight below its parent, indented, and the epic cell carries its first sentence", async ({ page, project }) => {
    await board(page, project);
    // The lane fills once the fixture's rows arrive over the live stream, so the order is polled, not read once (bd-zoom.1.3).
    await expect.poll(() => col(page, "open").first().locator("[data-card]").evaluateAll(els => els.map(e => (e as HTMLElement).dataset.card))).toEqual(["h-feat", "h-task", "h-other"]);
    const fx = (await card(page, "h-feat").locator(".id").boundingBox())!.x;
    const tx = (await card(page, "h-task").locator(".id").boundingBox())!.x;
    expect(tx).toBeGreaterThan(fx + 8);
    await expect(card(page, "h-task").locator(".l3")).toHaveCount(0);
    await expect(epic(page, "h-epic").locator(".es")).toHaveText("Ship a faster checkout.");
  });
});
