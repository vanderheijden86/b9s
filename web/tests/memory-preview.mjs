// Read-only proof against an explicitly started Memory preview server.
import { chromium, expect } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

const [url, evidence] = process.argv.slice(2);
if (!url || !evidence) throw new Error("Usage: node web/tests/memory-preview.mjs URL EVIDENCE_DIR");
const target = new URL(url);
if (!["127.0.0.1", "localhost", "[::1]"].includes(target.hostname)) throw new Error("The preview probe requires a loopback server");
await mkdir(evidence, { recursive: true });
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, reducedMotion: "reduce" });
page.setDefaultTimeout(5000);
page.setDefaultNavigationTimeout(15000);
const errors = [];
page.on("pageerror", e => errors.push(e.message));
page.on("console", message => { if (message.type() === "error") errors.push(message.text()); });
let pass = 0, fail = 0;
const check = async (name, action) => {
  try { await action(); pass++; console.log(`PASS ${name}`); }
  catch (e) { fail++; console.error(`FAIL ${name}: ${e.message}`); }
};
try {
  await page.goto(new URL("#/memory", target).href);
  await expect(page.locator("#boot")).toBeHidden({ timeout: 15000 });
  const response = await page.request.get(new URL("api/memory-graph", target).href, { timeout: 5000 });
  if (!response.ok()) throw new Error(`Memory API returned ${response.status()}`);
  const graph = await response.json();
  if (!graph.available || !graph.nodes.length) throw new Error("The server is not a populated Memory preview");
  console.log(`SOURCE version=${graph.version} nodes=${graph.nodes.length} edges=${graph.edges.length}`);
  await check("all cached nodes and edges render", async () => {
    await expect(page.locator("[data-node]")).toHaveCount(graph.nodes.length);
    await expect(page.locator("[data-edge-kind]")).toHaveCount(graph.edges.length);
  });
  await check("every directed edge has a marker", async () => {
    const missing = await page.locator("[data-edge-kind]").evaluateAll(lines => lines.filter(line => {
      const id = line.querySelector("path")?.getAttribute("marker-end")?.match(/#([^)]*)/)?.[1];
      return !id || !document.getElementById(id);
    }).length);
    expect(missing).toBe(0);
  });
  await page.screenshot({ path: path.join(evidence, "memory-desktop.png") });
  const memory = graph.nodes.find(n => n.kind === "memory" && n.body);
  if (!memory) throw new Error("Fixture has no Memory with a body");
  await check("Memory opens its body", async () => {
    await page.locator('#memoryRecord').selectOption(memory.id);
    await expect(page.locator("#memoryDetail h2")).toHaveText(memory.title);
    expect(await page.locator("#memoryDetail .md").innerText()).not.toBe("No body.");
  });
  await page.screenshot({ path: path.join(evidence, "memory-detail.png") });
  await page.getByRole("button", { name: "Close Memory details" }).click();
  await check("Type filter preserves records and native semantics", async () => {
    await page.locator('#memoryType').selectOption('cites');
    await expect(page.locator('[data-edge-kind]')).toHaveCount(graph.edges.filter(e=>e.kind==='cites').length);
    await expect(page.locator('[data-node]')).toHaveCount(graph.nodes.length);
    expect(graph.nodes.filter(n=>n.kind==='memory').every(n=>n.status==='')).toBe(true);
  });
  await page.screenshot({ path: path.join(evidence, "memory-citations.png") });
  await page.locator('#memoryType').selectOption('all');
  await page.setViewportSize({ width: 412, height: 915 });
  await check("phone keeps readable Memory cards", async () => {
    await page.locator('#memoryRecord').selectOption(memory.id);
    await expect(page.locator(`[data-node="${memory.id}"] rect`)).toBeVisible();
    const box = await page.locator(`[data-node="${memory.id}"] rect`).boundingBox();
    expect(box.width).toBeGreaterThanOrEqual(240);
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x+box.width).toBeLessThanOrEqual(412);
  });
  await page.screenshot({ path: path.join(evidence, "memory-phone.png") });
  await check("browser has no console or uncaught errors", async () => expect(errors).toEqual([]));
} catch (e) {
  fail++;
  console.error(e);
} finally {
  await browser.close();
  console.log(`=== MEMORY_WEB_PREVIEW DONE pass=${pass} fail=${fail} ===`);
  process.exitCode = fail ? 1 : 0;
}
