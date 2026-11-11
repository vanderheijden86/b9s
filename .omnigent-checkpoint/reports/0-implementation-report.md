# bd-teri.2 implementation report

Result: IMPLEMENTATION_GREEN. Step 1 only. Independent review and tester approval remain with Polly. No delivery commit, delivery push, or PR was made.

## Preflight

- Repository: /home/omnigent/workspace/b9s
- Base and eventual PR target: main
- Actual branch: omnigent/bd-teri.2
- HEAD and fetched origin/main: c2fccef8b537ed034ebd360ea7cf31c02250e083
- Exact remote delivery probe returned 2 (absent). Local delivery was absent. Created local branch from fetched origin/main.
- Required commits: none. Required-commit ancestry against base and HEAD: not applicable.
- Initial worktree clean. Checkpoint lookup succeeded and reported absent.
- Commands: `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`, `git -C /home/omnigent/workspace/b9s rev-parse --verify refs/remotes/origin/main`, `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-teri.2`, `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-teri.2 origin/main`.
- Read task and parent comments, repository/global instructions, docs/testing.md, query ADRs 0001/0002/0004/0057, executable guards and FSM guidance.
- Read `gh issue view 40 --repo vanderheijden86/b9s --comments`, then JSON body/comments. There were no comments. Downloaded and inspected its screenshot, which shows the TUI query caret at the end.
- Full preflight evidence: preflight.txt. Current HEAD remains unchanged.

## Result and changed files

Main still had the TUI defect. Query text had no caret position, Left/Right were ignored, and edits always affected the end. The shared query now owns a rune caret. Arrow keys move only that caret. Typed and pasted text inserts there. Backspace deletes before it. The query field renders the caret inside the text. Parsed query and derived views remain synchronized.

- pkg/ui/query_state.go: bounded rune caret, insertion, backspace, and editing/replacement initialization.
- pkg/ui/model.go: shared arrow handling, caret-preserving derived-view synchronization, visible caret rendering.
- pkg/ui/query_bar_test.go: regression across tree/flat/board and typed/pasted queries, Unicode insertion/deletion, boundary behavior, query parsing, and no result navigation from arrows.
- pkg/ui/query_state_test.go: migrate existing history test to insertion method.
- tests/e2e/tree_search_filter_e2e_test.go: actual PTY typed/bracketed-paste edits and matching results.
- web/tests/keys.spec.ts: prove native search input handles arrows and editing without navigation.
- docs/USER_GUIDE.md: document search caret keys and insertion/backspace.

The web input already worked. Its local key handler consumes only Enter, and the document handler excludes input elements. Browser proof passed without web product changes. No generated bundle changed.

Guard: behavioral regression tests exercise the model input route rather than only the edit helper. No new lifecycle state, storage, data access, dependency, or architecture decision was needed. Existing query/history/find semantics and ADRs remain in force. The user guide documents the visible behavior.

## Verification

All commands ran from the shared repository unless given an absolute project path. Counts below count leaf Go test cases and exclude parent aggregates. Build, vet, formatting and typecheck are commands, not test suites, so test counts do not apply.

| Command | Exit | Passed | Failed | Skipped | Report |
| --- | --- | --- | --- | --- | --- |
| `timeout 180s go mod download` | 0 | n/a | n/a | n/a | dependencies.txt |
| `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` | 0 | n/a | n/a | n/a | npm-ci.txt |
| `timeout 180s go test ./pkg/ui -run '^TestQueryCaretEditing$' -count=1 -timeout=60s -json` (RED, before product fix) | 1 | 0 | 6 | 0 | red.jsonl, red-counts.json |
| `timeout 180s go test ./pkg/ui -run 'Test(Query\|SearchHistory\|SlashStartsShared\|AcceptedQuery\|ModelQuery\|WithInitialQuery\|TreeQuery\|Global\|IssueQuery)' -count=1 -timeout=60s -json` | 0 | 88 | 0 | 0 | focused.jsonl, focused-counts.json |
| `GOFLAGS=-v timeout 660s make test-stable` (alone, no baseline comparison) | 0 | 3559 | 0 | 20 | stable.txt, stable-counts.json |
| `timeout 180s go build ./...` | 0 | n/a | n/a | n/a | build.txt |
| `timeout 180s go vet ./...` | 0 | n/a | n/a | n/a | vet.txt |
| `timeout 180s go test ./tests/e2e -run '^(TestSearchCaretEditingE2E\|TestSearchHistoryE2E\|TestSearchTabCompletesPlainLabelE2E\|TestTreeSearchKeepsLabelFilterE2E)$' -count=1 -timeout=120s -json` (first PTY run) | 1 | 3 | 2 | 0 | tui.jsonl, tui-counts.json |
| Same PTY command after test assertion correction | 0 | 5 | 0 | 0 | tui-green.jsonl, tui-green-counts.json, tui-evidence.json |
| `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-teri.2-evidence/browser-smoke.json --seconds 90` | 0 | 3 | 0 | 0 | browser-smoke.json |
| API prereq from web command below | 0 | 12 | 0 | 0 | web-search.txt |
| Poll-guard prereq from web command below | 0 | 16 | 0 | 0 | web-search.txt |
| Playwright web command below | 0 | 4 | 0 | 0 | web-search.json, web-search.txt |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck` | 0 | n/a | n/a | n/a | typecheck.txt |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build` | 0 | n/a | n/a | n/a | web-build.txt |

