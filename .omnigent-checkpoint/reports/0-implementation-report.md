# bd-jo57: IMPLEMENTATION_GREEN

All required implementation checks passed. Independent review and tester execution remain pending. No required blockers remain for this implementation. The delivery branch has no delivery commit or push. No PR was opened.

## Git preflight

- Repository: `/home/omnigent/workspace/b9s`.
- Assigned base and PR target: `main`.
- Initial checkout: clean `main`.
- Actual branch: `omnigent/bd-jo57`.
- HEAD: `906b708cb841c19f38bb571a7c72406a1be70417`.
- Fetched `origin/main`: `906b708cb841c19f38bb571a7c72406a1be70417`.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s show-ref --verify refs/remotes/origin/main`: exit 0.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-jo57`: exit 2, absent remote branch. No delivery ref existed to fetch or resume.
- No local delivery branch existed. Created it with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-jo57 origin/main`.
- Required commit ancestry against fetched base and HEAD: not applicable, dispatch named no prerequisite commits.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-jo57 --base origin/main`: checkpoint absent.
- `beads-connect` ran once and exited 0. Attributed comments record progress without changing dispatcher-owned task state.

## Root cause and fix

Live events discarded the promise returned by `applyChange`. A rejected snapshot fetch therefore escaped as an unhandled rejection. The independent server fixture could stop before the browser context closed, which exposed this rejection during teardown.

The live refresh path handles failed reads by retaining the visible snapshot, showing the reload error and offline banner, and reconnecting through the existing stream backoff. A page-lifetime AbortSignal cancels live snapshot reads, closes the stream, and clears deferred gesture/reconnect timers on pagehide. Persisted pages reconnect on pageshow. Superseded EventSource callbacks cannot act on the current stream.

The browser context explicitly depends on the server fixture and closes before server shutdown. A context-wide error collector asserts zero unexpected page errors through context closure. No page-error filters, skipped regressions, test retries, or timeout increases were introduced.

The original dispatch2 JSON and trace paths were absent in this Pod. Fresh RED runs reproduced missing stale UI, incorrect fixture lifetime, and missing pagehide cancellation. The final normal gate includes the regressions and unchanged board drag on both phone projects.

## Changed files

- `web/src/api.ts`: snapshot cancellation, stream cancellation, shared reconnect handling, stale-source guards.
- `web/src/actions.ts`: cancellation-aware snapshot refresh with a pre-apply abort guard.
- `web/src/main.ts`: live failure recovery, cancellation of deferred work, page lifecycle handling.
- `web/tests/harness.ts`: server/context lifetime dependency and unexpected-page-error assertions through teardown.
- `web/tests/live.spec.ts`: network rejection/recovery, server lifetime, and pagehide cancellation/resume regressions.
- `docs/ARCHITECTURE.md`: live failure and page lifecycle behavior.
- `docs/testing.md`: fixture lifetime and browser error contract.
- `pkg/web/dist/app.js`: regenerated embedded application.
- `pkg/web/dist/source.sha256`: regenerated source fingerprint.

No unrelated density test change, dependency change, Go source change, or ADR change. This restores the existing offline/reconnect contract without a new architectural decision.

## Commands and measured results

All commands ran from `/home/omnigent/workspace/b9s` unless the browser wrapper selected `/home/omnigent/workspace/b9s/web`. Redirected output was retained under `/tmp/bd-jo57`. Exit codes were captured after command completion.

Dependencies:

```sh
timeout 180s go mod download
timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci
```

Both exited 0. The committed JavaScript lockfile and vendored Go dependencies remain unchanged.

Browser environment:

```sh
lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-jo57/browser-smoke.json --seconds 90
lane-browser fingerprint --project /home/omnigent/workspace/b9s/web > /tmp/bd-jo57/browser-environment.json
```

Smoke: 3 passed, 0 failed, 0 skipped, exit 0. Fingerprint: Playwright 1.63.0, Node v22.23.4, x86_64, kernel 6.8.0-147-generic, CPUs 0–3, two graphics threads. Native runtime SHA-256 `a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6`. Go: go1.25.5 linux/amd64. npm: 10.9.9.

RED:

