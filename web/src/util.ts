// Small helpers shared by every view. No framework: the app renders HTML
// strings into a handful of fixed hosts, the way the mockup does.

export const $ = <T extends HTMLElement = HTMLElement>(sel: string): T => document.querySelector(sel) as T;
export const $opt = <T extends HTMLElement = HTMLElement>(sel: string): T | null => document.querySelector(sel) as T | null;

const ESC: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
export const esc = (s: unknown): string => String(s == null ? "" : s).replace(/[&<>"']/g, c => ESC[c]);

export function age(t: number): string {
  const s = Math.max(0, (Date.now() - t) / 1000);
  if (s < 60) return Math.floor(s) + "s";
  if (s < 3600) return Math.floor(s / 60) + "m";
  if (s < 86400) return Math.floor(s / 3600) + "h";
  if (s < 86400 * 14) return Math.floor(s / 86400) + "d";
  if (s < 86400 * 60) return Math.floor(s / 604800) + "w";
  return Math.floor(s / 2592000) + "mo";
}

export const fmtDate = (t: number): string => {
  if (!t) return "—";
  const d = new Date(t), p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

export const parseTime = (s: string): number => (s ? Date.parse(s) || 0 : 0);

/**
 * md renders the small Markdown subset issue text uses: headings, lists,
 * fences, inline code, bold and links. Everything is escaped first, and a
 * link keeps only its text: an issue must not be able to navigate the app.
 */
export function md(src: string): string {
  const out: string[] = [];
  let list: { t: "ul" | "ol"; items: string[] } | null = null;
  let code: string[] | null = null;
  const inline = (s: string) => esc(s)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>")
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "<u>$1</u>");
  const flush = () => {
    if (list) { out.push(`<${list.t}>${list.items.map(x => `<li>${inline(x)}</li>`).join("")}</${list.t}>`); list = null; }
  };
  for (const line of src.split("\n")) {
    if (line.startsWith("```")) {
      if (code) { out.push(`<pre><code>${esc(code.join("\n"))}</code></pre>`); code = null; } else { flush(); code = []; }
      continue;
    }
    if (code) { code.push(line); continue; }
    let m: RegExpMatchArray | null;
    if ((m = line.match(/^#{1,6}\s+(.*)/))) { flush(); out.push(`<h3>${inline(m[1])}</h3>`); }
    else if ((m = line.match(/^\s*[-*]\s+(.*)/))) {
      if (!list || list.t !== "ul") { flush(); list = { t: "ul", items: [] }; }
      list.items.push(m[1]);
    } else if ((m = line.match(/^\s*\d+[.)]\s+(.*)/))) {
      if (!list || list.t !== "ol") { flush(); list = { t: "ol", items: [] }; }
      list.items.push(m[1]);
    } else if (!line.trim()) flush();
    else { flush(); out.push(`<p>${inline(line)}</p>`); }
  }
  if (code) out.push(`<pre><code>${esc((code as string[]).join("\n"))}</code></pre>`);
  flush();
  return out.join("");
}

/** store keeps per-browser conveniences. Storage may be absent or refuse. */
export const store = {
  get<T>(k: string, d: T): T {
    try { const v = localStorage.getItem("b9s." + k); return v == null ? d : (JSON.parse(v) as T); } catch { return d; }
  },
  set(k: string, v: unknown): void {
    try { localStorage.setItem("b9s." + k, JSON.stringify(v)); } catch { /* storage is optional */ }
  },
  del(k: string): void {
    try { localStorage.removeItem("b9s." + k); } catch { /* storage is optional */ }
  },
};

export function haptic(ms: number): void {
  try { if (navigator.vibrate) navigator.vibrate(ms); } catch { /* not every platform vibrates */ }
}

/** uid makes idempotency keys. crypto.randomUUID needs a secure context, which plain LAN HTTP is not. */
export function uid(): string {
  try { if (crypto.randomUUID) return crypto.randomUUID(); } catch { /* fall through */ }
  return Date.now().toString(36) + "-" + Math.random().toString(36).slice(2, 12);
}

export async function copyText(text: string): Promise<boolean> {
  try { await navigator.clipboard.writeText(text); return true; } catch { /* fall back below */ }
  try {
    const ta = document.createElement("textarea");
    ta.value = text; ta.setAttribute("readonly", ""); ta.style.position = "fixed"; ta.style.opacity = "0";
    document.body.appendChild(ta); ta.select();
    const ok = document.execCommand("copy");
    ta.remove();
    return ok;
  } catch { return false; }
}
