// Boot: session, snapshot, first render, then the live stream.

import * as api from "./api";
import { ApiError } from "./api";
import { D } from "./data";
import { applyTextSize, closeSheet, refresh, setRetryLive, setText, toast } from "./actions";
import { busy, bindAll } from "./gestures";
import { bindHistory, openHash, parseHash, startHistory } from "./nav";
import { ensureVisible, render, renderBanner, renderStale, revealInTree } from "./render";
import { S } from "./state";
import { $, esc, store } from "./util";

function pulse(): void {
  const dot = document.getElementById("hdot");
  if (!dot) return;
  dot.classList.remove("pulse");
  void dot.offsetWidth;
  dot.classList.add("pulse");
}

let visualViewportFrame = 0;

function syncVisualViewport(): void {
  const app = document.getElementById("app");
  if (!app) return;
  const vv = window.visualViewport;
  const active = !!vv && vv.scale > 1.01;
  app.classList.toggle("vzoom", active);
  if (!active || !vv) {
    app.style.removeProperty("--vv-left");
    app.style.removeProperty("--vv-top");
    app.style.removeProperty("--vv-width");
    app.style.removeProperty("--vv-bar-y");
    return;
  }
  const px = (n: number) => `${Math.round(n * 100) / 100}px`;
  const layoutHeight = app.clientHeight || document.documentElement.clientHeight || vv.height;
  app.style.setProperty("--vv-left", px(vv.offsetLeft));
  app.style.setProperty("--vv-top", px(vv.offsetTop));
  app.style.setProperty("--vv-width", px(vv.width));
  app.style.setProperty("--vv-bar-y", px(vv.offsetTop + vv.height - layoutHeight));
}

function bindVisualViewport(): void {
  const schedule = () => {
    if (visualViewportFrame) return;
    visualViewportFrame = requestAnimationFrame(() => {
      visualViewportFrame = 0;
      syncVisualViewport();
    });
  };
  syncVisualViewport();
  window.addEventListener("resize", schedule);
  window.visualViewport?.addEventListener("resize", schedule);
  window.visualViewport?.addEventListener("scroll", schedule);
}

async function applyChange(signal: AbortSignal): Promise<void> {
  const list = document.getElementById("treeList");
  const top = list ? list.scrollTop : 0;
  const changed = await refresh(signal);
  if (signal.aborted) return;
  if (list) list.scrollTop = top;
  pulse();
  if (!changed.size) return;
  if (S.follow) {
    const id = [...changed][0];
    S.cursor = id;
    revealInTree(id);
    if (S.detail) S.detail.stack = [id];
    render();
    ensureVisible(id);
  }
}

function startLive(): void {
  const controller = new AbortController();
  const { signal } = controller;
  let pendingTimer = 0;
  const requestChange = () => {
    if (signal.aborted) return;
    // Replacing the row under a finger would interrupt its gesture.
    if (busy()) {
      if (!pendingTimer) pendingTimer = window.setTimeout(() => { pendingTimer = 0; requestChange(); }, 200);
      return;
    }
    void applyChange(signal).catch(error => {
      if (signal.aborted) return;
      toast("Reload failed: " + (error instanceof Error ? error.message : String(error)));
      stream.retryLater();
    });
  };
  const stream = api.live({
    onEvent(type, version) {
      if (type === "hello" && version !== D.version) requestChange();
      if (type === "changed" && version > D.version) requestChange();
      if (type === "project") requestChange();
      // Only b9s web --dev-assets sends it. The URL hash keeps view and selection.
      if (type === "assets") location.reload();
      if (type === "health") api.health().then(h => { D.health = h; render(); }, () => { /* the next snapshot carries it */ });
    },
    onState(state, retryIn) {
      S.live = state;
      S.retryIn = retryIn;
      renderStale();
      render();
    },
  }, signal);
  window.addEventListener("pagehide", () => {
    clearTimeout(pendingTimer);
    controller.abort();
  }, { once: true });
  setRetryLive(() => { stream.retryNow(); toast("Reconnecting…"); });
}

window.addEventListener("pageshow", event => { if (event.persisted) startLive(); });

async function boot(): Promise<void> {
  applyTextSize();
  const bootEl = $("#boot");
  try {
    const s = await api.session();
    setText(s.query || store.get("text", ""));
    D.public = s.public;
    D.build = { version: s.version, commit: s.commit, commit_url: s.commit_url, modified: s.modified };
    renderBanner(s.banner, s.banner_link);
    bindAll();
    bindVisualViewport();
    bindHistory(render, closeSheet);
    // The view first, so the list does not flash the tree; the issue once loaded.
    const link = parseHash();
    if (link) S.view = link.v;
    await refresh();
    // The link's place is the entry the app opened on, not a step after it.
    if (link && openHash(link)) history.replaceState(null, "");
    startHistory();
    render();
    bootEl.hidden = true;
    startLive();
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      bootEl.innerHTML = `<div><b>This browser is not paired.</b><br><br>Open the pairing link that <code>b9s web</code> printed when it started, once, on this device.</div>`;
      return;
    }
    bootEl.innerHTML = `<div><b>Could not load issues.</b><br><br>${esc(e instanceof Error ? e.message : String(e))}<br><br><button id="bootRetry">Retry</button></div>`;
    document.getElementById("bootRetry")?.addEventListener("click", () => location.reload());
  }
}

void boot();
