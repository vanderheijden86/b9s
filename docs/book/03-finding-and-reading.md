# Finding a project and reading it

A Beads project is a folder with a `.beads` directory in it. What is inside that directory has changed over Beads' life, and b9s has to cope with all of it.

## Three shapes of a store

The oldest shape is `issues.jsonl`: one JSON object per line, one line per issue. It is what `bd export` writes and what `bd import` reads, it diffs well in git, and it is the fixture format for nearly every test in the repository.

The middle shape is `beads.db`, a SQLite file. Some Beads installations kept one as a cache beside the JSONL.

The current shape is a Dolt server. Dolt is a SQL database with git semantics: tables have commits, branches and diffs. A `.beads/metadata.json` with `"dolt_mode": "server"` names a host, a port, a user and a database, and the issues live on that server, not in the folder. In the workflow b9s was built for, every project on a machine points at one shared Dolt server through an SSH tunnel on `127.0.0.1:3306`, and each project is one database on it.

`DiscoverSources` in `internal/datasource/source.go` looks in the `.beads` directory, or the one named in `BEADS_DIR`, and returns every shape it finds. Then `cmd/b9s` picks:

```
   metadata.json says server?  ──yes──▶  DoltReader
            │                                │ connect fails
            no                               ▼
            ▼                          record the error
      beads.db exists?      ──yes──▶  SQLiteReader
            │
            no
            ▼
      issues.jsonl exists?  ──yes──▶  pkg/loader
            │
            no
            ▼
      not a project
```

A Dolt server always wins when it answers. When it does not answer, b9s falls back down the list and keeps the error. Press `D` and the health popup says which source is live and why the better one is not. That popup exists because a silent fallback is the worst kind: you would edit a stale JSONL for an hour before noticing the tunnel was down.

The JSONL lookup has one more trick. A git worktree has its own `.beads` folder but often no `issues.jsonl` of its own, so `pkg/loader` follows the worktree back to its main checkout and reads the file there. That is why b9s works in a feature branch worktree without configuration.

## The Dolt reader

`DoltReader` in `internal/datasource/dolt.go` is about five hundred lines that turn three tables into Go values. It uses `go-sql-driver/mysql`, because Dolt speaks the MySQL wire protocol. One query reads the issues, one reads the dependencies, one reads the comments, and the result is a slice of `model.Issue` with everything joined in memory.

Reading in memory is a deliberate choice and a cheap one. A large project has a few thousand issues. The whole set fits in a few megabytes, loads in well under a second over the tunnel, and once loaded makes every filter and every sort a matter of microseconds. The store is asked for nothing until it changes.

The reader also fills two fields that older schemas do not have. `created_by` names who made the issue and `owner` holds the git email behind that name. `internal/datasource/creators.go` selects those columns when they exist and leaves the fields empty when they do not, so a project on an older `bd` still opens.

## The password and where it may go

The Dolt password comes from the environment variable `BEADS_DOLT_PASSWORD`. It is never read from a file and never written to one, and that rule is not a matter of taste.

Consider what `metadata.json` is: a file in a repository, which means a file anyone who can push to that repository can edit. If b9s sent the environment password to whatever host that file named, then cloning a repository would be enough to make your laptop hand your credential to a stranger's server on the first `b9s` you ran. Decision record 0013 closes that hole:

> The password is sent only to a loopback address, or to an endpoint listed in `B9S_TRUSTED_DOLT_ENDPOINTS`.

Loopback is safe because the SSH tunnel terminates there and the tunnel's other end is yours. Anything else has to be named in your own environment, which the repository cannot touch. A `metadata.json` that points elsewhere gets no password and therefore no connection, and the health popup says so.

## Nine projects at once

Press `0` and b9s shows every recent project in one tree. `MultiDoltReader` in `dolt_multi.go` does this by opening one `DoltReader` per database and concatenating the results, with a `project` field on each issue so the query can tell them apart.

The all-projects view is read-only. That is not a limitation of the reader. It is a consequence of how writes work, which chapter seven explains: a write needs a checkout folder to run `bd` in, and nine projects have nine of them or none.

## What a reader does not do

A reader never caches across runs, never writes, and never decides anything about the UI. It answers `Load()` with issues or an error, and it answers a few narrow questions such as whether the schema has a comments table. Everything about what to show, in what order, with what filter, is the UI's problem.

That boundary is what makes the browser version possible. `b9s web`, chapter eleven, uses the same readers with no changes at all.
