// Builds the web bundle and the b9s binary once, so every test serves the
// working tree rather than whatever b9s is on PATH.

import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default function globalSetup(): void {
  if (process.env.B9S_WEB_BIN) return;
  const root = path.resolve(__dirname, "../..");
  execFileSync("node", ["build.mjs"], { cwd: path.join(root, "web"), stdio: "inherit" });
  execFileSync("go", ["build", "-o", path.join(root, ".b9s-e2e", "b9s"), "./cmd/b9s"], { cwd: root, stdio: "inherit", timeout: 240000 });
}
