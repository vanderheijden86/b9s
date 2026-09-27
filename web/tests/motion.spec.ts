import { expect, open, test } from "./harness";

// Sheets and panels slide for 240 ms. Under load WebKit reports a sliding
// button as stable, and Playwright's tap lands where the button was, not
// where it is (bd-foit.18). Every project runs with reduced motion, which
// app.css honours by dropping transitions, so no test taps a moving target.
test("tests run with reduced motion, so nothing slides under a tap", async ({ page, project }) => {
  await open(page, project);
  expect(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches)).toBe(true);
});
