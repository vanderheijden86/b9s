# bd-e5u3.20.15 implementation report

Ready for independent review and testing. Required commands exited 0. No unresolved required-check blockers. No delivery commit, delivery push or PR performed. Only the separate unapproved checkpoint ref was pushed.

## Git preflight

Repository: `/home/omnigent/workspace/b9s`. Actual branch: `omnigent/bd-e5u3.20.15`.
HEAD, fetched `origin/omnigent/bd-e5u3.20.14`, and required commit all equal `816e4988ebbf96d942727c2014a80050664f413d`.
The assigned base is the PR target. Exact delivery `ls-remote --exit-code --heads` returned 2, absent. Explicit delivery fetch reported missing ref, exit 128. Local delivery branch was absent. Created it from the fetched assigned base, never bootstrap main. Both required-commit ancestry checks against fetched base and HEAD exited 0 before edits. Worktree was clean. Checkpoint inspect found no checkpoint.

Preflight report: `/tmp/bd-e5u3.20.15/preflight.txt`. Final branch and HEAD remain unchanged. Final status contains only the two files below.

## Changes and guard

- `web/tests/board-detail.spec.ts`: replace both project-name checks with Playwright `hasTouch` and rendered `#app.phone`. Assert exact 44px touch and 24px desktop short rows, before and after selection. Assert absent phone epic rail, two phone epic lane headers, and visible rich epic content elsewhere. Preserve detail navigation and page-error failure handling.
- `docs/testing.md`: document capability/layout assertions and alias-inclusive matrix coverage.

Read task, parent bd-e5u3.20, dependency bd-e5u3.20.14 and comments, applicable instructions, docs/testing.md, ADR0065/0052 and executable-guards guidance. Audit of all `web/tests` found only the two name assumptions. No remaining `project.name` or phone/tablet `startsWith` checks. No production, configuration, generated, lockfile or Go changes. No skips, retries, assertion loosening or file deletions added.

Regression guard: the existing required Pixel/iPhone CI gate reproduces both original failures. Exact height and explicit phone epic assertions prevent accepting the incorrect desktop expectation. No additional static source-scan guard is needed for a failure observable in the ordinary suite. No ADR change is needed because intended behavior is unchanged.

## RED and GREEN

All browser commands use the installed image and project lockfile. Per-test timeout 30s, assertion/action 5s, navigation 10s, retries 0.

RED, before any implementation edit:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/red.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 360 -- npm run test:e2e -- tests/board-detail.spec.ts --grep 'density, selected rows' --project pixel --project iphone --workers=1 --reporter=list,json
```

Exit 1. **0 passed, 2 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun.** Both Pixel and iPhone failed at the desktop `<=26px` assertion with actual 44px rows. Report `red.json`, log `red.log`, copied traces `red-artifacts/`, all under `/tmp/bd-e5u3.20.15`.

GREEN, all three board-detail tests in both CI aliases that include this spec and all seven size projects:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/green.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 360 -- npm run test:e2e -- tests/board-detail.spec.ts --project pixel --project iphone --project phone-360 --project phone-412 --project tablet-768 --project tablet-1024 --project desktop-1440 --project desktop-1920 --project wide-2560 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15/green-artifacts
```

Exit 0. **27 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun.** Duration 59.591s. Pixel, iPhone, both phones and both tablets measured 44px. Both desktops and wide measured 24px. Phone layout true only for Pixel, iPhone and both phone sizes. Reports `green.json`, `green-evidence.json`, `density-measurements.json`, log `green.log`, screenshots `green-artifacts/`.

## Required checks

All commands ran from `/home/omnigent/workspace/b9s` unless a project/prefix is explicit. Counts below come from completed reports, not console execution numbers.

