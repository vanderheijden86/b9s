# bd-2z6r implementation report

Implementer: Codex. Branch: `omnigent/bd-2z6r`. Required implementer checks passed. Independent review and tester execution remain Polly's next roles. No delivery commits, delivery pushes, PRs or GitHub comments were made.

## Git preflight

- Repository: `/home/omnigent/workspace/b9s`, assigned base and PR target `main`.
- Initial checkout: clean `main`, HEAD `eb77cef9f8e68e9854f3c1b6ec6a6074b812830a`.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0. Fetched `origin/main` is `eb77cef9f8e68e9854f3c1b6ec6a6074b812830a`.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-2z6r`: exit 2, remote delivery ref absent. Local delivery branch was absent.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-2z6r origin/main`: exit 0.
- Actual branch: `omnigent/bd-2z6r`. HEAD remains `eb77cef9f8e68e9854f3c1b6ec6a6074b812830a`.
- `git -C /home/omnigent/workspace/b9s merge-base --is-ancestor origin/main HEAD`: exit 0.
- Required commits: none. Individual prerequisite ancestry against base and HEAD: not applicable.
- Checkpoint inspect: exit 2, absent. No source edits preceded the preflight report.
- Initial evidence: `/tmp/bd-2z6r/preflight.txt`.

## Behavior and changed files

- `web/src/state.ts`: effective list layout is the saved preference OR a nonempty positive/negated `type:` predicate. Flat rows use the active sort across all matching issues and exclude parent context.
- `web/src/actions.ts`: restore browser layout preference on first snapshot and project changes, keyed by the API's project key. `t` changes the saved choice while a type predicate continues to force the effective list.
- `web/src/render.ts`: navigation label and row rendering use the effective layout.
- `web/src/keys.ts`: folding keys preserve tree fold state while the list is visible.
- `web/tests/read.spec.ts`: type forcing, negation, cross-parent Updated sorting, query removal, saved list choice, per-project persistence, and four responsive screenshots.
- `web/tests/keys.spec.ts`: folding preservation and `t` behavior under forced list layout.
- `README.md`: browser keyboard table including `t`.
- `docs/USER_GUIDE.md`: browser tree/list preference and type-forcing behavior, plus `t` in the keyboard table.
- `pkg/web/dist/app.js`, `pkg/web/dist/source.sha256`: rebuilt embedded bundle.

No new source files. No Go source, dependency lockfile, database read/write path or active ADR changed. Regression tests guard this behavior. The shared effective-layout function keeps row generation, rendering and fold keys aligned.

## Dependencies and non-browser checks

All commands below used the shared repository and completed under their bounds.

| Exact command | Result | Report |
|---|---|---|
| `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` | exit 0, 122 packages installed | `npm-ci.txt` |
| `timeout 180s go -C /home/omnigent/workspace/b9s mod download` | exit 0 | `go-mod-download.txt` |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck` | exit 0 | `typecheck.txt` |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build` | exit 0 | `web-build.txt` |
| `timeout 240s go -C /home/omnigent/workspace/b9s build ./...` | exit 0 | `go-build.txt` |
| `GOFLAGS=-v timeout 660s make -C /home/omnigent/workspace/b9s test-stable` | exit 0, 3769 passed / 0 failed / 20 skipped, including subtests, 22 packages passed | `stable.txt`, `stable-summary.json`, `stable-details.json` |
| `timeout 240s go -C /home/omnigent/workspace/b9s vet ./...` | exit 0 | `go-vet.txt` |
| `git -C /home/omnigent/workspace/b9s diff --check` | exit 0 | `diff-check.txt` |

Reports in this table are under `/tmp/bd-2z6r/`. Non-test commands have no test counts. The stable suite ran alone, without a base comparison. Top-level Go results: 2125 passed / 0 failed / 18 skipped. Subtests account for the remaining results. The 20 skipped results are optional: one stock/preview-bd probe, three S3 tests, seven Dolt-DSN tests, four memory/research opt-ins, two private-fixture subtests and three stress opt-ins. No shared Dolt writes occurred.

## Browser environment and red-green proof

