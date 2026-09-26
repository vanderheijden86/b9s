import { expect, type FixtureIssue, open, row, swipe, test } from "./harness";

// 1000 issues: 50 epics of 19 children each plus loose tasks. The budgets
// are for emulated phones on a laptop, which run faster than real ones; they
// catch a render path that went quadratic, not a slow handset.
const BIG: FixtureIssue[] = [];
for (let e = 0; e < 50; e++) {
  BIG.push({ id: `p-e${e}`, title: `[e${e}] Epic: area ${e}`, type: "epic" });
  for (let c = 0; c < 19; c++) BIG.push({ id: `p-e${e}.${c}`, title: `[e${e}.${c}] Task ${c} of area ${e}`, parent: `p-e${e}`, priority: c % 5, status: c % 4 === 0 ? "in_progress" : "open" });
}
for (let n = 0; n < 1000 - BIG.length; n++) BIG.push({ id: `p-l${n}`, title: `[l${n}] Loose task ${n}` });

test.use({ fixture: { issues: BIG } });

test("1000 issues load, scroll and swipe within budget", async ({ page, project }) => {
  const t0 = Date.now();
  await open(page, project);
  const loadMs = Date.now() - t0;
  expect(await page.locator("#treeRows .row").count()).toBeGreaterThan(900);
  expect(loadMs).toBeLessThan(5000);

  const renderMs = await page.evaluate(async () => {
    const chip = document.querySelector<HTMLElement>('[data-chip="closed"]')!;
    const t = performance.now();
    chip.click();
    await new Promise(r => requestAnimationFrame(() => r(null)));
    return performance.now() - t;
  });
  expect(renderMs).toBeLessThan(500);

  await row(page, "p-e40.3").scrollIntoViewIfNeeded();
  const before = await page.locator("#treeList").evaluate(el => el.scrollTop);
  expect(before).toBeGreaterThan(1000);
  await swipe(page, "p-e40.3", 0.35);
  await expect.poll(() => project.issues().find(i => i.id === "p-e40.3")?.status).toBe("in_progress");
  await expect(row(page, "p-e40.3").locator(".st")).toHaveText("◐");
  expect(Math.abs((await page.locator("#treeList").evaluate(el => el.scrollTop)) - before)).toBeLessThan(80);
  test.info().annotations.push({ type: "perf", description: `load ${loadMs} ms, full re-render ${Math.round(renderMs)} ms` });
});
