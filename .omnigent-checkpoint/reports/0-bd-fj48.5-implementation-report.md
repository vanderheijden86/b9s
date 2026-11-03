# bd-fj48.5 implementation report

Status: implementation complete, required lane evidence blocked by **bd-jsbf**. Delivery is not authorized. No delivery commit, delivery push or PR was made.

## Git preflight

- Repository: /home/omnigent/workspace/b9s
- Actual branch: omnigent/bd-fj48.5
- HEAD and fetched origin/main: 793407bd86c74ba9985c9870c39392974ec8380e
- Fetched origin main with timeout 60s and checked refs/remotes/origin/main exists.
- Local delivery branch absent. Exact remote probe `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-fj48.5` exited 2, meaning absent.
- Created delivery branch with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-fj48.5 origin/main`. Initial worktree clean.
- Required commits: none. Ancestry checks against base and HEAD: not applicable.
- Checkpoint inspection: no checkpoint existed. No restore attempted.
- Evidence: /tmp/bd-fj48.5-preflight.txt. Final HEAD and base still match preflight.

## Changes

- web/tests/harness.ts: acquire nullable row geometry within a five-second deadline before any pointer input. Each attempt resolves the locator again. The pointer gesture runs once. Missing geometry still fails explicitly.
- web/tests/write.spec.ts: regression replaces a row with a temporarily boxless replacement. It checks the real gesture, fake-bd command, persisted status, rendered status and browser page errors.
- docs/testing.md: document geometry acquisition and its deadline.

The perf spec, feedback assertions, fixture write checks, sample counts, budgets, CI allowance and retries are unchanged. No product code changed. No new source files. `git diff --check` exited 0.

Read project/baseline instructions, docs/testing.md, executable guard guidance and ADRs 0019, 0052, 0059 and 0064. The regression is the executable guard. No new architecture decision was needed.

## Commands and measured results

All commands ran from /home/omnigent/workspace/b9s unless lane-browser set the project directory. Go dependencies: `timeout 180s go mod download`, exit 0. Web dependencies: `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci`, exit 0. No lockfile changes.

Browser preflight:

```sh
lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-fj48.5-browser-smoke.json --seconds 90
lane-browser fingerprint --project /home/omnigent/workspace/b9s/web
```

Smoke: exit 0, 3 passed, 0 failed, 0 skipped. Fingerprint is /tmp/bd-fj48.5-browser-fingerprint.json.

Every browser command below used `CI=1 B9S_KEEP_TEST_FIXTURES=1`, with B9S_E2E_CPU_THROTTLE unset. Each test:e2e invocation ran the existing test:guards first: 16 passed, 0 failed, 0 skipped, 0 cancelled, on each of five invocations. Each Playwright report was fresh. All final Playwright outcomes had 0 flaky, 0 interrupted and 0 unrun.

Initial focused reproduction on the unchanged helper:

```sh
CI=1 B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-fj48.5-original.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 360 -- npm run test:e2e -- tests/perf.spec.ts -g 'write and live updates' --project pixel-perf --workers=1 --no-deps --reporter=list,json --output=/tmp/bd-fj48.5-original-output
```

Exit 1: 0 passed, 1 failed, 0 skipped. The unchanged 30-second test deadline failed in this run. It did not show null.y. Evidence: /tmp/bd-fj48.5-original.json and /tmp/bd-fj48.5-original.log.

RED regression, before changing the helper:

```sh
CI=1 B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-fj48.5-regression-red.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 120 -- npm run test:e2e -- tests/write.spec.ts -g 'swipe waits for a replacement row' --project pixel --workers=1 --no-deps --reporter=list,json --output=/tmp/bd-fj48.5-regression-red-output
```

Exit 1: 0 passed, 1 failed, 0 skipped. Exact failure: TypeError reading null.y at harness.ts:205. No bd calls occurred. Evidence: /tmp/bd-fj48.5-regression-red.json.

GREEN regression:

```sh
CI=1 B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-fj48.5-regression-green.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 120 -- npm run test:e2e -- tests/write.spec.ts -g 'swipe waits for a replacement row' --project pixel --project iphone --project desktop --workers=1 --no-deps --reporter=list,json --output=/tmp/bd-fj48.5-regression-green-output
```

Exit 0: 3 passed, 0 failed, 0 skipped. No browser page errors observed by the regression. Evidence: /tmp/bd-fj48.5-regression-green.json.

Required focused GREEN:

```sh
CI=1 B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-fj48.5-focused-green.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 120 -- npm run test:e2e -- tests/perf.spec.ts -g 'write and live updates' --project pixel-perf --workers=1 --no-deps --reporter=list,json --output=/tmp/bd-fj48.5-focused-green-output
```

Exit 0: 1 passed, 0 failed, 0 skipped. Test duration 28.7s. Medians: write feedback 35ms, write round trip 82ms, live update 1158ms. The live sample used the existing 1.5x CI allowance. Evidence: /tmp/bd-fj48.5-focused-green.json and /tmp/bd-fj48.5-focused-evidence.json.

Required stable suite, run alone with no base comparison:

```sh
GOFLAGS=-v timeout 660s make -C /home/omnigent/workspace/b9s test-stable
```

Exit 0: 21 packages passed. Top-level tests: 2057 passed, 0 failed, 18 skipped. Including nested subtests: 3345 passed, 0 failed, 20 skipped. No interrupted or unrun selected tests. Original Makefile exclusions retained: tests/e2e plus DoltIntegration, MIME sniffing, embedded-bd real store, opener, bd-contract and debouncer timing cases. Runtime skips require opt-in real bd binaries, S3, Dolt DSN, Memory previews/research, stress flags or two external fixture files. No shared Dolt writes. Reports: /tmp/bd-fj48.5-stable-summary.json, /tmp/bd-fj48.5-stable-detailed-summary.json. Log: /tmp/bd-fj48.5-stable.log.

Other required checks, each exit 0:

```sh
timeout 180s go build ./...
timeout 180s go vet ./...
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build
```

No web lint script is declared. The project supplies test:guards, typecheck and build. Go vet supplies the Go static check. Individual reports: /tmp/bd-fj48.5-go-build-report.txt, /tmp/bd-fj48.5-go-vet-report.txt, /tmp/bd-fj48.5-typecheck-report.txt, /tmp/bd-fj48.5-web-build-report.txt.

Project web-gate equivalent, default four workers, separate from timed perf:

```sh
CI=1 B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-fj48.5-web-gate.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm run test:e2e -- --project tokens --project pixel --project iphone --project desktop --project ipad --reporter=list,json --output=/tmp/bd-fj48.5-web-gate-output
```

Process exit 0: 431 passed, 0 failed, 4 skipped, 0 flaky, 0 interrupted, 0 unrun. Duration 209.3s. Report /tmp/bd-fj48.5-web-gate.json. The lane helper derived FAILED for milestone 28 because any skip prevents a passed milestone. No passed gate milestone is claimed.

## Required blocker

**bd-jsbf**, filed discovered-from:bd-fj48.5 and deferred, records the only unresolved required-check evidence failure: the declared project gate has four legitimate device/viewport skips, but lane-progress requires zero skips for success.

Exact existing exclusions:

| Project | Test | Source | Recorded reason |
| --- | --- | --- | --- |
| pixel | wheel zooms toward the pointer | web/tests/memory.spec.ts:269 | phones have no wheel |
| iphone | wheel zooms toward the pointer | web/tests/memory.spec.ts:269 | phones have no wheel |
| desktop | uses the extra width while prose keeps a readable measure | web/tests/desktop.spec.ts:198 | wide only |
| ipad | wheel over a cell scrolls the board or cursor cell | web/tests/wide.spec.ts:458 | Playwright has no wheel in mobile WebKit, and an iPad scrolls the board by touch |

No unrelated follow-up work or other unresolved required-check failures were found. The initial pre-fix deadline failure is retained in the evidence and the final focused check passed. Independent review and testing remain pending under the dispatch. Task state, assignee, dependencies and lane labels were not changed.

## Residual risks and environment

The focused perf test passed near its unchanged 30-second deadline. This run does not prove that all timing variability is gone. The fix covers nullable geometry before input. It does not freeze rendering during the gesture. Existing write and feedback assertions remain responsible for the resulting behavior.

Browser runtime: Playwright 1.63.0, Node v22.23.4, x86_64, kernel 6.8.0-147-generic, CPU affinity [0,1,2,3], two graphics threads, Pod boundary sandbox. Native runtime SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. Image digest was not exposed by runtime metadata or environment. Go: go1.25.5 linux/amd64.

## Checkpoint and delivery

The separate unapproved checkpoint ref refs/heads/omnigent-checkpoints/bd-fj48.5 was saved successfully during implementation and validation. A final save includes this report and measured gate evidence. It does not update the delivery branch. No delivery commit/push, PR or deployment was attempted. All three changed tracked files remain uncommitted for independent review/testing.
