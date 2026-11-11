# bd-teri.4 implementation report

Refs #39. Implementer: Codex (implementer-codex).
Repository: /home/omnigent/workspace/b9s

## Result

Added Close Reason and Metadata to terminal and web issue details and both Markdown copy paths. The domain model preserves original metadata JSON, nested values and exact large numbers. Metadata renders as literal code, with HTML escaped in the browser. Empty/null metadata and absent close reasons have no section. SQL readers preserve optional fields independently of older schema columns, including full/simple Dolt, full/simple SQLite, first-page/lean prefetch and on-demand closed-issue text. Existing owning files contain every change. No new source files.

Current main did not implement the request. Issue #39 has no comments or screenshots. Current main uses c for Markdown copy in detail, while board y copies an ID. Existing bindings remain intact. The requested Markdown content is included through the existing copy action.

Read repository/baseline instructions, architecture, testing, executable guards, data/variable guidance and relevant ADRs (0030 terminal Mermaid, 0031 web Mermaid, 0052 responsive detail, 0060 lean text). No state, workflow or architectural decision changed. Regression tests provide the executable guard against dropping these fields. User guide updated. Generated API types and embedded web bundle regenerated.

## Git preflight (completed before edits)

- Initial branch main, clean tree, bootstrap HEAD c2fccef8b537ed034ebd360ea7cf31c02250e083.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s show-ref --verify refs/remotes/origin/main`: c2fccef8b537ed034ebd360ea7cf31c02250e083.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-teri.4`: exit 2, exact remote delivery branch absent.
- Local delivery branch absent. `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-teri.4 origin/main`: exit 0.
- Actual branch omnigent/bd-teri.4. HEAD and fetched origin/main both c2fccef8b537ed034ebd360ea7cf31c02250e083, unchanged throughout implementation.
- Required commits: none. Ancestry against fetched base and HEAD: not applicable.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-teri.4 --base origin/main`: exit 2, no checkpoint. No restore needed.
- Checkpoint saves succeeded to refs/heads/omnigent-checkpoints/bd-teri.4. No delivery commit, push or PR. No file deletions or unrelated work.

Preflight evidence: /tmp/bd-teri.4/preflight.txt.

## Dependencies

- `timeout 120s go mod download`: exit 0. Committed go.mod/go.sum unchanged.
- `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web ci`: exit 0, 122 packages installed. Committed lock unchanged.
- Runtime: Go go1.25.5 linux/amd64, Node v22.23.4, Playwright 1.63.0.
- Native browser fingerprint: /tmp/bd-teri.4/browser-fingerprint.json, native SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6.
- npm reported two low-severity dependency advisories. No dependency changes were needed or made.

## RED evidence

Counts below are leaf tests, excluding parent containers.

1. `timeout 120s go test -json -count=1 -timeout=60s ./pkg/ui -run '^TestIssueDetail(ExtraSections|OmitsEmptyExtraSections)$'`: exit 124 during initial cold compilation, before tests ran. Incomplete evidence, never counted as a pass. Cache compilation subsequently completed. No deadline was raised.
2. `timeout 120s go test -json -count=1 -timeout=60s ./pkg/ui -run '^Test(IssueDetail(ExtraSections|OmitsEmptyExtraSections)|DetailCopyIncludesExtraDetailSections)$'`: exit 1, 1 passed, 3 failed, 0 skipped. Both fields absent from terminal and Markdown. Report focused-red-completed.jsonl.
3. `timeout 180s go test -json -count=1 -timeout=60s ./internal/datasource ./pkg/web -run '^(TestReadersPreserveExtraDetailFields|TestIssueTextPreservesExtraDetailFields|TestConvertIssueExtraDetailFields)$'`: exit 1, 0 passed, 4 failed, 0 skipped. Fields absent after SQL loads/API conversion. Report readers-red.jsonl.
4. `PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-teri.4/browser-red.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/read.spec.ts -g 'detail shows (close reason|literal metadata)' --project pixel --workers=1 --reporter=list,json`: exit 1, 0 passed, 2 failed, 0 skipped/flaky/interrupted/unrun. Missing sections reproduced. Reports browser-red.json and browser-red.txt.

## Final verification

All Go commands ran from /home/omnigent/workspace/b9s.

- `timeout 120s env B9S_UPDATE_TS=1 go test -json -count=1 -timeout=60s ./pkg/ui ./internal/datasource ./pkg/web ./pkg/model -run '^(TestIssueDetail(ExtraSections|OmitsEmptyExtraSections)|TestDetailCopyIncludesExtraDetailSections|TestReadersPreserveExtraDetailFields|TestIssueTextPreservesExtraDetailFields|TestConvertIssueExtraDetailFields|TestIssueCloneKeepsMetadataIndependent|TestGeneratedTypesAreCurrent|TestLeanIssuesKeepEpicSummaryAndDue)$'`: exit 0, 11 passed, 0 failed/skipped. Regenerated API types. Report focused-green.jsonl.
- `timeout 120s go test -json -count=1 -timeout=60s ./pkg/ui ./internal/datasource ./pkg/web ./pkg/model -run '^(TestIssueDetail(ExtraSections|OmitsEmptyExtraSections)|TestDetailCopyIncludesExtraDetailSections|TestReadersPreserveExtraDetailFields|TestIssueTextPreservesExtraDetailFields|TestDoltReadsExtraDetailFields|TestConvertIssueExtraDetailFields|TestIssueCloneKeepsMetadataIndependent|TestGeneratedTypesAreCurrent|TestLeanIssuesKeepEpicSummaryAndDue)$'`: exit 0, 14 passed, 0 failed/skipped/interrupted/unrun. Final focused set adds full/lean/first-page SQL checks. Report focused-final.jsonl.
- `timeout 660s env GOFLAGS=-v make test-stable`: exit 0, 3564 passed, 0 failed, 20 skipped, 0 interrupted/unrun (leaf tests). Top-level: 2132 passed, 0 failed, 18 skipped. Ran alone, without baseline checkout or comparison. Reports stable.txt and stable-summary.json.
- The 20 stable skips are explicit optional external/tool/research/stress checks: stock/preview bd, S3 endpoint, seven live Dolt tests, four Memory research tests, two absent optional real-world JSONL fixtures and three PERF_TEST stress tests. Exact names in stable-summary.json. No required regression was skipped.
- `timeout 180s go build ./...`: exit 0. Report go-build.txt, actual exit go-build.exit. Build has no test counts.
- `timeout 180s go vet ./...`: exit 0. Report go-vet.txt, actual exit go-vet.exit. Vet has no test counts.
- `timeout 120s go vet ./tests/e2e`: exit 0 after the final TUI test-only ANSI assertion change. Reports tui-vet.txt and tui-vet.exit.
- `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck`: exit 0. Report typecheck.txt.
- `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build`: exit 0. Report web-build.txt.
- gofmt applied to the 15 changed Go files. `git diff --name-only -- '*.go'` followed by `gofmt -l` on each absolute path produced 0 unformatted files. `git diff --check`: exit 0. Report formatting.txt.
- `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-teri.4/browser-smoke.json --seconds 90`: exit 0, Chromium/Firefox/WebKit 3 passed, 0 failed/skipped. Followed by `lane-browser fingerprint --project /home/omnigent/workspace/b9s/web`.
- `PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-teri.4/browser-green.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/read.spec.ts -g 'detail shows (close reason|literal metadata)|detail omits absent extra sections|Markdown copy keeps extra detail sections' --project pixel --project iphone --project desktop --workers=1 --reporter=list,json`: exit 0. 12 passed, 0 failed/skipped/flaky/interrupted/unrun. Includes real local JSONL services, rendered sections, escaped nested metadata, absent sections, browser Markdown clipboard capture, and page-error guard. Bundled API prerequisites: 12 passed, 0 failed/skipped/cancelled. AST guards: 16 passed, 0 failed/skipped/cancelled. Reports browser-green.json and browser-green.txt.
- `timeout 120s go test -json -count=1 -timeout=60s ./tests/e2e -run '^TestDetailExtraSectionsE2E$'`: final exit 0, 1 passed, 0 failed/skipped/interrupted/unrun. Real binary under script, 15-second PTY deadline and 2500ms auto-close. Report tui-e2e-green.jsonl. Initial run failed one assertion because treeFinalFrame retains color codes inside the phrase. The test now uses the existing ANSI stripping library before checking exact screen text. The content and headings were present in the first run. Initial failure preserved in tui-e2e.jsonl, not concealed or counted as a pass.

All reports above are under /tmp/bd-teri.4/ unless an absolute path is given. Browser screenshot inspected by eye: /home/omnigent/workspace/b9s/web/test-results/read-reading-detail-shows-literal-metadata-pixel/issue-metadata.png. Corresponding iphone and desktop screenshots are beside it in their project directories.

## Changed files

- docs/USER_GUIDE.md: document both sections and copy action.
- internal/datasource/dolt.go: optional detail-column reads in whole/simple SQL loads.
- internal/datasource/dolt_lean.go: carry optional text through lean prefetch and on-demand loads.
- internal/datasource/dolt_lean_test.go: closed text and whole/lean/first-page regression tests using isolated SQLite-backed SQL readers.
- internal/datasource/sqlite.go: preserve optional fields in full/simple SQLite loads.
- internal/datasource/sqlite_test.go: full/simple reader regressions.
- pkg/model/types.go: lossless metadata JSON, close reason, formatting and deep clone.
- pkg/model/types_test.go: metadata clone isolation guard.
- pkg/ui/helpers.go: terminal sanitization for close reasons.
- pkg/ui/issue_detail.go: both detail sections.
- pkg/ui/issue_detail_test.go: terminal/Markdown sections, empty omission and actual detail copy key regression.
- pkg/ui/model.go: both Markdown sections with literal metadata.
- pkg/web/convert.go: API conversion and snapshot text omission.
- pkg/web/convert_test.go: API regression and lean snapshot guard.
- pkg/web/types.go: browser detail fields.
- pkg/web/dist/app.js: generated browser bundle.
- pkg/web/dist/source.sha256: generated bundle-input hash.
- tests/e2e/detail_footer_keys_e2e_test.go: real PTY field proof.
- web/src/actions.ts: both browser Markdown copy sections.
- web/src/api.gen.ts: generated browser contract.
- web/src/render.ts: browser detail sections with escaped metadata code block.
- web/tests/harness.ts: real JSONL fixture fields.
- web/tests/read.spec.ts: field, omission, XSS-literal, clipboard and screenshot regressions.

## Risks and handoff

No unresolved required-check failures. No blocker issue IDs. No unrelated work or follow-up issues filed. No test touched shared Dolt. Live Dolt was not exercised because no disposable scratch server was configured. Production SQL paths were tested against isolated SQLite with the same SQL, and the stable suite preserved older-schema behavior. Optional external suites remain skipped as declared.

Two additional SELECTs retrieve optional detail columns on SQL whole/text loads. This preserves older schemas but adds read round trips. No live Dolt latency measurement was made. Browser API metadata is formatted JSON text to preserve large numbers. Native domain/JSONL metadata remains raw JSON.

Implementation self-review found no unresolved issue. Independent reviewer and tester must still execute their roles. Delivery remains unapproved. No delivery branch push or PR creation was attempted. The PR target is main and its body must contain Refs #39 and Refs: bd-teri.4. No persistent preview URL exists: browser proof used disposable local loopback services.
