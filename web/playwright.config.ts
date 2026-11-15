import { defineConfig, devices } from "@playwright/test";

// measure.spec.ts times the app on emulated devices against 1000 and 5000
// issues and writes .b9s-e2e/measure/report/. It is listed only when it is
// asked for by name, so a plain run and CI never pay for it:
//   npx playwright test --project measure
// A worker process reads this file again without the command line, so the
// answer travels to it in the environment, which workers inherit.
const measureAsked = process.env.B9S_MEASURE === "1" || process.argv.some((a, n, all) => a === "--project=measure" || (a === "--project" && all[n + 1] === "measure"));
if (measureAsked) process.env.B9S_MEASURE = "1";

// One project per viewport the responsive plan names, across the size classes
// of docs/adr/0052 (web/src/util.ts BREAKPOINTS): two phones, a tablet upright
// and on its side, two desktops and a wide monitor. shell.spec.ts runs in each
// of them; a later spec joins by adding itself to their testMatch.
const sizes = [
  { name: "phone-360", use: { ...devices["Pixel 7"], viewport: { width: 360, height: 780 } } },
  { name: "phone-412", use: { ...devices["Pixel 7"] } },
  { name: "tablet-768", use: { ...devices["iPad Mini"] } },
  { name: "tablet-1024", use: { ...devices["iPad Mini landscape"] } },
  { name: "desktop-1440", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
  { name: "desktop-1920", use: { ...devices["Desktop Chrome"], viewport: { width: 1920, height: 1080 } } },
  { name: "wide-2560", use: { ...devices["Desktop Chrome"], viewport: { width: 2560, height: 1440 } } },
];
const perfBaseDependencies = ["pixel", "iphone", "desktop", "desktop-safari", "desktop-firefox", "ipad", ...sizes.map(size => size.name)];
const webkitPerfTimeout = process.env.CI ? 90000 : 30000;
const perfProjects = [
  { name: "pixel-perf", use: devices["Pixel 7"], dependencies: perfBaseDependencies },
  // b9m-vdp.4.19: CI WebKit records budgets without enforcing them, so its perf tests run every sample on a slow runner.
  { name: "iphone-perf", use: devices["iPhone 14"], dependencies: [...perfBaseDependencies, "pixel-perf"], timeout: webkitPerfTimeout },
  { name: "tablet-perf", use: devices["iPad Mini landscape"], dependencies: [...perfBaseDependencies, "iphone-perf"], timeout: webkitPerfTimeout },
  { name: "desktop-perf", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } }, dependencies: [...perfBaseDependencies, "tablet-perf"] },
];

// Each test starts its own `b9s web` on a free port (tests/harness.ts), so
// there is no shared webServer. globalSetup builds the binary under test.
export default defineConfig({
  testDir: "tests",
  globalSetup: "./tests/global-setup.ts",
  timeout: 30000,
  expect: { timeout: 5000 },
  fullyParallel: true,
  workers: 4,
  reporter: [["list"]],
  // Reduced motion drops the app's transitions (app.css), so a tap never
  // lands on a sheet that is still sliding in. tests/motion.spec.ts says why.
  use: { trace: "retain-on-failure", actionTimeout: 5000, navigationTimeout: 10000, reducedMotion: "reduce" },
  // Phones get the one-column board; wide.spec.ts and keys.spec.ts cover
  // laptops and iPads, which have a keyboard. The desktop browsers also run
  // the reading and writing specs. detail.spec.ts measures the phone's
  // bottom sheet, which a desktop shows as a side panel instead.
  //
  // perf.spec.ts times a render against a budget, so it runs in projects of
  // its own that start once every other project has finished. Timed beside
  // three other workers on a four-core CI runner, the same render misses the
  // budget, so a pull request does not run them at all: nightly.yml runs
  // them alone on one worker (docs/adr/0059).
  projects: [
    { name: "tokens", use: { ...devices["Desktop Chrome"] }, testMatch: /tokens\.spec/ },
    { name: "pixel", use: { ...devices["Pixel 7"] }, testIgnore: /(wide|keys|perf|measure|tokens|shell|desktop|accessibility)\.spec/ },
    { name: "iphone", use: { ...devices["iPhone 14"] }, testIgnore: /(wide|keys|perf|measure|tokens|shell|desktop|accessibility)\.spec/ },
    ...(["Desktop Chrome", "Desktop Safari", "Desktop Firefox"] as const).map((device) => ({
      name: device === "Desktop Chrome" ? "desktop" : device.toLowerCase().replace(" ", "-"),
      use: { ...devices[device], viewport: { width: 1440, height: 900 } },
      testMatch: /(wide|keys|read|write|memory|desktop|fleet)\.spec/,
    })),
    { name: "ipad", use: { ...devices["iPad Pro 11"] }, testMatch: /(wide|keys)\.spec/ },
    // desktop.spec.ts covers the desktop and wide classes; at 1440 px the three desktop browsers run it already.
    ...sizes.map(size => ({ ...size, testMatch: size.name === "desktop-1920" || size.name === "wide-2560" ? /(shell|desktop|board-detail|accessibility)\.spec/ : /(shell|board-detail|accessibility)\.spec/ })),
    ...perfProjects.map(({ name, use, dependencies, timeout }) => ({
      name,
      use,
      testMatch: /perf\.spec/,
      dependencies,
      ...(timeout ? { timeout } : {}),
    })),
    // The spec launches the browsers and devices it measures itself; one
    // worker at a time keeps the timings from sharing the CPU. Its output
    // lives beside the e2e binary, where a plain run in the same checkout
    // neither cleans it nor shares its artifacts, and it keeps no Playwright
    // trace: the spec records its own.
    ...(measureAsked ? [{ name: "measure", testMatch: /measure\.spec/, fullyParallel: false, timeout: 20 * 60 * 1000, outputDir: "../.b9s-e2e/measure", use: { ...devices["Desktop Chrome"], trace: "off" as const } }] : []),
  ],
});
