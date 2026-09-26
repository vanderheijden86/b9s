// Builds the b9s web SPA into pkg/web/dist, which the Go binary embeds.
//
// dist is committed so `go build` needs no Node toolchain. dist/source.sha256
// records a hash of every input, and TestEmbeddedBundleIsCurrent in
// pkg/web recomputes it, so a source change without a rebuild fails the Go
// tests. Keep the input list and the hash format in step with that test.

import { build } from "esbuild";
import { createHash } from "node:crypto";
import { deflateSync } from "node:zlib";
import { copyFileSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const dist = join(root, "..", "pkg", "web", "dist");

rmSync(dist, { recursive: true, force: true });
mkdirSync(dist, { recursive: true });

await build({
  entryPoints: [join(root, "src", "main.ts")],
  bundle: true,
  minify: true,
  format: "iife",
  target: ["es2020", "safari15"],
  outfile: join(dist, "app.js"),
  legalComments: "none",
  logLevel: "warning",
});

copyFileSync(join(root, "src", "app.css"), join(dist, "app.css"));
for (const name of readdirSync(join(root, "public"))) copyFileSync(join(root, "public", name), join(dist, name));

writeFileSync(join(dist, "icon-192.png"), icon(192));
writeFileSync(join(dist, "icon-512.png"), icon(512));
writeFileSync(join(dist, "apple-touch-icon.png"), icon(180));

writeFileSync(join(dist, "source.sha256"), sourceHash(root) + "\n");

/** sourceHash is the hash TestEmbeddedBundleIsCurrent recomputes. */
function sourceHash(dir) {
  const files = [];
  const walk = d => {
    for (const name of readdirSync(d)) {
      const p = join(d, name);
      if (statSync(p).isDirectory()) walk(p);
      else files.push(p);
    }
  };
  walk(join(dir, "src"));
  walk(join(dir, "public"));
  files.push(join(dir, "build.mjs"), join(dir, "package-lock.json"));
  const rel = files.map(f => relative(dir, f).split(sep).join("/")).sort();
  const h = createHash("sha256");
  for (const r of rel) {
    const data = readFileSync(join(dir, r));
    h.update(r + "\n" + data.length + "\n");
    h.update(data);
  }
  return h.digest("hex");
}

/**
 * icon draws the app icon: the TUI accent as a rounded square with three
 * tree rows, the second one indented. PNG needs no image library.
 */
function icon(size) {
  const px = Buffer.alloc(size * size * 4);
  const bg = [0x16, 0x17, 0x1d], accent = [0xbd, 0x93, 0xf9], ink = [0x16, 0x17, 0x1d], cyan = [0x8b, 0xe9, 0xfd];
  const r = size * 0.22, pad = size * 0.06;
  const inRounded = (x, y) => {
    const lo = pad, hi = size - pad;
    if (x < lo || x > hi || y < lo || y > hi) return false;
    const cx = Math.min(Math.max(x, lo + r), hi - r), cy = Math.min(Math.max(y, lo + r), hi - r);
    return (x - cx) ** 2 + (y - cy) ** 2 <= r * r;
  };
  const bars = [
    { x: 0.24, y: 0.28, w: 0.52, c: ink },
    { x: 0.34, y: 0.45, w: 0.42, c: ink },
    { x: 0.34, y: 0.62, w: 0.30, c: cyan },
  ];
  const bh = size * 0.1;
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      let c = bg;
      if (inRounded(x + 0.5, y + 0.5)) {
        c = accent;
        for (const b of bars) {
          const bx = b.x * size, by = b.y * size;
          if (x >= bx && x < bx + b.w * size && y >= by && y < by + bh) c = b.c;
        }
      }
      const o = (y * size + x) * 4;
      px[o] = c[0]; px[o + 1] = c[1]; px[o + 2] = c[2]; px[o + 3] = 255;
    }
  }
  return png(size, size, px);
}

function png(w, h, rgba) {
  const raw = Buffer.alloc((w * 4 + 1) * h);
  for (let y = 0; y < h; y++) rgba.copy(raw, y * (w * 4 + 1) + 1, y * w * 4, (y + 1) * w * 4);
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8; ihdr[9] = 6; ihdr[10] = 0; ihdr[11] = 0; ihdr[12] = 0;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr), chunk("IDAT", deflateSync(raw, { level: 9 })), chunk("IEND", Buffer.alloc(0)),
  ]);
}

function chunk(type, data) {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
  const td = Buffer.concat([Buffer.from(type, "ascii"), data]);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(td) >>> 0);
  return Buffer.concat([len, td, crc]);
}

function crc32(buf) {
  let c = ~0;
  for (const b of buf) {
    c ^= b;
    for (let k = 0; k < 8; k++) c = c & 1 ? (c >>> 1) ^ 0xedb88320 : c >>> 1;
  }
  return ~c;
}
