# Sample project

A made-up web shop in 52 Beads issues. Five epics (Checkout, Product search,
Spring launch, Mobile shop, Seller onboarding) each hold features, and the
features hold tasks and bugs. Two milestones ship the epics as releases. It has
work in progress, blocked and deferred issues, dependency chains, labels,
comments, a Mermaid diagram and closed work. It is the seed of the public demo
at [demo.b9s.osen.co](https://demo.b9s.osen.co). Every
name and email in it is invented. Use it to try b9s, or `b9s web`, when you
have no project of your own at hand.

## Browse it

The project is one JSONL file, so it needs no Dolt server and no `bd init`:

```bash
cd examples/sample-project    # or a folder holding a copy of .beads/issues.jsonl
b9s                           # the terminal UI
b9s web                       # the browser UI, on 127.0.0.1:7979
```

Browsing this way is read-only. Every write in b9s runs `bd`, and `bd` finds no
database here, so an edit fails with `no beads database found` and changes
nothing.

## Edit it

To try the writes too, import a copy into an embedded Beads project:

```bash
cp -R examples/sample-project /tmp/b9s-sample && cd /tmp/b9s-sample
bd init --from-jsonl --prefix shop --skip-hooks
b9s
```

`--skip-hooks` leaves out the git hooks. `bd init` still writes agent files
such as `CLAUDE.md` and `AGENTS.md` into the folder.

## Regenerate it

`generate.sh` builds the issues again with the `bd` on your `PATH`, in a
temporary embedded project, under a made-up git identity, and writes
`.beads/issues.jsonl`. The IDs and dates change on every run.
`TestSampleProjectOpensAsJSONL` in `internal/datasource` checks that the result
still opens, still contains each kind of issue the screens show, and still
nests tasks under features under epics.

```bash
sh examples/sample-project/generate.sh
```
