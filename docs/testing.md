# Testing b9s

b9s has three test layers. Unit tests cover packages, integration tests cover the Dolt reader against a real server, and end-to-end tests drive the built binary in a pseudo-terminal. This document says how to run each, what the rules are, and how to write a new test.

## Contents

- [Running the tests](#running-the-tests)
- [Layers](#layers)
- [Dolt rules](#dolt-rules)
- [End-to-end tests](#end-to-end-tests)
- [Browser tests](#browser-tests)
- [Writing tests](#writing-tests)
- [Continuous integration](#continuous-integration)

## Running the tests

```bash
go build ./...
go test ./... -skip DoltIntegration          # everything that needs no Dolt server
go test ./pkg/ui/ -run TestTreeView -v       # one package, one pattern
go test ./... -race -skip DoltIntegration    # with the race detector
go test ./tests/e2e/ -v -timeout 300s        # end-to-end, slow
go test ./... -short -skip DoltIntegration   # skips the slow cases
```

`make test` runs the default set. Every run must have a bounded timeout. A test that hangs is a bug in the test or the code, never a reason to raise the timeout.

## Layers

| Layer | Where | Needs |
|-------|-------|-------|
| Unit | `*_test.go` beside the code in `pkg/` and `internal/` | Nothing |
| Dolt integration | `internal/datasource/*_test.go`, names contain `DoltIntegration` | A Dolt server, see below |
| End-to-end | `tests/e2e/` | The `script` command and a terminal |
| Browser | `web/tests/*.spec.ts` | Node, `npx playwright install chromium webkit` |
| Shell | `tests/preview_contract_test.sh`, `tests/mobile_*_e2e.sh` | A deployed preview, see [preview-contract.md](preview-contract.md) |

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

## Browser tests

`make web-e2e` (or `npm --prefix web run test:e2e`) runs the web UI in Playwright, as a Pixel 7 in Chromium and an iPhone 14 in WebKit. `wide.spec.ts` runs only in two wide projects, a 1440 x 900 desktop Chrome and an iPad Pro 11 in WebKit, and the phone projects skip it. Global setup builds the bundle and `.b9s-e2e/b9s` from the working tree. Set `B9S_WEB_BIN` to test another binary.

- Each test gets its own temp folder with `.beads/issues.jsonl`, an isolated `XDG_CONFIG_HOME`, and its own `b9s web --no-token` on a free loopback port (`web/tests/harness.ts`). No test reaches a Beads database.
- `web/tests/fake-bd.mjs` stands in for `bd` on the `PATH`. It applies `update`, `close`, `delete`, `defer`, `comments add` and `create` to the JSONL, and logs each call to `.beads/bd-calls.log`, so a test checks both the command and its effect. `FAKE_BD_FAIL=<command>` makes that command fail.
- Gestures use real pointer events through `page.mouse`, so swipes, long-presses and drags run the same handlers as a finger.
- `perf.spec.ts` loads 1000 issues and checks load, a full re-render and a swipe deep in the list. Emulated phones on a laptop are faster than real ones, so this catches a render path that went quadratic, not a slow handset.
- A failed test attaches the `bd` call log and the server output.

## Writing tests

b9s follows test-driven development: write the failing test first, make it pass, then clean up. A bug fix includes a regression test that fails before the fix.

- **Use real data, not mocks.** Build issues with `pkg/testutil` generators or small literals, and run the real code. The generators are deterministic, so a failure reproduces.
- **One behaviour per test.** A name with "and" in it is two tests.
- **Table-driven tests** for several inputs of one function, with `t.Run` per case.
- **Golden files** through `testutil.NewGoldenFile`. Set `GENERATE_GOLDEN=1` to rewrite them, and review the diff before committing.
- **Bound every wait.** A poll loop has a deadline, and a test that waits on a channel uses a timeout.
- **Never open a browser or an editor.** Code that would do so checks `B9S_TEST_MODE` first.
- **Assert the decided behaviour.** When the code and an [ADR](adr/) disagree, the ADR wins, and the difference is a bug to report, not an expectation to loosen.

## Continuous integration

`.github/workflows/ci.yml` builds the binary, runs the unit tests for `./pkg/...` and `./cmd/b9s` with coverage, and runs the end-to-end tests with a ten-minute timeout. It has no Dolt server, so the integration tests are not run there. A change to the Go version in `go.mod` must be mirrored in that workflow.