```sh
PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-jo57/focused-red.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/live.spec.ts --grep 'failed live snapshot|fixture server outlives' --project iphone --workers=1
PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-jo57/cancellation-red.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/live.spec.ts --grep 'leaving the page' --project iphone --workers=1
```

First RED: 0 passed, 2 failed, 0 skipped/flaky/interrupted/unrun, exit 1. The stale banner remained hidden after a failed snapshot. The context-close assertion observed the server already killed.

Cancellation RED: 0 passed, 1 failed, 0 skipped/flaky/interrupted/unrun, exit 1. The bounded requestfailed waiter expired because pagehide did not cancel the pending fetch.

Focused GREEN:

```sh
PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-jo57/focused-green.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/live.spec.ts tests/board.spec.ts --project pixel --project iphone --workers=1
```

28 passed, 0 failed/skipped/flaky/interrupted/unrun, exit 0. The final gate also checks these cases against the final bundle.

Project checks:

```sh
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build
GOFLAGS=-v timeout 900s make -C /home/omnigent/workspace/b9s test-stable
timeout 300s go build ./...
timeout 300s go vet ./...
git -C /home/omnigent/workspace/b9s diff --check
```

All exited 0. The Go stable suite ran alone. Top-level results: 2072 passed, 0 failed, 18 skipped. Including subtests: 3366 passed, 0 failed, 20 skipped. All 3386 RUN events have terminal results, with no interruption or unrun started tests. The Makefile's declared suite exclusions remain unchanged. Runtime skips concern optional Dolt, S3, Memory preview/research, stress environments and two missing real-data fixtures. No shared Dolt test writes ran. Go vet is the declared Go lint check. No separate JavaScript lint script is declared.

Normal required web gate:

```sh
PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-jo57/web-gate.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- make -C /home/omnigent/workspace/b9s web-gate
```

Exit 0. 434 passed, 0 failed, 4 existing declared exclusions, 0 flaky, 0 interrupted, 0 unrun. Global reporter errors: 0. No unexpected page errors, including teardown. Duration: 189734.992 ms. Each `test:e2e` invocation also ran the Node guard suite: 16 passed, 0 failed, 0 skipped.

Playwright keeps the declared 30-second test cap, 5-second assertions/actions, 10-second navigation cap, and zero retries. Node guards have a 20-second per-test cap. Browser runs have the required 900-second outer deadline. Go stable has its declared 10-minute package cap and a 900-second outer deadline. No deadline was increased.

## Existing web exclusions

The four skips match unchanged source:

- Pixel `memory.spec.ts:269`: wheel zoom, phones have no wheel.
- iPhone `memory.spec.ts:269`: wheel zoom, phones have no wheel.
- Desktop `desktop.spec.ts:198`: prose width, wide-only check.
- iPad `wide.spec.ts:458`: mobile WebKit has no wheel.

The generic lane-progress report is advisory `blocked` because it flags all skips. These are declared optional device/viewport exclusions, not skipped required checks. No required blocker remains. The scope/exclusion review report preserves this distinction.

## Evidence, durability and residual limits

Primary evidence: `/tmp/bd-jo57/web-gate.json`, `/tmp/bd-jo57/web-gate-evidence.json`, `/tmp/bd-jo57/test-stable-summary.json`, `/tmp/bd-jo57/focused-red.json`, `/tmp/bd-jo57/cancellation-red.json`, `/tmp/bd-jo57/focused-green.json`, `/tmp/bd-jo57/browser-smoke.json`, `/tmp/bd-jo57/browser-environment.json`, `/tmp/bd-jo57/exclusion-review.txt`. Full check logs and captured exit files remain in the same directory.

Unapproved checkpoint saves succeeded only on `refs/heads/omnigent-checkpoints/bd-jo57`. No delivery authorization was used or requested. The final checkpoint preserves this report, measured evidence, and all tracked changes. HEAD remains the preflight HEAD.

Residual limits: native browser BFCache eligibility is not asserted. The cancellation regression exercises pagehide/pageshow lifecycle events and a real pending fetch. Nightly-only browser projects and optional external services were not required or executed. Independent reviewer and tester evidence is still required from Polly's next roles.
