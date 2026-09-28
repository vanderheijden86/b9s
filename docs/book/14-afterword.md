# Afterword {.unnumbered}

Some numbers, as of the day this was built.

The first commit is from 26 November 2025 and the repository has 1,427 commits since. There are 412 tracked files outside `vendor/`. The Go code outside tests is about 39,800 lines, of which `pkg/ui` is the largest package by a wide margin, `model.go` alone at 7,284 lines and `tree.go` at 3,810. The tests are about 42,400 lines in 153 files. The browser app is 2,684 lines of TypeScript and its specs 751. There are twenty-one decision records.

None of those numbers is the point. The point is the shape they describe: one model, one query, one write path, one place for each decision, and a set of tests that keep every "one" from quietly becoming two.

## Where to go next

If you want to change b9s, start with `docs/ARCHITECTURE.md` for the current map and `docs/adr/` for the reasons. Read `pkg/ui/model.go` from `Update` downward to see the key routing in code. Run `go test ./pkg/ui/ -v` to see the model tested without a screen, and `go test ./tests/e2e/ -v -timeout 300s` to see it tested with one.

If you want to change the browser app, `web/src/main.ts` is the boot, `render.ts` is the screen, and `web/tests/harness.ts` shows how a test gets its own server and its own fake `bd`. `make web` rebuilds the bundle, and the two guard tests will tell you if you forgot.

If you are building something else that reads a store it does not own, take three things from this book. Read directly and write through the owner's tool. Detect change with the cheapest signal the store offers, and route every change through one reload. And write the decision down before the code, in a file you promise never to edit.

## Thanks

b9s owes its shape to k9s, its rendering to Charm's Bubble Tea, lipgloss, bubbles, huh and glamour, its store to Dolt, and its reason to exist to Beads. The people who built those made this a small program rather than a large one.
