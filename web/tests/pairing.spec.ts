import fs from "node:fs";
import { expect, open, row, SAMPLE, startProject, stopProject, test } from "./harness";

test.use({ paired: true });

test("an unpaired browser is told how to pair, and the printed link pairs it", async ({ page, project }) => {
  await page.goto(project.url);
  await expect(page.locator("#boot")).toContainText("This browser is not paired");
  await expect(page.locator("#boot")).toContainText("b9s web");
  await expect(page.locator("#boot")).not.toContainText("--pair");
  await expect.poll(() => project.pairLink()).toContain("/pair?t=");
  await page.goto(project.pairLink());
  await expect(page.locator("#boot")).toBeHidden({ timeout: 10000 });
  await expect(row(page, "t-1")).toBeVisible();
  await open(page, project);
  await expect(row(page, "t-1")).toBeVisible();
});

test("an open page writes after the header-auth server restarts with a new secret", async ({ page }, info) => {
  const project = await startProject(SAMPLE, {}, true, undefined, ["--trust-header", "X-Forwarded-Email", "--owner", "owner@example.com"]);
  try {
    const pageErrors: string[] = [];
    page.on("pageerror", error => pageErrors.push(error.message));
    await page.setExtraHTTPHeaders({ "X-Forwarded-Email": "owner@example.com" });
    const initialSession = page.waitForResponse(r => r.url().endsWith("/api/session"));
    await open(page, project);
    const oldCSRF = (await (await initialSession).json()).csrf;
    await page.evaluate(() => { (window as unknown as { restartMarker: string }).restartMarker = "open"; });
    await row(page, "t-2").click();
    await page.locator('.detail .dacts [data-act="comment"]').click();
    await page.locator("#fComment").fill("Written after server restart");

    const previousPID = project.proc.pid;
    await project.restart(true);
    expect(project.proc.pid).not.toBe(previousPID);
    const writes: { status: number; csrf: string | undefined; body: string | null }[] = [];
    let sessionReads = 0;
    page.on("response", response => {
      if (response.url().endsWith("/api/session")) sessionReads++;
      if (response.url().endsWith("/api/write")) {
        const req = response.request();
        writes.push({ status: response.status(), csrf: req.headers()["x-b9s-csrf"], body: req.postData() });
      }
    });
    await page.locator('[data-sa="docomment"]').click();
    await expect(page.locator(".detail .cm", { hasText: "Written after server restart" })).toBeVisible();
    expect(writes.map(w => w.status)).toEqual([403, 200]);
    expect(writes[0].csrf).toBe(oldCSRF);
    expect(writes[1].csrf).toBeTruthy();
    expect(writes[1].csrf).not.toBe(oldCSRF);
    expect(writes[1].body).toBe(writes[0].body);
    expect(sessionReads).toBe(1);
    expect(project.calls().filter(c => c[0] === "comments")).toHaveLength(1);
    expect(await page.evaluate(() => (window as unknown as { restartMarker?: string }).restartMarker)).toBe("open");
    expect(pageErrors).toEqual([]);
    const proof = info.outputPath("restart-proof.json");
    fs.writeFileSync(proof, JSON.stringify({ previousPID, restartedPID: project.proc.pid, writes, sessionReads, calls: project.calls(), pageErrors }, null, 2));
    await info.attach("restart-proof", { path: proof, contentType: "application/json" });
  } finally {
    stopProject(project);
  }
});
