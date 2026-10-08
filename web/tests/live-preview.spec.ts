// b9s web --dev-assets: a change to the served files reloads an open browser,
// and the URL hash brings it back to the same issue (make web-dev).
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { expect, open, row, SAMPLE, startProject, stopProject, test } from "./harness";

test("a changed stylesheet reloads the page on the same issue", async ({ page }) => {
  const assets = fs.mkdtempSync(path.join(os.tmpdir(), "b9s-web-dev-"));
  fs.cpSync(path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../pkg/web/dist"), assets, { recursive: true });
  const project = await startProject(SAMPLE, {}, false, ["--dev-assets", assets]);
  try {
    await open(page, project);
    await row(page, "t-1").click();
    await expect(page).toHaveURL(/#\/.+\/t-1$/);
    const hash = await page.evaluate(() => location.hash);
    await page.evaluate(() => { (window as unknown as { marker: number }).marker = 1; });

    fs.appendFileSync(path.join(assets, "app.css"), "\nbody { outline: 3px solid rgb(1, 2, 3); }\n");

    await expect.poll(() => page.evaluate(() => getComputedStyle(document.body).outlineColor), { timeout: 10000 })
      .toBe("rgb(1, 2, 3)");
    expect(await page.evaluate(() => (window as unknown as { marker?: number }).marker)).toBeUndefined();
    expect(await page.evaluate(() => location.hash)).toBe(hash);
    await expect(page.locator("body")).toHaveAttribute("data-live", "live", { timeout: 10000 });
  } finally {
    stopProject(project);
    fs.rmSync(assets, { recursive: true, force: true });
  }
});
