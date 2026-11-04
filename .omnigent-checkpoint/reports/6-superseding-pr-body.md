Fixes bd-4qh8. Ports bd-6e8h.12 from PR #47 onto current main.

Supersedes #47. The operator can close #47 after this replacement is accepted.

A checkout whose database changes refreshes its recent-project entry in place and collapses stale duplicates. Local identity uses resolved checkout path and host. Pathless projects use host/database identity. Locks, stable slots and successful-open timestamps retain their contracts.

The port preserves PR #47's regressions and main's symlink/path coverage. Main's canonical path helper is retained. Overlapping symlink fixtures follow the source PR's host and metadata refresh contracts. The Projects guide describes the resulting identity rules.

Symlink components resolve before lexical cleanup, including a symlink followed by `..`. Home expansion preserves those components when reading saved recent projects. Regression tests distinguish the physical checkout from a separate checkout at the lexical path and preserve in-place database refresh.

## Verification

Run from the shared repository, with Go 1.25.5 on Linux:

```sh
GOFLAGS='-v -count=1' timeout 660s make test-stable
timeout 180s go test ./pkg/config ./pkg/ui -run 'TestExpandHome|TestTouchRecent|TestRecentProject_SameProjectIdentity|TestSameProject|TestSaveRecentTo|TestProjectTable|TestLoadFrom_PreservesSymlinkParentInHomePath' -count=1 -timeout=60s -json
timeout 60s go run /tmp/bd-4qh8/reviewer-symlink-probe.go
timeout 180s go build ./...
timeout 180s go vet ./...
timeout 60s gofmt -l /home/omnigent/workspace/b9s/pkg/config/config.go /home/omnigent/workspace/b9s/pkg/config/recent.go /home/omnigent/workspace/b9s/pkg/config/recent_test.go /home/omnigent/workspace/b9s/pkg/ui/project_table_test.go
timeout 60s git -C /home/omnigent/workspace/b9s diff --check
```

| Check | Passed | Failed | Skipped |
| --- | ---: | ---: | ---: |
| Stable suite | 3281 | 0 | 20 optional |
| Focused regressions | 44 | 0 | 0 |

Leaf counts exclude parents. Focused results overlap stable results. Interrupted and unrun counts are zero for both selected suites. No automatic retries or flaky outcomes observed. Build, vet, formatting and whitespace checks exited 0. Formatting output was empty. The reviewer probe exited 0. Initial source tests reproduced the stale-database defect. Symlink-parent regressions reproduced R1 before correction: 0 passed, 4 failed, zero skips. A saved-home-path regression also failed before home expansion was corrected.

## Review

Review round 1 requested changes for R1. R1 is addressed in the implementation, with independent re-review and final independent testing pending. This body must be updated with those observed verdicts before approved PR creation.

## Risks and preview

Linux-only checks. Stable omits full PTY/browser E2E and declared host-sensitive cases. The 20 runtime skips require optional services, binaries, fixtures or opt-in settings. No shared Dolt test writes. No preview deployed and no preview URL available.

Evidence: `/tmp/bd-4qh8/correction-r1/implementation-report.md`, `/tmp/bd-4qh8/correction-r1/stable-summary.json`, `/tmp/bd-4qh8/correction-r1/focused-green-summary.json`.
