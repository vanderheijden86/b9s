# Space programme sample

Red Harbor, a made-up crewed Moon-and-Mars programme, in 60 Beads issues: seven
epics broken into features and tasks, bugs and chores, every priority from P0
to P4, work in progress, blocked and deferred issues, dependency chains that
cross epics, labels, comments and closed work. Three releases are modelled as
milestone issues that depend on the epics they ship (see
[ADR 0026](../../docs/adr/0026-model-releases-as-milestone-issues.md)): Flight
Test 1 is released, Lunar Demo and Crewed Landing are planned, and the Mars
cargo epic has no release yet.

The epics follow the shape of publicly announced crewed lunar and Mars plans: a
reusable heavy launcher with a booster catch, ship-to-ship propellant transfer
in orbit, an uncrewed then crewed lunar landing, life support and a surface
habitat, a Mars cargo flight in a launch window, and ground and launch
operations. Each epic description links a public page for background, such as
[NASA's Artemis overview](https://www.nasa.gov/humans-in-space/artemis/).

**Red Harbor is fictional. It is not affiliated with, endorsed by or based on
the internal work of any space agency or company.** Every status, date, test
result and decision in it belongs to the made-up programme and says nothing
about any real mission. The crew and engineers (mira, tomas, ines, kofi, yuki,
noor, lena and dario, at `example.com`) are invented.

## Browse it

The project is one JSONL file, so it needs no Dolt server and no `bd init`:

```bash
cd examples/space-programme   # or a folder holding a copy of .beads/issues.jsonl
b9s                           # the terminal UI
b9s web                       # the browser UI, on 127.0.0.1:7979
```

Browsing this way is read-only. Every write in b9s runs `bd`, and `bd` finds no
database here, so an edit fails with `no beads database found` and changes
nothing.

## Edit it

To try the writes too, import a copy into an embedded Beads project:

```bash
cp -R examples/space-programme /tmp/b9s-space && cd /tmp/b9s-space
bd init --from-jsonl --prefix mars --skip-hooks
b9s
```

`--skip-hooks` leaves out the git hooks. `bd init` still writes agent files
such as `CLAUDE.md` and `AGENTS.md` into the folder.

## Regenerate it

`generate.sh` builds the issues again with the `bd` on your `PATH`, in a
temporary embedded project, under a made-up git identity, and writes
`.beads/issues.jsonl`. It unsets every `BEADS_*`, `BD_*` and `DOLT_*` variable
of the calling shell first and refuses to write unless the project is
embedded, so it cannot reach a Dolt server even when run from a shell that has
one configured. The IDs and timestamps change on every run; it takes about a
minute and a half. `TestSampleSpaceProgrammeOpensAsJSONL` in `internal/datasource`
checks that the result still opens and still contains each kind of issue the
screens show.

```bash
sh examples/space-programme/generate.sh
```
