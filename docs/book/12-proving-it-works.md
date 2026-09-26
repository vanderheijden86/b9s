# Proving it works

b9s has roughly as many lines of tests as of code: about 42,000 against 40,000, in 153 Go test files plus the Playwright specs. That ratio is not a target anyone set. It is what test-driven development produces when it is the rule rather than the aspiration, and in this repository it is the rule: the failing test first, the smallest code that passes it, then the cleanup. A bug fix includes a test that fails before the fix.

The tests sit in four layers, and each layer has a rule that keeps it honest.

## Unit: a model without a screen

Most tests live beside the code in `pkg/` and `internal/`. The UI ones build a `Model` from fixture issues, send it key messages and read accessors. They never render. When a test needs to know something about the state, the rule is to add an accessor such as `TreeSelectedID()` rather than to parse the rendered string, because a string assertion breaks on every change of styling and an accessor breaks only when the behaviour does.

Fixtures come from `pkg/testutil`, whose generators are deterministic: the same seed gives the same issues, so a failure reproduces. Golden files, through `testutil.NewGoldenFile`, cover the few places where the rendered output is the thing under test, and `GENERATE_GOLDEN=1` rewrites them for review in a diff.

## Integration: the rule about the shared server

The Dolt reader has to be tested against a real Dolt server, and the one every project on the machine points at is the shared server that holds all of them. **Never write to the shared Dolt server from a test.** Package-wide runs pass `-skip DoltIntegration`, and CI has no server, so it does not run those tests at all.

Read-only tests through the tunnel run when named explicitly with `B9S_TEST_DOLT_HOST` and a catalog database to read. Tests that create, fill and drop databases run only against a disposable local server named in `B9S_TEST_DOLT_SCRATCH_ADDR`, which must be loopback and must not be port 3306. A test that creates a database without checking for that variable fails `TestDatabaseCreatingTestsRequireScratchServer`, which reads the test source and looks for the check. The rule is enforced by a test, not by a comment.

## End to end: a real terminal

`tests/e2e` builds the binary once, then runs it under the Unix `script` command so it sees a real pseudo-terminal, sends keys through stdin and reads the captured screen. `B9S_TEST_MODE` and `B9S_NO_BROWSER` stop it opening anything, an isolated `XDG_CONFIG_HOME` keeps it away from the developer's config, and `sizedScriptTUICommand` fixes the terminal size so the screen is deterministic.

The rule here is about waiting. `B9S_TUI_AUTOCLOSE_MS` makes the program exit by itself after a delay, so a test that sends no quit key still ends. `runCmdToFile` bounds the run with a context and fails on timeout. A test that hangs is a bug, in the test or in the code, and never a reason to raise the timeout.

## Browser: a fake bd and a real server

The Playwright suite in `web/tests` runs on four emulated devices: a Pixel 7 in Chromium, an iPhone 14 in WebKit, a 1440 by 900 desktop Chrome and an iPad Pro in WebKit. The phone projects skip `wide.spec.ts` and the wide projects run only it.

Each test gets its own temp folder with an `issues.jsonl` fixture, its own isolated config, and its own real `b9s web --no-token` on a free loopback port. No test reaches a database. On the `PATH` sits `fake-bd.mjs`, a stand-in for `bd` that applies `update`, `close`, `delete`, `defer`, `comments add` and `create` to the JSONL and logs every call. A test asserts both the command b9s ran and the effect it had, and `FAKE_BD_FAIL=close` makes one command fail so the 422 path is covered too.

Gestures use real pointer events through `page.mouse`, so a swipe test runs the same handlers a finger does. A failed test attaches the `bd` call log and the server output.

## Two tests that guard the build

Two tests in `pkg/web` are worth naming because they test the repository rather than the program. `TestGeneratedTypesAreCurrent` fails when `api.gen.ts` no longer matches `types.go`. `TestEmbeddedBundleIsCurrent` fails when `web/src` changed and the committed bundle did not. Both turn a "remember to run this" into a red test, which is the only kind of reminder that works.

## Which behaviour to assert

The last rule is the one that decides arguments. When the code and a decision record disagree, the record wins, and the difference is a bug to report, not an expectation to loosen. A test written to match what the code happens to do today is a test that will defend the bug. A test written to match the decision is one that finds it.

That rule only works if the decisions are written down somewhere a test author will read them. The next chapter is about where.
