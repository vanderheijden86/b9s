# bd-6e8h.7 implementation report

Implementation complete for independent review and testing. No delivery commit, delivery push or pull request was made. The shared worktree contains the implementation.

## Behavior

`:forget <number>` forgets the inactive recent project on the current visible header key 1-9. It works without contacting the project server. The prompt supports completion, and the help overlay lists the command. Invalid or absent numbers produce an error. The active project requires switching first. `lock_recent: true` prevents removal.

`Config.ForgetRecent` uses existing project identity and preserves other entries' relative order. Its separate result slice protects retained model state if saving fails. `SaveRecentTo` accepts explicit forgotten identities and excludes them from both lists before its existing merge. Other windows' additions, unrelated settings and their YAML comments survive. The UI applies the shorter list only after the atomic save succeeds. Future header refreshes use the shorter list.

This is a routine extension of the existing command and recent-project patterns. No state machine, credential policy or ADR changed. User and architecture documentation describe the interaction and persistence limits.

## Git preflight

- Repository: `/home/omnigent/workspace/b9s`
- Assigned base and PR target: `main`
- Actual branch: `omnigent/bd-6e8h.7`
- HEAD and fetched `origin/main`: `793407bd86c74ba9985c9870c39392974ec8380e`
- Bootstrap: clean main, no local delivery branch, no conflicting worktree.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s show-ref --verify refs/remotes/origin/main`: exit 0.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-6e8h.7`: exit 2, exact delivery ref absent.
- Created branch with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-6e8h.7 origin/main`.
- Required commits: none. Ancestry against fetched base and HEAD: not applicable.
- Initial `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-6e8h.7 --base origin/main`: absent.
- HEAD and fetched base remain unchanged. No files were deleted.

## Changed files

- `pkg/config/recent.go`: identity-based removal, lock guard and explicit save exclusions.
- `pkg/config/recent_test.go`: identity, path, lock, unknown target and concurrent merge regressions.
- `pkg/ui/commands.go`: closed command table, completion and argument parsing.
- `pkg/ui/command_prompt.go`: route the forget command.
- `pkg/ui/project_table.go`: save-before-apply removal and header rebuild.
- `pkg/ui/model.go`: in-app help entry.
- `pkg/ui/command_prompt_test.go`: actual prompt regressions for removal, comments, lock, active project, visible numbers, invalid input and save errors.
- `tests/e2e/recent_projects_e2e_test.go`: real PTY proof for unlocked and locked configurations.
- `docs/USER_GUIDE.md`: command syntax and project interaction.
- `docs/ARCHITECTURE.md`: targeted persistence and count-refresh behavior.

All ten files are tracked-file edits. No new source files or dependency changes.

## Exact verification commands and results

Counts distinguish top-level Go tests from subtests. Property-test iterations and package containers are not counted as tests. Evidence paths below are relative to `/tmp/bd-6e8h.7-evidence/`.

1. `timeout 180s go mod download`: exit 0. Dependency installation, no tests. Evidence `dependencies.txt`.
2. `timeout 180s go test ./pkg/ui -run '^TestForgetRecentProjectRemovesOfflineEntryAndPreservesComments$' -count=1 -timeout 60s -json`: first fixture run exited 1, 0 passed, 1 failed, 0 skipped. Missing test fixture directory, corrected before behavioral RED. Evidence `red.jsonl`, `red-exit.txt`.
3. Same focused command after fixture correction and before implementation: exit 1, 0 passed, 1 failed, 0 skipped. Actual prompt returned `unknown command: forget` and retained the target. Evidence `red-behavior.jsonl`, `red-behavior-exit.txt`.
4. `timeout 180s go test ./pkg/config ./pkg/ui -run 'TestForgetRecent|TestSaveRecentTo|TestEveryCommandNameResolves' -count=1 -timeout 60s -json`: exit 0, 17 top-level passed, 0 failed, 0 skipped. Subtests: 7 passed, 0 failed, 0 skipped. Evidence `green.jsonl`, `green-exit.txt`, `green-evidence.json`.
5. `timeout 180s go test ./tests/e2e -run '^TestForgetRecentProjectE2E$' -count=1 -timeout 90s -json`: exit 0, 1 top-level passed, 0 failed, 0 skipped. Subtests: 2 passed, 0 failed, 0 skipped. Evidence `tui.jsonl`, `tui-exit.txt`, `tui-evidence.json`.
6. `GOFLAGS='-v -count=1' timeout 660s make test-stable`: exit 0, 2069 top-level passed, 0 failed, 18 skipped. Subtests: 1295 passed, 0 failed, 2 skipped. All 21 selected packages passed. Evidence `stable.log`, `stable-exit.txt`, `stable-summary.json`, `stable-evidence.json`.
7. `timeout 180s go build ./...`: exit 0, one command passed. No test cases. This includes Go type checking. Evidence `build.txt`.
8. `timeout 180s go vet ./...`: exit 0, one command passed. No test cases. This is the Go lint gate. Evidence `vet.txt`.
9. `git -C /home/omnigent/workspace/b9s diff --check`: exit 0. `gofmt -l` on all eight changed Go files: exit 0, no filenames. Evidence `final-tree.txt`.

The later stable run covers the final source tree, including the final slice-alias assertion and help entry. Preliminary GREEN evidence is not offered as final-tree proof. No saved check was reused. The independent tester must still execute the required checks.

The stable suite ran alone, with no base-branch comparison. Its declared Makefile exclusions remain unchanged. No retries were used. No flaky result was observed. Completed selected suites had no interrupted or unrun executions. Explicit Makefile exclusions are outside the selected suite and are not counted as observed skips.

Observed skips: one optional real-binary probe, three S3 tests, seven Dolt tests without `B9S_TEST_DOLT_DSN`, four memory preview/research tests and three opt-in stress tests. Two real-data fixture subtests skipped because their JSONL datasets are absent. Exact names and reasons are retained in `stable-summary.json` and `stable-skips.txt`. No required task regression skipped.

## Running TUI evidence

The test built the current binary and drove it through `script`. Each PTY had a 15-second deadline and `B9S_TUI_AUTOCLOSE_MS=3500`. The suite had a 90-second timeout and outer 180-second deadline. It used isolated configuration directories and local JSONL projects. The offline entry used closed loopback endpoint `127.0.0.1:1`. No shared database was written. No browser check was needed for this terminal-only action.

- Unlocked final frame: alpha on key 1, beta on key 2, no offline row. Status: `Forgot recent project offline`. Saved list: alpha, beta. Comments retained.
- Locked final frame: offline on key 2, beta on key 3. Status: `Recent projects are locked (lock_recent: true)`. Saved membership and comments retained.
- Final frames: `tui-unlocked.txt`, `tui-locked.txt`.
- Runtime: `go version go1.25.5 linux/amd64`, `CGO_ENABLED=1`, `TERM=screen-256color`. Fingerprint in `tui-evidence.json`.

## Risks, blockers and handoff

- Unresolved required-check failures: none. No blocker issue IDs.
- Unrelated discoveries needing follow-up: none. No follow-up issues filed.
- An in-flight count refresh may finish. Later refreshes exclude the forgotten project.
- Another stale window can save its copy again. Opening the project again intentionally adds it. Both limits are documented.
- The active project requires switching first. The command uses visible header numbers, which differ from stored positions when denied projects are hidden.
- Independent review and independent testing remain Polly's next roles. This report is implementer evidence, not their verdict.
- Delivery remains unapproved. No commit, delivery push or PR was attempted. Checkpoint pushes use only `refs/heads/omnigent-checkpoints/bd-6e8h.7`.

Final checkpoint save output is recorded in `checkpoint-final.txt`. Only a successful checkpoint push establishes durability. Reports and changed source files remain available in the shared Pod.
