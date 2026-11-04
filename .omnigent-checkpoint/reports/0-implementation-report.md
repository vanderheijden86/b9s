# bd-e5u3.20.15 implementation report

Reported 2026-11-04 21:51 CET. Role: implementer, Codex session 155746c7862341e3ae0dfd4a06b9b4aa.

## Result

Scoped test fix passes all fresh required checks. Independent review and testing remain pending. No delivery commit, delivery push or PR was authorized or performed. A separate unapproved lane checkpoint preserves the work.

Required-check failures remaining: **none**. Required prerequisite integration gap: **bd-jo57**, closed and delivered in PR54 on `omnigent/bd-jo57`, whose fix is absent from both assigned base and HEAD. This was reported before restoring changes. Polly must resolve that prerequisite context before delivery. No main substitution, dependency recreation, merge or cherry-pick occurred. The historical teardown failure did not reproduce in this fresh gate.

Unrelated follow-up: **bd-969k**, existing npm dependency audit work, outside scope. No new issue was filed and no task state, assignee, dependency or lane label was changed.

## Git preflight

- Initial clean bootstrap: branch main, HEAD 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb.
- Actual branch: `omnigent/bd-e5u3.20.15`.
- HEAD and fetched assigned base `origin/omnigent/bd-e5u3.20.14`: `816e4988ebbf96d942727c2014a80050664f413d`.
- Required ancestor `816e4988ebbf96d942727c2014a80050664f413d`: `git merge-base --is-ancestor` against fetched base **exit 0**, against HEAD **exit 0**.
- Exact delivery remote probe **exit 2**, absent. Local delivery branch absent. Created delivery from assigned fetched remote base. Bootstrap branch preserved.
- PR target remains `omnigent/bd-e5u3.20.14`.
- Commands: `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin omnigent/bd-e5u3.20.14:refs/remotes/origin/omnigent/bd-e5u3.20.14`; `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-e5u3.20.15`; `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-e5u3.20.15 origin/omnigent/bd-e5u3.20.14`.
- `beads-connect` ran once, exit 0. `bd show`, `bd dep list` and comments succeeded, including closed bd-jo57 status discovery. Direct Polly session notification was refused because session orchestration is disabled. Attributed Beads comments and progress milestones record readiness.
- Full evidence: `/tmp/bd-e5u3.20.15-fresh/preflight.txt`, `/tmp/bd-e5u3.20.15-fresh/dependency-final.txt`, `/tmp/bd-e5u3.20.15-fresh/final-tree.txt`.

## Recovery and changed files

Read baseline/project instructions, docs/testing.md and ADR0065, ADR0052, ADR0059. The linked shared executable-guards reference is absent from this Pod. No new guard mechanism was designed. Existing alias-inclusive CI tests reproduce this class and the fixed assertions pin exact heights.

Matching checkpoint `936733b29725d0e9fa08a65fbf9887e090779b62` was inspected and its source diff read before reuse. No context mismatch existed. Restored only after fresh RED, with a clean worktree. Previous successful test evidence was never reused. Fresh checkpoint comparison after GREEN reports compatible=true, complete=true, advisory only.

- `web/tests/board-detail.spec.ts`: use Playwright `hasTouch`, exact 44px touch / 24px desktop CSS height before and after selection, rendered phone layout for epic assertions. Phone headers remain visible without an epic rail. Non-phone rail mode is an explicit saved preference and asserted before rich epic facts. All detail/navigation/context assertions remain. Unexpected page errors still fail the spec.
- `docs/testing.md`: document capability/layout assertions, explicit rail preference and alias-inclusive matrix.
- Full `web/tests` audit found exactly two original `project.name` checks, both in this test. None remain (`rg` exit 1).
- No product, generated, lockfile or Go source changed. No new source files, deletions, added skips, relaxed assertions, retries or timeout increases. No new ADR needed for a test fixture correction.

## Fresh RED and GREEN

| Run | Exit | Passed | Failed | Skipped | Flaky | Interrupted | Unrun |
| --- | --- | --- | --- | --- | --- | --- | --- |
| red | 1 | 0 | 2 | 0 | 0 | 0 | 0 |
| green | 0 | 27 | 0 | 0 | 0 | 0 | 0 |
| tablet-regression | 0 | 12 | 0 | 0 | 0 | 0 | 0 |
| web-gate | 0 | 430 | 0 | 4 | 0 | 0 | 0 |

RED reproduced the original failure in both Pixel and iPhone: expected <=26px, actual44px at board-detail.spec.ts:122. The matching saved fix was restored afterward. GREEN executes all three board-detail tests on Pixel, iPhone and all seven size projects. The separate12-repetition tablet run checks the previously intermittent rail precondition with original deadlines. Raw reports and traces remain on disk.

Measured density: six touch projects44px, desktop-1440/desktop-1920/wide-2560 each24px. Selected rows satisfy the same exact assertions. Phone epic lane headers, non-phone rich summaries and details/navigation passed. Inspected generated Pixel and desktop screenshots. Observations `/tmp/bd-e5u3.20.15-fresh/density-observations.json`.

