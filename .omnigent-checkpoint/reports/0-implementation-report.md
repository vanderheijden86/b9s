# bd-q4ge implementation report

## Scope and delivery status

PR #46's compact web board work is included on current assigned main, with a fix for its CI web-e2e failures. Delivery branch: `omnigent/bd-q4ge`. Assigned base and future PR target: `main`. The future PR must state that it supersedes #46. No delivery commit, push, PR, GitHub comment or review has been made. Polly must approve delivery first. No preview deployment has been requested or performed.

## Git preflight (reported before editing)

- Repository: `/home/omnigent/workspace/b9s`.
- Initial branch: main, clean. No unrelated uncommitted changes.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- Fetched `origin/main`: `3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`.
- Exact delivery probe: `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin omnigent/bd-q4ge`: exit 2, absent.
- No local delivery branch existed. Created it from origin/main. Actual branch `omnigent/bd-q4ge`, HEAD `3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`.
- Required commits: none. Required-commit ancestry checks against fetched base and HEAD: not applicable.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-q4ge --base origin/main`: no checkpoint exists.
- Fetched `omnigent/bd-e5u3.20.14`, PR #46 head `816e4988ebbf96d942727c2014a80050664f413d`. Its one-commit diff applied cleanly without creating a commit.
- Evidence: `/tmp/bd-q4ge/preflight.txt`.
- `beads-connect`: exit 0, connected to b9s as bd_b9s. Polly can repeat its exact lane_issues call. Attributed progress comments recorded on bd-q4ge.

## Cause and fix

Actions run 41724043346 reported 428 passed, 2 failed and 4 skipped. Both failures were the density test in `board-detail.spec.ts`, on pixel and iphone. The test guessed the app size class from project-name prefixes phone/tablet. These CI device aliases matched neither prefix, so the test applied a desktop maximum of 26px to correct 44px phone rows. Its later epic-summary assertion had the same project-name error and would request desktop epic markup on a phone.

The test uses the existing `sizeClassOf(page.viewportSize()!.width)` contract and checks that #app has the expected class. Touch-target and epic assertions branch on that class. The 44px touch minimum and 26px desktop maximum remain enforced. docs/testing.md explains why device project names cannot determine size class. No additional production behavior change was necessary beyond PR #46.

Test-first proof: focused original test reproduced both failures locally before the fix. `/tmp/bd-q4ge/red-browser.json`, process exit 1: 0 passed, 2 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun. Both failed at actual 44 versus expected <=26. After the fix, the same test on both gate aliases and seven size projects passed: `/tmp/bd-q4ge/green-browser.json`, exit 0, 9 passed, all other counts 0. Node guard tests in each invocation: 16 passed, 0 failed/skipped/cancelled/todo.

Guard decision: the existing browser regression observes this error when run on the actual gate projects. Reusing the app's size-class function avoids another independently maintained device-name classification. No new source-scan guard is warranted for this single defect.

## Changed files

PR #46 work retained:

- `docs/ARCHITECTURE.md`: title-focused board contract.
- `docs/USER_GUIDE.md`: row and detail behavior.
- `docs/adr/0022-make-the-phone-epic-rail-an-index.md`: superseded status and link only.
- `docs/adr/0065-keep-web-board-issue-rows-title-focused.md`: original PR decision, new source file.
- `web/src/app.css`: borderless rows and touch targets.
- `web/src/render.ts`: title-focused card markup.
- `pkg/web/dist/app.css`, `pkg/web/dist/app.js`, `pkg/web/dist/source.sha256`: embedded bundle.
- `web/tests/board.spec.ts`: details retain nested parent context.
- `web/tests/tokens.spec.ts`: metadata remains in details.
- `web/tests/wide.spec.ts`: ordinary row and selection assertions.
- `web/tests/board-detail.spec.ts`: original PR density coverage, plus this task's width-derived expectations.

Additional file for this task:

- `docs/testing.md`: responsive test classification rule.

## Commands and evidence

All commands use local fixture projects. Browser tests use JSONL and fake bd, never the shared database. B9S_KEEP_TEST_FIXTURES=1 retains browser fixtures. Build commands run from the shared repository. Each suite has the declared per-test deadline and an outer deadline.

Dependency installation (both exit 0):

```sh
timeout 180s go mod download
timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci
```

Browser environment (both exit 0):

```sh
lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-q4ge/browser-smoke.json --seconds 90
lane-browser fingerprint --project /home/omnigent/workspace/b9s/web
```

Smoke: 3 passed, 0 failed, 0 skipped. Fingerprint: Playwright 1.63.0, Node v22.23.4, x86_64, kernel 6.8.0-147-generic, CPU affinity 0-3, graphics threads 2, Pod browser boundary. Native runtime SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. Image digest is not supplied by the fingerprint. Full fingerprint: `/tmp/bd-q4ge/browser-environment.json`.

Red command (exit 1, expected regression proof):

```sh
env PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-q4ge/red-browser.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 180 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/board-detail.spec.ts -g 'density, selected rows' --project pixel --project iphone --workers=1 --reporter=list,json --output=/tmp/bd-q4ge/red-pw
```

Focused green command (exit 0):

```sh
env PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-q4ge/green-browser.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 180 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/board-detail.spec.ts -g 'density, selected rows' --project pixel --project iphone --project phone-360 --project phone-412 --project tablet-768 --project tablet-1024 --project desktop-1440 --project desktop-1920 --project wide-2560 --workers=1 --reporter=list,json --output=/tmp/bd-q4ge/green-pw
```

Full CI web gate command:

```sh
env CI=1 PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-q4ge/web-gate.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- make -C /home/omnigent/workspace/b9s web-gate
```

This executes the exact workflow target with its tokens, pixel, iphone, desktop and ipad projects and the declared four workers. PW_TEST_REPORTER=json adds the installed Playwright JSON reporter without changing the target or repository configuration.

## UI observations

Focused test screenshots were inspected. Pixel rows measure 44px, desktop-1440 rows measure 24px. Borderless rows and full parent/details context are visible. The tested board specs fail on unexpected browser page errors. Screenshots: `/tmp/bd-q4ge/pixel-compact-board.png`, `/tmp/bd-q4ge/desktop-1440-selected-board.png`. Each browser test starts the built application and checks it through real browser interactions.

## Completed gate results

The exact `make web-gate` command exited 0. Fresh JSON `/tmp/bd-q4ge/web-gate.json`: 433 passed, 0 failed, 4 skipped, 0 flaky, 0 interrupted, 0 unrun. Reporter errors: none. Node guard precheck: 16 passed, 0 failed, 0 skipped, 0 cancelled, 0 todo. The lane-progress helper emitted its documented advisory blocked milestone because it flags any skips for required-check review. These four skips are the existing declared project exclusions:

- pixel / memory.spec.ts:269: wheel zoom, phones have no wheel.
- iphone / memory.spec.ts:269: wheel zoom, phones have no wheel.
- desktop / desktop.spec.ts:198: extra-width layout, wide class only.
- ipad / wide.spec.ts:462: wheel dispatch, Playwright has no wheel in mobile WebKit and iPad uses touch.

No unexpected skipped required test was observed. Polly and the independent tester must review that classification. Counts from overlapping commands are not added together.

All four commands below exited 0. They are compilation/static checks, so test pass/fail/skip counts do not apply:

```sh
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build
timeout 240s go build ./...
timeout 240s go vet ./...
```

Reports: `/tmp/bd-q4ge/typecheck.txt`, `/tmp/bd-q4ge/web-build.txt`, `/tmp/bd-q4ge/go-build.txt`, `/tmp/bd-q4ge/go-vet.txt`. The project has no npm lint script. Go vet is its declared static check. No Go source changed, so gofmt is not applicable.

## Residual scope and existing issues

`npm ci` reported 2 low and 1 critical audit findings from the unchanged lockfile. Existing triage task bd-969k and runtime follow-up bd-w8yz already cover these dependency risks. The related lockfile work was delivered separately in PR #56, targeting main, and is not part of this assigned base. No dependency upgrade or new issue was made here. These are unrelated follow-up work, not failures of this task's required build/test checks.

There is no dedicated long-title browser fixture in the inherited PR. Selected title wrapping remains covered by the inherited CSS and visual review rather than a dedicated new assertion. This task corrects the failing responsive regression, without expanding the presentation feature.

Independent review/testing and hosted GitHub checks have not yet run for a delivery commit. There is no delivery commit or PR until Polly approves. The final PR body must name #46, explain that it supersedes it, and include measured verification, review verdict, residual risks and the preview URL if one is produced. No new issue IDs were created.

## Stable suite and final review

```sh
timeout 660s env GOFLAGS=-v make -C /home/omnigent/workspace/b9s test-stable
```

The command ran alone, without a base comparison, and exited 0. Including subtests: **3447 passed, 0 failed, 20 skipped**. Top-level only: **2085 passed, 0 failed, 18 skipped**. Passing packages: 22. Go does not report Playwright-style flaky/interrupted/unrun counters. Those counters remain unknown for this Go run and are not represented as zero. Skips require optional real bd, S3, Dolt, Memory preview/research or stress environments, plus two absent real-project fixtures. Exact skip names are in `/tmp/bd-q4ge/test-stable-summary.json`. The raw verbose output is `/tmp/bd-q4ge/test-stable.txt`. No shared database was used.

`git -C /home/omnigent/workspace/b9s diff --check` exited 0. `/tmp/bd-q4ge/diff-check.txt` records that result. The original PR's CSS, render source, board/token/wide tests, embedded bundle and ADR 0065 match the original PR exactly. Existing newer main documentation is retained. The only additions beyond applying PR #46 to the assigned base are the focused test correction and the one-line testing guide rule.

Final branch and HEAD remain `omnigent/bd-q4ge`, `3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`. Fourteen source/generated/documentation files are changed, including the explicitly checkpointed new ADR. No unrelated modifications observed. `/tmp/bd-q4ge/final-git.txt` and `/tmp/bd-q4ge/final-diff-stat.txt` record the tree. No delivery commit was created. Checkpoints use only `refs/heads/omnigent-checkpoints/bd-q4ge`.

Required local check failures unresolved: **none observed**. The pre-fix red failures were resolved and are retained as regression evidence. The four web gate skips and twenty stable skips are documented optional exclusions, subject to independent review. No new issue IDs. Existing unrelated audit work: bd-969k and bd-w8yz.

Implementation is ready for Polly's independent read-only review and tester. Hosted CI status cannot be established before an authorized delivery commit and PR. The final PR must supersede #46 and target main. Nothing was merged, and PR #46 was not changed or closed.
