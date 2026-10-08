# Native Memory reader investigation

Investigation: `bd-mu5r`, 2026-10-03. Recommendation: retain the CLI reader for
the embedded preview. Do not replace it with direct SQL on this evidence.
[ADR 0041](adr/0041-retain-cli-memory-reads-after-transport-probes.md) records
the decision and the conditions for another evaluation.

## Scope and isolation

The measured source is the local `b9s-memory-poc` preview: **57 Beads
(32 Memories, 25 Issues), 42 Links**, and a 162,394-byte CLI inventory.
The pinned preview binary and source revision are
`1f3466b5dfcb8c76ecb9ceaf4b847df695131f96`. The external SQL server is Dolt
2.3.2. These results describe that schema-6 preview, not a released Memory
storage contract.

The host is macOS arm64. The pinned `bd`, embedded probe and b9s test toolchain
run as amd64, while the external Dolt executable is arm64. That difference
also limits direct timing comparisons. b9s tests used Go 1.25.5.

Source file hashes matched before and after copying. All experiments used
copies beneath `/private/tmp/b9s-mu5r-qwp2qemo`. No test wrote to the shared
server or the original preview. All source snapshots, mutated copies, command
outputs and process evidence remain in that directory. There was no GitHub
publication.

A plain copy returned `not_authority`. For these isolated experiments only,
each copy's metadata and `graph_preview_scope.workspace` were rebound to its
own canonical directory. Resource IDs, scope, data and retained versions were
preserved. This is a research fixture preparation step, not a supported
workspace migration or a proposal to bypass authority checks in b9s.

The SQL copy listened on `127.0.0.1:58441`, and its BDP service on
`127.0.0.1:58442`. Requests used that transport alias while records kept their
original canonical scope. Neither service was externally exposed.
Both owned services were stopped after the probes. Their ports were checked
closed, and all copied data was preserved. The original preview's file hashes
still matched the starting hashes at the end of the investigation.

## Measured costs

Four samples were taken per transport. “First” means the first measured call
in that run. It is **not a disk-cold benchmark**: OS caches were not cleared,
the binaries had run before, and no data was discarded. Warm values below
are medians of the next three samples. Compilation is excluded.

| Operation | First | Warm median | Result |
|---|---:|---:|---|
| CLI process, `bd version`, outside a workspace | 402.8 ms | 404.3 ms | No store read |
| CLI graph admission refusal, `version` inside preview | 273.6 ms | 265.1 ms | No graph store open |
| Embedded engine open, in-process | 35.3 ms | 20.7 ms | Matching pinned engine |
| Embedded query of current retained generic records | 1.77 ms | 0.98 ms | 74 records, excludes Issue projection |
| Embedded engine close | 17.7 ms | 14.8 ms | Checked cleanup result |
| CLI complete inventory | 624.8 ms | 599.9 ms | 57 Beads and their owned Links |
| CLI Memory summaries | 603.8 ms | 608.1 ms | 32 Memories |
| CLI incident Links for ADR 0027 | 434.1 ms | 418.8 ms | One selected Memory |
| CLI Memory-only full records | 588.7 ms | 578.9 ms | 32 Memories, 9 owned Links |
| Existing complete graph reader | 7.784 s | 5.144 s | 57 nodes, 42 edges |
| Persistent SQL, complete retained projection | 7.67 ms | 3.33 ms | 57 Beads, 42 Links |
| BDP, both collections, page size 10 | 417.6 ms | 398.3 ms | 99 resources, 11 pages |

The complete graph samples were 7.784, 7.958, 5.144 and 5.071 seconds. Embedded
lock scheduling and traversal overlap make this path variable. This small
sample does not support a latency percentile or a universal speedup claim.
The standalone process baseline was measured later while the project tests
were running. Do not subtract it from another row to infer validation cost.

The persistent SQL measurement excludes server startup, TCP connection setup
and initial schema/workspace checks. It deliberately omits many validations
that the CLI and BDP perform. Its speed is not the cost of a correct replacement.
The BDP measurement includes decoding and consuming every page, but excludes
service startup and discovery.

### ADR parsing versus native Memory data

The same captured inventory was decoded and assembled 1,000 times:

| Local work per snapshot | Mean |
|---|---:|
| Decode the complete CLI JSON inventory | 2.788 ms |
| Parse frontmatter for all Memory bodies | 44.8 microseconds |
| Assemble a graph with plain native Memory bodies | 338.2 microseconds |
| Assemble a graph with the captured ADR bodies | 385.0 microseconds |

The plain-body control replaces bodies in memory only. It does not change
the database or compare different stored snapshots. Both assembly controls
use inventory-owned Links, not the additional traversal results. Frontmatter
parsing is not the source of the multi-second delay.

The Memory-only CLI query is also not a complete graph substitute. It omits
25 Issues and Links they own or source. Its 9 owned Links cannot replace the
42-Link graph, including incoming Issue-to-Memory Links.

