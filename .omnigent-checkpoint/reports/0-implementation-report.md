# bd-teri.7 implementation report

Status: GREEN, ready for independent review and tests. Delivery is not authorized.
Source: GitHub issue #22. A future PR body must contain `Refs #22`.

## Git preflight

- Shared repository: `/home/omnigent/workspace/b9s`.
- Assigned base and PR target: `main`.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- Fetched `origin/main`: `c2fccef8b537ed034ebd360ea7cf31c02250e083`.
- Exact remote probe: `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-teri.7`: exit 2, absent.
- Local delivery branch was absent. Created `omnigent/bd-teri.7` from `origin/main`.
- Actual branch: `omnigent/bd-teri.7`.
- HEAD: `c2fccef8b537ed034ebd360ea7cf31c02250e083`, unchanged after implementation.
- Required commits: none. Ancestry checks against base and HEAD: not applicable.
- Initial worktree: clean, no foreign changes. Final changes are the eight files below.
- Checkpoint inspection explicitly reported no checkpoint, exit 2.
- No delivery commit, delivery push, PR, GitHub comment or review was made.

## Result and changed files

Current main did not implement mouse persistence. Read issue title/body/comments (no comments) and inspected its screenshot. The screenshot concerns a separate contrast request and does not change this scope.

`ui.mouse` is a boolean in `config.yaml`, enabled by default. Startup config controls Bubble Tea mouse capture. `:mouse` saves both toggle directions through the same YAML-node patch and atomic write path as column choices. Existing comments, unknown settings and unrelated UI preferences remain intact. Save failures appear in the status bar while the runtime toggle still works.

- `cmd/b9s/main.go`: choose the startup mouse option from the model.
- `pkg/config/config.go`: mouse setting/default and shared targeted YAML save helper.
- `pkg/config/config_test.go`: default, explicit false round-trip, alternating column/mouse saves and malformed-parent guards.
- `pkg/ui/model.go`: load the mouse preference and share the preference config path.
- `pkg/ui/command_prompt.go`: persist mouse toggles and report write errors.
- `pkg/ui/command_prompt_test.go`: startup, restart, file creation, preservation and error regressions.
- `docs/USER_GUIDE.md`: document `ui.mouse`, default and persistence.
- `tests/e2e/mouse_e2e_test.go` (new): real terminal startup and restart proof across four process runs.

The regressions guard this defect class. The existing column persistence pattern applies, so no new architectural decision was needed. ADR 0003's default mouse capture remains enabled. No Beads data access or schema changes were made.

## Exact checks and observed counts

Counts are leaf Go test cases, excluding parent containers when they have subtests. No flaky or interrupted outcome occurred.

| Command (repository working directory) | Limits | Exit | Passed | Failed | Skipped |
| --- | --- | --- | --- | --- | --- |
| `timeout 180s go mod download` | 180s outer | 0 | n/a | n/a | n/a |
| `timeout 180s go test ./pkg/ui -run '^TestMouse(ConfigAtStartup\|CommandPersistsAcrossRestart\|CommandCreatesConfig\|CommandReportsSaveFailure)$' -count=1 -timeout=60s -json` | 180s outer, 60s package | 1, expected RED | 2 | 4 | 0 |
| `timeout 180s go test ./pkg/ui ./pkg/config -run '^(TestMouse\|TestSaveMouse\|TestDefaultConfig$\|TestSaveAndLoad_RoundTrip$)' -count=1 -timeout=60s -json` | 180s outer, 60s package | 0 | 22 | 0 | 0 |
| `GOFLAGS=-v timeout 660s make test-stable` | 660s outer, 10m package | 0 | 3563 | 0 | 20 |
| `timeout 180s go build ./...` | 180s outer | 0 | n/a | n/a | n/a |
| `timeout 180s go vet ./...` | 180s outer | 0 | n/a | n/a | n/a |
| `timeout 180s go test ./tests/e2e -run '^TestMouseCapturePersistsE2E$' -count=1 -timeout=90s -json` | 180s outer, 90s package, 15s per PTY subprocess, 1800ms autoclose | 0 | 1 | 0 | 0 |

The Markdown table escapes the regex alternation characters. The report JSON command strings retain their literal shell spelling.

The RED run preceded product edits. It failed because explicit false was ignored, restart lost the choice, no config was created, and save errors were absent. All four regressions passed in GREEN.

Stable suite top-level outcomes: 2130 passed, 0 failed, 18 skipped, across 22 packages. Its 20 leaf skips are declared optional: one real-bd capability probe, three S3 cases, seven Dolt cases, four Memory investigation cases, two absent legacy real-data fixtures and three opt-in stress tests. No required regression was skipped. `DoltIntegration` and the Makefile's host-sensitive exclusions were left unchanged.

The PTY test observed default capture, `:mouse` release, disabled capture after restart, `:mouse` capture, and enabled capture after restart. Saved config retained comments, column choices and an unknown custom key after startup and toggles. Fixtures used isolated JSONL and XDG config, with no shared Dolt test writes.

Build supplies Go typecheck. Vet supplies the Go lint gate. Both produced no diagnostics. `gofmt -l` on all seven changed Go files produced no output. `git diff --check` exited 0. The stable suite ran alone, without a base comparison. No browser test applies to this terminal-only change.

## Reports and milestones

All paths are absolute and shared by lane roles in this Pod:

- Preflight: `/tmp/bd-teri.7/preflight.txt`.
- Dependencies: `/tmp/bd-teri.7/dependencies.txt`.
- RED: `/tmp/bd-teri.7/red.txt`, `/tmp/bd-teri.7/red-summary.json`, `/tmp/bd-teri.7/red.jsonl`, `/tmp/bd-teri.7/red.exit`.
- GREEN: `/tmp/bd-teri.7/green-summary.json`, `/tmp/bd-teri.7/green.jsonl`, `/tmp/bd-teri.7/green.exit`.
- Stable: `/tmp/bd-teri.7/stable-summary.json`, `/tmp/bd-teri.7/stable.log`, `/tmp/bd-teri.7/stable.exit`.
- Build: `/tmp/bd-teri.7/build-summary.txt`, `/tmp/bd-teri.7/build.log`, `/tmp/bd-teri.7/build.exit`.
- Vet: `/tmp/bd-teri.7/vet-summary.txt`, `/tmp/bd-teri.7/vet.log`, `/tmp/bd-teri.7/vet.exit`.
- Runtime: `/tmp/bd-teri.7/runtime-summary.json`, `/tmp/bd-teri.7/runtime.jsonl`, `/tmp/bd-teri.7/runtime.exit`.
- Diff: `/tmp/bd-teri.7/diff-check.txt`.
- Final implementation milestone: `/tmp/bd-teri.7/implementation-report.md`.

Lane milestones include running states before checks and completed results after process exit. Expected RED is the only failed milestone and is resolved by GREEN. Checkpoints use only `refs/heads/omnigent-checkpoints/bd-teri.7`, never the delivery ref.

## Remaining risks and handoff

No unresolved required-check failures or scope discoveries. Independent reviewer and tester gates remain with Polly. Optional integrations and stress cases listed above were not exercised. The targeted terminal test is the runtime proof, rather than the full slow PTY suite. Delivery commits, delivery pushes and PR creation remain unauthorized. Preview URL: not applicable to the terminal change.