Exact commands, all from repository working directory:

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15-fresh/red.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 360 -- npm run test:e2e -- tests/board-detail.spec.ts --grep 'density, selected rows' --project pixel --project iphone --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15-fresh/red-artifacts
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15-fresh/green.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 360 -- npm run test:e2e -- tests/board-detail.spec.ts --project pixel --project iphone --project phone-360 --project phone-412 --project tablet-768 --project tablet-1024 --project desktop-1440 --project desktop-1920 --project wide-2560 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15-fresh/green-artifacts
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15-fresh/tablet-regression.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm run test:e2e -- tests/board-detail.spec.ts --grep 'density, selected rows' --project tablet-768 --repeat-each 12 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.15-fresh/tablet-regression-artifacts
PW_TEST_REPORTER=json PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.15-fresh/web-gate.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- make -C /home/omnigent/workspace/b9s web-gate
```

Every browser run has declared30s tests,5s assertions,5s action and10s navigation caps. JSON reports were fresh, commands exited before counts were published. Four gate exclusions are existing declarations:

- pixel: wheel zooms toward the pointer. phones have no wheel
- iphone: wheel zooms toward the pointer. phones have no wheel
- desktop: uses the extra width while prose keeps a readable measure. wide only
- ipad: the wheel over a cell without the cursor scrolls the board, and over the cursor's cell scrolls that cell. Playwright has no wheel in mobile WebKit, and an iPad scrolls the board by touch

The progress helper derives an advisory **blocked** milestone for the4skips despite command exit0. It is preserved. Required device coverage passed under the task's existing exclusion policy. Global JSON errors:0. No unexpected page errors reported. No browser failures were retried.

## Other required checks

`GOFLAGS='-v -count=1' timeout 660s make -C /home/omnigent/workspace/b9s test-stable` ran **alone**, with no base comparison, after the web gate exited. Exit0,21packages uncached. Including subtests: **3345 passed,0 failed,20 skipped**. Top-level: **2057 passed,0 failed,18 skipped**. No unfinished observed RUN entries. Go flaky and full unrun inventories are unknown. Optional Dolt/S3/Memory/stress/real-project-fixture skips listed in `/tmp/bd-e5u3.20.15-fresh/stable-results.json`. Existing Makefile host/E2E exclusions remain unchanged. Docs guards within this run:3passed/0failed/0skipped.

- `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web run typecheck`: exit 0, case counts not applicable. Report `/tmp/bd-e5u3.20.15-fresh/web-typecheck.json`.
- `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web run build`: exit 0, case counts not applicable. Report `/tmp/bd-e5u3.20.15-fresh/web-build.json`.
- `timeout 240s go build ./...`: exit 0, case counts not applicable. Report `/tmp/bd-e5u3.20.15-fresh/go-build.json`.
- `timeout 240s go vet ./...`: exit 0, case counts not applicable. Report `/tmp/bd-e5u3.20.15-fresh/go-vet.json`.
- `timeout 30s git -C /home/omnigent/workspace/b9s diff --check`: exit 0, case counts not applicable. Report `/tmp/bd-e5u3.20.15-fresh/diff-check.json`.

Node guard script ran before every browser suite: each invocation16passed/0failed/0skipped, with20s Node test cap. Go vet is the declared Go lint check. Web has no separate lint script.

Dependencies were installed before checks: `timeout 180s go mod download` and `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci`, both exit0. npm installed122packages from committed lockfile, reported3vulnerabilities(2low,1critical), covered by unrelated bd-969k. No audit fix attempted.

Browser smoke before browser tests: `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-e5u3.20.15-fresh/browser-smoke.json --seconds 90`, exit0,3passed/0failed/0skipped. `lane-browser fingerprint --project /home/omnigent/workspace/b9s/web` preserved in `/tmp/bd-e5u3.20.15-fresh/browser-fingerprint.json`: Playwright1.63.0, Nodev22.23.4, x86_64, kernel6.8.0-147-generic, affinity[0,1,2,3], two graphics threads, native SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. OCI image digest unavailable in exposed runtime metadata. No browser/system package download or dependency-check bypass.

## Handoff and residuals

Independent reviewer/tester must assess this exact restored tree and fresh evidence. Latest rail fix still awaits their review. Device checks use browser emulation, optional environments were not supplied, and nightly/performance/full terminal E2E suites were outside this bounded dispatch. Required prerequisite integration gap **bd-jo57** remains for Polly. Unrelated **bd-969k** remains outside scope. Current required-check failures: **none**.

Reports `/tmp/bd-e5u3.20.15-fresh/checks-summary.json` and `/tmp/bd-e5u3.20.15-fresh/browser-results.json` retain measured counts, test outcomes, raw-report hashes and exclusions without screenshot payloads. Raw browser reports, artifact directories and logs remain under `/tmp/bd-e5u3.20.15-fresh`. Checkpoint evidence names only the fresh27-test matrix and does not substitute for independent testing. Final checkpoint save result will be `/tmp/bd-e5u3.20.15-fresh/checkpoint-final.json`. No delivery commit, delivery push, PR, merge or production data write.