## Semantic parity

The opt-in tests compare canonical IDs, exact Type URLs, all properties
(including bodies and Issue owner), revision tokens, Link endpoints and owned
Link records. They normalize only the private CLI versus public BDP encoding
of references and ownership groups. They do not shorten IDs for comparison.
Attribution representation is outside this comparison.

- CLI inventory supplies every Bead. `bd links` for each Bead supplies all
  incident Links, including unowned informational Links and their properties.
- SQL joins live catalog heads to retained generic records. Issues require a
  separate join through `graph_preview_issue_versions` and `issue_versions`.
  Embedded Issue dependencies are omitted from properties, matching the graph
  projection. The number of reconstructed records must equal the live catalog.
- BDP exhausts `next` for both `beads/` and `links/`. Duplicate IDs, incomplete
  CLI responses and a continuation outside the canonical scope fail the probe.
- The initial current-record comparison passed for 57 Beads and 42 Links.
  The stronger comparison of complete owned records also passed after the two
  writer probes: 59 Beads, 44 Links and 11 older Memory versions.
  Additional checks compare retained Memory versions with `bd show --version`.
  The source contains nine older Memory versions, no older Link versions, and
  separate Issue version mappings. This is not a claim of exhaustive history
  coverage for every resource lifecycle.
- A BDP historical `?version=TOKEN` request returns HTTP 400,
  `invalid-parameter`. Its discovery document advertises Read, not History.

The SQL probe is a measured shortcut, not a storage adapter. It has no generic
handling for future backing kinds, descriptor evolution, deletion invariants,
retained attribution validation or the preview's complete acquisition limits.

## Failure and concurrency probes

| Probe | Observed result | Consequence |
|---|---|---|
| Copy without rebinding | CLI refuses `not_authority` | Directory identity is part of the contract |
| Embedded `bd serve` | Capability unavailable in the pinned implementation | BDP requires server mode |
| Keep an embedded engine open, then run a CLI writer | Writer reaches the 2-second test deadline | A persistent embedded handle prevents normal writing |
| Release that engine, retry writer | Write succeeds in 469 ms | Lock ownership causes the delay |
| Keep a SQL read transaction open, run a CLI writer | Writer succeeds in 350 ms | Server readers can coexist with writers |
| Read coordination token twice in that transaction | Same token | The read transaction retains its snapshot |
| Read token and `DOLT_HASHOF_DB()` after commit | Both change | Both detect the controlled write |
| Fetch BDP page 1, add an owned Link, exhaust old continuation | Original Memory revision remains in the continuation | Pagination retains one collection's snapshot |
| Read that Link and Memory afresh | New Link and new Memory revision appear | Separate requests can represent different snapshots |
| Change copied schema version to 999 | CLI rejects persisted binding/schema | Table existence alone is insufficient |
| Change copied current Memory payload to `{}` | CLI rejects current/retained mismatch | Retained snapshots alone can return stale-looking success |

The last two probes leave the original copy intact and mutate separate
`schema-drift` and `payload-drift` directories. A naive retained-head query
still returns a revision in both cases. These are observed differences in
failure semantics, even though the clean-data parity check passes.

Concurrency probes add unique Memories and Links to the copies. Their counts
therefore differ from the original 99-resource baseline. Failed probe runs
are retained as evidence, including an early test error that assumed BDP
always encoded a Link endpoint as an object. The corrected probe accepts the
actual reference representation through the same comparison projection.

## Optional live updates

No Memory live-update behavior changes in this investigation.

For an ordinary SQL server, a cheap working-set revision check is more useful
than repeatedly reading the complete graph. `DOLT_HASHOF_DB()` observes
uncommitted working-set changes. `HEAD` alone does not. The preview's
`writer_token` also changed on the controlled mutation, but it is private
schema and cannot detect arbitrary out-of-band SQL.

A future bounded refresh should:

1. Keep one refresh in flight and coalesce subsequent invalidations.
2. Read a revision before and after acquiring all graph collections. Accept
   the result only if the revision is stable. Bound retries and show the last
   accepted snapshot with a stale indicator when a writer remains active.
3. Preserve selection by canonical ID. Cache current content by ID and
   revision, and retained content by ID and version token. Invalidate incident
   Link caches when the graph revision changes.
4. Preserve the selected retained version. Label its incoming Links as current,
   consistent with ADR 0037. Clear selection explicitly if a node disappears.
5. Treat stored Link direction as source-to-target semantics. Update activity
   is a separate annotation, not an arrow reversal or animated data flow.

The pinned BDP service provides resource ETags and retained collection cursors,
but no global graph revision or event stream. A Memory's revision does not
cover every incoming Link, and Issue-sourced informational Links are unowned.
Polling only selected Bead ETags is therefore insufficient for graph freshness.
Separate Bead and Link collection reads need a common snapshot/fence before
automatic refresh can claim consistency. A hybrid SQL fence plus BDP reads
adds credentials and schema coupling, so it is not adopted by this research.