| Check | Exact command | Result |
| --- | --- | --- |
| Go dependencies | `timeout 180s go mod download` | exit 0 |
| Web dependencies | `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` | exit 0 |
| Browser smoke | `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-e5u3.20.15/browser-smoke.json --seconds 90` | exit 0, 3 passed / 0 failed / 0 skipped |
| Normal CI web gate | `PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15/web-gate.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- make -C /home/omnigent/workspace/b9s web-gate` | exit 0, 430 passed / 0 failed / 4 skipped / 0 flaky / 0 interrupted / 0 unrun, 219.978s |
| Stable suite, alone | `GOFLAGS=-v timeout 660s make -C /home/omnigent/workspace/b9s test-stable` | exit 0, 3345 passed / 0 failed / 20 skipped including subtests, 2057 passed / 0 failed / 18 skipped top-level, 21 packages, none cached |
| Typecheck | `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck` | exit 0 |
| Web build | `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build` | exit 0 |
| Go build | `timeout 180s go build ./...` | exit 0 |
| Lint, Go vet | `timeout 180s go vet ./...` | exit 0 |
| Diff hygiene | `timeout 30s git -C /home/omnigent/workspace/b9s diff --check` | exit 0 |

`make web-gate` expands to `npm --prefix web run test:e2e -- --project tokens --project pixel --project iphone --project desktop --project ipad`, with the declared four workers. `PW_TEST_REPORTER=json` adds the installed Playwright JSON reporter without changing source/config. No separate web lint script is declared. The Node guard precheck completed **16 passed / 0 failed / 0 skipped / 0 cancelled** in each RED, GREEN and gate invocation. Docs tests completed **3 passed / 0 failed / 0 skipped** within stable.

The four unchanged web exclusions are phone wheel zoom (Pixel and iPhone), wide-only layout in desktop, and mobile WebKit wheel scrolling in iPad. `web-gate-skips.json` preserves exact annotations. No global reporter errors. The helper marks any Playwright skip failed, so stream milestone 14 correctly retains state failed despite process exit 0 and zero failed tests. Task policy comments expressly preserve these declared exclusions and prohibit duplicate policy blockers. This is not an unresolved required test failure.

Stable exclusions require optional Dolt, S3, Memory binaries/research configuration, stress opt-in or missing real-project fixtures. `stable-results.json` lists all 20 skip outcomes. The Makefile deliberately excludes E2E and listed host-sensitive tests. No base comparison or simultaneous check ran with stable. No shared Dolt test writes occurred. Browser tests used local isolated JSONL fixtures and fake bd.

## Runtime and milestone evidence

`lane-browser fingerprint --project /home/omnigent/workspace/b9s/web` output: `/tmp/bd-e5u3.20.15/browser-fingerprint.json`. Playwright 1.63.0, Node v22.23.4, Linux x86_64, kernel 6.8.0-147-generic, CPU affinity 0-3, graphics threads 2, native runtime SHA256 `a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6`. Go go1.25.5, npm 10.9.9. Image digest is not exposed by the runtime fingerprint or the checked image-digest environment variables.

All milestone evidence paths use `/tmp/bd-e5u3.20.15/`:

| Stream sequence | Result | Evidence |
| --- | --- | --- |
| 2 | preflight passed | `preflight.txt` |
| 4 | dependencies passed | `npm-dependencies.txt` |
| 6 | browser smoke passed | `browser-smoke.json` |
| 8 | expected RED failed | `red.json` |
| 11 | GREEN passed, 27/0/0 | `green.json` |
| 12 | implementation passed | `implementation.txt` |
| 14 | gate helper failed because 4 declared skips, process exit 0 | `web-gate.json` |
| 16 | stable passed | `stable-results.json` |
| 18,20,22,24,26 | typecheck, web build, Go build, vet, diff passed | `typecheck.json`, `web-build.json`, `go-build.json`, `go-vet.json`, `diff-check.json` |

## Checkpoint, limits and handoff

Separate unapproved checkpoint ref: `refs/heads/omnigent-checkpoints/bd-e5u3.20.15`. Tracked source changes included automatically, no new source files. Final comparison exited 0, compatible=true, complete=true, approved=false, report `checkpoint-compare.json`. No check was repeated or substituted with checkpoint evidence. Independent tester must execute required checks. Final checkpoint save result is stored in `checkpoint-final.json`.

Required blockers: **none**. No new blocker issues filed. Unrelated existing dependency audit work: **bd-969k**, two low and one critical lockfile advisory, unchanged. No production behavior changes or new architecture risks. Browser proof uses emulated devices. Optional Go environments and the four declared device exclusions remain outside this measured coverage. Independent review/testing and Polly's explicit delivery approval remain pending. No delivery commit, push, PR or merge.
