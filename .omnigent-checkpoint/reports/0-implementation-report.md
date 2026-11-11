# bd-teri.5 implementation report

Implementation is ready for Polly's independent review and tests. No unresolved required-check failure remains. Delivery is not approved.

## Preflight

- Shared repository: `/home/omnigent/workspace/b9s`, origin `https://github.com/vanderheijden86/b9s.git`.
- Actual branch: `omnigent/bd-teri.5`.
- HEAD and fetched base `origin/main`: `c2fccef8b537ed034ebd360ea7cf31c02250e083`.
- Base fetch: `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`, exit 0. Base ref checked with `git show-ref --verify`.
- Exact remote delivery probe: `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-teri.5`, exit 2, absent. No local delivery branch existed. Created with `git switch -c omnigent/bd-teri.5 origin/main` from a clean tree.
- Required commits: none. Required-commit ancestry against both fetched base and HEAD: not applicable.
- Initial checkpoint lookup: absent. No prior work was overwritten or restored.
- Beads connected successfully as `bd_b9s`. Polly can repeat `lane_issues` once. Parent-session messaging was refused because session orchestration is disabled.
- Read repository/global instructions, testing guide, architecture, relevant query/hotkey/lazy-text ADRs, and finite state design guidance.
- Issue read with `gh issue view 24 --repo vanderheijden86/b9s --json title,body,comments,url`. It has no comments or screenshots. A combined `--comments --json` request was rejected, then corrected.
- Dependencies installed with `timeout 180s go mod download`, exit 0, using committed `go.mod`/`go.sum`. Manifests are unchanged.

## Behavior

Main already had child drill-down on 1-9, Backspace history, and tree detail n/p navigation. It lacked the requested read-only modal and row preview. The initial RED tests on unchanged production code reproduced that absence.

`Ctrl-O` is free in the default keymap after ADR 0057 moved occur to `&`. In detail it opens an ID/title chooser, with related and mentioned issues first. On a tree row, flat list row, or board card it previews that selected issue directly. The chooser searches all loaded issues, including filtered-out and closed issues.

The preview has its own issue selection, Markdown renderer, and viewport. It permits scrolling, n/p sibling cycling with wrapping, Esc return, and Ctrl-C quit. Contextual issue actions are intercepted. Esc returns directly to the original detail or row, preserving query, cursor, scroll, history, and original action target. Lazy descriptions/comments and load errors appear inside the preview. Project changes discard the preview.

Regression guards cover contextual key and mouse isolation, original action targets, filtered siblings, wrapping, chooser cancellation, mentions/relations, lazy text/error handling, query-entry ownership, and resize context. Real PTY proof covers row/flat-row preview, related detail preview, sibling cycling, read-only keys, and original edit targeting after Esc.

## Changed files

- `pkg/ui/quick_view.go` (new): chooser, read-only preview, input allowlist, independent scrolling, siblings, and rendering.
- `pkg/ui/quick_view_test.go` (new): regression coverage and original-context proof.
- `tests/e2e/quick_view_e2e_test.go` (new): six bounded real-terminal proof cases and final frame logs.
- `pkg/ui/model.go`: modal routing, updates, rendering, Ctrl-O binding, footer and detail help.
- `pkg/ui/issue_text.go`: discard preview with project text state.
- `pkg/ui/hotkeys.go`: classify Ctrl-O as a built-in key requiring explicit override in user hotkeys.
- `pkg/ui/context_help.go`: list/tree/detail shortcut documentation.
- `pkg/ui/detail_footer_keys_test.go`: exercise the advertised preview shortcut.
- `pkg/ui/open_key_test.go`: close the preview before testing ampersand occur, keeping the original assertions.
- `docs/USER_GUIDE.md`: keys, chooser, read-only restrictions, sibling wrapping, and return behavior.

## Verification

All commands ran from `/home/omnigent/workspace/b9s`. Runtime: `go version go1.25.5 linux/amd64`, `Linux x86_64`. Counts below are leaf Go test cases, excluding parent containers. Raw reports and named-node counts are also retained.

