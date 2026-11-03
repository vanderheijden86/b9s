# bd-e5u3.20.15 implementation dispatch 2

**BLOCKED handoff:** the assigned tablet density failure is fixed and passed fresh focused and nine-project runs. A separate required full-web-gate failure remains unresolved on **bd-jo57**. No earlier passing run clears this failure. No delivery commit, delivery push or PR performed.

## Required blocker and unrelated work

**Complete unresolved required-check failure list: bd-jo57 only.** Normal `make web-gate` exited **2**, with **429 passed, 1 failed, 4 declared exclusions, 0 flaky, 0 interrupted, 0 unrun**. Failed case: unchanged `web/tests/board.spec.ts:10`, iPhone, “the board stacks the columns as boxes and moves a card by drag onto another box”. The actual failure is unexpected page error `Unhandled Promise Rejection: TypeError: Load failed`, at live-refresh function `Ls` in `app.js:77:31804`, during context teardown. Raw JSON reports the page error as the failure and no global reporter errors. The trace also records a context teardown error. An intermediate polling mismatch inside the trace is not the final reported failure.

Filed **bd-jo57**, attributed its evidence, linked it through `discovered-from:bd-e5u3.20.15`, and deferred it under lane rules. This failure is outside the density fixture change but blocks the required gate. No production/harness expansion, suppression, skip, retry or timeout increase was used. The live-refresh fetch and fixture shutdown ordering need diagnosis under that issue. Server shutdown causing the rejection is a lead, not a proven root cause.

Evidence: `/tmp/bd-e5u3.20.15/dispatch2/web-gate.json`, `web-gate-failures.json`, `web-gate.log`, `gate-drag-failure/trace.zip`, and `gate-drag-failure/errors.json`.

Unrelated non-blocking work: **bd-969k**, unchanged dependency advisories (two low, one critical). The prior tablet failure on **bd-e5u3.20.15** is addressed by the new local evidence below. Independent reviewer/tester round 2 remains pending.

## Preflight and changed files

Repository `/home/omnigent/workspace/b9s`. Actual delivery branch `omnigent/bd-e5u3.20.15` retained with both existing uncommitted files. HEAD, explicitly fetched `origin/omnigent/bd-e5u3.20.14`, and required ancestor all equal **816e4988ebbf96d942727c2014a80050664f413d**. Both `merge-base --is-ancestor` checks, against HEAD and fetched base, exited 0 before edits. Exact remote delivery `ls-remote --exit-code --heads` exited 2, absent. The assigned PR target remains `omnigent/bd-e5u3.20.14`. No branch reset/switch or bootstrap-main substitution. Checkpoint inspect matched existing context, no restore over dirty work. Final branch, HEAD and fetched base remain the same.

Evidence: `/tmp/bd-e5u3.20.15/dispatch2/preflight.txt`.

Changed files remain exactly:

- `web/tests/board-detail.spec.ts`: explicitly select the saved epic-rail preference before navigation. Assert non-phone toolbar says `lanes rail`. Retain every existing exact 44px touch/24px desktop, selected-row, phone epic-header, rich epic-fact, detail-navigation and page-error assertion.
- `docs/testing.md`: explain that board lane preference is independent of responsive size, and density fixtures select the mode whose rich epic content they assert.

Dispatch 2 adds three test lines and extends the existing documentation line. No production, generated, configuration, lockfile or Go changes. No file deletions. Prior reports and tester evidence preserved.

## Exact tablet failure cause

The test inferred epic-rail mode from `#app` being non-phone. That implication is false. `S.board.lanes` is independent saved state, defaulted from `split()` during module initialization (`state.ts:47`). Responsive renders choose a phone or side-by-side board but preserve that lane preference. A tablet board in rows mode renders a rich `.lane` header, not `[data-epic]` rail cells.

Read ADR0065/0052/0053/0054, the tester report, raw JSON and trace. The unchanged focused test reproduced the failure in **7 of 12 executions**. All seven failure traces show initial `app phone`, then `app tablet`, toolbar `lanes rows`, and no `lanes rail`. This matches the tester's recorded startup sequence. The 44px density assertion succeeds, then the test spends its full unchanged 5000ms assertion deadline waiting for an element absent by the selected mode. This is a missing test precondition, not a slow rail render.

Fix: select `b9s.boardLanes="rail"` with `page.addInitScript` before `open`, following the existing epic-lane fixture pattern in `board.spec.ts`. Phone rendering still produces two epic lane headers without a rail. Non-phone rendering must explicitly report rail mode and preserve all rich facts. No acceptance assertion was weakened. The explicit fixture preference and toolbar assertion guard the missing precondition. Audit found no remaining project-name/prefix assumptions in `web/tests`.

Evidence: `tester-trace-actions.json`, `red-layout-observations.json`, `red-artifacts/*/trace.zip`, and `implementation.txt`, under `/tmp/bd-e5u3.20.15/dispatch2`.

## Focused RED and GREEN commands

All commands below ran from `/home/omnigent/workspace/b9s`. Declared per-test timeout 30s, assertion/action 5s, navigation 10s, retries 0. No timeout or retry configuration changed. Repetitions are separate executions, not automatic retries. Playwright reports flaky=0 even though the pre-fix repeated executions have both passes and failures.

RED before edits:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/dispatch2/red.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm run test:e2e -- tests/board-detail.spec.ts --grep 'density, selected rows' --project tablet-768 --repeat-each=12 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15/dispatch2/red-artifacts
```

Exit **1**, **5 passed / 7 failed / 0 skipped / 0 flaky / 0 interrupted / 0 unrun**, 66.339s. Every failure reproduces the missing epic rail at line 130.

Focused GREEN after fix, identical selection, repeat count and deadlines:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/dispatch2/focused-green.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm run test:e2e -- tests/board-detail.spec.ts --grep 'density, selected rows' --project tablet-768 --repeat-each=12 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15/dispatch2/focused-green-artifacts
```

