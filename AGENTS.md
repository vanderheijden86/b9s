> **PREREQUISITE**: Read and follow `~/.config/agents/AGENTS.md` first. It contains baseline instructions (beads workflow, commit strategy, progress reporting) that apply to ALL projects. The instructions below are project-specific and supplement those global rules.

# AGENTS.md — b9s

## RULE 0 - THE FUNDAMENTAL OVERRIDE PEROGATIVE

If I tell you to do something, even if it goes against what follows below, YOU MUST LISTEN TO ME. I AM IN CHARGE, NOT YOU.

---

## RULE 1 – ABSOLUTE (DO NOT EVER VIOLATE THIS)

You may NOT delete any file or directory unless I explicitly give the exact command **in this session**.

- This includes files you just created (tests, tmp files, scripts, etc.).
- You do not get to decide that something is "safe" to remove.
- If you think something should be removed, stop and ask. You must receive clear written approval **before** any deletion command is even proposed.

Treat "never delete files without permission" as a hard invariant.

---

### IRREVERSIBLE GIT & FILESYSTEM ACTIONS

Absolutely forbidden unless I give the **exact command and explicit approval** in the same message:

- `git reset --hard`
- `git clean -fd`
- `rm -rf`
- Any command that can delete or overwrite code/data

Rules:

1. If you are not 100% sure what a command will delete, do not propose or run it. Ask first.
2. Prefer safe tools: `git status`, `git diff`, copying to backups, etc.
3. After approval, restate the command verbatim, list what it will affect, and wait for confirmation.
4. When a destructive command is run, record in your response:
   - The exact user text authorizing it
   - The command run
   - When you ran it

If that audit trail is missing, then you must act as if the operation never happened.

---

## What b9s is

A Go terminal UI for Beads issues, modelled on k9s. It reads a Dolt server, SQLite or JSONL directly and makes every write by running the `bd` CLI. Read [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) before changing how data is read or written, and the [ADRs](docs/adr/) before changing behaviour they decide.

## Go toolchain

- Go **1.25** (`go.mod` is the source of truth; CI mirrors it).
- Build: `go build ./...`
- Test: `go test ./... -skip DoltIntegration` (add `-race` for the race detector)
- Vet: `go vet ./...` before committing
- Format: `gofmt -w .`
- Dependencies: `go mod tidy`; never edit `go.sum` by hand.

Key libraries: bubbletea (Elm-style TUI), lipgloss (styling), bubbles (components), huh (forms), glamour (Markdown). When unsure of an API, read the current docs rather than guessing.

## Code editing discipline

- No bulk-modifying scripts (codemods, invented one-off scripts, giant `sed` or regex refactors). Break large mechanical changes into explicit edits and review the diffs.
- We optimise for a clean architecture now, not backwards compatibility. No compat shims, no `v2` file clones. When changing behaviour, migrate callers and remove the old code in the same file.
- The bar for a new file is high. `pkg/ui` is split by concern (`tree.go`, `board.go`, `query_state.go`, `issue_writer.go`, …); add to the file that owns the concern.
- Comments carry the why, never the change history. See the baseline's "Code Comments" rule.

## Logging and output

- TUI output goes through lipgloss styling. Never mix raw prints with styled output.
- Debug logging goes through `pkg/debug`, enabled by `B9S_DEBUG`, or the `--debug` file log.
- Wrap errors with context: `fmt.Errorf("loading config: %w", err)`.

## Testing

Read [docs/testing.md](docs/testing.md). The rules that bite:

- **Never write to the shared Dolt server from a test.** Package-wide runs pass `-skip DoltIntegration`. Database-creating tests need `B9S_TEST_DOLT_SCRATCH_ADDR` pointing at a disposable loopback server that is not on port 3306.
- **Tests never open a browser or an editor.** Check `B9S_TEST_MODE` first in any code that would.
- **E2E tests** live in `tests/e2e`, run the built binary under `script`, and use `B9S_TUI_AUTOCLOSE_MS` to bound a run. The identity E2E test needs the scratch server as well.
- **Bound every wait.** A hung test is a bug to fix, not a timeout to raise.

### Test-driven development (mandatory)

1. **RED**: write a failing test and watch it fail for the right reason.
2. **GREEN**: write the minimal code to pass, then run the whole suite.
3. **REFACTOR**: clean up with the tests green.

No production code before a failing test. A bug fix includes a regression test that fails before the fix. One behaviour per test. Throwaway prototypes, generated code and configuration-only changes may skip TDD with explicit permission.

## Go practices

```go
// Wrap errors with context and check them immediately.
if err != nil {
    return fmt.Errorf("loading config: %w", err)
}

// Guard division and nil dereferences.
if len(items) > 0 {
    avg := total / float64(len(items))
}
if dep != nil && dep.Type.IsBlocking() {
    // safe to use dep
}

// Protect shared state, and capture channels before unlocking.
mu.RLock()
ch := someChannel
mu.RUnlock()
for item := range ch {
    // process
}
```

## Concurrent agents

Other agents work on this repository at the same time, and their uncommitted changes appear in your working tree. Never stash, revert, overwrite or otherwise disturb them. Treat them as changes you made yourself and carry on.

## Built-in TODO functionality

If I explicitly ask you to use your built-in TODO functionality, comply without objecting that you need to use beads.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
