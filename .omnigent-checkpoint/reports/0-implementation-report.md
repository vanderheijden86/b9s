# Implementation report: bd-e5u3.20.14

Implementation is ready for independent review and testing. No delivery commit, delivery push or PR was made. Only the separate unapproved checkpoint ref was pushed.

## Git preflight

- Repository: /home/omnigent/workspace/b9s (https://github.com/vanderheijden86/b9s.git).
- beads-connect was the first command and exited 0.
- Initial main checkout was clean. No local delivery branch existed.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main` exited 0.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-e5u3.20.14` exited 2, establishing absence.
- Created omnigent/bd-e5u3.20.14 from origin/main, not the bootstrap HEAD.
- Actual branch: omnigent/bd-e5u3.20.14.
- HEAD and fetched base origin/main: 793407bd86c74ba9985c9870c39392974ec8380e.
- Required commits: none. Required commit ancestry against base and HEAD: not applicable.
- Additional final `git merge-base --is-ancestor origin/main HEAD` exited 0.
- Initial checkpoint inspect exited 2: no checkpoint existed.
- Read global/project instructions, docs/testing.md, task/parent/.10 records, ADRs 0022, 0030, 0052, 0053 and 0054.

## Change

Ordinary web board cards are borderless rows showing status, short ID, title and P0/P1 only. Selection highlights the row and can wrap its title. It adds no metadata or parent breadcrumb. Complete issue details retain metadata and parent navigation. Same-cell order and indentation remain. Epic summary rendering, data and write paths are unchanged.

Measured desktop/wide row height is 24px: ten rows per 240px. Phone/tablet targets retain 44px: five rows per 240px. These are row-height measurements, not a claim that every row is visible in a Compact lane. Existing lane heights and fades remain.

ADR 0065 records the web presentation decision. ADR 0022 changes only status and superseded_by. Its remaining navigation decisions continue under ADR 0065. ADR 0030 remains unchanged for the terminal. Architecture and user guide document current web behavior.

## Changed files

- web/src/render.ts: ordinary row markup and sparse priority, remove breadcrumb bookkeeping.
- web/src/app.css: borderless spacing and selection, preserve touch targets.
- web/tests/board-detail.spec.ts: density, both cursor states, detail metadata, parent navigation, epic facts, screenshots and page-error failures.
- web/tests/board.spec.ts: parent information in details, page-error failures.
- web/tests/wide.spec.ts: compact selection regression and page-error failures.
- web/tests/tokens.spec.ts: check secondary metadata colors in details.
- pkg/web/dist/app.css, pkg/web/dist/app.js, pkg/web/dist/source.sha256: regenerated embedded assets.
- docs/adr/0065-keep-web-board-issue-rows-title-focused.md: new decision.
- docs/adr/0022-make-the-phone-epic-rail-an-index.md: permitted supersession fields only.
- docs/USER_GUIDE.md, docs/ARCHITECTURE.md: current behavior.

## Commands and measured outcomes

All commands ran from the shared repository unless an absolute prefix is shown. Counts below are separate runs and must not be added as unique coverage.

```sh
timeout 240s go mod download
timeout 240s npm --prefix /home/omnigent/workspace/b9s/web ci
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck
timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build
timeout 660s env GOFLAGS=-v make -C /home/omnigent/workspace/b9s test-stable
timeout 240s go build ./...
timeout 240s go vet ./...
timeout 60s go test ./tests/docs -count=1 -timeout=30s -v
git -C /home/omnigent/workspace/b9s diff --check
```

Every command above exited 0. Build/typecheck/vet/dependency/whitespace commands have no test counts. The stable suite ran alone, without a base comparison: 2057 top-level passed, 0 failed, 18 skipped across 21 passing packages. Including subtests: 3345 passed, 0 failed, 20 skipped. Skips require optional real bd binaries, S3, Dolt, Memory research/preview, PERF_TEST or two absent real-project fixtures. Makefile exclusions and tests/e2e are outside these measured outcome counts. Documentation checks: 3 passed, 0 failed, 0 skipped.

```sh
lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-e5u3.20.14/browser-smoke.json --seconds 90
lane-browser fingerprint --project /home/omnigent/workspace/b9s/web
```

Smoke: 3 passed, 0 failed, 0 skipped. Fingerprint: Playwright 1.63.0, Node v22.23.4, x86_64, kernel 6.8.0-147-generic, native runtime SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. CPU affinity 0-3, two graphics threads, Pod boundary sandbox. No image digest was supplied by the fingerprint, so none is asserted.

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.14/browser-focused-1.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 480 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/board-detail.spec.ts --project phone-360 --project tablet-1024 --project desktop-1440 --project wide-2560 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.14/pw-focused-1
```

Initial visual checks: exit 0, 12 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun. Screenshots were inspected. The screenshot test then changed to portable Playwright attachments, and the final run below checked that version.

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.14/browser-board-1.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 600 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/wide.spec.ts tests/board.spec.ts tests/tokens.spec.ts --project desktop --project desktop-safari --project desktop-firefox --project ipad --project pixel --project iphone --project tokens --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.14/pw-board-1
```

Supplemental interaction matrix: exit 0, 101 passed, 0 failed, 1 skipped, 0 flaky, 0 interrupted, 0 unrun. The existing isMobile skip at wide.spec.ts covers mouse-wheel behavior unsupported by mobile WebKit. Desktop Chromium, WebKit and Firefox executed it. The strict lane-progress parser correctly marks this report failed because a skipped test prevents its passed state. This is not presented as a skip-free milestone. No skip was added or broadened.

```sh
PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-e5u3.20.14/browser-final.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 240 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/board-detail.spec.ts --project phone-360 --project phone-412 --project tablet-768 --project tablet-1024 --project desktop-1440 --project desktop-1920 --project wide-2560 --workers=1 --reporter=list,json --output=/tmp/bd-e5u3.20.14/pw-final
```

Final required behavior checks: exit 0, 21 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun. Each browser run also executed the declared Node guard suite: 16 passed, 0 failed, 0 skipped. Playwright per-test cap 30s, assertion/action caps 5s, navigation cap 10s. Real local fixture servers served the built binary. No shared database was used. Unexpected page errors fail the board specs.

```sh
PUPPETEER_SKIP_DOWNLOAD=1 timeout 180s npx -y @mermaid-js/mermaid-cli -i /tmp/bd-e5u3.20.14/adr-0065.mmd -o /tmp/bd-e5u3.20.14/adr-0065.svg -p /tmp/bd-e5u3.20.14/puppeteer.json
```

ADR diagram render exited 0 using the image's Chromium. Input was extracted from the new ADR. No browser was downloaded.

## Evidence and limitations

Reports, exit files and runtime fingerprint are under /tmp/bd-e5u3.20.14. Key reports: preflight.txt, stable-2-summary.json, stable-2.txt, browser-final.json, browser-board-1.json, browser-fingerprint.json, visual-evidence.json, final-evidence.json. Playwright attachments in pw-final include compact/selected screenshots for all seven viewport classes. Two directly inspected desktop captures are compact-desktop-1440.png and selected-desktop-1440.png. Phone capture compact-phone-360.png was also inspected.

My first stable command used GOFLAGS=-json, which also changed go list output. It exited 2 and did not establish repository-suite results. Its output is retained, but its counts are not used. The corrected verbose invocation above passed.

Checkpoint comparison exited 2 because saved evidence was stale/incomplete after final documentation and screenshot changes. No cached evidence was reused. Fresh final browser checks passed. Checkpoint saves pushed only refs/heads/omnigent-checkpoints/bd-e5u3.20.14. Delivery HEAD and branch remain untouched by checkpoint commits.

Required blockers: none from the completed scoped checks. The supplemental matrix's declared iPad wheel skip and stable-suite environment skips remain explicit. Independent reviewer/tester gates are still pending. npm ci reported 2 low and 1 critical dependency audit findings from the unchanged committed lockfile. Dependencies were not changed within this presentation task. Selected long titles can wrap, and cross-column parent context requires details, as ADR 0065 decides.
