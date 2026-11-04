# bd-g94q implementation report

Status: implementation checks passed. Ready for Polly's independent reviewer and tester. Delivery remains unapproved.

## Git preflight

- Repository: /home/omnigent/workspace/b9s, https://github.com/vanderheijden86/b9s.git.
- Initial checkout: clean main at 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb.
- Actual branch: omnigent/bd-g94q.
- Actual HEAD and fetched origin/main: 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb. Both remained unchanged through implementation.
- No local delivery branch existed. Exact remote probe `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-g94q` exited 2, proving absence.
- Created delivery branch with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-g94q origin/main` after fetching and checking the base ref.
- Required commits: none. Required-commit ancestry against fetched base and HEAD: not applicable.
- PR #38 source branch: omnigent/bd-md8p, head 510620bed7dd0ab2d4f5ffbbd298bc6565695bbe.
- Source commits: dbb5743332441ade887ff4488d7ba7e3128e5e73 (route view keys and preserve picker input), 510620bed7dd0ab2d4f5ffbbd298bc6565695bbe (preserve picker context and detail action targets).
- Source/base merge base: c62c9bd39e89fa20f751982e6c75aef3e8fc1f2f.
- Base fetch: `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`. Source explicitly fetched into refs/remotes/origin/omnigent/bd-md8p. An initial branch-name fetch populated FETCH_HEAD only, so the explicit ref fetch followed before any source edits.
- Checkpoint inspection found none. No other worktree, base test run, reset, stash, deletion, merge commit, delivery commit, delivery push or PR creation occurred.

## Intent and conflict decisions

Applied the complete two-commit source PR diff using `git apply --3way`, preserving all 21 changed paths. Three conflicts occurred in pkg/ui/model.go.

1. Retained current main's deferred detail rendering and pending command dispatch. ADR 0060 text requests still use pendingCmd after the source removes the external editor. Preserved the field and revised its comment to describe text requests.
2. Retained the source removal of obsolete handleListKeys. Main's added wholeIssue guard belonged only to that removed path. Supported edit and other text-reading paths keep their guards.
3. Retained the source removal of the external editor implementation, including main's guard on that removed action.

The source restores tree bookmarks on Alt+b, board g/gg/0, Escape precedence, project and label picker ownership, refresh selection, graph Enter identity and detail action targets. It derives advertised help from shortcut tables and retains :update. The source removes obsolete list/priority handlers and O editor action. Intentional q/list removal remains.

Current main's asynchronous reloads, lean text reads, first-page loading, deferred detail rendering, scheduling dates, release model and selected Memory reads remain. Original regression assertions remain unchanged. Gofmt adjusted one space alignment in main's scheduling E2E fixture. The formatted staged file compared byte-for-byte equal to the working file. No new ADR or regression test was needed for this conflict-only adaptation. Existing source routing tests and main's seven text-loading tests guard the affected behavior.

## Changed files

- docs/ARCHITECTURE.md
- docs/USER_GUIDE.md
- pkg/ui/context_help.go
- pkg/ui/context_help_test.go
- pkg/ui/counts_without_checkout_test.go
- pkg/ui/detail_footer_keys_test.go
- pkg/ui/editor_env_test.go
- pkg/ui/hotkeys.go
- pkg/ui/k9s_keys_test.go
- pkg/ui/model.go
- pkg/ui/open_key_test.go
- pkg/ui/project_picker.go
- pkg/ui/project_switch_discard_test.go
- pkg/ui/query_bar_test.go
- pkg/ui/tutorial.go
- pkg/ui/tutorial_content.go
- tests/e2e/board_status_filter_e2e_test.go
- tests/e2e/detail_footer_keys_e2e_test.go
- tests/e2e/help_search_e2e_test.go
- tests/e2e/tree_find_mode_e2e_test.go
- tests/e2e/tree_view_e2e_test.go

No files were added or deleted. No dependency manifest, lockfile, generated browser bundle or web source changed.

## Verification

All commands ran from /home/omnigent/workspace/b9s. Dependencies were installed from committed go.mod/go.sum before checks. Go runtime: go1.25.5 linux/amd64. Stable ran alone with no base comparison and fresh -count=1 results. Counts include parent tests and subtests. No retries or flaky outcomes were observed.

| Check | Exit | Passed | Failed | Skipped | Interrupted | Unrun |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| focused | 0 | 303 | 0 | 0 | 0 | 0 |
| stable | 0 | 3696 | 0 | 20 | 0 | 0 |
| e2e | 0 | 25 | 0 | 0 | 0 | 0 |

Focused: 53 top-level tests passed, 250 subtests passed, one package passed.
Stable: 2093 top-level tests passed, 18 top-level tests skipped, 1603 subtests passed, two subtests skipped, all 22 packages passed.
Terminal E2E: 10 top-level tests and 15 subtests passed. The suite ran the built TUI under script with isolated JSONL fixtures and bounded PTY runs. Its package elapsed time was 142.141 seconds, below the unchanged source 150-second package cap.

Build/typecheck and vet/lint each exited 0. Formatting reported no filenames. Whitespace check exited 0. Build, vet, dependency and formatting commands are not test suites and do not have test counts.

Exact commands:

```sh
# dependencies
timeout 180s go mod download

