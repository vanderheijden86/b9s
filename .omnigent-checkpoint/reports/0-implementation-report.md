# bd-4qh8 required correction R1

R1 is addressed in the implementation. Independent re-review remains pending. This report supersedes the earlier implementation report for final-tree commands and counts. No delivery commit, delivery push or PR creation occurred. Only the separate unapproved checkpoint ref was pushed.

## Git context

Repository: `/home/omnigent/workspace/b9s`. Actual branch: `omnigent/bd-4qh8`. Assigned base and PR target: `main`.

HEAD and freshly fetched `origin/main`: `3945b45ab6f3932b0195f3482d6cd0d2025d5ecb`. Delivery remote remains absent, exact `ls-remote --exit-code --heads` exit 2. Existing local branch and all prior uncommitted implementation work were preserved. Required commits: none, required ancestry checks against base and HEAD not applicable. Source PR #47 head: `44a92da087e2877691a824e6ad94d10313d67a5f`.

The prior checkpoint matched context and was inspected, not restored over existing work. Evidence comparison returned exit 2, incompatible/incomplete, with `saved evidence is stale, failed, skipped or incomplete`. No saved test evidence was reused. All required checks were executed on the corrected source. Preflight report: `/tmp/bd-4qh8/correction-r1/preflight.txt`.

## Correction and files

R1 concerns lexical cleanup before filesystem resolution. For `link -> real/sub`, `link/../checkout` must identify `real/checkout`, not the distinct lexical `checkout` beside `link`.

This correction changes three files:

- `pkg/config/recent.go`: evaluate symlinks before `filepath.Abs` cleans path components. Convert the resolved path to absolute form and retain the missing-checkout lexical fallback.
- `pkg/config/config.go`: preserve components during home expansion. The initial fix exposed that `expandHome` also cleaned `..` through `filepath.Join`. This affects paths loaded from `recent_projects`, so fixing only the recent helper would leave saved home paths incorrect.
- `pkg/config/recent_test.go`: three new test functions cover absolute/relative/home physical identity with a distinct lexical checkout, in-slot metadata refresh without duplicate entries, and a home path read from saved configuration. Existing test bodies remain unchanged by this correction.

The complete task changes five files against main: the three above, `pkg/ui/project_table_test.go`, and `docs/USER_GUIDE.md`. The latter two retain the prior PR #47 port. All 27 source config test bodies and four main path regression bodies remain. No new source files, deleted files, lockfile changes, or unrelated changes. `correction.diff` records the changes against the pre-review implementation tree.

The existing guide already specifies resolved checkout identity. This correction restores that documented behavior and requires no additional product documentation or ADR. The executable guards are the physical-identity, slot-refresh and saved-home-path regression tests.

## Test-first evidence and final results

All commands ran from `/home/omnigent/workspace/b9s`, with Go 1.25.5 on Linux amd64. Committed dependencies were downloaded before checks. No shared Dolt test writes or browser/editor opens were performed.

| Run | Passed | Failed | Skipped | Interrupted | Unrun | Exit |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Initial symlink-parent RED | 0 | 4 | 0 | 0 | 0 | 1 |
| Intermediate focused attempt | 41 | 1 | 0 | 0 | 0 | 1 |
| Saved-home-path RED | 0 | 1 | 0 | 0 | 0 | 1 |
| Final focused GREEN | 44 | 0 | 0 | 0 | 0 | 0 |
| Final stable suite, alone | 3281 | 0 | 20 optional | 0 | 0 | 0 |

Counts use leaf tests and exclude parents. The final stable run has 3471 passed results including parents. Final focused results overlap stable results and must not be added to them. Counts describe selected suites, not Makefile-excluded tests. No automatic retries or flaky outcomes were observed.

The initial RED failed because absolute, relative and home symlink-parent paths rejected the physical checkout, matched the unrelated lexical checkout, and duplicated the recent entry. After correcting resolution order, absolute/relative and slot behavior passed. The remaining home case exposed cleaning in home expansion. The saved-config RED independently reproduced that path before the home expansion change. No assertions were weakened.

