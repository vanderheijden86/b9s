import { defineConfig, devices } from "@playwright/test";

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
  // budget.
  projects: [
    { name: "pixel", use: { ...devices["Pixel 7"] }, testIgnore: /(wide|keys|perf)\.spec/ },
    { name: "iphone", use: { ...devices["iPhone 14"] }, testIgnore: /(wide|keys|perf)\.spec/ },
    ...(["Desktop Chrome", "Desktop Safari", "Desktop Firefox"] as const).map((device) => ({
      name: device === "Desktop Chrome" ? "desktop" : device.toLowerCase().replace(" ", "-"),
      use: { ...devices[device], viewport: { width: 1440, height: 900 } },
      testMatch: /(wide|keys|read|write)\.spec/,
    })),
    { name: "ipad", use: { ...devices["iPad Pro 11"] }, testMatch: /(wide|keys)\.spec/ },
    ...(["Pixel 7", "iPhone 14"] as const).map((device) => ({
      name: device === "Pixel 7" ? "pixel-perf" : "iphone-perf",
      use: { ...devices[device] },
      testMatch: /perf\.spec/,
      dependencies: ["pixel", "iphone", "desktop", "desktop-safari", "desktop-firefox", "ipad"],
    })),
  ],
});
