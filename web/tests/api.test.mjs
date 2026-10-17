import assert from "node:assert/strict";
import { build } from "esbuild";
import { afterEach, mock, test } from "node:test";

const bundle = await build({
  entryPoints: [new URL("../src/api.ts", import.meta.url).pathname],
  bundle: true, write: false, format: "esm", platform: "browser",
});
const moduleURL = "data:text/javascript;base64," + Buffer.from(bundle.outputFiles[0].text).toString("base64");
const csrfError = "missing or wrong X-B9s-CSRF header";

afterEach(() => mock.restoreAll());

async function client(t, replies) {
  const calls = [];
  mock.method(globalThis, "fetch", async (path, options) => {
    calls.push({ path, ...options });
    const reply = replies.shift();
    assert.ok(reply, `unexpected request: ${path}`);
    if (reply instanceof Error) throw reply;
    return new Response(JSON.stringify(reply.body), { status: reply.status || 200 });
  });
  const api = await import(moduleURL + "#" + encodeURIComponent(t.name));
  await api.session();
  return { api, calls };
}

test("open-project refreshes stale CSRF and retries the original POST once", { timeout: 5000 }, async t => {
  const project = { key: "other" };
  const { api, calls } = await client(t, [
    { body: { csrf: "old" } }, { status: 403, body: { error: csrfError } },
    { body: { csrf: "fresh" } }, { body: project },
  ]);
  assert.deepEqual(await api.openProject("other"), project);
  assert.deepEqual(calls.map(c => c.path), ["api/session", "api/projects/open", "api/session", "api/projects/open"]);
  assert.equal(calls[1].headers["X-B9s-CSRF"], "old");
  assert.equal(calls[3].headers["X-B9s-CSRF"], "fresh");
  assert.equal(calls[3].body, calls[1].body);
  assert.equal(calls[3].credentials, "same-origin");
  assert.equal(calls[2].cache, "no-store");
});

test("write keeps its generated idempotency key through CSRF recovery", { timeout: 5000 }, async t => {
  const { api, calls } = await client(t, [
    { body: { csrf: "old" } }, { status: 403, body: { error: csrfError } },
    { body: { csrf: "fresh" } }, { body: { ok: true } },
  ]);
  assert.equal((await api.write({ op: "update", ids: ["t-2"], fields: { status: "in_progress" } })).ok, true);
  assert.equal(calls.length, 4);
  assert.equal(calls[3].body, calls[1].body);
  assert.ok(JSON.parse(calls[3].body).key);
});

test("a repeated CSRF rejection stops after one refresh and one retry", { timeout: 5000 }, async t => {
  const { api, calls } = await client(t, [
    { body: { csrf: "old" } }, { status: 403, body: { error: csrfError } },
    { body: { csrf: "fresh" } }, { status: 403, body: { error: csrfError } },
  ]);
  const result = await api.write({ op: "update", ids: ["t-2"] });
  assert.equal(result.ok, false);
  assert.equal(result.error, csrfError);
  assert.equal(calls.length, 4);
});

for (const status of [401, 403, 500]) {
  test(`an unrelated ${status} does not refresh or retry`, { timeout: 5000 }, async t => {
    const { api, calls } = await client(t, [
      { body: { csrf: "old" } }, { status, body: { error: "permission denied" } },
    ]);
    await assert.rejects(api.openProject("other"), e => e instanceof api.ApiError && e.status === status && e.message === "permission denied");
    assert.equal(calls.length, 2);
  });
}

for (const status of [401, 403]) {
  test(`a session refresh rejected with ${status} stops recovery`, { timeout: 5000 }, async t => {
    const { api, calls } = await client(t, [
      { body: { csrf: "old" } }, { status: 403, body: { error: csrfError } },
      { status, body: { error: "session refused" } },
    ]);
    const result = await api.write({ op: "update", ids: ["t-2"] });
    assert.equal(result.ok, false);
    assert.equal(result.error, "session refused");
    assert.equal(calls.length, 3);
  });
}

test("a failed session fetch does not trigger the write network retry", { timeout: 5000 }, async t => {
  const { api, calls } = await client(t, [
    { body: { csrf: "old" } }, { status: 403, body: { error: csrfError } },
    new TypeError("offline"),
  ]);
  const result = await api.write({ op: "update", ids: ["t-2"] });
  assert.equal(result.ok, false);
  assert.match(result.error, /offline/);
  assert.equal(calls.length, 3);
});

test("an ambiguous write network failure retries with the same idempotency key", { timeout: 5000 }, async t => {
  const { api, calls } = await client(t, [
    { body: { csrf: "current" } }, new TypeError("response dropped"), { body: { ok: true } },
  ]);
  assert.equal((await api.write({ op: "update", ids: ["t-2"] })).ok, true);
  assert.equal(calls.length, 3);
  assert.equal(calls[2].body, calls[1].body);
});

test("a network failure after CSRF recovery cannot start another replay", { timeout: 5000 }, async t => {
  const { api, calls } = await client(t, [
    { body: { csrf: "old" } }, { status: 403, body: { error: csrfError } },
    { body: { csrf: "fresh" } }, new TypeError("response dropped"),
  ]);
  const result = await api.write({ op: "update", ids: ["t-2"] });
  assert.equal(result.ok, false);
  assert.equal(result.error, "response dropped");
  assert.equal(calls.length, 4);
});

test("a CSRF failure after a network retry cannot start another replay", { timeout: 5000 }, async t => {
  const { api, calls } = await client(t, [
    { body: { csrf: "old" } }, new TypeError("response dropped"),
    { status: 403, body: { error: csrfError } },
  ]);
  const result = await api.write({ op: "update", ids: ["t-2"] });
  assert.equal(result.ok, false);
  assert.equal(result.error, csrfError);
  assert.equal(calls.length, 3);
});
