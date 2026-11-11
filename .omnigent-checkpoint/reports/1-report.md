# bd-teri.4 implementer round 2

Refs #39. Repository /home/omnigent/workspace/b9s. Delivery branch omnigent/bd-teri.4, base and PR target main. No required commits.

## Fix and regression

Resolved reviewer round 1's sole P2: applyIssueDetails constructed one bind parameter per issue. SQLite accepted 32766 parameters and rejected 32767. Whole-project detail loads now call loadIssueDetailsWhere without an ID predicate. The helper still applies only the supplied issue slice. Restricted lean/on-demand reads and optional-column handling remain unchanged.

Round 2 changes only internal/datasource/dolt.go and internal/datasource/sqlite_test.go. No new source files, documentation behavior changes, dependency changes or asset changes. The new regression creates 32768 rows in an isolated SQLite database, exercises issue slices of 32766/32767/32768, and checks both unique CloseReason and Metadata values on every issue. This is the executable guard against an unbounded whole-project ID predicate.

RED before production edit: 1 leaf pass (32766), 2 leaf failures (32767/32768), 0 skips, exit 1. Both failures report too many SQL variables. GREEN after edit: 3 leaf passes, 0 failures/skips, exit 0. The original reviewer overlay reproduction also passes both cases (32766/32767), exit 0.

## Preflight and dependencies

Existing branch and all 23 uncommitted implementation paths preserved. HEAD and newly fetched origin/main both c2fccef8b537ed034ebd360ea7cf31c02250e083. Exact remote delivery lookup returned exit 2 (absent). Required commit ancestry against base/HEAD: N/A, none required. Checkpoint inspection found matching task/base/HEAD/branch. Dirty worktree was not restored or overwritten. Prior evidence comparison became incompatible after the boundary regression edit, so this round executed fresh focused and required checks.

Commands:

```
timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main
git -C /home/omnigent/workspace/b9s show-ref --verify refs/remotes/origin/main
timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-teri.4
lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-teri.4 --base origin/main
timeout 120s go mod download
```

Fetch, ref check, checkpoint inspection and dependency installation exited 0. No dependency lock changed. Evidence: preflight.txt and dependencies.txt under /tmp/bd-teri.4/round2/.

## Exact verification commands and results

All Go/make commands ran from /home/omnigent/workspace/b9s. Counts are leaf tests, excluding parent containers.

RED and GREEN used the same exact command, with separate fresh reports:

```
timeout 120s go test -json -count=1 -timeout=60s ./internal/datasource -run '^TestApplyIssueDetailsSQLiteParameterBoundary$'
```

RED 1/2/0, exit 1: boundary-red.jsonl and boundary-red.exit. GREEN 3/0/0, exit 0: boundary-green.jsonl and boundary-green.exit. Count order passed/failed/skipped.

Focused final:

```
timeout 120s go test -json -count=1 -timeout=60s ./pkg/ui ./internal/datasource ./pkg/web ./pkg/model -run '^(TestApplyIssueDetailsSQLiteParameterBoundary|TestIssueDetail(ExtraSections|OmitsEmptyExtraSections)|TestDetailCopyIncludesExtraDetailSections|TestReadersPreserveExtraDetailFields|TestIssueTextPreservesExtraDetailFields|TestDoltReadsExtraDetailFields|TestConvertIssueExtraDetailFields|TestIssueCloneKeepsMetadataIndependent|TestGeneratedTypesAreCurrent|TestEmbeddedBundleIsCurrent|TestLeanIssuesKeepEpicSummaryAndDue)$'
```

18 passed, 0 failed/skipped/interrupted/unrun, exit 0. Evidence focused.jsonl, focused.exit and focused-evidence.json.

Original reviewer reproduction:

```
timeout 120s go test -json -overlay /tmp/bd-teri.4/reviewer-round1-overlay.json -count=1 -timeout=60s ./internal/datasource -run '^TestReviewerDetailQueryParameterLimit$'
```

2 passed, 0 failed/skipped/interrupted/unrun, exit 0. Evidence reviewer-repro-green.jsonl and reviewer-repro-green.exit. Overlay changes no repository files.

Required stable suite, executed alone:

```
timeout 660s env GOFLAGS=-v make test-stable
```

3567 passed, 0 failed, 20 declared optional skips, 0 interrupted/unrun, exit 0. Top-level 2133 passed, 0 failed, 18 skipped. No baseline worktree or comparison. Normal Go caching reused 14 unchanged packages. Changed datasource and dependent packages executed. Evidence stable.txt, stable.exit and stable-summary.json. The summary lists every skipped test and cached package.

Optional skip groups: stock/preview bd probe (1), S3 endpoint (3), live Dolt (7), Memory research (4), absent optional real-world JSONL fixtures (2), PERF_TEST stress tests (3). No required new regression skipped.

```
timeout 180s go build ./...
timeout 180s go vet ./...
gofmt -w /home/omnigent/workspace/b9s/internal/datasource/dolt.go /home/omnigent/workspace/b9s/internal/datasource/sqlite_test.go
gofmt -l /home/omnigent/workspace/b9s/internal/datasource/dolt.go /home/omnigent/workspace/b9s/internal/datasource/sqlite_test.go
git -C /home/omnigent/workspace/b9s diff --check
```

Build, vet, format and diff checks exited 0. Zero unformatted files. These are checks without test counts. Evidence build.txt/build.exit, vet.txt/vet.exit, formatting.txt and diff-check.txt/diff-check.exit.

No web source changed this round. Assets were not rebuilt. TestEmbeddedBundleIsCurrent and TestGeneratedTypesAreCurrent passed in the focused set. Real browser/TUI suites and TypeScript checks were not repeated because the correction affects only whole-project SQL parameter construction. Prior reviewer evidence is historical: 20 browser passes, 1 real TUI pass, 15 focused Go passes and build/typecheck passes. Independent re-review must still assess the current correction.

## All cumulative changed paths

docs/USER_GUIDE.md
internal/datasource/dolt.go
internal/datasource/dolt_lean.go
internal/datasource/dolt_lean_test.go
internal/datasource/sqlite.go
internal/datasource/sqlite_test.go
pkg/model/types.go
pkg/model/types_test.go
pkg/ui/helpers.go
pkg/ui/issue_detail.go
pkg/ui/issue_detail_test.go
pkg/ui/model.go
pkg/web/convert.go
pkg/web/convert_test.go
pkg/web/dist/app.js
pkg/web/dist/source.sha256
pkg/web/types.go
tests/e2e/detail_footer_keys_e2e_test.go
web/src/actions.ts
web/src/api.gen.ts
web/src/render.ts
web/tests/harness.ts
web/tests/read.spec.ts

## Remaining risks and delivery

No unresolved required-check failure. The sole reviewer finding on bd-teri.4 is fixed and its exact reproduction passes. No external blockers, new issues or unrelated follow-up work. Live Dolt remains untested. The two optional-column SELECT round trips remain unmeasured against live Dolt. All regression databases are isolated local SQLite, with no shared Dolt writes.

The original acceptance and existing c/C TUI detail / c web detail Markdown copy bindings remain intact. Board y copies only ID. No built-in view has a y Markdown path. Both Markdown outputs retain Close Reason and Metadata. Issue #39 body and comments response were actually read in round 1: zero comments, no screenshot attachments to inspect. The later PR body must contain Refs #39 and Refs: bd-teri.4.

No delivery commit, delivery push or PR was authorized or attempted. Only the unapproved checkpoint ref is updated. Source HEAD and all original uncommitted work remain preserved.
