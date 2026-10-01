// The diagram module loads only after a detail contains a Mermaid fence.

type Mermaid = typeof import("mermaid").default;

let library: Promise<Mermaid> | undefined;
let nextID = 0;
const cache = new Map<string, Promise<string>>();
const assetBase = (document.currentScript as HTMLScriptElement | null)?.src || document.baseURI;

function engine(): Promise<Mermaid> {
  if (library) return library;
  const pending = new Promise<Mermaid>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = new URL("mermaid.min.js", assetBase).href;
    script.onload = () => {
      const mermaid = (window as Window & { mermaid?: Mermaid }).mermaid;
      if (!mermaid) { reject(new Error("Mermaid did not load")); return; }
      try {
        mermaid.initialize({ startOnLoad: false, securityLevel: "strict", suppressErrorRendering: true, theme: "dark", sequence: { mirrorActors: true } });
        resolve(mermaid);
      } catch (error) { reject(error); }
    };
    script.onerror = () => reject(new Error("Mermaid could not load"));
    document.head.appendChild(script);
  });
  library = pending;
  pending.catch(() => { if (library === pending) library = undefined; });
  return pending;
}

function diagram(source: string): Promise<string> {
  const existing = cache.get(source);
  if (existing) return existing;
  const pending = engine().then(async mermaid => (await mermaid.render(`b9s-mermaid-${++nextID}`, source)).svg);
  cache.set(source, pending);
  if (cache.size > 16) cache.delete(cache.keys().next().value!);
  pending.catch(() => { if (cache.get(source) === pending) cache.delete(source); });
  return pending;
}

export function renderMermaidBlocks(root: ParentNode): void {
  for (const block of root.querySelectorAll<HTMLElement>(".mermaid-diagram")) {
    const source = block.querySelector("code")?.textContent;
    if (!source) continue;
    block.dataset.renderState = "loading";
    void diagram(source).then(svg => {
      if (!block.isConnected) return;
      block.innerHTML = svg;
      block.dataset.renderState = "ready";
      const image = block.querySelector("svg");
      if (image?.viewBox.baseVal.width) image.style.width = `${Math.ceil(image.viewBox.baseVal.width)}px`;
    }).catch(() => { if (block.isConnected) block.dataset.renderState = "error"; });
  }
}