The unmodified reviewer probe now exits 0. Its JSON reports `same_actual_want_true: true`, `same_unrelated_want_false: false`, and `after_touch_count_want_1: 1`.

Final required build, vet, formatting and whitespace checks each exited 0. Formatting produced zero output bytes. These commands and the standalone probe have no test-case counts.

Exact commands:

```sh
timeout 180s go mod download
# Initial RED
timeout 180s go test ./pkg/config -run '^(TestSameProject_ResolvesSymlinkBeforeParent|TestTouchRecent_UpdatesCheckoutThroughSymlinkParent)$' -count=1 -timeout=60s -json
# Intermediate focused attempt
timeout 180s go test ./pkg/config ./pkg/ui -run 'TestTouchRecent|TestRecentProject_SameProjectIdentity|TestSameProject|TestSaveRecentTo|TestProjectTable' -count=1 -timeout=60s -json
# Saved-home-path RED
timeout 180s go test ./pkg/config -run '^TestLoadFrom_PreservesSymlinkParentInHomePath$' -count=1 -timeout=60s -json
# Final GREEN and reviewer probe
timeout 180s go test ./pkg/config ./pkg/ui -run 'TestExpandHome|TestTouchRecent|TestRecentProject_SameProjectIdentity|TestSameProject|TestSaveRecentTo|TestProjectTable|TestLoadFrom_PreservesSymlinkParentInHomePath' -count=1 -timeout=60s -json
timeout 60s go run /tmp/bd-4qh8/reviewer-symlink-probe.go
# Final required suite, run alone with no base comparison
GOFLAGS='-v -count=1' timeout 660s make test-stable
timeout 180s go build ./...
timeout 180s go vet ./...
timeout 60s gofmt -l /home/omnigent/workspace/b9s/pkg/config/config.go /home/omnigent/workspace/b9s/pkg/config/recent.go /home/omnigent/workspace/b9s/pkg/config/recent_test.go /home/omnigent/workspace/b9s/pkg/ui/project_table_test.go
timeout 60s git -C /home/omnigent/workspace/b9s diff --check
```

Focused Go checks use a 60-second Go timeout and 180-second outer deadline. Stable uses its declared 10-minute Go timeout and a 660-second outer deadline. Existing bounded helper waits remain. No hung runs or raised timeouts.

## Evidence and residual risks

All correction evidence is under `/tmp/bd-4qh8/correction-r1/`:

- `preflight.txt`, `dependencies.txt`, `checkpoint-compare.json`, `correction.diff`, `implementation.txt`.
- `red.jsonl`, `red.stderr`, `red.exit`, `red-summary.json`.
- `focused.jsonl`, `focused.stderr`, `focused.exit`, `focused-summary.json` retain the intermediate home failure.
- `home-red.jsonl`, `home-red.stderr`, `home-red.exit`, `home-red-summary.json`.
- `focused-green.jsonl`, `focused-green.stderr`, `focused-green.exit`, `focused-green-summary.json`.
- `reviewer-probe.json`, `reviewer-probe.stderr`, `reviewer-probe.exit`.
- `stable-verbose.txt`, `stable-verbose.stderr`, `stable-verbose.exit`, `stable-summary.json`.
- `build.txt`, `vet.txt`, `format.txt`, `format-output.txt`, `whitespace.txt`.

The 20 optional skips retain their documented prerequisites: one real Memory binary probe, three S3 cases, seven Dolt DSN cases, four Memory preview/research cases, two absent external JSONL fixtures, and three PERF_TEST stress cases. Their exact names and package results are in `stable-summary.json`.

Residual coverage limits: Linux-only checks, optional integrations not provisioned, and full PTY/browser E2E and host-sensitive cases excluded by the declared stable suite. No preview deployed. Home expansion is shared with legacy/project configuration, whose existing tests pass in the stable suite.

Required blocker ledger: **bd-4qh8 / R1 addressed**, pending independent re-review. No unresolved required-check failures remain. No external blocker or unrelated follow-up issue was filed. Delivery remains unapproved. The prepared PR body at `/tmp/bd-4qh8/superseding-pr-body.md` names #47 and states independent re-review/testing are pending.