| Check | Exact command | Result | Report |
|---|---|---|---|
| Focused final GREEN | Exact command below | exit 0, 42 passed, 0 failed, 0 skipped, 0 interrupted/unrun (46 passed named nodes) | `focused-final.json`, `focused-final-report.json` |
| Stable suite, run alone | `GOFLAGS=-v timeout 660s make test-stable` | exit 0, 3587 passed, 0 failed, 20 optional skips, 0 interrupted/unrun. 3797 passed named nodes and 22 passed packages | `stable-final.txt`, `stable-final-report.json` |
| Real PTY proof | `timeout 120s go test ./tests/e2e -run '^TestQuickViewE2E$' -count=1 -timeout=90s -json` | exit 0, 6 passed, 0 failed/skipped/interrupted/unrun (7 passed named nodes). Each PTY has a 15s deadline and 3500ms app autoclose | `e2e-final.json`, `e2e-final-report.json`, `terminal-frames.txt` |
| Final compilation/type check | `timeout 180s go build ./...` | exit 0 | `build-final.txt` |
| Lint/static analysis | `timeout 180s go vet ./...` | exit 0, no diagnostics | `vet.txt` |
| Formatting | `gofmt -l` on all nine changed Go files | exit 0, no files listed | `format.txt` |
| Diff whitespace | `git -C /home/omnigent/workspace/b9s diff --check` | exit 0 | `diff-check.txt` |

The focused command was:

```sh
timeout 90s go test ./pkg/ui -run 'TestQuickView|TestDetailFooter|TestHotkey|TestBuiltin' -count=1 -timeout=60s -json
```

The 20 stable-suite skips are explicit opt-in or absent-fixture cases: one real Memory binary probe, three S3 tests, seven live Dolt reader/watcher tests, four Memory preview/research tests, two absent loader fixture files, and three 10+ minute stress tests. Every skipped test name is in `stable-final-report.json`. No shared Dolt writes were made by tests. No browser check was required for this terminal-only change.

Stable-suite results cover the final production code and all final unit tests. Subsequent edits only corrected and logged the E2E fixture in the package that `test-stable` excludes. The final PTY run covers those fixture edits. Final build/vet/format checks ran afterward. No retries were configured. No flaky outcome was recorded or counted as a pass.

## Earlier failures and resolution

- Initial focused command reached its 90s outer deadline during cold dependency compilation, exit 124, before any test events. Counts were unknown and no success was claimed. The same bounded command then ran normally.
- Initial RED: `timeout 90s go test ./pkg/ui -run '^TestQuickView' -count=1 -timeout=60s -json`, exit 1, 0 passed, 29 failed, 0 skipped. This reproduced the missing feature before implementation (`red2.json`).
- First focused GREEN used the final focused command pattern above: 37 passed, 0 failed/skipped. Later coverage brought it to 42.
- Resize proof: `timeout 90s go test ./pkg/ui -run '^TestQuickViewResizeRestoresOriginalScroll$' -count=1 -timeout=60s -json`, exit 0, 1 passed, 0 failed/skipped (`resize-red.json`, named before the outcome was known). Existing viewport sizing already retained scroll.
- Lazy-status RED: `timeout 90s go test ./pkg/ui -run '^TestQuickViewLoadsAbridgedText$' -count=1 -timeout=60s -json`, exit 1, 0 passed, 2 failed, 0 skipped. Fixed preview loading status leaking into the original detail. Both cases pass in final GREEN.
- First stable run: exit 2, 3586 passed, 1 failed, 20 optional skips. `TestTreeAmpersandTogglesOccur` assumed Ctrl-O was a no-op. It now closes the preview before asserting `&` behavior. Final stable run passes (`stable.txt`, `stable-report.json`).
- First PTY proof: exit 1, 2 passed, 4 failed, 0 skipped. Fixture issues had empty timestamps, and the initial global query was sent as one grouped key event. Corrected timestamps and discrete keystrokes while retaining deadlines and assertions. Final PTY run passes (`e2e.json`, `e2e-report.json`).
- Checkpoint comparisons refused stale/failed/incomplete evidence with exit 2 before reruns. No stale results were reused. New checks were executed. These refusals are recorded in `compare-before-stable.txt` and `compare-before-e2e.txt`.

## Limits and delivery

The chooser searches the loaded snapshot. An issue absent from that snapshot cannot be previewed. Sibling ordering follows the detail's sorted child IDs. No new data backend or write path was introduced.

No unresolved required-check failures remain, and no unrelated follow-up issues were filed. Independent reviewer and tester verdicts are pending Polly's dispatch. No delivery commit, delivery push, or PR was made. Any later PR must target `main` and include `Refs #24`.

Unapproved work is preserved only on `refs/heads/omnigent-checkpoints/bd-teri.5`. New source files are explicitly listed in each checkpoint save. Checkpoint evidence is advisory and does not replace independent checks.

## Evidence directory

All reports are under `/tmp/bd-teri.5/`. Preflight: `preflight.txt`. Final checkpoints: `checkpoint-final.txt`. Final measured E2E evidence: `e2e-evidence.json`. Final proof frames: `terminal-frames.txt`. The source files remain uncommitted in the shared worktree for review.