The regex table escapes pipes for Markdown rendering. The actual shell regexes used literal `|`, without backslashes.

Exact browser command:

```sh
PLAYWRIGHT_JSON_OUTPUT_NAME=/tmp/bd-teri.2-evidence/web-search.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/keys.spec.ts -g 'search arrows edit' --project desktop --project desktop-safari --project desktop-firefox --project ipad --workers=1 --reporter=list,json
```

Browser fingerprint command: `lane-browser fingerprint --project /home/omnigent/workspace/b9s/web`. Full result: browser-fingerprint.json. Runtime: Node v22.23.4, Playwright 1.63.0, kernel 6.8.0-149-generic, x86_64, native runtime SHA-256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. The command did not expose an image digest. Go: go1.25.5 linux/amd64.

Playwright used a fresh JSON file and its completed exit code. Final counts: 4 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun. The project fixture fails on unexpected page errors. None occurred.

Go checks have bounded suite deadlines. PTY runs additionally have the existing harness's 15-second subprocess deadline and a 3000ms TUI autoclose for the new cases. Playwright retained the project's 30-second test, 5-second action and 10-second navigation deadlines. Tests used isolated local JSONL fixtures and fake bd where required. No shared Dolt writes were made.

Formatting: `gofmt -w` ran on pkg/ui/model.go, pkg/ui/query_state.go, pkg/ui/query_state_test.go, pkg/ui/query_bar_test.go, tests/e2e/tree_search_filter_e2e_test.go. Final `gofmt -l` emitted no paths. `git -C /home/omnigent/workspace/b9s diff --check` exited 0.

The stable suite ran before correcting only the excluded PTY test's frame assertion. Production code and stable-suite tests did not change after that run. The corrected PTY suite was executed in full again. No completed test was silently reused as a new run.

## Findings, skips, and residual scope

The expected RED failed because Left left the caret at the end in all six cases. The first PTY run correctly showed `/ alp█ha`, but its assertion expected unchanged result rows in the last incremental draw. The assertion now reads the edit sequence from the completed invalid query and separately requires the final caret and correct result rows. Both real PTY input modes then passed. This was a test error, not a product behavior exception.

The 20 stable skips were inspected: seven Dolt tests without a DSN, four opt-in Memory research/preview checks, three S3 checks without an endpoint, three opt-in long stress tests, two absent optional real-data fixtures, and one opt-in real-binary probe. Their names are in stable-counts.json. Required caret checks did not skip. The Makefile's existing exclusions were unchanged. Full Dolt/S3/stress and the complete unrelated browser/PTTY suites were outside the required bounded scope.

npm ci reported two low-severity dependency audit advisories. No dependency changes were made. This is an existing unrelated observation, not a required-check blocker or proposed follow-up task.

Progress seq 11 incorrectly stated 122 focused passes. Seq 12 and the attributed Beads correction record the measured count of 88. This report uses the measured count throughout.

Unresolved required-check failures: none. No blocker issues were filed. No unrelated product fixes were made. No files were deleted. Review and independent tester execution remain required before delivery approval.

## Handoff

All evidence is under /tmp/bd-teri.2-evidence. The latest unapproved checkpoint is saved after this report and its receipt is checkpoint-final.json. Delivery HEAD and index remain unchanged. This report and measured evidence are included in that checkpoint for recovery.

Future PR body must include `Refs #40` and Beads task `bd-teri.2`. Do not create a delivery commit, push the delivery branch, or open a PR until Polly approves the next step.
