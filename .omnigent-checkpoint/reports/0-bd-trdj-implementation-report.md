# bd-trdj implementation report

Implementation is complete for independent review and testing. No delivery commit or delivery push was made. No task status, dependency or label was changed.

## Git preflight

- Repository: `/home/omnigent/workspace/b9s`
- Assigned base and PR target: `main`
- Actual branch: `omnigent/bd-trdj`
- HEAD: `eb77cef9f8e68e9854f3c1b6ec6a6074b812830a`
- Fetched `origin/main`: `eb77cef9f8e68e9854f3c1b6ec6a6074b812830a`
- Required commits: none. Required-commit ancestry against both base and HEAD is not applicable.
- Initial worktree clean. Exact remote delivery probe exited 2 (absent), local delivery branch absent. Created the assigned branch from fetched `origin/main`.
- Initial checkpoint absent. Current source was saved to the separate unapproved checkpoint ref `refs/heads/omnigent-checkpoints/bd-trdj`.
- Full preflight commands and observed results: `/tmp/bd-trdj-preflight.txt`.

## Changes

- `pkg/config/config.go`: remove `pagerank` from accepted `ui.sort.field` names.
- `pkg/config/config_test.go`: regression for rejection with the complete error listing supported alternatives, plus config loading coverage for the eight supported fields.
- `pkg/ui/model.go`: remove the inactive PageRank enum entry and label.
- `pkg/ui/tree.go`: remove the unused score map, setter, getter and comparator branch.
- `pkg/ui/tree_test.go`: update field count, labels, direction and popup expectations. Reject PageRank in the popup. Check each supported field independently in both sort directions.
- `docs/USER_GUIDE.md`: align the field list and YAML example, and document rejection and replacement guidance.

The current base includes Deferred in addition to the seven fields named in dispatch. All eight working fields are preserved: priority, created, updated, title, status, type, deps and deferred. ADR 0010 remains immutable. Its configuration ownership and invalid-file behavior remain in effect. This is a routine removal of an inactive option, with no new architectural abstraction or ADR.

The guard is the config rejection regression and removal of the exported inactive enum/setter. The existing config/UI vocabulary parity check remains in place. No new source files or file deletions.

## Commands and measured results

All commands ran from `/home/omnigent/workspace/b9s`. Go runtime: `go1.25.5 linux/amd64`. Dependencies were downloaded from committed `go.mod`/`go.sum` before checks. Go uses the committed vendor tree. No check wrote to shared Dolt.

Counts below count completed Go test outcomes, including parent tests and subtests, and exclude package summary events. Leaf counts are supplied separately. Expected RED failures were resolved by the implementation.

| Check | Exact command | Exit | Passed | Failed | Skipped | Evidence |
|---|---|---:|---:|---:|---:|---|
| Dependencies | `timeout 180s go mod download` | 0 | n/a | n/a | n/a | `/tmp/bd-trdj-dependencies.txt` |
| Config RED | `timeout 180s go test ./pkg/config -run '^TestLoadFrom_UISortRejectsPageRank$' -count=1 -timeout=60s -json` | 1 | 0 | 1 | 0 | `/tmp/bd-trdj-red-config.jsonl`, `/tmp/bd-trdj-red-config-summary.json` |
| UI RED | `timeout 240s go test ./pkg/ui -run '^(TestSortFieldCount\|TestSortPopupRendering)$' -count=1 -timeout=60s -json` | 1 | 0 | 2 | 0 | `/tmp/bd-trdj-red-ui.jsonl`, `/tmp/bd-trdj-red-ui-summary.json` |
| Initial GREEN | `timeout 180s go test ./pkg/config ./pkg/ui -run 'Sort\|TestWithConfig' -count=1 -timeout=60s -json` | 0 | 115 | 0 | 0 | `/tmp/bd-trdj-green.jsonl`, `/tmp/bd-trdj-green-summary.json` |
| Build | `timeout 180s go build ./...` | 0 | n/a | n/a | n/a | `/tmp/bd-trdj-build.txt` |
| Vet | `timeout 180s go vet ./...` | 0 | n/a | n/a | n/a | `/tmp/bd-trdj-vet.txt` |
| Final GREEN after isolating test fixtures | `timeout 180s go test ./pkg/config ./pkg/ui -run 'Sort\|TestWithConfig' -count=1 -timeout=60s -json` | 0 | 115 | 0 | 0 | `/tmp/bd-trdj-green-final.jsonl`, `/tmp/bd-trdj-green-final-summary.json` |
| Required stable suite, alone | `timeout 660s env GOFLAGS=-v make test-stable` | 0 | 3804 | 0 | 20 | `/tmp/bd-trdj-stable.txt`, `/tmp/bd-trdj-stable-summary.json` |
| Real-terminal sort smoke | `timeout 120s go test ./tests/e2e -run '^TestTreeViewSortCycle$' -count=1 -timeout=60s -json` | 0 | 1 | 0 | 0 | `/tmp/bd-trdj-tui-e2e.jsonl`, `/tmp/bd-trdj-tui-e2e-summary.json` |

