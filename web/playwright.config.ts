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
  use: { trace: "retain-on-failure", actionTimeout: 5000, navigationTimeout: 10000 },
  // Phones get the one-column board; wide.spec.ts covers laptops and iPads.
  projects: [
    { name: "pixel", use: { ...devices["Pixel 7"] }, testIgnore: /wide\.spec/ },
    { name: "iphone", use: { ...devices["iPhone 14"] }, testIgnore: /wide\.spec/ },
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } }, testMatch: /wide\.spec/ },
    { name: "ipad", use: { ...devices["iPad Pro 11"] }, testMatch: /wide\.spec/ },
  ],
});
