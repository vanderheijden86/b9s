# bd-zsfu implementation report

Status: implementation complete, validation complete. Delivery BLOCKED by required CI failure bd-9d0c. Delivery unapproved.

## Git preflight

- Repository: /home/omnigent/workspace/b9s, https://github.com/vanderheijden86/b9s.git.
- Initial bootstrap: clean main at 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-zsfu`: exit 2, absent.
- No local delivery branch existed. Created `omnigent/bd-zsfu` from fetched `origin/main`.
- Actual branch: omnigent/bd-zsfu. HEAD and fetched base: 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb.
- Required commits: none. Required-commit ancestry against fetched base and HEAD: not applicable.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-zsfu --base origin/main`: exit 0, no checkpoint.
- Preflight completed and reported before source edits. Evidence: /tmp/bd-zsfu/preflight.txt.

## Source intent and changed files

Read baseline AGENTS.md, repository AGENTS.md, docs/testing.md, ADR0028, ADR0059 and executable-guards.md.

Source PR52 https://github.com/vanderheijden86/b9s/pull/52 is open. Its head is b5444541a5c227c832a42ed9b18a5fb7c1b10f01.
Source PR45 https://github.com/vanderheijden86/b9s/pull/45 is merged. Its head was 0fb809a91023ab20305e6e923a0bfb69d956509f.
Its squash 7239ff26466218ed10237b7c16dd9695e083955c is an ancestor of fetched main (`git merge-base --is-ancestor`, exit 0).