- `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-2z6r/browser-smoke.json --seconds 90`: 3 passed / 0 failed / 0 skipped.
- `lane-browser fingerprint --project /home/omnigent/workspace/b9s/web`: `/tmp/bd-2z6r/browser-environment.json`.
- Runtime: Playwright 1.63.0, Node v22.23.4, x86_64 Linux 6.8.0-149-generic, four CPUs, native runtime hash `a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6`. Image digest was not exposed in environment metadata.
- Project test timeout: 30 seconds. Expect/action timeout: 5 seconds. Navigation timeout: 10 seconds. Retries: zero.

RED and GREEN used this exact command from `/home/omnigent/workspace/b9s/web`, with REPORT set to `/tmp/bd-2z6r/red.json` and `/tmp/bd-2z6r/green.json` respectively:

```sh
B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=REPORT lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 300 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/read.spec.ts tests/keys.spec.ts --project desktop -g 'flat list:' --workers=1 --reporter=list,json
```

- RED, before production edits: actual process exit 1, 1 passed / 5 failed / 0 skipped / 0 flaky / 0 interrupted / 0 unrun. Failures reproduced parent injection, negated type layout, forced-layout toggle behavior, preference leakage and fold-state mutation.
- GREEN: actual process exit 0, 6 passed / 0 failed / 0 skipped / 0 flaky / 0 interrupted / 0 unrun. The sorting fixture was strengthened to include a newer task under a second epic.
- Reports: `red.json`, `red.txt`, `red.exit`, `green.json`, `green.txt`, `green.exit`, `green-evidence.json` under `/tmp/bd-2z6r/`.
- Each npm invocation also ran API tests: 12 passed / 0 failed / 0 skipped, and test guards: 16 passed / 0 failed / 0 skipped, with no cancellations.

## Expanded browser check

Exact command, from `/home/omnigent/workspace/b9s/web`:

```sh
B9S_KEEP_TEST_FIXTURES=1 PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-2z6r/browser.json lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 900 -- npm --prefix /home/omnigent/workspace/b9s/web run test:e2e -- tests/read.spec.ts tests/keys.spec.ts --project pixel --project iphone --project desktop --project ipad --workers=1 --reporter=list,json --output=/tmp/bd-2z6r/browser-artifacts
```

Actual process exit 0: 179 passed / 0 failed / 0 skipped / 0 flaky / 0 interrupted / 0 unrun. The JSON reporter completed with no global errors. API tests passed 12/12 and test guards passed 16/16. Browser fixtures fail on unexpected page errors. Type-chip checks also assert no console errors. These checks passed.

The suite runs real Go servers against disposable JSONL projects and a fake write CLI. It checks query filtering through the real API, keyboard behavior, navigation, details and persistence. Screenshots at 390, 820, 1440 and 2560 pixels show matching task rows without parent context, indentation or chevrons. Phone, tablet and desktop screenshots were read by the implementer.

Reports: `/tmp/bd-2z6r/browser.json`, `browser.txt`, `browser.exit`, `browser-evidence.json`. Artifacts: `/tmp/bd-2z6r/browser-artifacts/`.

## Residual risks and handoff

- Required blockers: none. Required external prerequisites: none.
- Layout preferences are browser-local and per project, separate from terminal `.beads/tree-state.json`. The previous global browser list preference is not migrated.
- Unrelated observation: `npm ci` reported two low-severity vulnerabilities in the existing lockfile. No dependency changes were made or proposed in this task.
- No unresolved product failure or filed follow-up issue.
- The independent reviewer and tester must still assess and execute their checks. Delivery needs Polly's approval.

## Checkpoint evidence

Separate unapproved checkpoint pushes succeeded to `refs/heads/omnigent-checkpoints/bd-2z6r`. They did not change the delivery HEAD or index. `lane-checkpoint compare --repo /home/omnigent/workspace/b9s --task bd-2z6r --base origin/main --evidence /tmp/bd-2z6r/green-evidence.json` exited 0 and reported compatible=true, complete=true, approved=false. This identifies the saved focused GREEN run and remains advisory. All required checks reported above were executed in this attempt.