For embedded workspaces, b9s already watches manifest contents and journal
size. That can trigger a refresh without opening the store, but it is not a
documented semantic snapshot token. A 500 ms poll must not start a 5–8 second
graph read on every tick. No event feed was advertised by the pinned graph
Read service.

## Reproduction and evidence

The investigation files are local:

- `/private/tmp/b9s-mu5r-qwp2qemo/source-hashes.json`
- `/private/tmp/b9s-mu5r-qwp2qemo/research-second.log`
- `/private/tmp/b9s-mu5r-qwp2qemo/parity-history.log`
- `/private/tmp/b9s-mu5r-qwp2qemo/parity-complete-owned.log`
- `/private/tmp/b9s-mu5r-qwp2qemo/parity-final.log`
- `/private/tmp/b9s-mu5r-qwp2qemo/open-lock.log`
- `/private/tmp/b9s-mu5r-qwp2qemo/concurrent-second.log`
- `/private/tmp/b9s-mu5r-qwp2qemo/corruption-probes.json`
- `/private/tmp/b9s-mu5r-qwp2qemo/native-only.json`
- `/private/tmp/b9s-mu5r-qwp2qemo/process-startup.json`

The reusable probes are in
`internal/datasource/memory_read_research_test.go`. They skip by default and
require a canonical directory beneath `/private/tmp/b9s-mu5r-`. Prepare fresh
copies for comparable timings. Their metadata must bind each copy to itself,
use schema 6 and the pinned database name, and use the fixed loopback ports
above. Do not point these probes at an existing shared service. The probe
checks metadata before invoking the CLI and checks the SQL workspace before
running the optional writer.

Start the isolated Dolt and matching `bd serve` processes again before running
the server probes. Use the `server` copy's embedded data directory as the SQL
server data directory, its server-mode metadata for `bd serve`, and the two
loopback ports above. The CLI binary is the symlink under each copy's
`.memory-preview/bin/bd`. Clear inherited `BEADS_*` and `BD_*` settings and keep
`BEADS_DOLT_AUTO_START=0` when starting these processes.

```sh
B9S_MEMORY_RESEARCH_ROOT=/private/tmp/b9s-mu5r-qwp2qemo \
  go test ./internal/datasource -run '^TestMemoryReadResearch(Stages|TransportParity)$' \
  -count=1 -timeout=250s -v

# Adds one uniquely named Memory and Link to the server copy.
B9S_MEMORY_RESEARCH_ROOT=/private/tmp/b9s-mu5r-qwp2qemo \
B9S_MEMORY_RESEARCH_MUTATE=1 \
  go test ./internal/datasource -run '^TestMemoryReadResearchConcurrentWriter$' \
  -count=1 -timeout=45s -v
```

The embedded helper is preserved as
`tests/testdata/memory-read-open-probe_test.go.txt`. It is an overlay test for
the pinned preview source, because importing that engine into b9s would itself
change dependencies. Map the helper to the virtual path
`internal/storage/embeddeddolt/mu5r_probe_test.go` in a Go `-overlay` JSON file,
then run from the pinned preview source:

```sh
B9S_MEMORY_RESEARCH_ROOT=/private/tmp/b9s-mu5r-qwp2qemo \
  go test -overlay=/private/tmp/b9s-mu5r-qwp2qemo/overlay.json \
  -tags gms_pure_go ./internal/storage/embeddeddolt \
  -run '^TestMu5r' -count=1 -timeout=60s -v
```

The helper adds `mu5r-lock-probe` only after releasing the embedded handle.
Use a fresh copy for another run. It preserves the data directory and stops
only the child it started. No production reader code changed. No UI behavior
changed, so a browser video would not test the transport decision.

Validation: `go build ./...` and `go vet ./...` passed. The full
`go test ./... -skip DoltIntegration -timeout=1200s -json` run recorded
3,095 passing tests/subtests, 25 skips and no failures. Skips cover opt-in
services, preview binaries, stress runs and absent external fixtures. The
research probes ran separately with their explicit local settings. The ADR
diagram rendered successfully and was inspected with the dark theme.

## Limits and recommendation

This is one small snapshot on one machine. There is no large-graph scaling
result, network latency study, deletion lifecycle matrix or full history
equivalence proof. First-call timings are not disk-cold timings. Clean parity
does not override the observed corruption and locking differences.

Retain the CLI boundary for embedded mode and historical reads. BDP is the
preferred candidate for a future ordinary-server reader, subject to a common
snapshot contract and a history strategy. Direct SQL's speed alone does not
justify duplicating the preview's storage validation. No production transport
implementation task is opened because the replacement criteria are not met.
