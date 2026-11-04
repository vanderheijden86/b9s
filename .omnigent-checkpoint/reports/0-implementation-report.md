# bd-hsqf implementation report

Implementation and regression tests pass. The whole-tree formatting check reports 12 unchanged Go files. Required formatting gate assessment remains with Polly under **bd-xrbu**. No other required-check failure remains. There is no separate unrelated follow-up.

## Git preflight and provenance

- Repository: `/home/omnigent/workspace/b9s`, origin `https://github.com/vanderheijden86/b9s.git`.
- Actual branch: `omnigent/bd-hsqf`.
- Initial bootstrap checkout: clean `main`.
- Current HEAD and fetched base `origin/main`: `3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`.
- Fetch: `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`, exit 0. Base ref exists.
- Exact delivery probe: `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin omnigent/bd-hsqf`, exit 2, remote absent. Local branch was also absent.
- Branch created with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-hsqf origin/main`.
- Required commits: none. Required-commit ancestry checks are not applicable.
- Extra ancestry: `git merge-base --is-ancestor origin/main HEAD`, exit 0.
- Source: open PR #35, target main, head `omnigent/bd-olpr`, commit `03a5dbed3625286a8a4bcdff468204c33fb23521`, `fix(web): recover stale CSRF sessions after server restart`, `Refs: bd-olpr`.
- Source parent/common base: `c62c9bd39e89fa20f751982e6c75aef3e8fc1f2f`. `git merge-base --is-ancestor 03a5dbed^ origin/main` and the same check against HEAD both exited 0.
- Source fetched with `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin refs/heads/omnigent/bd-olpr:refs/remotes/origin/omnigent/bd-olpr`. The initial fetch populated FETCH_HEAD only, so the explicit refspec established the remote source ref.
- Checkpoint inspect exited 2: no checkpoint existed. Implementation began only after the preflight report and passed milestone.
- Full preflight evidence: `/tmp/bd-hsqf-evidence/preflight.txt`.

## Implementation

Applied PR #35 as uncommitted changes onto the fetched main. No delivery commit, merge commit, force operation, delivery push or PR creation occurred.

The API client, API regressions, package scripts and pairing spec preserve the source PR exactly. The API test blob matches source blob `3af7e67f52300cc500ffc7bf1dd0e46dcfd6e85b`. The harness combines its restart support with main's bounded swipe-geometry guard. Architecture documentation retains main's hosted discovery paragraph and adds the original session-recovery paragraph. The embedded bundle and source fingerprint were regenerated from the combined current source, preserving later main features.

Intent remains: only the exact CSRF 403 triggers session refresh and one POST replay. Writes keep the same idempotency key. Network retry and CSRF retry share one replay limit. Repeated errors and failed session refreshes reach the existing error UI. Renewing a pairing secret still requires pairing again.

Existing PR regression tests were reused test-first. Before the source fix, the API suite reproduced missing recovery: **5 passed, 7 failed, 0 skipped**, exit 1. After the source fix: **12 passed, 0 failed, 0 skipped**, exit 0. No new product behavior or additional test-first cycle was needed. Existing tests enforce the recovery and replay-limit contract.

## Changed files

- `docs/ARCHITECTURE.md`: original session-recovery contract beside current auth/discovery documentation.
- `docs/testing.md`: original API and real restart test instructions.
- `pkg/web/dist/app.js`: rebuilt combined bundle.
- `pkg/web/dist/source.sha256`: rebuilt source fingerprint.
- `web/package.json`: original test:api and E2E precheck scripts.
- `web/src/api.ts`: original CSRF recovery and shared replay limit.
- `web/tests/api.test.mjs`: new file, copied from PR #35, 12 existing regression cases.
- `web/tests/harness.ts`: original bounded restart support, retaining main's swipe guard.
- `web/tests/pairing.spec.ts`: original real process restart regression.

No files deleted. No Go files changed. No ADR change is needed for this intent-preserving transplant. Architecture and test documentation from the source PR are retained.

## Commands and observed results

All commands ran in the shared repository or use its absolute path. The browser wrapper uses the actual Node project root, `/home/omnigent/workspace/b9s/web`.

| Check | Passed | Failed | Skipped | Exit |
| --- | ---: | ---: | ---: | ---: |
| API RED before source fix | 5 | 7 | 0 | 1, expected |
| API GREEN | 12 | 0 | 0 | 0 |
| Stable top-level Go results, 22 packages | 2085 | 0 | 18 | 0 |
| Stable named results including parents/subtests | 3447 | 0 | 20 | 0 |
| Focused Go tests | 5 | 0 | 0 | 0 |
| Browser smoke | 3 | 0 | 0 | 0 |
| Browser E2E | 28 | 0 | 0 | 0 |
| E2E API precheck | 12 | 0 | 0 | 0 |
| E2E browser guards | 16 | 0 | 0 | 0 |

Browser E2E also had **0 flaky, 0 interrupted, 0 unrun**, with no global reporter errors. Do not add the two stable rows together. The second includes parent and subtest results. Stable runtime skips cover optional real binaries, S3, live Dolt, Memory fixtures/research and opt-in stress tests. Makefile's declared host-sensitive exclusions and terminal E2E package exclusions also apply. The stable suite ran alone, without a baseline comparison.

Exact dependency and static commands:

```sh
timeout --kill-after=10s 240s go -C /home/omnigent/workspace/b9s mod download
timeout --kill-after=10s 180s npm --prefix /home/omnigent/workspace/b9s/web ci
timeout --kill-after=5s 60s npm --prefix /home/omnigent/workspace/b9s/web run typecheck
timeout --kill-after=5s 60s npm --prefix /home/omnigent/workspace/b9s/web run build
timeout --kill-after=10s 240s go -C /home/omnigent/workspace/b9s build ./...
timeout --kill-after=5s 120s go -C /home/omnigent/workspace/b9s vet ./...
git -C /home/omnigent/workspace/b9s diff --check
```

All exited 0. Static commands do not have test counts. npm ci installed 122 packages and printed its dependency audit advisory. Dependencies were not changed.

Exact test commands:

```sh
# RED, before API source transplant
timeout --kill-after=5s 30s node --test --test-timeout=20000 /home/omnigent/workspace/b9s/web/tests/api.test.mjs
# GREEN
timeout --kill-after=5s 30s npm --prefix /home/omnigent/workspace/b9s/web run test:api
GOFLAGS='-v -count=1' timeout --kill-after=10s 660s make -C /home/omnigent/workspace/b9s test-stable
timeout --kill-after=5s 120s go -C /home/omnigent/workspace/b9s test ./pkg/web -run '^(TestWriteWithoutCSRFHeaderIsRefused|TestHeaderAuthStillNeedsCSRFForWrites|TestRetriedWriteRunsBdOnce|TestEmbeddedBundleIsCurrent|TestGeneratedTypesAreCurrent)$' -count=1 -timeout=60s -json
lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-hsqf-evidence/browser-smoke-web.json --seconds 90
lane-browser fingerprint --project /home/omnigent/workspace/b9s/web
B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_NAME=/tmp/bd-hsqf-evidence/playwright-results.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm run test:e2e -- tests/pairing.spec.ts tests/write.spec.ts --project pixel --project iphone --workers=1 --reporter=list,json --output=/tmp/bd-hsqf-evidence/playwright-output
```

Focused Go results are exactly the five named tests. Browser tests retain the project's 30-second per-test limit, 5-second actions and assertions, 10-second navigation, zero retries, and bounded restart waits. The browser command completed in 45.3 seconds. Its actual exit 0 and fresh JSON report were passed to lane-progress, which derived the measured result.

Runtime: Go 1.25.5 linux/amd64, Node v22.23.4, npm 10.9.9, Playwright 1.63.0, CPU affinity 0-3. Native browser runtime fingerprint `a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6`.

Both browser restart proofs show a changed server process, 403 then 200, a changed CSRF value, an identical request body, one session refresh, one bd call, and no page errors. Fixtures are disposable JSONL projects with fake bd and isolated configuration. No test writes to shared Dolt.

## Findings and remaining gate

**Required formatting gate requiring Polly assessment: bd-xrbu.**

```sh
timeout --kill-after=5s 60s gofmt -l /home/omnigent/workspace/b9s/cmd /home/omnigent/workspace/b9s/internal /home/omnigent/workspace/b9s/pkg /home/omnigent/workspace/b9s/tests
```

The gofmt process exited 0 but reported 12 files, so the formatting assertion exited 1. Every reported file is unchanged from fetched main. No formatting edits were made outside the assigned transplant. bd-xrbu is filed with discovered-from:bd-hsqf and deferred. Polly must assess this existing-tree finding against the required gate. Changed-file whitespace check passes, and no Go file is part of this delivery.

Files: `internal/datasource/watch.go`, `pkg/ui/background_worker.go`, `pkg/ui/background_worker_test.go`, `pkg/ui/detail_edit_key_test.go`, `pkg/ui/integration_test.go`, `pkg/ui/model_test.go`, `pkg/ui/status_picker_test.go`, `pkg/ui/tree_bench_test.go`, `pkg/ui/tree_debug_test.go`, `pkg/ui/tree_detail_filtered_child_test.go`, `pkg/ui/truncate_test.go`, `tests/e2e/tree_view_e2e_test.go`.

Resolved environment invocation error: the first browser smoke and fingerprint used repository root and reported missing root node_modules/playwright/package.json. Using the actual web package root passed smoke and fingerprint. No browser installation or dependency-check bypass was used. No unresolved environment failure remains.

## Evidence and handoff

All evidence: `/tmp/bd-hsqf-evidence/`.

- `preflight.txt`, `implementation.txt`: passed stage evidence.
- `api-red.tap`, `api-green.tap`: observed RED/GREEN proof.
- `test-stable-summary.json`, `test-stable.log`, `test-stable.exit`: stable results and runtime skip list.
- `go-focused-summary.json`, `go-focused.jsonl`, `go-focused.exit`: focused results.
- `typecheck.txt`, `web-build.txt`, `go-build.txt`, `go-vet.txt`, `diff-check.txt`, `formatting.txt`: static gate outputs.
- `browser-smoke-web.json`, `browser-environment-web.json`: browser smoke and runtime.
- `playwright-results.json`, `playwright.exit`, `playwright.log`: fresh measured browser report and precheck counts.
- `restart-summary.json` and the two referenced `playwright-output/*/restart-proof.json` files: real restart proof.
- `artifact-hashes.txt`: tested binary and embedded bundle hashes.
- `api-evidence.json`, `checkpoint-api-compare.json`, `browser-evidence.json`: structured retry evidence. API comparison before the E2E precheck returned compatible=true, complete=true, approved=false. Tests reported here were executed in this attempt.

Separate unapproved checkpoint ref `refs/heads/omnigent-checkpoints/bd-hsqf` was pushed successfully. Delivery work remains uncommitted. The final checkpoint save result is recorded separately in `checkpoint-final.json`.

Independent reviewer and tester have not run. Delivery commit/push and PR creation remain unauthorized. No preview was requested or deployed. A later PR must target main and explicitly name **#35** as superseded. Pairing-secret rotation and consistency across replicas retain the source PR's limitations.
