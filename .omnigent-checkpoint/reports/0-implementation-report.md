# bd-4qh8 implementation report

Implementation complete for independent review. No delivery commit, delivery push or PR creation was performed. Only the separate unapproved checkpoint ref was pushed.

## Git preflight

- Shared repository: `/home/omnigent/workspace/b9s`.
- Assigned and actual branch: `omnigent/bd-4qh8`.
- HEAD and fetched `origin/main`: `3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`.
- PR target: `main`.
- Exact delivery remote probe returned exit 2, absent. No local delivery branch existed. Created delivery branch from fetched `origin/main`, not bootstrap HEAD.
- Required commits: none. Required ancestry checks against base and HEAD: not applicable.
- PR #47 source branch: `omnigent/bd-6e8h.12`. Fetched source head: `44a92da087e2877691a824e6ad94d10313d67a5f`.
- Checkpoint inspection: absent before implementation.
- Approach: port the source single-commit diff without committing, reconcile overlapping recent-project logic with main. No merge commits.
- Corrected evidence: `/tmp/bd-4qh8/preflight-corrected.txt`. Initial report used an unavailable remote tracking name after a FETCH_HEAD-only fetch. Explicit source tracking fetch and corrected report preceded implementation.

## Changes

- `pkg/config/recent.go`: PR #47 checkout identity, database refresh in existing slot, stale duplicate collapse, timestamp and lock contracts. Retains main's `canonicalCheckoutPath` helper name. Production behavior matches source PR.
- `pkg/config/recent_test.go`: all PR #47 regressions retained, alongside main's four symlink/path regressions. The main symlink identity fixture uses the same host on both entries. Its slot test expects metadata refresh on first touch, stable slot/order, and no change on repeated touch. These resolve the tests' overlap with PR #47's explicit host and metadata contracts.
- `pkg/ui/project_table_test.go`: source PR's declared Dolt metadata fixture. Original assertions retained.
- `docs/USER_GUIDE.md`: source PR's Projects identity description applied to the current paragraph, retaining main's other documentation changes.
- No new source files or deleted files. No lockfile changes. No unrelated work found.

## Commands and measured results

All Go commands ran from `/home/omnigent/workspace/b9s`. Runtime: `go version go1.25.5 linux/amd64`, Linux x86_64. Image digest unavailable in runtime metadata. `beads-connect` ran once before repository work. Dependencies downloaded from committed `go.mod` and `go.sum` with `timeout 180s go mod download`, exit 0.

| Completed check | Passed | Failed | Skipped | Interrupted | Unrun | Exit |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Focused RED, before implementation | 7 | 10 | 0 | 0 | 0 | 1 |
| Focused GREEN | 38 | 0 | 0 | 0 | 0 | 0 |
| Stable suite | 3276 | 0 | 20 optional | 0 | 0 | 0 |

Counts are leaf tests, excluding parent results. Stable output has 3465 passed results including parents. Focused tests overlap stable tests and must not be added to stable totals. No retries or flaky outcomes observed. Unrun counts describe the selected suite, excluding the Makefile's declared omissions.

Exact commands:

```sh
timeout 180s go mod download
# RED
timeout 180s go test ./pkg/config -run 'TestTouchRecent_(UpdatesCheckoutDatabase|CollapsesStaleCheckoutEntries|UpdatesCheckoutBehindCatalogEntry|UpdatesSymlinkedCheckoutDatabase|LockedListKeepsCheckoutDatabase|PreservesCheckoutOpenedAt)$|TestRecentProject_SameProjectIdentity$|TestSaveRecentTo_KeepsCurrentCheckoutDatabase$' -count=1 -timeout=60s -json
# GREEN
timeout 180s go test ./pkg/config ./pkg/ui -run 'TestTouchRecent|TestRecentProject_SameProjectIdentity|TestSameProject|TestSaveRecentTo|TestProjectTable' -count=1 -timeout=60s -json
# Required stable suite, run alone with no base comparison
GOFLAGS='-v -count=1' timeout 660s make test-stable
timeout 180s go build ./...
timeout 180s go vet ./...
timeout 60s gofmt -l /home/omnigent/workspace/b9s/pkg/config/recent.go /home/omnigent/workspace/b9s/pkg/config/recent_test.go /home/omnigent/workspace/b9s/pkg/ui/project_table_test.go
timeout 60s git -C /home/omnigent/workspace/b9s diff --check
```

Build and vet each exited 0. Formatting exited 0 with zero output bytes. Whitespace check exited 0. These checks have no test case counts. Go build checks types. Stable uses the Makefile's declared Go timeout of 10 minutes with a 660-second outer deadline. Focused tests use a 60-second Go timeout and 180-second outer deadline. Existing helper deadlines were retained. No shared Dolt test writes were performed.

The stable suite's 20 runtime skips have explicit optional prerequisites: one real-binary Memory probe, three S3 endpoint tests, seven Dolt DSN tests, four Memory preview/research tests, two absent external JSONL fixtures, and three PERF_TEST stress tests. Names and package results are in `stable-summary.json`. Required focused checks have no skips.

One initial stable invocation used `GOFLAGS='-json -count=1' timeout 660s make test-stable`, exit 2. The JSON option changed `go list` output and corrupted package selection. Its results are invalid and excluded from all counts. The corrected verbose invocation completed successfully. The invalid invocation is retained as `stable.jsonl`, `stable.stderr`, and `stable.exit`. No unresolved required-check failures remain.

## Evidence

All files below are under `/tmp/bd-4qh8/`:

- `preflight-corrected.txt`: assigned Git context.
- `dependencies.txt`: dependency command result.
- `implementation.txt`, `implementation.diff`: change summary and Go diff.
- `red.jsonl`, `red.stderr`, `red.exit`, `red-summary.txt`: reproduced defect before implementation.
- `focused.jsonl`, `focused.stderr`, `focused.exit`, `focused-summary.json`: completed focused run.
- `stable-verbose.txt`, `stable-verbose.stderr`, `stable-verbose.exit`, `stable-summary.json`: completed isolated stable run, measured leaf counts, package results and skip list.
- `build.txt`, `vet.txt`, `format.txt`, `format-output.txt`, `whitespace.txt`: required static/build checks.
- `superseding-pr-body.md`: prepared PR body explicitly naming #47. No PR created.

## Residual risks and handoff

Linux-only validation. The declared stable suite omits full PTY/browser E2E and host-sensitive tests. Optional integrations were not provisioned. No preview was deployed. Independent review and independent testing remain for Polly's next roles. The prepared PR body states those verdicts are pending.

Discovered issues: none. Required blockers: none. Unrelated follow-up work: none. Delivery approval remains with Polly. Do not create a PR or commit/push the delivery branch until explicit approval.