# focused
timeout 180s go test -timeout 120s ./pkg/ui -run '^(TestViewKeyRouting|TestPickerKeyRoutingIsolation|TestRemovedEditorKeyRouting|TestAdvertisedViewKeysRouteThroughModel|TestAdvertisedCommonKeysRouteThroughModel|TestProjectPickerFilterSelectionRouting|TestHotkeysAppearInFooterAndHelp|TestProjectFilterEntryRespectsOverlays|TestProjectFilterSurvivesRefreshRouting|TestStatusHelpMatchesSupportedViews|TestLabelPickerRestoresOpeningFocus|TestDetailSelectionSurvivesLabelPicker|TestGraphKeysAfterLabelPicker|TestDetailActionsAfterExcludedLabel|TestDetailActionsIgnoreTreeMarks|TestProjectFilterReopensWithAllEntries|TestProjectPicker_.*|TestUpdateCommandOpensAvailableUpdateFromTreeAndDetail|TestAbridgedDetailShowsPlaceholderThenText|TestEditWaitsForAbridgedText|TestCopyWaitsForAbridgedText|TestReloadKeepsFetchedTextWhileIssueIsUnchanged|TestTextFromAnEarlierProjectIsDropped|TestFailedTextLoadIsShownAndNotRetriedInALoop|TestOpenEditModalNeverOpensOnAnAbridgedIssue)$' -count=1 -json

# stable
env 'GOFLAGS=-v -count=1' timeout 720s make test-stable

# e2e
timeout 180s go test -timeout 150s ./tests/e2e -run '^(TestTreeBookmarkRoutingE2E|TestTreeOccurEscapeRoutingE2E|TestBoardNavigationRoutingE2E|TestGraphEnterRoutingE2E|TestGraphLabelPickerReturnRoutingE2E|TestDetailActionsAfterExcludedLabelE2E|TestDetailIgnoresRemovedEditorKeyE2E|TestListViewSortsAcrossHierarchyAndPersistsE2E|TestBoardStatusKeysFillQueryBarE2E|TestHelpOverlaySearchWithCtrlS)$' -count=1 -json

# build
timeout 180s go build ./...

# vet
timeout 180s go vet ./...

# formatting
gofmt -l /home/omnigent/workspace/b9s/pkg/ui/context_help.go /home/omnigent/workspace/b9s/pkg/ui/context_help_test.go /home/omnigent/workspace/b9s/pkg/ui/counts_without_checkout_test.go /home/omnigent/workspace/b9s/pkg/ui/detail_footer_keys_test.go /home/omnigent/workspace/b9s/pkg/ui/editor_env_test.go /home/omnigent/workspace/b9s/pkg/ui/hotkeys.go /home/omnigent/workspace/b9s/pkg/ui/k9s_keys_test.go /home/omnigent/workspace/b9s/pkg/ui/model.go /home/omnigent/workspace/b9s/pkg/ui/open_key_test.go /home/omnigent/workspace/b9s/pkg/ui/project_picker.go /home/omnigent/workspace/b9s/pkg/ui/project_switch_discard_test.go /home/omnigent/workspace/b9s/pkg/ui/query_bar_test.go /home/omnigent/workspace/b9s/pkg/ui/tutorial.go /home/omnigent/workspace/b9s/pkg/ui/tutorial_content.go /home/omnigent/workspace/b9s/tests/e2e/board_status_filter_e2e_test.go /home/omnigent/workspace/b9s/tests/e2e/detail_footer_keys_e2e_test.go /home/omnigent/workspace/b9s/tests/e2e/help_search_e2e_test.go /home/omnigent/workspace/b9s/tests/e2e/tree_find_mode_e2e_test.go /home/omnigent/workspace/b9s/tests/e2e/tree_view_e2e_test.go

# whitespace
git -C /home/omnigent/workspace/b9s diff HEAD --check

```

The stable skips are declared optional environment checks: five Memory/preview/research checks, three S3 checks, seven Dolt checks with no test DSN, two absent loader fixtures and three opt-in stress checks. No required focused regression skipped. The Makefile's declared stable exclusions remain unchanged.

## Evidence

- preflight.txt: Git branch/ref and source commit evidence.
- commands.json: exact argv for every verification command.
- environment.json: observed Go, CGO and kernel fingerprint.
- focused.jsonl, focused.stderr, focused.exit, focused-summary.json, focused-evidence.json.
- stable.log, stable.exit, stable-summary.json, stable-evidence.json.
- e2e.jsonl, e2e.stderr, e2e.exit, e2e-summary.json, e2e-evidence.json.
- build.txt/build.exit and vet.txt/vet.exit: completed commands and actual exits.
- source-preservation.json: source-PR file comparisons. Differences in docs, tutorial and tree fixture retain newer main changes. The model contains the explicit adaptations above.
- formatting.txt, whitespace.txt, fixture-formatting.txt.

All evidence paths above are under /tmp/bd-g94q-evidence. Lane milestones were published at stage changes and before and after long checks. Attributed Beads progress comments are on bd-g94q.

## Residual limits and handoff

No unresolved required-check failures or product findings. No follow-up issues filed. Optional environment skips remain as listed. Focused terminal checks opened edit/close/delete dialogs without executing real issue writes or self-update installation. Web checks are outside this TUI-only change. No shared Dolt writes were made by tests.

Independent reviewer and tester have not yet reported. Polly owns those roles and delivery approval. The implementation remains uncommitted on the exact delivery branch. Only the separate unapproved checkpoint ref may be pushed. A replacement PR must name #38 as superseded after approval. No replacement PR exists yet.
