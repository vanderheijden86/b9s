# bd-bezu implementation report, review round 2

Addressed the required round 1 finding on bd-bezu. Footer folding hints use IsListView(), the same effective layout predicate as the renderer. The guard covers type:task, !type:epic and F3 with the saved flat preference explicitly off. Both reported reviewer diagnostic cases now pass. Independent review round 2 remains pending.

## Preserved Git context

Repository: /home/omnigent/workspace/b9s
Assigned base and PR target: main
Actual delivery branch: omnigent/bd-bezu
Actual HEAD and fetched origin/main: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a
Required commits: none. Required-commit ancestry checks against base and HEAD: not applicable.

Bounded base fetch succeeded. Exact remote delivery probe, `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-bezu`, exited 2 (absent). The existing local delivery branch and its three modified files were preserved. No branch switch, reset or restore was performed. Checkpoint inspection reported matching context. HEAD and base remain unchanged after verification.

## All changed files

- /home/omnigent/workspace/b9s/pkg/ui/model.go: footer fold-hint condition uses IsListView() rather than IsFlatMode(). Other round 1 footer edits are preserved. No key handlers changed.
- /home/omnigent/workspace/b9s/pkg/ui/detail_footer_keys_test.go: extend the rendered-footer matrix to positive and negated type predicates and the F3 entry path in all three picker modes. Each forced-list fixture asserts saved flat preference false and effective list layout true. Existing action assertions account for clearing an active query, toggling its existing type, and flat-list branch selection. Clear the setup status message before reading hints.
- /home/omnigent/workspace/b9s/docs/testing.md: document query-forced flat-list coverage and ADR 0027.

The guard exercises all advertised shortcuts in each added state. Reintroducing either fold hint fails its action assertion. These changes follow ADR 0027 and the existing flat-list branch contract in ADR 0056. No new ADR is needed.

## Test-first proof

Before changing the production predicate:

`timeout 120s go test ./pkg/ui -run '^TestViewFooterKeysAllWork/(type:task|!type:epic|F3_type_query)' -count=1 -timeout=60s -json`

Exit 1. Terminal leaf counts: 270 passed, 18 failed, 0 skipped, 0 interrupted. Every failure is Tab or Shift-Tab folding in one of the three forced-list states and three picker modes. Evidence: round2-red-confirmed.jsonl and round2-red-confirmed-summary.json.

An earlier expanded run also exposed two fixture assumptions. Flat-list branch selection was corrected against ADR 0056, and the F3 setup status was cleared before reading hint text. The corrected red run above isolates only the required product regression. Production changed after that run.

## Fresh required verification

All commands ran from /home/omnigent/workspace/b9s on Go go1.25.5 linux/amd64. Counts are terminal leaf checks, excluding parent groups.

| Exact command | Exit | Passed | Failed | Skipped | Interrupted | Unrun |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `timeout 120s go test ./pkg/ui -run 'Footer' -count=1 -timeout=60s -json` | 0 | 761 | 0 | 0 | 0 | 0 |
| `GOFLAGS='-v -count=1' timeout 660s make test-stable` | 0 | 4296 | 0 | 20 | 0 | 0 |
| `timeout 300s go build ./...` | 0 | N/A | N/A | N/A | N/A | N/A |
| `timeout 300s go vet ./...` | 0 | N/A | N/A | N/A | N/A | N/A |

The focused suite has 17 passing top-level tests. The stable suite has 2126 passing and 18 skipped top-level tests across 22 passing packages. Stable ran alone without a baseline comparison. Build checks Go types. Vet supplies the Go lint check. No flaky outcome was observed.

The exact reviewer diagnostic was also rerun:

`timeout 120s go test -overlay /tmp/bd-bezu/reviewer-overlay.json ./pkg/ui -run '^TestReviewerFooterForcedFlatTypeQuery$' -count=1 -timeout=60s -v`

Exit 0. 2 passed leaf cases, 0 failed, 0 skipped, 0 interrupted, 0 unrun. Both type:task and !type:epic retain three flat rows, and neither advertises Tab or Shift-Tab. The original reviewer overlay and probe source were preserved.

Dependencies: `timeout 180s go mod download`, exit 0, using committed module files. No dependency files changed.

Formatting: `gofmt -l /home/omnigent/workspace/b9s/pkg/ui/model.go /home/omnigent/workspace/b9s/pkg/ui/detail_footer_keys_test.go`, exit 0 with no paths printed. `git -C /home/omnigent/workspace/b9s diff --check`, exit 0.

## Evidence files

All paths below are under /tmp/bd-bezu:

- round2-preflight.txt and round2-dependencies.txt
- round2-red-confirmed.jsonl, round2-red-confirmed.exit, round2-red-confirmed-summary.json
- round2-footer.jsonl, round2-footer.stderr, round2-footer.exit, round2-footer-summary.json
- round2-reviewer-probe.txt, round2-reviewer-probe.exit, round2-reviewer-probe-summary.json
- round2-stable.txt, round2-stable.exit, round2-stable-summary.json, round2-stable-skips.txt
- round2-build.txt, round2-build.exit, round2-build-summary.txt
- round2-vet.txt, round2-vet.exit, round2-vet-summary.txt
- round2-review.txt and round2-checkpoint-compare.txt

Old round 1 evidence was rejected after source changes. Fresh checks above were executed. The saved passing focused run is round2-footer. Final checkpoint comparison accepted its command, runtime, counts and current source: compatible=true, complete=true, approved=false. Independent testing remains required.

## Residual risks and unresolved work

The stable suite retains 20 declared optional leaf skips: one real-binary probe, three S3 checks, seven Dolt reader/watcher checks, four preview/research checks, two absent real-project fixture cases and three opt-in stress checks. Exact names and reasons are in round2-stable-skips.txt. None is a footer check. The footer guard uses isolated Model fixtures, not a real terminal, clipboard write, project switch or database write.

Unresolved required-check failures: none. The two reproductions of the bd-bezu finding pass. No unrelated follow-up issue was filed.

No files were deleted. No delivery commits, delivery pushes or PR were made. Only the separate unapproved checkpoint ref is used. Independent review and tester verdicts remain Polly's next stages.