Exit **0**, **12 passed / 0 failed / 0 skipped / 0 flaky / 0 interrupted / 0 unrun**, 35.597s.

Fresh full nine-project matrix:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/dispatch2/green.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 360 -- npm run test:e2e -- tests/board-detail.spec.ts --project pixel --project iphone --project phone-360 --project phone-412 --project tablet-768 --project tablet-1024 --project desktop-1440 --project desktop-1920 --project wide-2560 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15/dispatch2/green-artifacts
```

Exit **0**, **27 passed / 0 failed / 0 skipped / 0 flaky / 0 interrupted / 0 unrun**, 54.718s. All three tests run in each project. Tablet-768 reaches every assertion, including selected height, rich epic facts and parent detail navigation. Pixel/iPhone/phones/tablets measure 44px, desktops/wide 24px. See `density-measurements.json` and screenshots in `green-artifacts/`.

## Other required checks

Every required command executed on the fixed delivery working tree. No prior implementation, reviewer or tester pass was reused. Stable ran alone, without a base comparison or another check. The package timeout remains 10m, with a 660s outer deadline. Its package results contain no cache hits. No shared Dolt test writes occurred. Browser fixtures use isolated local JSONL and fake bd.

| Exact command | Observed result |
| --- | --- |
| `timeout 180s go mod download` | exit 0 |
| `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` | exit 0 |
| `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-e5u3.20.15/dispatch2/browser-smoke.json --seconds 90` | exit 0, 3 passed / 0 failed / 0 skipped |
| `PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/dispatch2/web-gate.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- make -C /home/omnigent/workspace/b9s web-gate` | **exit 2, 429 passed / 1 failed / 4 skipped**, 0 flaky/interrupted/unrun, 205.106s, blocker **bd-jo57** |
| `GOFLAGS='-v -count=1' timeout 660s make -C /home/omnigent/workspace/b9s test-stable` | exit 0, **3345 passed / 0 failed / 20 optional skips** including subtests, **2057 passed / 0 failed / 18 skips** top-level, 21 uncached packages |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck` | exit 0 |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build` | exit 0 |
| `timeout 180s go build ./...` | exit 0 |
| `timeout 180s go vet ./...` | exit 0, project lint check |
| `timeout 30s git -C /home/omnigent/workspace/b9s diff --check` | exit 0 |

The normal gate expands to `npm --prefix web run test:e2e -- --project tokens --project pixel --project iphone --project desktop --project ipad`, four declared workers. Each of the four E2E invocations first ran the Node guard: **16 passed / 0 failed / 0 skipped / 0 cancelled**. Stable includes docs: **3 passed / 0 failed / 0 skipped**. Go does not expose a complete test inventory or flake classification, so its unrun/flaky counts are unknown. Static/build checks do not report test counts. No separate web lint script is declared.

The four unchanged web exclusions remain phone wheel zoom (Pixel/iPhone), wide-only layout in desktop and mobile WebKit wheel scrolling in iPad. The 20 stable skips are the documented optional Dolt/S3/Memory/stress and missing real-fixture cases. These are separate from the real unexpected page-error failure on **bd-jo57**. No exclusions were added.

## Runtime and milestone evidence

`lane-browser fingerprint --project /home/omnigent/workspace/b9s/web` is preserved in `browser-fingerprint.json`: Playwright 1.63.0, Node v22.23.4, Linux x86_64, kernel 6.8.0-147-generic, CPU affinity [0,1,2,3], graphics threads 2, native runtime SHA256 `a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6`. Go go1.25.5, npm 10.9.9. Image digest is not exposed by the fingerprint.

All evidence paths below are under `/tmp/bd-e5u3.20.15/dispatch2/`:

| Stream sequence | Milestone | Evidence |
| --- | --- | --- |
| 60 | preflight passed | `preflight.txt` |
| 62 | dependencies passed | `npm-dependencies.log` |
| 64 | smoke passed | `browser-smoke.json` |
| 66 | RED failed, 5/7/0 | `red.json` |
| 69 | focused GREEN passed, 12/0/0 | `focused-green.json` |
| 71 | matrix passed, 27/0/0 | `green.json` |
| 72 | implementation fix passed | `implementation.txt` |
| 74 | required web gate failed, 429/1/4, exit 2 | `web-gate.json` |
| 76 | stable passed | `stable-results.json` |
| 78,80,82,84,86 | static/build checks passed | `typecheck.json`, `web-build.json`, `go-build.json`, `go-vet.json`, `diff-check.json` |

`checks-summary.json` collects measured command results. Every stage change and finding was also posted as an attributed Beads comment. No task state, assignee, lane label or existing dependency changed.

## Checkpoint and handoff

Old checkpoint evidence comparison exited 2 after source changed, compatible=false, complete=true. No old evidence was reused. New checkpoint saves succeeded to the separate unapproved `refs/heads/omnigent-checkpoints/bd-e5u3.20.15` ref. Final comparison exited 0, compatible=true, complete=true, approved=false, for the fresh 27-case matrix source/command/environment only. This comparison does not clear the failed full web gate. Final save result is in `checkpoint-final.json`.

Remaining required blocker: **bd-jo57**, the one full-gate unexpected live-refresh page error. Unrelated non-blocking follow-up: **bd-969k**. Reviewer/tester round 2 must repeat on this source. No claim of delivery readiness. No delivery commit/push/PR, no merge, no production writes. Reports and fix are preserved without deleting or overwriting another agent's source work.
