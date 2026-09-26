# Preface {.unnumbered}

This is a small book about a small program.

b9s is a terminal user interface for Beads, the issue tracker that lives in a `.beads` folder next to your code. If you have used k9s to look at a Kubernetes cluster, b9s will feel familiar within a minute: a tree of issues instead of a list of pods, a `:` prompt instead of a `:` prompt, number keys that jump between projects instead of namespaces.

The program is about forty thousand lines of Go, a few thousand lines of TypeScript for the browser version, and roughly as many lines of tests as of code. That is not a large codebase. It is, however, a codebase with a lot of decisions in it, and most of those decisions are small enough to be invisible until you trip over them. Why does b9s never write to the database it reads? Why does the password from your environment refuse to travel to a host named in a cloned repository? Why does a project switch keep the old project on screen for up to ten seconds? Why does the browser app not know it has a history?

This book answers those questions. It is written for three readers:

- **The contributor** who wants to change b9s and would rather understand the shape of the thing before opening `model.go`.
- **The curious user** who wonders what happens between pressing `K` and seeing the issue vanish.
- **The designer of some other tool** who is thinking about the same problems: reading a store you do not own, keeping a UI live, putting a keyboard-first program in front of a phone.

It is not a reference manual. The README lists every key, `docs/ARCHITECTURE.md` lists every package, and the twenty-one decision records in `docs/adr/` hold the arguments in full. This book is the narrative that connects them. Where it says a thing, the code says it too, and where the two disagree, the code is right and the book needs a fix.

## How to read it

The chapters follow the data. The first chapters cover how b9s finds and reads a project, and how it notices when the project changes. The middle chapters cover what happens inside the terminal: one model, one query, one write path, and the small state machines that keep switching projects safe. Then the board, the browser, and the tests. The last chapter is about the practice of writing decisions down, which is the part of the design that made all the other parts possible.

Each chapter stands on its own well enough to be read out of order. Diagrams are drawn in plain text, so they survive any reader. Paths in `monospace` point at real files in the repository.

## A note on the name

Beads issues are called beads. A viewer for beads, in the style of k9s, became b9s. The Go module is still called `beadwork`, the program's earlier name, and you will see that word in import paths. Nobody has yet found the afternoon to rename it.
