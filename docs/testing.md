# Testing b9s

b9s has three test layers. Unit tests cover packages, integration tests cover the Dolt reader against a real server, and end-to-end tests drive the built binary in a pseudo-terminal. This document says how to run each, what the rules are, and how to write a new test.

## Contents

- [Running the tests](#running-the-tests)
- [Layers](#layers)
- [Dolt rules](#dolt-rules)
- [End-to-end tests](#end-to-end-tests)
- [Memory preview terminal checks](#memory-preview-terminal-checks)
- [Browser tests](#browser-tests)
- [bd releases](#bd-releases)
- [Writing tests](#writing-tests)
- [Continuous integration](#continuous-integration)

## Running the tests

```bash
go build ./...
go test ./... -skip DoltIntegration          # everything that needs no Dolt server
go test ./pkg/ui/ -run TestTreeView -v       # one package, one pattern
go test ./... -race -skip DoltIntegration    # with the race detector
go test ./tests/e2e/ -v -timeout 1200s       # end-to-end, slow
go test ./... -short -skip DoltIntegration   # skips the slow cases
```

`make test` runs the default set. Every run must have a bounded timeout. A test that hangs is a bug in the test or the code, never a reason to raise the timeout.

The E2E suite runs its PTY tests one after another and takes about seven minutes, or up to eleven on a loaded machine. The 1200s limit covers the whole suite, not one test: each PTY run has its own deadline of 10 to 15 seconds, and a run that reaches it fails, so a hang still shows within seconds. Count the results of a verbose run with `grep -c '^--- SKIP' e2e.log` (and `PASS`, `FAIL`): a skip that no missing server or tool explains needs a look.

## Layers

| Layer | Where | Needs |
|-------|-------|-------|
| Unit | `*_test.go` beside the code in `pkg/` and `internal/` | Nothing |
| Dolt integration | `internal/datasource/*_test.go`, names contain `DoltIntegration` | A Dolt server, see below |
| End-to-end | `tests/e2e/` | The `script` command and a terminal |
| bd contract | `pkg/web/bd_contract_test.go` | The `bd` command on `PATH` |
| Browser | `web/tests/*.spec.ts` | Node, `npx playwright install chromium webkit firefox` |
| Shell | `tests/preview_contract_test.sh` | A deployed preview, see [preview-contract.md](preview-contract.md) |

Unit tests in `pkg/ui` build a `Model` with fixture issues and send it key messages. Accessors such as `TreeSelectedID()` and `TreeNodeCount()` in `pkg/ui/model.go` expose the state a test needs without rendering.

## Dolt rules

**Never write to the shared Dolt server from a test.** It holds every project's Beads data. Package-wide runs pass `-skip DoltIntegration`, and CI does not run those tests at all.

Read-only live tests connect through the tunnel, list databases and open projects. Name them explicitly:

```bash
B9S_TEST_DOLT_HOST=127.0.0.1 B9S_TEST_DOLT_USER=bd_b9s \
B9S_TEST_DOLT_CATALOG_DB=b9s B9S_TEST_DOLT_OTHER_DB=LP_Team \
  go test ./internal/datasource/ -run 'DoltIntegration_(ListProjectDatabases|OpenProject)' -v
```

Tests that create, fill and drop databases run only against a disposable local server named in `B9S_TEST_DOLT_SCRATCH_ADDR`, and skip without it. The address must be loopback and must not use port 3306, which is the tunnel to the shared server. `TestDatabaseCreatingTestsRequireScratchServer` fails any test that creates a database without that check.

```bash
mkdir -p /tmp/b9s-scratch-dolt && (cd /tmp/b9s-scratch-dolt && dolt init && dolt sql-server --port 13306) &
B9S_TEST_DOLT_SCRATCH_ADDR=127.0.0.1:13306 go test ./internal/datasource/ -run DoltIntegration -v
B9S_TEST_DOLT_SCRATCH_ADDR=127.0.0.1:13306 go test ./tests/e2e/ -run AssigneePickerMergesAliases -v
```

The second command runs the E2E test for identity aliases in `tests/e2e/identity_dolt_e2e_test.go`, which also skips without the scratch server.

| Variable | Meaning |
|----------|---------|
| `B9S_TEST_DOLT_HOST`, `B9S_TEST_DOLT_PORT`, `B9S_TEST_DOLT_USER` | The shared server, read-only tests |
| `B9S_TEST_DOLT_CATALOG_DB`, `B9S_TEST_DOLT_OTHER_DB` | Two databases the user can read |
| `B9S_TEST_DOLT_SCRATCH_ADDR`, `B9S_TEST_DOLT_SCRATCH_USER` | The disposable server for tests that create databases |
| `BEADS_DOLT_PASSWORD` | The password for both |

## End-to-end tests

The tests in `tests/e2e` build the binary once, then run it under the Unix `script` command so it sees a real terminal. Keys go in through a pipe on stdin, and the screen comes back as the captured output.

- `TestMain` builds the binary and sets `B9S_TEST_MODE`, `B9S_NO_BROWSER` and an isolated `XDG_CONFIG_HOME`, so a test never reads or writes your own configuration.
- `B9S_TUI_AUTOCLOSE_MS` makes the program exit by itself after a delay, so a test that sends no quit key still ends.
- `sizedScriptTUICommand` fixes the terminal size, so the rendered screen is deterministic.
- `runCmdToFile` bounds the run with a context and fails on timeout rather than hanging.
- Arrow keys are sent as escape sequences: `\x1b[A` up, `\x1b[B` down, `\x1b[C` right, `\x1b[D` left.
- `skipIfNoScript` skips on systems without a usable `script`, and `testing.Short()` skips the slow cases.

The fixtures live in `tests/testdata`. `minimal.jsonl` and `synthetic_complex.jsonl` are the common ones. A test that needs a Dolt server uses the same rules as the integration tests.

The mobile and preview shell scripts in `tests/` are not Go tests. They check a deployed preview through the URL a reviewer uses and take the base URL, task ID and commit SHA as arguments, so a run against the wrong build fails instead of passing on a sibling.

## Memory preview terminal checks

`memory_preview_e2e_test.go` creates fresh embedded graph workspaces with the
explicit preview binary. It checks body updates, retained versions, Link
ownership and direction, literal search, ordinary Issues and terminal version
selection. Every subprocess has a deadline and receives isolated Beads settings.

```bash
B9S_TEST_MEMORY_BD="$HOME/Documents/b9s-memory-poc/.memory-preview/bin/bd" \
  go test ./tests/e2e -run '^TestMemoryPreview' -count=1 -timeout=180s
B9S_MEMORY_PREVIEW_WORKSPACE="$HOME/Documents/b9s-memory-poc" \
  go test ./internal/datasource -run '^TestMemoryPreviewGraphReadTiming$' -count=1 -timeout=190s -v
```

The Memory capability probe in `internal/bdrun` runs against fake bd scripts.
To check it against real builds, name a released bd and a preview bd. The probe
reads only help output, from an empty directory, so it never opens a database:

```bash
B9S_TEST_STOCK_BD="$(command -v bd)" \
B9S_TEST_PREVIEW_BD="$HOME/Documents/b9s-memory-poc/.memory-preview/bin/bd" \
  go test ./internal/bdrun -run '^TestProbeMemory_RealBinaries$' -count=1 -v
```

`memory_gating_e2e_test.go` runs the same built b9s three times against an
embedded graph workspace with a fake bd on a private `PATH` and a private probe
cache: a released-like bd (`M` explains, no MEMORY LINKS column), a
preview-like bd (`M` opens the Memory) and the preview with `B9S_MEMORY=off`
(Issues load, Memory hidden). It needs no real bd:

```bash
go test ./tests/e2e -run '^TestMemory(Key|Lever)' -count=1 -timeout=120s
```

The timing test reads an existing embedded preview without writes. It compares
the optimized graph with an unpruned traversal, then requires startup below
11 seconds. Run it without other processes reading that embedded workspace.

`tests/e2e/memory_preview.py` reads an existing embedded Memory preview through its pinned `bd`, in a real `script` terminal. It checks the list, focus wires, context expansion and collapse, removed mode keys, switching columns, opening a Memory and quitting. It never creates or changes Beads. The workspace must contain `.memory-preview/bin/bd` and ready graph metadata. Unit tests additionally cover distant endpoints, paging when relevant rows alone overflow, reverse arrows and narrow terminals.

```bash
python3 -m venv /tmp/b9s-memory-checks
/tmp/b9s-memory-checks/bin/pip install pyte
go build -o /tmp/b9s-memory-checks/b9s ./cmd/b9s
/tmp/b9s-memory-checks/bin/python tests/e2e/memory_preview.py \
  --binary /tmp/b9s-memory-checks/b9s \
  --workspace "$HOME/Documents/b9s-memory-poc" \
  --evidence /tmp/b9s-memory-checks/evidence
```

Each screen has a deadline, and the TUI has a three-minute exit timer. The test terminates only its own process group on failure. Evidence includes terminal bytes, screen text, and SVG captures. The final line is `=== MEMORY_PREVIEW DONE pass=N fail=M ===`. To compare a prior build, run the same command with its binary and a separate evidence directory.

Add `--integrated` to start regular b9s and test `M` wires entry, rejected graph-view keys,
`A/O/C` status filters, literal search, `\` detail expansion, and return to Issues.
The integrated run also checks that wires reach Issue labels when selecting from
either column. Use `--columns 300` to expose gaps beside short labels in a wide
terminal. Rendering tests cover reverse arrows, Unicode labels and truncated titles.

The opt-in Memory transport probes, isolated-copy preparation requirements and
recorded measurements are in [memory-read-research.md](memory-read-research.md).
`TestMemoryReadResearchStages` and `TestMemoryReadResearchTransportParity` read
prepared copies. `TestMemoryReadResearchConcurrentWriter` additionally requires
`B9S_MEMORY_RESEARCH_MUTATE=1` and adds probe data only to the checked local copy.

## Browser tests

`web/tests/memory.spec.ts` covers the Memory constellation across Chromium, WebKit and Firefox, with bounded browser operations and local fixture projects. It checks shapes, directed Links, selection, neighbourhood highlighting, the authoritative problem filter, graph caching, errors and responsive layout.

For the real embedded preview, start the server through the isolated launcher, then run the read-only probe in another terminal:

```bash
scripts/memory-preview b9s tui web --no-token --listen 127.0.0.1:58431
node web/tests/memory-preview.mjs http://127.0.0.1:58431/ /tmp/b9s-memory-web-evidence
```

The probe refuses non-loopback URLs, compares visible nodes and Links with the cached endpoint, opens a Memory, checks the problem filter and saves desktop and phone screenshots. It ends with `=== MEMORY_WEB_PREVIEW DONE pass=N fail=M ===` and a nonzero exit on failure.

`make web-e2e` (or `npm --prefix web run test:e2e`) runs the web UI in Playwright, as a Pixel 7 in Chromium and an iPhone 14 in WebKit. `wide.spec.ts` and `keys.spec.ts` run only in the wide projects: 1440 x 900 desktop Chrome, Safari (WebKit) and Firefox, and an iPad Pro 11 in WebKit. The phone projects skip them. The three desktop projects also run `read.spec.ts` and `write.spec.ts`, so the tree, search, the detail panel and every write are checked in each desktop browser. `shell.spec.ts` runs in one project per size class of ADR 0052 (`phone-360`, `phone-412`, `tablet-768`, `tablet-1024`, `desktop-1440`, `desktop-1920` and `wide-2560`): it checks the class on `#app`, the bottom bar or rail, the sheet or side panel, the tablet's split of list pane and detail in both orientations with its keys, and that a rotation or a resize keeps the selection, the scroll and the open detail. A later spec joins those projects by adding itself to their `testMatch`. `desktop.spec.ts` runs in the three desktop browsers and in `desktop-1920` and `wide-2560`: the sidebar, the detail pane beside the views, hover, the context menu with its keys, the sheet as a dialog, and the cursor row and focus ring. Global setup builds the bundle and `.b9s-e2e/b9s` from the working tree. Set `B9S_WEB_BIN` to test another binary. Every project runs with `reducedMotion: "reduce"`, which drops the app's transitions: under load WebKit reports a sliding sheet as stable, and a tap then lands where its button was (bd-foit.18). A pull request runs `make web-gate`, the `tokens`, `pixel`, `iphone`, `desktop` and `ipad` projects, in the `web-e2e` job; the rest of the matrix runs nightly ([Continuous integration](#continuous-integration)).

- Each test gets its own temp folder with `.beads/issues.jsonl`, an isolated `XDG_CONFIG_HOME`, and its own `b9s web --no-token` on a free loopback port (`web/tests/harness.ts`). No test reaches a Beads database.
- `web/tests/fake-bd.mjs` stands in for `bd` on the `PATH`. It applies `update`, `close`, `delete`, `defer`, `comments add` and `create` to the JSONL, and logs each call to `.beads/bd-calls.log`, so a test checks both the command and its effect. `FAKE_BD_FAIL=<command>` makes that command fail.
- Gestures use real pointer events through `page.mouse`, so swipes, long-presses and drags run the same handlers as a finger.
- `perf.spec.ts` loads 1000 issues and checks load, a full re-render and a swipe deep in the list. Emulated phones on a laptop are faster than real ones, so this catches a render path that went quadratic, not a slow handset. It runs in the `pixel-perf`, `iphone-perf`, `tablet-perf` and `desktop-perf` projects, which start once every other project has finished, so the timing does not share the CPU with other workers. A failure elsewhere therefore skips it. A budget is a claim about the app alone, so a pull request does not run these projects: the nightly run times them on one worker (`--workers 1`), Chromium against the budget with the CI allowance and WebKit recorded in the annotations without a verdict (ADR 0059, bd-zoom.1.6). A budget that fails nightly is a Beads bug, never a wider budget.
- `open(page, project)` and `settled(page)` in `harness.ts` wait for the first render and the live stream through one `page.waitForFunction` over two scalars (`#boot` gone, `body[data-live="live"]`). Call `settled` after every `page.goto` and `page.reload`. A locator assertion for the same thing snapshots the ARIA tree on each poll that misses, which on a loaded runner costs the app the CPU its own timers need (bd-zoom.1.4).
- Read the page through an assertion that retries: `expect(locator).toHaveAttribute(...)`, `toHaveCount(...)`, `toHaveText(...)`, or `await expect.poll(() => ...)` for a value the matchers cannot reach, such as the order of `data-card` ids. `expect(await locator.evaluateAll(...)).toEqual(...)` reads once, and on a slow runner once is before the rows have arrived. `TestWebSpecsReadThePageThroughRetryingAssertions` in `pkg/web` fails on that shape (bd-zoom.1.3).
- A failed test attaches the `bd` call log and the server output.
- Tests that trigger a document reload use `reloadAndSettle(page, trigger)` from `web/tests/harness.ts`. It arms a bounded load waiter before the trigger, then waits for the initial snapshot and live stream. Read computed styles after it returns. `npm --prefix web run test:guards` parses `web/tests` with Playwright's TypeScript parser and rejects direct `page.evaluate` calls inside `expect.poll`, which can lose their execution context during navigation. `test:e2e` runs this guard before Playwright. Its fixtures cover nested and split assertions, template interpolation, regex literals and safe outside evaluations.
- A test that fails on CI and passes here may race the app's own timers, which a slower CI runner reaches between two of the test's steps. `B9S_E2E_CPU_THROTTLE=<n>` runs every Chromium page `n` times slower (CDP `Emulation.setCPUThrottlingRate`), so the failure shows before anything changes: `B9S_E2E_CPU_THROTTLE=10 npm --prefix web run test:e2e -- tests/wide.spec.ts -g '<title>' --project desktop --repeat-each 20 --workers 4`. On an Apple M4 Max, 4 reproduced nothing and 8 to 12 matched the GitHub runner of b9m-vdp.4.16. Fix the race, never with a retry or a longer timeout.

## bd releases

b9s depends on the `bd` command line: the flags it passes for each write, the `Created issue:` line it reads the new ID from, and the JSONL that `bd export` prints for an embedded project. `TestBDContract_EveryWriteB9sMakes` runs each write the TUI and the browser make through the `bd` first on `PATH`, in a new embedded project in a temporary directory, and reads the result back the way b9s does. It covers every flag in the browser's allowlists (`updateFields` and `createFields` in `pkg/web/write.go`), so a flag added there fails the test until the test sends it too.

The test blanks every `BEADS_*` and `BD_*` variable of your shell and refuses to write unless `bd init` made an embedded project, so it cannot reach a Dolt server. It skips when `bd` is missing or with `-short`.

To check a bd release, put its binary first on `PATH`:

```bash
gh release download v1.3.0 -R steveyegge/beads -p '*darwin_arm64*.tar.gz' -D /tmp/bd-v1.3.0
tar -xzf /tmp/bd-v1.3.0/*.tar.gz -C /tmp/bd-v1.3.0
PATH=/tmp/bd-v1.3.0:$PATH go test ./pkg/web -run BDContract -count=1 -v
```

Change the version in the README's install section when a release passes or fails. bd 1.2.2 and 1.3.0 passed on 2026-09-28. bd prints a warning about duplicate binaries on `PATH`, which is harmless here.

## Writing tests

Bug fixes and pure logic follow test-driven development: write the failing test first, make it pass, then clean up. A bug fix includes a regression test that fails before the fix. Visual and layout work, spikes and refactors that existing tests cover may add tests afterwards, after a check by eye; `AGENTS.md` has the full rule.

- **Use real data, not mocks.** Build issues with `pkg/testutil` generators or small literals, and run the real code. The generators are deterministic, so a failure reproduces.
- **One behaviour per test.** A name with "and" in it is two tests.
- **Table-driven tests** for several inputs of one function, with `t.Run` per case.
- **Golden files** through `testutil.NewGoldenFile`. Set `GENERATE_GOLDEN=1` to rewrite them, and review the diff before committing.
- **Bound every wait.** A poll loop has a deadline, and a test that waits on a channel uses a timeout.
- **Never open a browser or an editor.** Code that would do so checks `B9S_TEST_MODE` first.
- **Assert the decided behaviour.** When the code and an [ADR](adr/) disagree, the ADR wins, and the difference is a bug to report, not an expectation to loosen.

## Continuous integration

Two workflows share the tests ([ADR 0059](adr/0059-gate-pull-requests-on-a-subset-and-run-the-matrix-nightly.md)): a pull request is gated on the tests that fail for a wrong change, and the rest of the matrix runs every night.

`.github/workflows/ci.yml` runs on every push to `main` and every pull request: it builds the binary, runs the unit tests for `./pkg/...` and `./cmd/b9s` with the coverage threshold, runs the terminal end-to-end tests with a twenty-minute timeout, and runs `make web-gate` (the `tokens`, `pixel`, `iphone`, `desktop` and `ipad` projects) in its own `web-e2e` job, which uploads the Playwright traces when a test fails. It has no Dolt server, so the integration tests are not run there. A change to the Go version in `go.mod` must be mirrored in both workflows.

`.github/workflows/nightly.yml` runs at 03:00 UTC and on demand (`gh workflow run nightly.yml`), one job each:

- `web-matrix`: every web project except the perf ones and `measure`, so Firefox, Safari and the seven size classes.
- `web-perf`: the four perf projects on one worker, nothing beside them.
- `web-measure`: `measure.spec.ts`, with its report as the `measure-report` artifact.
- `web-flakes`: the gate's projects with `--repeat-each 5` on eight workers. `scripts/playwright-flakes.mjs` reads the JSON report and fails the job naming every test that both passed and failed, and every test that failed in each repeat.
- `go-race`: `go test -race -skip DoltIntegration -p 2` over every package but `tests/e2e`.
- `go-e2e`: the terminal end-to-end tests again, on the nightly schedule.

Both workflows keep `retries: 0`; `TestPlaywrightRetriesStayAtZero` in `pkg/web` fails when the config or a workflow adds a retry, and `TestEveryPlaywrightProjectRunsInAWorkflow` fails when a project is in the config and in neither workflow. A nightly failure is a bug to file, never a retry: `scripts/nightly-to-beads.sh <run-id>` prints a `bd create` with the failing jobs, their tests, the run URL and the artifacts, for the reader to check and run. `scripts/ci-flake-report.sh` lists the failed runs of a branch with the failing tests and whether the commit touched the file they live in, which is the measurement behind the ADR: a failure in a file the commit did not touch is a flake candidate.
