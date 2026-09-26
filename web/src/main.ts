// Boot: session, snapshot, first render, then the live stream.

import * as api from "./api";
import { ApiError } from "./api";
import { D } from "./data";
import { applyTextSize, closeSheet, refresh, setRetryLive, setText, toast } from "./actions";
import { busy, bindAll } from "./gestures";
import { bindHistory, openHash, parseHash } from "./nav";
import { ensureVisible, render, renderStale, revealInTree } from "./render";
import { S } from "./state";
import { $, esc, store } from "./util";

function pulse(): void {
  const dot = document.getElementById("hdot");
  if (!dot) return;
  dot.classList.remove("pulse");
  void dot.offsetWidth;
  dot.classList.add("pulse");
}

/**
 * A change that arrives mid-gesture waits: a re-render would replace the
 * row under the finger. The next poll after the gesture applies it.
 */
let pending = false;
async function applyChange(): Promise<void> {
  if (busy()) {
    if (!pending) { pending = true; const wait = () => (busy() ? setTimeout(wait, 200) : (pending = false, void applyChange())); setTimeout(wait, 200); }
    return;
  }
  const list = document.getElementById("treeList");
  const top = list ? list.scrollTop : 0;
  const changed = await refresh();
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
  const stream = api.live({
    onEvent(type, version) {
      if (type === "hello" && version !== D.version) void applyChange();
      if (type === "changed" && version > D.version) void applyChange();
      if (type === "project") void applyChange();
      if (type === "health") api.health().then(h => { D.health = h; render(); }, () => { /* the next snapshot carries it */ });
    },
    onState(state, retryIn) {
      S.live = state;
      S.retryIn = retryIn;
      renderStale();
      render();
    },
  });
  setRetryLive(() => { stream.retryNow(); toast("Reconnecting…"); });
}

async function boot(): Promise<void> {
  applyTextSize();
  const bootEl = $("#boot");
  try {
    const s = await api.session();
    setText(s.query || store.get("text", ""));
    bindAll();
    bindHistory(render, closeSheet);
    // The view first, so the list does not flash the tree; the issue once loaded.
    const link = parseHash();
    if (link) S.view = link.v;
    await refresh();
    // The link's place is the entry the app opened on, not a step after it.
    if (link && openHash(link)) { history.replaceState(null, ""); render(); }
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