Backslashes before the pipe characters in this table escape Markdown table separators. The actual regular expressions contained plain `|`, as recorded in the reports and evidence JSON.

Focused GREEN has 101 passing leaf tests and 63 passing top-level tests across 2 packages. Stable has 3586 passing leaf tests and 20 skipped leaf tests, with 2128 passing and 18 skipped top-level tests across 22 passing packages. Started and finished records reconcile. No failed or incomplete selected tests. No retry or flaky outcome was observed.

The Makefile stable recipe retains `go test -timeout 10m -skip 'DoltIntegration|TestAdd_MimeSniff_TxtAndMd|TestEmbeddedBD_ReadsAndWatchesARealStore|TestEnterInTestModeRecordsOpenerWithoutRunningIt|TestEnterOpenerAllowlist|TestBDContract_EveryWriteB9sMakes|TestDebouncer_CoalescesRapidTriggers' $(go list ./... | grep -v /tests/e2e)`. It ran alone without base-branch comparison. Its exclusions are separate from runtime skip counts. The terminal smoke used the project's bounded PTY harness. No browser suite was needed for this Go/TUI-only change.

Formatting commands: `gofmt -w` on the five changed Go files, then `gofmt -l` on those files returned no paths. `git -C /home/omnigent/workspace/b9s diff --check` exited 0. Evidence: `/tmp/bd-trdj-format.txt`. Environment and lockfile hashes: `/tmp/bd-trdj-go-environment.txt`.

## Optional skips and limits

The stable suite's 20 runtime skips are declared optional checks: real bd binaries not configured (1), S3 endpoint not configured (3), Dolt test DSN not configured (7), isolated Memory preview or research not configured (4), optional real-issue fixture files absent (2), and opt-in stress tests disabled (3). Names and reasons are in the stable summary and verbose report. These checks were not claimed as passed.

Existing configurations using `pagerank` fail config loading with supported alternatives. Existing application behavior ignores an invalid config file as a whole, starts from defaults and preserves the file, as documented in ADR 0010 and the guide. Users with such a config must choose a supported field.

No unresolved required-check failures or required blocker issues. Independent review and independent testing remain pending. No delivery action is authorized yet.

## Evidence and handoff

- Final implementation report: `/tmp/bd-trdj-implementation-report.md`
- Preflight: `/tmp/bd-trdj-preflight.txt`
- Implementation milestone: `/tmp/bd-trdj-implementation.txt`
- Focused checkpoint evidence JSON: `/tmp/bd-trdj-green-evidence.json`
- Raw reports, summaries and command exit files use the `/tmp/bd-trdj-` prefix as listed above.
- Attributed progress comments and lane-progress milestones were published throughout.
- Worktree retains exactly the six changed files listed above. HEAD remains unchanged. No delivery commit or push.
