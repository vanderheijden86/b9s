# Data IO: how b9s and bd read and write Beads

This document explains how data moves between b9s, the `bd` CLI and the Dolt
store. It also covers the newer IO mechanisms in upstream Beads (`bd serve`,
the proxied server, the HTTP backend) and the BDP wire protocol. It is
background reading. [ARCHITECTURE.md](ARCHITECTURE.md) and the
[ADRs](adr/) remain the references for what b9s decides.

Status as of 2026-10-14: b9s v1.3.4. Upstream Beads `main` at `048f5e5`, latest
release tag `v1.3.1` (2026-09-30), release candidate `v1.3.2-rc.2`
(2026-10-10).

## Contents

- [The short version](#the-short-version)
- [Background: Dolt and the Beads storage modes](#background-dolt-and-the-beads-storage-modes)
- [How b9s finds a store](#how-b9s-finds-a-store)
- [How b9s reads](#how-b9s-reads)
- [How b9s notices changes](#how-b9s-notices-changes)
- [How b9s writes](#how-b9s-writes)
- [Inside bd: how a write reaches Dolt](#inside-bd-how-a-write-reaches-dolt)
- [Inside bd: starting and finding the server](#inside-bd-starting-and-finding-the-server)
- [New in recent bd](#new-in-recent-bd)
- [BDP, the bead-graph wire protocol](#bdp-the-bead-graph-wire-protocol)
- [What this means for b9s](#what-this-means-for-b9s)
- [Glossary](#glossary)
- [Sources](#sources)

## The short version

b9s reads the store directly and writes only by running `bd`. Reads go
straight to the data because they must be fast and support live reload. Writes
go through `bd` so that every rule `bd` enforces on create, update, close and
delete also applies to changes made in b9s.

```mermaid
flowchart LR
    subgraph b9s
        R[datasource readers]
        W[IssueWriter / attach]
        WT[watcher]
    end
    BD[bd CLI]
    D[(Dolt server<br/>or proxied server)]
    E[(embedded Dolt<br/>.beads/embeddeddolt)]
    F[(SQLite / JSONL)]

    R -- "SQL over MySQL protocol" --> D
    R -- "bd export (JSONL on stdout)" --> BD
    R -- "open file" --> F
    W -- "bd create / update / close ..." --> BD
    BD -- "SQL tx + DOLT_COMMIT" --> D
    BD -- "in-process engine" --> E
    WT -- "SELECT DOLT_HASHOF_DB()" --> D
    WT -- "stat manifest + journal" --> E
```

There is one exception to "read directly": an embedded Dolt store is read by
running `bd export`, because opening the store would lock out the user's own
`bd` writes ([ADR 0025](adr/0025-read-embedded-dolt-through-bd-export.md)).
The Memory Beads preview is also read only through `bd`
([ADR 0041](adr/0041-retain-cli-memory-reads-after-transport-probes.md)).

b9s does not use `bd serve` or BDP today.

## Background: Dolt and the Beads storage modes

**Dolt** is a SQL database with Git-style versioning. A Dolt server speaks the
MySQL wire protocol, so any MySQL client, including Go's
`go-sql-driver/mysql`, can query it. Two layers matter for IO:

- The **working set** holds the current rows. A normal SQL `COMMIT` ends a
  transaction and changes the working set, but it does not create a Dolt
  commit.
- A **Dolt commit** (`CALL DOLT_COMMIT(...)`) snapshots the working set into
  history, like `git commit`. `HASHOF('HEAD')` changes only then.
  `DOLT_HASHOF_DB()` hashes the working set, so it changes on every write,
  committed or not. That is why b9s polls it.

**Beads** keeps issues in Dolt (or, for older projects, SQLite or JSONL).
`.beads/metadata.json` names how the store is reached in `dolt_mode`:

| `dolt_mode` | What runs | Who can open it |
|---|---|---|
| `embedded` (default for `bd init`) | Dolt engine inside each `bd` process. Data in `.beads/embeddeddolt/<db>/` | One process at a time. `bd` opens it per command and releases it on exit |
| `server` | A `dolt sql-server`, started by `bd` on demand or run by you. Port recorded in `.beads/dolt-server.port` | Many clients at once over TCP |
| `proxied-server` | A loopback proxy in front of a private `dolt sql-server`, supervised by `bd`. Used by Gas City workspaces and rigs | Many clients, through the proxy, as `root` without a password |

A shared server (`BEADS_DOLT_SHARED_SERVER`, port 3308) is a setting, not a
separate `dolt_mode` value.

## How b9s finds a store

`DiscoverSources` in `internal/datasource/source.go` reads files only. It
never asks `bd` where the data is.

```mermaid
sequenceDiagram
    autonumber
    participant CLI as cmd/b9s
    participant DS as datasource.DiscoverSources
    participant FS as .beads directory
    participant A as resolveDoltAddress
    CLI->>DS: OpenProject(folder)
    DS->>FS: read metadata.json (BEADS_DIR overrides the folder)
    alt dolt_mode = server
        DS->>A: which host:port would bd dial?
        A->>FS: env BEADS_DOLT_SERVER_PORT, then dolt-server.port,<br/>then listener.port, then config.yaml, then metadata
        A-->>DS: 127.0.0.1:PORT
    else dolt_mode = proxied-server
        DS->>FS: proxy root (env, client-info sidecar, Dolt dir)
        DS->>FS: read proxy.pid for the proxy port
    else dolt_mode = embedded
        DS->>DS: source dolt_embedded (read through bd export)
    end
    DS->>FS: beads.db? issues.jsonl?
    DS-->>CLI: sources, best first (Dolt, then SQLite, then JSONL)
```

A Dolt source always wins. If the Dolt connection fails, b9s falls back to
SQLite and then JSONL and records the error for the `D` health popup. An
embedded store never falls back to a JSONL file beside it, because that file
may be months old.

The password from `BEADS_DOLT_PASSWORD` goes only to a loopback address or an
endpoint listed in `B9S_TRUSTED_DOLT_ENDPOINTS`, so a cloned repository cannot
redirect it ([ADR 0013](adr/0013-trust-dolt-endpoints-before-sending-environment-credentials.md)).
A proxied store never gets the password.

## How b9s reads

### Dolt server: plain SQL

`DoltReader` in `internal/datasource/dolt.go` opens a small MySQL connection
pool (two connections) and runs four queries per load. It reads the `issues`
table first, then labels, dependencies and comments in one batch each, so a
load costs four queries, not one per issue.

```mermaid
sequenceDiagram
    autonumber
    participant UI as ui.Model
    participant R as DoltReader
    participant D as Dolt sql-server
    UI->>R: LoadIssues()
    R->>D: SELECT id, title, ... FROM issues WHERE status != 'tombstone'
    D-->>R: rows
    alt a column is missing (older bd schema)
        R->>D: SELECT id, title, description, status, ... (narrow set)
        D-->>R: rows
    end
    R->>D: SELECT issue_id, label FROM labels
    R->>D: SELECT issue_id, depends_on, type FROM dependencies
    R->>D: SELECT id, issue_id, author, text, created_at FROM comments
    D-->>R: rows
    R-->>UI: issues with labels, deps and comments
    Note over R,D: four separate autocommit reads, not one transaction:<br/>a write between them can show up in one table only,<br/>and the next reload corrects it
```

The fallback to the narrow column set fires only on a "missing column" error.
Any other failure, such as a dropped connection, is reported unchanged.

### Proxied server: the same SQL through bd's proxy

```mermaid
sequenceDiagram
    autonumber
    participant R as DoltReader
    participant P as bd loopback proxy
    participant D as private dolt sql-server
    R->>R: port from proxy.pid (else listener.port)
    R->>P: MySQL handshake as root, no password
    P->>D: forward bytes (io.Copy)
    R->>P: SELECT ... FROM issues
    P->>D: forward
    D-->>P: rows
    P-->>R: rows
    Note over P: if no proxy is running, b9s does not start one,<br/>and the dial fails with a clear error
```

### Embedded Dolt: through `bd export`

```mermaid
sequenceDiagram
    autonumber
    participant L as embedded loader
    participant BR as internal/bdrun
    participant BD as bd export
    participant S as .beads/embeddeddolt
    L->>BR: Output(ctx, "export"), BEADS_DIR set, deadline
    BR->>BD: exec in its own process group
    BD->>S: open engine, read tx
    S-->>BD: issues, labels, deps, comments
    BD->>S: close (lock released)
    BD-->>BR: one JSON object per issue on stdout,<br/>hints on stderr
    BR-->>L: stdout only
    L->>L: parse with pkg/loader (the JSONL parser)
    Note over L,S: b9s holds no handle between reads,<br/>so a user's bd write never meets a lock held by b9s
```

A small project exports in about 0.4 s. The watcher triggers a reload only
after a write, so the cost is paid once per change.

### Memory Beads preview: many `bd` calls

A graph workspace (`graph_mode: "link"` in `metadata.json`) holds Memories and
typed Links that the issue readers do not understand. b9s reads it entirely
through the matching preview `bd`, after a capability probe
([ADR 0045](adr/0045-probe-bd-help-before-memory-reads.md)).

```mermaid
sequenceDiagram
    autonumber
    participant V as Memory view
    participant C as GraphPreviewClient
    participant BD as preview bd
    participant S as graph workspace
    V->>C: DetectMemory(project)
    C->>BD: versions --help, links --help, memories --help<br/>(empty temp dir, BEADS_DIR unset, 2 s each)
    BD-->>C: dialect preview-v2 (cached per binary)
    C->>C: metadata: graph_mode=link, graph_ready, schema 6
    V->>C: Graph()
    C->>BD: list --format records-json --all
    BD->>S: open, validate, read
    BD-->>C: every Bead and its owned Links
    loop each component with Issues or unknown Types
        C->>BD: graph SEED --view generic --direction both
        BD-->>C: unowned Links
    end
    C-->>V: graph (progress reported per traversal)
    V->>C: on select: show ID --json, links ID --json, versions ID --json
```

A complete read of a 57-Bead graph took 5 to 8 seconds in the
[transport investigation](memory-read-research.md). Each `bd` process costs
about 0.4 s to start. That cost is why BDP was measured as an alternative
(see [below](#bdp-the-bead-graph-wire-protocol)).

### SQLite and JSONL

`SQLiteReader` opens `beads.db` read-only. JSONL is parsed by `pkg/loader`,
which picks the file by canonical name first (`beads.jsonl`, `issues.jsonl`,
`beads.base.jsonl`).

## How b9s notices changes

```mermaid
sequenceDiagram
    autonumber
    participant W as DoltWatcher
    participant D as Dolt / embedded store
    participant UI as ui.Model
    participant R as reader
    loop every refresh.poll_interval (500 ms)
        alt Dolt server or proxied
            W->>D: SELECT DOLT_HASHOF_DB()
        else embedded
            W->>D: stat each manifest's content + chunk journal size
        end
        D-->>W: hash or fingerprint
        W->>W: compare with the last value
    end
    W->>UI: changed (non-blocking, duplicates dropped)
    UI->>R: LoadIssues()
    R-->>UI: issues
    UI->>UI: keep cursor, marks and folds
```

- `DOLT_HASHOF_DB()` covers the working set, so a `bd` write appears even with
  `dolt.auto-commit` set to `off` or `batch`.
- The embedded fingerprint ignores `journal.idx` and temporary manifests,
  because `bd` rewrites those even when it only reads. Counting them would make
  each reload trigger the next.
- JSONL and SQLite use fsnotify, with polling on NFS and SMB.

## How b9s writes

### Issues: `IssueWriter` runs `bd`

```mermaid
sequenceDiagram
    autonumber
    participant U as User
    participant UI as ui.Model
    participant IW as IssueWriter
    participant BR as internal/bdrun
    participant BD as bd
    participant S as store
    participant W as watcher
    U->>UI: K on marked issues, confirm
    UI->>IW: CloseIssues([id1, id2])
    IW->>IW: Checkout set? (no .beads dir means read-only)
    IW->>BR: Run(ctx, "close", id1, id2), deadline 60 s + 2 s per issue
    BR->>BD: literal argv, no shell, own process group, cwd = checkout
    BD->>S: write (see "Inside bd")
    BD-->>BR: exit status, output
    BR-->>IW: ok, or ErrTimeout / ErrCancelled / bd's error
    IW-->>UI: BdResultMsg
    S-->>W: hash changes
    W->>UI: reload
    Note over UI: the UI never patches its own copy,<br/>the reload shows what bd actually stored
```

Every write is one of `bd create`, `bd update`, `bd close`, `bd delete --force`,
`bd defer` or `bd comments add`. Bulk actions pass all marked IDs in one call.
At the deadline `bdrun` kills `bd` and its whole process group. If `bd` exits
on its own, its exit status decides the result, even when a child process it
started still holds the output pipe open.

### Attachments: blob first, then a comment through `bd`

```mermaid
sequenceDiagram
    autonumber
    participant A as b9s attach
    participant B as blob store (S3 or local)
    participant BD as bd
    participant S as store
    A->>A: sha256 and MIME sniff
    A->>B: Put(prefix/db/sha256) if absent
    A->>BD: comments add ID "beads-attachment/v1 ..."
    BD->>S: INSERT comment
    Note over A,S: a crash after Put leaves an unreferenced blob,<br/>never a comment without bytes. GC removes old orphans
```

See [ADR 0024](adr/0024-attach-files-through-reference-comments.md).

### The web UI

`b9s web` uses the same readers, watcher and `IssueWriter`. A browser write is
an HTTP request that ends in the same `bd` call.

```mermaid
sequenceDiagram
    autonumber
    participant Br as browser
    participant WS as pkg/web Server
    participant IW as IssueWriter
    participant BD as bd
    participant St as web Store
    Br->>WS: POST /api/write (session cookie, X-B9s-CSRF)
    WS->>WS: check the operation against a closed set
    WS->>IW: e.g. SetStatus(id, "in_progress")
    IW->>BD: bd update id --status in_progress
    BD-->>IW: ok or error
    WS-->>Br: 200, or 422 with bd's output
    St->>St: watcher fires, version + 1
    St-->>Br: SSE "changed"
    Br->>WS: GET /api/snapshot
```

## Inside bd: how a write reaches Dolt

A `bd` write is two steps: a SQL transaction that changes rows, then a Dolt
commit that records them in history. `dolt.auto-commit` decides whether the
second step happens on every command:

| `dolt.auto-commit` | Effect |
|---|---|
| `on` (default) | One Dolt commit per write command |
| `batch`, `off` | Changes stay in the working set until `bd dolt commit` (or an explicit commit point such as `bd batch`) |

### Server mode

```mermaid
sequenceDiagram
    autonumber
    participant CLI as bd update
    participant Ops as issueops
    participant St as storage/dolt
    participant D as dolt sql-server
    CLI->>Ops: Update(ctx, id, fields)
    Ops->>St: withRetryTx
    St->>D: BEGIN
    St->>D: UPDATE issues ... / INSERT events journal row
    St->>D: COMMIT
    Note over St,D: rows now in the working set:<br/>DOLT_HASHOF_DB() has changed, b9s reloads
    opt auto-commit on
        St->>D: CALL DOLT_ADD(table) per dirty table (fresh connection)
        St->>D: CALL DOLT_COMMIT('-m', msg, '--author', actor)
    end
    St-->>CLI: ok
```

Dolt merges concurrent transactions cell by cell. Two writers that change
different columns of the same row both succeed. A conflicting write to the
same cell fails with error 1213, and `withRetryTx` replays it. Wisps and other
"no history" rows live in `dolt_ignore`d tables and are never committed.

### Embedded mode

```mermaid
sequenceDiagram
    autonumber
    participant CLI as bd create
    participant G as workspace gate
    participant E as embedded engine
    CLI->>G: shared flock on the store's *.gate.lock
    CLI->>E: open engine on .beads/embeddeddolt/DB
    CLI->>E: BEGIN, INSERT ..., COMMIT
    CLI->>E: DOLT_ADD per dirty table, DOLT_COMMIT (skipped if empty)
    CLI->>E: close
    CLI->>G: release
    Note over CLI,E: a failure after the SQL commit returns ErrCommitIndeterminate.<br/>Another process holding the engine open blocks this write
```

### Proxied mode and `bd serve`: one pinned connection

```mermaid
sequenceDiagram
    autonumber
    participant CLI as bd close (or a bd serve handler)
    participant U as storage/uow
    participant P as proxy
    participant D as dolt sql-server
    CLI->>U: RunTx(fn)
    U->>P: pin one connection
    U->>P: START TRANSACTION
    U->>P: UPDATE / INSERT ...
    alt changes and not deferred
        U->>P: CALL DOLT_COMMIT('-Am', msg)
    else nothing changed or auto-commit deferred
        U->>P: COMMIT
    end
    P->>D: forwarded
```

`bd serve` always commits per request. It does not run hooks or the per-command
auto-commit step.

## Inside bd: starting and finding the server

### Server mode auto-start

```mermaid
sequenceDiagram
    autonumber
    participant CLI as bd (any command)
    participant DS as doltserver.EnsureRunning
    participant FS as .beads
    participant D as dolt sql-server
    CLI->>DS: need a server
    DS->>FS: read dolt-server.port, dial
    alt answers
        DS-->>CLI: 127.0.0.1:PORT
    else no server
        DS->>FS: flock dolt-server.lock
        DS->>D: start on an OS-assigned port
        DS->>FS: write dolt-server.port, .pid, .log
        DS-->>CLI: 127.0.0.1:PORT
    end
    Note over DS: skipped for an external server:<br/>dolt_server_port in metadata or BEADS_DOLT_SERVER_PORT set
```

This is why b9s reads `dolt-server.port` before anything in `metadata.json`.
A server `bd` started for itself is recorded only there.

### Proxied-server lifecycle

```mermaid
sequenceDiagram
    autonumber
    participant CLI as bd
    participant FS as proxy root
    participant P as db-proxy
    participant D as db-proxy-child (dolt sql-server)
    CLI->>FS: read proxy.pid (pid, port, upstream_id, ...)
    CLI->>P: dial 127.0.0.1:port
    alt proxy not answering
        CLI->>FS: flock proxy.lock
        CLI->>P: spawn proxy
        P->>D: spawn or attach upstream server
        P->>FS: write proxy.pid
    end
    CLI->>P: MySQL session as root
    P->>D: io.Copy both ways
    Note over P: exits after 30 s idle (DefaultProxyIdleTimeout)
```

The proxy root is `BEADS_PROXIED_SERVER_ROOT_PATH`, else `root_path` in
`.beads/proxied_server_client_info.json`, else the Dolt data directory. b9s
follows the same order but never spawns a proxy.

## New in recent bd

| Mechanism | What it is | Since | Status |
|---|---|---|---|
| `bd serve` HTTP API v0 | JSON over HTTP in front of a SQL-server workspace | v1.2.1 (6 operations), about 43 today | Shipped |
| Events journal | Append-only `bd_events_journal` table, one row per write, readable as pages or an SSE stream | v1.2.1 | Shipped, opt-in (`events-journal` config or `BD_EVENTS_JOURNAL`) |
| Revisions and CAS | Opaque `revision` per issue; writes can require `expected_version`, `--if-revision`, `--if-status`, `--if-assignee` | `--if-*` in v1.2.1, `revision` in v1.3.0 | Shipped |
| Proxied server | Loopback proxy plus private `dolt sql-server`, supervised by `bd` | v1.3.0 for most users | Shipped. Default for managed local setups, but plain `bd init` still writes `embedded` |
| HTTP backend, `bd connect` | `bd` itself talks to a remote `bd serve` instead of Dolt | upstream `main` only | Unreleased, marked experimental |
| Versioned beads | `issue_versions` history written beside each change | upstream `main` | Plumbing only, not enabled in the CLI |
| Bead graph, Memory Beads, BDP | Typed Beads and Links in a graph store, served over BDP | feature branches | Design and preview builds |

### `bd serve`: the HTTP API v0

`bd serve` runs in the foreground and listens on `127.0.0.1:0` by default (a
free port). It prints one line, `bd serve: listening on http://127.0.0.1:PORT`.
There is no pid or port file, so a client must be given the address. It
refuses an embedded workspace: "bd serve requires a Dolt SQL server".

Wire basics:

- Cleartext HTTP. Success bodies are `application/json`. Every error is
  `application/problem+json` (RFC 9457) with `status`, `title`, `code` and
  `request_id`.
- Paths live under `/v0/beads/`. Custom actions use a `:verb` suffix, for
  example `POST /v0/beads/issues/{id}:close`.
- Requests with a body must send `Content-Type: application/json`, as a CSRF
  guard. The `Host` header is checked against an allowlist against DNS
  rebinding.
- Auth is off on loopback. `--auth-token-file` (or `BEADS_SERVE_TOKEN_FILE`)
  turns on `Authorization: Bearer`. Listening beyond loopback requires a token
  or an explicit `--insecure-no-auth`.
- Optional request headers: `Bd-Project-Id` (refused on mismatch) and
  `Bd-Wire-Revision` (the wire shape the client expects; the server is at 2).

```mermaid
sequenceDiagram
    autonumber
    participant C as client
    participant S as bd serve
    participant D as dolt sql-server
    C->>S: GET /v0/beads/context
    S-->>C: 200 {api_version, bd_version, dolt_mode, project_id,<br/>capabilities[], wire_revision, ...}
    Note over C,S: answered from a startup snapshot, no DB access
    C->>S: GET /v0/beads/issues?limit=100
    S->>D: read tx
    S-->>C: 200 {items[], has_more: true, next_cursor: "v2..."}
    C->>S: GET /v0/beads/issues?limit=100&cursor=v2...
    S-->>C: 200 {items[], has_more: false}
    C->>S: GET /v0/beads/issues/bd-42
    S-->>C: 200 {..., revision: "17"}
    C->>S: POST /v0/beads/issues/bd-42:close {expected_version: "17"}
    S->>D: START TRANSACTION, UPDATE, DOLT_COMMIT
    alt revision still 17
        S-->>C: 200 {..., revision: "18"}
    else someone wrote first
        S-->>C: 409 problem {code: precondition_failed}
    end
```

`capabilities[]` lists operation tokens such as `issues.list`, `issues.update`,
`events.list` and `memories.list`. The `memories.*` tokens refer to the
key/value memories stored as `kv.memory.*` config rows, not to Memory Beads.

### The events journal and its stream

When the journal is on, each write appends a row in the same transaction. A
client can read it by sequence number, or stream it.

```mermaid
sequenceDiagram
    autonumber
    participant C as client
    participant S as bd serve
    C->>S: GET /v0/beads/events?since=4211&limit=500
    S-->>C: 200 {records: [{seq: 4212, op: "create", issue_id: "bd-100", issue: {...}}], head: 4980}
    C->>S: GET /v0/beads/events:watch?since=4980
    S-->>C: text/event-stream, retry: 3000
    loop each write
        S-->>C: id: 4981 / data: {EventRecord}
    end
    S-->>C: ": heartbeat" after 20 s of silence
    Note over C,S: reconnect with Last-Event-ID to resume.<br/>410 events_journal_truncated if the client fell too far behind
```

This is the first push-style change feed in Beads. Today b9s detects changes by
polling a hash instead.

### The HTTP backend: `bd` as a client of `bd serve`

On upstream `main` only, `bd connect URL` checks the server's `/context`
(API version, wire revision, optional expected project id). It then writes
`.beads/http_target.json` and sets `backend: "http"` in `metadata.json`. From
then on every `bd` command in that checkout sends HTTP requests instead of SQL.

```mermaid
sequenceDiagram
    autonumber
    participant U as bd update (laptop)
    participant H as internal/httpclient
    participant S as bd serve (server host)
    participant D as dolt sql-server
    U->>H: Update(id, fields)
    H->>S: PATCH /v0/beads/issues/ID<br/>Authorization: Bearer, Bd-Wire-Revision, Bd-Project-Id
    S->>D: one transaction, DOLT_COMMIT
    S-->>H: 200 {revision}
    H-->>U: ok
```

Tokens come from `BEADS_HTTP_TOKEN=host[:port]=TOKEN`, then
`BEADS_HTTP_TOKEN_COMMAND`, then a credentials file. For b9s this matters
because such a checkout has no Dolt address to read
([below](#what-this-means-for-b9s)).

## BDP, the bead-graph wire protocol

BDP is an external specification (`gastownhall/bdp`, `docs/specs/bdp.md`) for
serving typed **Beads** and **Links** with stable identity, revisions and
ownership. The beads repository never spells out the acronym. Upstream plans to
serve BDP for the bead graph (the Memory Beads work), not for ordinary issues.

Status: the plan (`engdocs/BDP_BEAD_GRAPH_PLAN.md`) and architecture
(`engdocs/BDP_GRAPH_ARCHITECTURE.md`) are drafts on upstream `main`. The P0 Read
schema is adopted. The implementation lives on feature branches
(`feat/memory-beads` and related). No released `bd` serves BDP.

How it is meant to fit:

- BDP routes mount inside the same `internal/httpapi` server as the v0 API, as
  a second route table behind the same deadline, bearer and project checks.
- Only `bd --graph-mode link serve` creates ("mints") the graph scope.
  Plain `bd serve` shows no BDP routes when there is no graph.
- v0 serves BDP from SQL-server workspaces only. Embedded workspaces never
  serve it.
- Revisions are opaque and compared only for equality. A deleted resource
  carries `deleted: {resourceKind, resource: {id, type, revision}}`. Errors are
  RFC 9457 problems with types such as `resource-pruned` and `resource-erased`.

### A BDP read, end to end (design)

Every read checks, inside its own transaction, that the store is still the one
this workspace has authority over. This guards against a copied or restored
database answering as if it were the original.

```mermaid
sequenceDiagram
    autonumber
    participant C as client
    participant H as internal/httpapi (BDP routes)
    participant G as graphops.Reader
    participant D as dolt sql-server
    C->>H: GET /SCOPE/beads/ID
    H->>H: deadline, bearer, Bd-Project-Id, DB slot
    H->>G: Bead(path)
    G->>D: read tx
    G->>D: scope row matches the local witness
    G->>D: ledger head hash matches
    G->>D: lease row is held and not expired
    G->>D: graph state version (hash of 8 graph tables)
    alt state moved since the witness
        G-->>H: ErrStateChanged, revalidate, retry once
    end
    G->>D: bead row, type descriptors, owned Links (bounded)
    D-->>G: rows
    G-->>H: BeadRecord {bead, ownedLinks}
    H-->>C: 200 BDP JSON, or a BDP problem
```

### What b9s measured against a preview build

The [transport investigation](memory-read-research.md) (2026-10-03) ran a
pinned preview `bd serve` on an isolated copy. What it observed there is
preview behaviour, not a released contract:

- Two collections, `beads/` and `links/`, paged with a `next` continuation.
- A continuation keeps its collection's original snapshot. A fresh request can
  see a later write, so a Bead page and a Link page can disagree.
- Resource ETags, but no graph-wide revision and no event stream.
- No history: `?version=TOKEN` answered HTTP 400 `invalid-parameter`, and the
  discovery document advertised Read only.
- An embedded workspace could not serve BDP.

```mermaid
sequenceDiagram
    autonumber
    participant B as b9s (research probe)
    participant S as preview bd serve (BDP)
    participant D as dolt sql-server
    B->>S: GET beads/?limit=10
    S->>D: snapshot read
    S-->>B: 10 Beads + next
    loop until no next
        B->>S: GET next
        S-->>B: page (same snapshot as page 1)
    end
    B->>S: GET links/?limit=10
    S-->>B: Links (a new snapshot)
    Note over B,S: both collections in about 0.4 s, versus 5 to 8 s through the CLI.<br/>No common fence across the two collections, no retained versions
```

## What this means for b9s

| Path | Today | What would change it |
|---|---|---|
| Issue reads | SQL to Dolt, `bd export` for embedded | A checkout on the HTTP backend has no Dolt address. b9s would need to read through `bd serve` (or through `bd export`) to open it |
| Issue writes | `bd` CLI | Nothing pressing. `bd serve` writes would save process startup and offer CAS through `expected_version` |
| Change detection | Poll `DOLT_HASHOF_DB()` or the embedded fingerprint | `events:watch` could replace polling where a `bd serve` runs and the journal is on |
| Memory graph | Many preview `bd` calls | BDP is the preferred candidate for server workspaces once it offers a common snapshot or graph revision and a history path ([ADR 0041](adr/0041-retain-cli-memory-reads-after-transport-probes.md)) |

None of these are planned work. They are the points where upstream changes
would touch b9s.

## Glossary

- **Working set**: Dolt's current, uncommitted state of the rows.
- **Dolt commit**: a history snapshot made by `DOLT_COMMIT`, separate from a SQL `COMMIT`.
- **Checkout**: a folder with a `.beads` directory. b9s writes only for a checkout.
- **UOW**: unit of work. `bd`'s pinned-connection transaction helper for proxied and served writes.
- **Revision**: an opaque per-issue token returned by `bd serve`. Compared for equality, never ordered.
- **Wire revision**: the version of the HTTP API's JSON shape, not of the data.
- **Scope, witness, lease**: BDP graph terms. The scope is the graph's identity, the witness is the local record of which store this workspace owns, and the lease is the row that proves it still holds that store.

## Sources

b9s: `internal/datasource/{source,dolt,dolt_address,proxied_server,embedded,dolt_watcher,graph_preview,memory_graph}.go`,
`internal/bdrun/{bdrun,capability}.go`, `pkg/ui/issue_writer.go`,
`internal/attach/add.go`, `pkg/web/write.go`, [ARCHITECTURE.md](ARCHITECTURE.md),
ADRs 0013, 0024, 0025, 0041, 0045, [memory-read-research.md](memory-read-research.md).

Upstream Beads (`gastownhall/beads` `main` at `048f5e5`, tags `v1.3.1`, `v1.3.2-rc.2`):
`internal/httpapi/spec/openapi.v0.yaml`, `internal/httpapi/{routes,wire_revision,cursor,problem,auth}.go`,
`cmd/bd/serve.go`, `engdocs/SERVE_RUNBOOK.md`, `docs/reference/events-journal.md`,
`docs/reference/configuration.md`, `internal/storage/{dolt,embeddeddolt,uow,dbproxy}`,
`internal/doltserver/doltserver.go`, `internal/httpclient`, `cmd/bd/http_backend.go`,
`VERSIONED-BEADS-WRITE-PATHS.md`, `PROPOSAL-cas-conditional-update.md`,
`engdocs/BDP_BEAD_GRAPH_PLAN.md`, `engdocs/BDP_GRAPH_ARCHITECTURE.md`, `CHANGELOG.md`.
