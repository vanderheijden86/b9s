# bd-bezu implementation report

Implementation and required verification are complete. Delivery is unapproved. No delivery commit, delivery push or pull request was made. Only the separate unapproved checkpoint ref was pushed.

## Git preflight

- Repository: /home/omnigent/workspace/b9s
- Assigned base and PR target: main
- Actual branch: omnigent/bd-bezu
- Actual HEAD: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a
- Fetched origin/main: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main` succeeded.
- Exact delivery probe: `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-bezu`, exit 2 (absent).
- No local delivery branch existed. The clean checkout was switched with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-bezu origin/main`.
- Required commits: none. Required-commit ancestry checks against base and HEAD: not applicable.
- Initial checkpoint: absent. Final checkpoint comparison: compatible=true, complete=true, approved=false.
- HEAD and fetched base remain unchanged. The final worktree contains only the three intended modified files below.

## Changed files

- `pkg/ui/detail_footer_keys_test.go`: extend the existing rendered-footer guard to tree, flat tree, board, graph, split and list. Exercise project, label and assignee modes, every numeric/function-key alternative, both navigation directions, paging, gg and 0, and both resize directions. Assert selection, returned messages, clipboard contents, confirmation/modal state, query, folds, layout and focus. Refresh checks its FileChangedMsg. Fixtures avoid external project changes, clipboard access and database writes.
- `pkg/ui/model.go`: omit fold hints in flat tree, omit unsupported fold/edit hints in fallback split, omit unsupported split/filter/edit hints in fallback list, advertise the existing Ctrl-N create action, and describe numeric hints as project, label or assignee according to picker mode. No handlers were changed.
- `docs/testing.md`: document guard scope, fixture boundaries and the focused command.

No new ADR is needed. The footer follows existing behavior and ADRs 0027, 0030 and 0034. Assertions use the existing query and confirmation contracts. The guard fails on a new advertised key without an action assertion.

## Required verification

All commands ran from /home/omnigent/workspace/b9s. Go runtime: go1.25.5 linux/amd64. Counts below are terminal leaf checks, excluding parent test-group counts.

| Exact command | Exit | Passed | Failed | Skipped | Interrupted | Unrun |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `timeout 120s go test ./pkg/ui -run 'Footer' -count=1 -timeout=60s -json` | 0 | 491 | 0 | 0 | 0 | 0 |
| `GOFLAGS='-v -count=1' timeout 660s make test-stable` | 0 | 4026 | 0 | 20 | 0 | 0 |
| `timeout 300s go build ./...` | 0 | N/A | N/A | N/A | N/A | N/A |
| `timeout 300s go vet ./...` | 0 | N/A | N/A | N/A | N/A | N/A |

The focused run has 17 passing top-level tests. The stable run has 2126 passing and 18 skipped top-level tests across 22 passing packages. Stable ran alone, with no baseline comparison. Build also checks Go types. Vet supplies the required Go lint check.

Stable's 20 declared leaf skips are: one real-binary capability probe, three S3 tests, seven Dolt reader/watcher tests, four memory preview/research tests, two absent real-project JSONL fixture cases, and three opt-in stress tests. Exact names and reasons are in stable-skips.txt. None belongs to the footer guard. No flaky outcome was observed.

Dependency preparation: `timeout 180s go mod download`, exit 0, using committed go.mod/go.sum. No dependency files changed.

Final formatting checks: `git -C /home/omnigent/workspace/b9s diff --check`, exit 0. `gofmt -l /home/omnigent/workspace/b9s/pkg/ui/model.go /home/omnigent/workspace/b9s/pkg/ui/detail_footer_keys_test.go`, exit 0 with no paths printed.

## Test-first evidence and diagnostics

- The first bounded attempt reached the 120s outer deadline during cold compilation, exit 124, without test results. It was not reported as a pass. The same deadline was retained on subsequent runs.
- Before production edits, the initial guard measured 308 passed, 16 failed and 0 skipped leaf checks, reproducing unsupported hints.
- Stronger numeric-label assertions exposed incorrect picker labels. A board-top expectation was corrected to exclude epic lane headers, consistent with the ADR.
- The final pre-edit guard (`footer-red4`) measured 268 passed, 56 failed and 0 skipped leaf checks with the exact command `timeout 120s go test ./pkg/ui -run '^TestViewFooterKeysAllWork$' -count=1 -timeout=60s -json`.
- The assignee-mode guard reproduced its label mismatch before that production fix: 416 passed, 49 failed, 0 skipped, using the same focused TestViewFooterKeysAllWork command.
- After footer edits, the final required focused run passed 491 leaf checks. Earlier passing runs were superseded after stronger test assertions were added. Saved evidence was compared with the current tree, rather than reused after changes.

## Evidence

All files below are under /tmp/bd-bezu:

- preflight.txt
- dependencies.txt
- footer-red2.jsonl, footer-red3.jsonl, footer-red4.jsonl, assignee-red.jsonl
- footer-verified.jsonl, footer-verified.stderr, footer-verified.exit, footer-verified-summary.json
- stable.txt, stable.exit, stable-summary.json, stable-skips.txt
- build.txt, build.exit, build-summary.txt
- vet.txt, vet.exit, vet-summary.txt
- review.txt, checkpoint-compare.txt

The saved passing focused run is footer-verified. Its command, runtime and measured counts were saved as checkpoint evidence. The final comparison accepted it against the current source. The independent tester must still run required checks.

## Remaining state

- Unresolved required-check failures: none.
- Filed follow-up issues: none.
- External services and opt-in stress runs retain their declared skips.
- Independent review and tester verdicts remain Polly's next stages.
- Delivery commits, pushes to omnigent/bd-bezu and PR creation remain unapproved.
