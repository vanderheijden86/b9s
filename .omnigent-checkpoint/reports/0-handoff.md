# bd-teri.10 implementation handoff

Implementation is green and ready for independent review. Delivery is not approved.
Refs #22: https://github.com/vanderheijden86/b9s/issues/22

## Scope and result

Current main already advertises working uppercase L for label filters. The regression passed before product edits. The remaining list hints were wrong: flat lists advertised folding, and label/assignee modes described numeric keys as project selection.

The footer omits Tab/Shift-Tab folding in both explicit flat mode and a type-query list. Numeric hints name the active project, label or assignee picker. Key routing and write behavior are unchanged.

## Git preflight

- Repository: /home/omnigent/workspace/b9s
- Origin: https://github.com/vanderheijden86/b9s.git
- Assigned base and PR target: main
- Actual branch: omnigent/bd-teri.10
- HEAD and fetched origin/main: c2fccef8b537ed034ebd360ea7cf31c02250e083
- Initial worktree was clean on main. Local delivery branch did not exist.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s rev-parse --verify origin/main`: found the SHA above.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-teri.10`: exit 2, absent.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-teri.10 origin/main`: exit 0.
- Required commits and ancestry checks: none.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-teri.10 --base origin/main`: exit 0, checkpoint absent.
- HEAD remains unchanged after implementation. No delivery commit, push or PR.

## Exact changed files

- `pkg/ui/model.go`: truthful flat-list fold and numeric-picker footer hints.
- `pkg/ui/list_mode_test.go`: flat-mode/type-query footer regression, working label hint, numeric-picker hints and selected-row edit/close/Delete tests.
- `tests/e2e/list_view_e2e_test.go`: six real-terminal fixture cases, with final screen evidence.
- `docs/testing.md`: command, isolation and behavior covered by the terminal proof.

No new source files or file deletions. No ADR decision changed. Read docs/testing.md, ADRs 0011, 0027 and 0035, and the executable-guards guide. Read GitHub #22 including comments (none) and its screenshot (a contrast example).

## Verification

Commands ran in /home/omnigent/workspace/b9s. Counts below are leaf test cases. Parent/subtest result-event counts are separate to avoid double counting. All final test runs completed, with zero interrupted or unrun cases. No retries or flaky outcomes were observed.

| Command | Exit | Passed | Failed | Skipped | Evidence under /tmp/bd-teri.10 |
|---|---:|---:|---:|---:|---|
| `timeout 180s go mod download` | 0 | N/A | N/A | N/A | dependencies.txt |
| `timeout 120s go test ./pkg/ui -run '^TestListMode(FooterOmitsFoldingKeys\|LabelHintUsesWorkingKey\|AdvertisedActionsTargetSelectedRow)$' -count=1 -timeout=60s -json` (expected red) | 1 | 4 | 2 | 0 | focused-red-2.json |
| `timeout 120s go test ./pkg/ui -run '^TestListModePickerNumberHints$' -count=1 -timeout=60s -json` (expected red) | 1 | 0 | 2 | 0 | picker-red.json |
| `timeout 120s go test ./pkg/ui -run '^TestListMode(FooterOmitsFoldingKeys\|LabelHintUsesWorkingKey\|PickerNumberHints\|AdvertisedActionsTargetSelectedRow)$' -count=1 -timeout=60s -json` | 0 | 8 | 0 | 0 | focused-final.json, focused-final-counts.json |
| `timeout 180s go build ./...` | 0 | N/A | N/A | N/A | build-final.txt |
| `GOFLAGS=-v timeout 660s make test-stable` | 0 | 3560 | 0 | 20 | stable-final.txt, stable-final-counts.json |
| `timeout 180s go vet ./...` | 0 | N/A | N/A | N/A | vet-final.txt |
| `timeout 180s go test ./tests/e2e -run '^TestListViewAdvertisedShortcutsE2E$' -count=1 -timeout=120s -json` | 0 | 6 | 0 | 0 | tui-final.json, tui-final-counts.json, tui-final-frames.txt |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0 | N/A | N/A | N/A | diff-check-final.txt |
| `gofmt -l /home/omnigent/workspace/b9s/pkg/ui/model.go /home/omnigent/workspace/b9s/pkg/ui/list_mode_test.go /home/omnigent/workspace/b9s/tests/e2e/list_view_e2e_test.go` | 0 | N/A | N/A | N/A | format-final.txt (empty) |

The escaped table pipes above represent literal regex alternation characters, not shell backslashes. Exact argv is also in commands.json.

Final event counts including parent tests: focused 11 passed, stable 3770 passed and 20 skipped, terminal 7 passed. Stable ran alone without a baseline comparison. Go may cache unaffected packages. The changed UI package ran freshly. Its existing target supplies the 10-minute package timeout. Each PTY session has a 15-second hard deadline and an auto-close timer.

The initial focused invocation failed to compile because the regression setup compared a value struct with nil. No tests ran. The test setup was corrected before the valid red proof. Initial successful checks were superseded after terminal evidence exposed the numeric hint defect. All final gates above ran after the final product edits.

The 20 declared optional skips are: real bd capability probe (1), S3 endpoint (3), Dolt reader/watcher DSN (7), Memory preview/research inputs (4), absent external loader fixtures (2), opt-in PERF_TEST stress cases (3). These are not missing required shortcut checks.

## Running TUI proof and guards

The built binary ran under util-linux script in a 160x40 terminal, against isolated development JSONL. Final captured screens show the flat list without fold hints, L label filters with `0-9:label`, A assignee filters with `0-9:assignee`, and edit/close/Delete targeting lv-deep (Cheetah task). Close/Delete stop at confirmation. Tests do not write to shared Dolt or open browsers/editors. No browser suite is applicable to this terminal-only change.

The executable guards read the actual rendered footer for both flat-list entry paths and active picker modes. Selected-row tests run Model.Update, and PTY tests run the built program. Independent review/test verdict is pending.

Runtime fingerprint: go1.25.5 linux/amd64, Linux 6.8.0-149-generic x86_64, script from util-linux 2.41.5. See runtime.txt. Image digest was unavailable.

## Checkpoints and handoff

Unapproved checkpoint target: refs/heads/omnigent-checkpoints/bd-teri.10. Checkpoint saves succeeded after meaningful edits and before long validation. The final save includes this report and measured terminal evidence. No new source-file flags are needed because all changes are tracked files.

Prior stable/vet/terminal evidence comparisons exited 2: `saved evidence is stale, failed, skipped or incomplete`, `compatible=false`, `complete=false`. No results were reused. Final required checks ran again. See stable-compare.txt, vet-compare.txt and tui-compare.txt. These advisory rejections are not required-check failures.

No unresolved required-check failures or blocker issue IDs. No unrelated follow-up work was filed. Beads progress comments are attributed to Codex, role implementer. The task state, assignee, dependencies and labels were not changed. Do not deliver until Polly approves.