`timeout 60s git -C /home/omnigent/workspace/b9s fetch origin refs/pull/52/head:refs/remotes/origin/pr-52 refs/pull/45/head:refs/remotes/origin/pr-45`: exit 0.
Public API PR bodies and metadata: /tmp/bd-zsfu/pr52.json, /tmp/bd-zsfu/pr45.json.
`gh pr view 52` and `gh pr view 45 --repo vanderheijden86/b9s ...` each returned HTTP 401: Bad credentials (https://api.github.com/graphql). Public API reads succeeded. Existing issue bd-tav7 tracks the observed gh limitation. Git fetch and checkpoint pushes succeeded.

The PR45 updater files already matched current main exactly. Ported only the PR52-versus-PR45 delta, without replaying squash-merged ancestor work.
Both resulting source files exactly match PR52 head. Evidence: /tmp/bd-zsfu/pr52-only.patch, /tmp/bd-zsfu/source-parity.diff (empty).

- pkg/updater/updater.go: extracts the existing ancestor scan into isInsideAppBundle. Rejection, symlink resolution and DMG guidance retain their behavior.
- pkg/updater/fileops_test.go: nine pure classifier cases, six portable subprocess cases, and a guard against executable filenames ending in .app/.APP.

No new files. No unrelated changes. No file deletion. No release asset, checksum or provenance changes. ADR0028 remains unchanged.
No product documentation change is needed for this behavior-preserving port.
The guard makes unsafe subprocess fixture names fail on Linux before copying or executing them. It directs filename coverage to the pure classifier test.

## Completed commands

All Go commands ran in /home/omnigent/workspace/b9s. Counts are leaf tests excluding parent aggregates. Overlapping suites must not be added.

| Command | Exit | Passed | Failed | Skipped | Interrupted | Unrun |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| timeout 180s go mod download | 0 | N/A | N/A | N/A | N/A | N/A |
| timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci | 0 | N/A | N/A | N/A | N/A | N/A |
| timeout 180s go test ./pkg/updater -run '^TestPerformUpdateExecutableLocation$' -count=1 -timeout=60s -json | 1 expected RED | 6 | 1 | 0 | 0 | 0 |
| timeout 180s go test ./pkg/updater -count=1 -timeout=60s -json | 0 GREEN | 133 | 0 | 0 | 0 | 0 |
| timeout 900s make -C /home/omnigent/workspace/b9s test-stable GOFLAGS='-v -count=1' | 0 | 3267 | 0 | 20 | 0 | 0 |
| timeout 300s go build ./... | 0 | N/A | N/A | N/A | N/A | N/A |
| timeout 300s go vet ./... | 0 | N/A | N/A | N/A | N/A | N/A |
| timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck | 0 | N/A | N/A | N/A | N/A | N/A |
| lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-zsfu/browser-smoke.json --seconds 90 | 0 | 3 | 0 | 0 | 0 | 0 |
| timeout 600s go test -timeout=5m -covermode=atomic -coverprofile=/tmp/bd-zsfu/coverage.out -count=1 -json ./pkg/... ./cmd/b9s | 1 | 2739 | 1 | 5 | 0 | 0 |

Expected RED fails only executable_suffix at the source guard before execution. GREEN includes all nine pure path cases and six runnable subprocess cases.
Stable ran alone with no baseline worktree, no base comparison and no concurrent checks. Its 20 declared optional skips cover: real bd probe 1, S3 3, Dolt DSN 7, Memory/research 4, optional fixture files 2, opt-in stress 3. 22 packages passed.
CI Go's 5 optional skips cover optional fixture files 2 and opt-in stress 3. 13 packages passed and one failed.
Coverage threshold: pkg/* 78.57%, 13228/16835 statements, above 65%. This threshold result does not certify the failed CI Go suite.

An earlier stable invocation used GOFLAGS='-json -count=1', affecting the Makefile's go list and sending JSON tokens as package names. It exited 2 and is INVALID suite evidence. No project test counts are claimed from it. Corrected invocation above passed. Evidence: /tmp/bd-zsfu/stable-invocation-error.txt.

`gofmt -l /home/omnigent/workspace/b9s/pkg/updater/fileops_test.go /home/omnigent/workspace/b9s/pkg/updater/updater.go`: exit 0, empty output.
`git -C /home/omnigent/workspace/b9s diff --check`: exit 0.
`git -C /home/omnigent/workspace/b9s diff origin/pr-52 -- pkg/updater/updater.go pkg/updater/fileops_test.go`: exit 0, empty output.

## Runtime and bounded checks

Go go1.25.5 linux/amd64. Node v22.23.4, npm 10.9.9. Playwright 1.63.0, CPU affinity 0-3, graphics threads 2, kernel 6.8.0-147-generic.
Browser native runtime SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. No image digest was provided, so none is claimed.
`lane-browser fingerprint --project /home/omnigent/workspace/b9s/web`: exit 0, /tmp/bd-zsfu/browser-environment.json.
Subprocess test Go timeout 8s and context deadline 10s are unchanged. Focused package timeout 60s, stable package timeout 10m, coverage package timeout 5m. Outer deadlines are recorded above.

## Required blocker observed

- bd-9d0c: TestBDContract_EveryWriteB9sMakes failed at pkg/web/bd_contract_test.go:37, `bd init: exit status 3`.
  Exact guard: `bd: a lane role does not write lane state (bd init). The dispatcher owns status, assignee, claims, closing, dispatch-* labels and dependencies; report it to Polly instead.`
  This is outside the updater port. No guard bypass, test suppression or source workaround was attempted.

Existing issue bd-tav7 covers observed gh API HTTP401. PR creation is not authorized and was not attempted. Git delivery push is not authorized and was not attempted.
No new issues were filed. Task state, assignee and dependencies were not changed.

## Evidence

- /tmp/bd-zsfu/preflight.txt
- /tmp/bd-zsfu/dependencies.txt and npm-ci.log
- /tmp/bd-zsfu/implementation.txt
- /tmp/bd-zsfu/red.jsonl and red-summary.json
- /tmp/bd-zsfu/green.jsonl and green-summary.json
- /tmp/bd-zsfu/stable-corrected.log and stable-corrected-summary.json
- /tmp/bd-zsfu/build-summary.txt and vet-summary.txt
- /tmp/bd-zsfu/typecheck-summary.txt
- /tmp/bd-zsfu/browser-smoke.json and browser-environment.json
- /tmp/bd-zsfu/ci-go.jsonl, ci-go-summary.json and coverage-summary.txt
- /tmp/bd-zsfu/self-review.txt (implementer only, independent review pending)

Only unapproved checkpoint refs were pushed. Delivery HEAD is unchanged. No delivery commits, delivery push, PR, GitHub comment, review or merge.

## Completed CI browser gate

Exact command (working directory /home/omnigent/workspace/b9s):

```sh
PLAYWRIGHT_JSON_OUTPUT_NAME=/tmp/bd-zsfu/web-gate.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm run test:e2e -- --project tokens --project pixel --project iphone --project desktop --project ipad --workers=1 --reporter=list,json
```

Exit 0. Fresh JSON reporter: **431 passed, 0 failed, 4 skipped, 0 flaky, 0 interrupted, 0 unrun**. Reporter errors: empty. Duration: 374016.703 ms.
The npm command first ran `node --test --test-timeout=20000 tests/poll-guard.mjs`: **16 passed, 0 failed, 0 skipped, 0 cancelled**.
Playwright retained declared 30s test timeout, 5s action and assertion deadlines, and 10s navigation timeout. Runtime fingerprint is preserved separately.

Completed result published with:

```sh
lane-progress publish --repo /home/omnigent/workspace/b9s --task bd-zsfu --role implementer --stage tests --playwright-report /tmp/bd-zsfu/web-gate.json --exit-code 0
```

The helper published advisory **blocked** due to skips. All four match pre-existing source conditions:

- pixel/memory.spec.ts:269, wheel zoom: `phones have no wheel`.
- iphone/memory.spec.ts:269, wheel zoom: `phones have no wheel`.
- desktop/desktop.spec.ts:198, wide monitor layout: `wide only`.
- ipad/wide.spec.ts:458, wheel scrolling: `Playwright has no wheel in mobile WebKit, and an iPad scrolls the board by touch`.

These are declared device/viewport exclusions, not new required-check failures. Independent tester/Polly must judge the helper's required-check advisory.
No flaky outcomes, global reporter errors or browser page error failures were observed. No retry or test suppression was added.
Raw reporter: /tmp/bd-zsfu/web-gate.json. Summarized annotations: /tmp/bd-zsfu/web-gate-summary.json. Console/guard result: /tmp/bd-zsfu/web-gate.log.

Source SHA256:

- pkg/updater/fileops_test.go: a17dc0489e76d363017b5b5ecf0138f906264f204de70c95f43dfefe16ff8fee
- pkg/updater/updater.go: d98c95f6cec68f5d100dcf03e4913a6e217c0c234af264b119a90a6ba5d560c7

## Completed terminal E2E and final assessment

Exact command:

```sh
timeout 1260s go test -timeout=20m -v ./tests/e2e -count=1 -json
```

Exit 0. **114 passed, 0 failed, 8 declared optional skipped, 0 interrupted, 0 unrun**. One passing package, 585.554 seconds.
The eight skip reasons were read from the completed JSON report:

- Scratch Dolt server unset, B9S_TEST_DOLT_SCRATCH_ADDR: TestAssigneePickerMergesAliasesE2E, TestReloadKeepsTreeFoldsUnderStatusFilterE2E, TestReloadKeepsDetailOnSameIssueE2E.
- Memory preview binary unset, B9S_TEST_MEMORY_BD: TestMemoryPreviewClientReadsRealWorkspace, TestMemoryPreviewRefusesOrdinaryWorkspace, TestMemoryPreviewPrintsOneMemory, TestMemoryPreviewBrowserE2E.
- Live catalog configuration unset: TestProjectCommandListsServerDatabasesE2E.

All eight match existing explicit environment conditions. No timeout was skipped, and no selected test failed. No skip or workaround was introduced by this task.
Evidence: /tmp/bd-zsfu/terminal-e2e.jsonl, /tmp/bd-zsfu/terminal-e2e-summary.json.

Final validation outcome: **BLOCKED for delivery**. The implementation is complete and source-identical to PR52's intended updater files. Stable, focused updater, build, vet, web typecheck, browser smoke, coverage threshold and terminal E2E completed as recorded. Browser reporter exit 0 with four existing device/viewport skips triggers the helper's advisory required-check review.

Every unresolved required-check failure:

1. **bd-9d0c**, CI Go gate: TestBDContract_EveryWriteB9sMakes failed because the lane bd guard rejects `bd init` with exit 3. Full CI Go leaf counts: 2739 passed, 1 failed, 5 optional skipped, 0 interrupted, 0 unrun. The required suite remains failed, despite coverage reaching 78.57%.

Separate delivery limitation, not an additional failed test:

- **bd-tav7**, gh API authentication: both source PR reads returned `HTTP 401: Bad credentials (https://api.github.com/graphql)`. Public API reads, Git fetch and unapproved checkpoint push succeeded. Future PR creation through gh requires working authentication. No delivery operation was attempted.

Unrelated follow-up work: none filed or implemented. Existing MIME issues bd-shnu/bd-jcbi and attachment issue bd-jt4s were read only. They did not fail this run and are not blockers reported for this task.

Residual risks and limits:

- Native macOS execution, signing and DMG installation were not tested in this Linux lane. Source parity preserves the previously reviewed PR52 fixture design without claiming its historical test results as this run's evidence.
- Full successful release download/replacement was not performed. Asset, checksum and provenance code is unchanged.
- Independent review and independent testing remain pending with Polly. Implementer self-check does not replace either role.
- No preview was deployed, so this attempt has no preview URL.
- No delivery commit or delivery push is authorized. No PR was opened. Replacement PR must target main and name/supersede #52 after approval.

Milestones were published through lane-progress. Their evidence files are listed above. Beads comments on bd-zsfu, bd-9d0c and bd-tav7 record attributed results.

Final Git check after all suites: actual branch omnigent/bd-zsfu. HEAD and fetched origin/main both 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb. Only the two intended updater files are modified, 38 insertions and 3 deletions. Final diff against source PR52 for both files is empty.
Checkpoint delivery is restricted to refs/heads/omnigent-checkpoints/bd-zsfu. The source tree in saved checkpoints is 8c7be90230e31b17674ca47d286f76a1cc9b2a53. Delivery branch HEAD remains unchanged.
