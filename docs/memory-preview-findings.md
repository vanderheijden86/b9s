# Memory Beads preview findings

Local review draft for task `bd-db5q.8`, evaluated 2026-10-03.

## Publication status

**Do not publish this draft.** The user prohibited GitHub comments and similar
publication. The possible destination recorded by the task is the
`versioned-beads/beads` repository, as a preview feedback issue or discussion.
No destination thread has been selected, and no posting action is authorized.

The exact draft text follows.

## Preview revision and setup

We evaluated `versioned-beads/beads` at
`1f3466b5dfcb8c76ecb9ceaf4b847df695131f96`, using an isolated embedded graph
workspace and its matching CLI. The snapshot has 32 Memories, 25 Issues and
42 Links. Thirty Memories contain complete ADR source documents. The other
two describe workspace isolation and consent for publication.

The b9s launcher uses a clean environment, the pinned CLI, and an isolated
`XDG_CONFIG_HOME`. Ordinary b9s and the experimental graph workspace use
different binaries and credentials. No shared database was migrated.

## Reproduction

From the b9s feature checkout with the documented POC workspace:

```sh
scripts/memory-preview bd status --graph --json
scripts/memory-preview bd memories 'embedded Dolt' --all --details --format records-json
scripts/memory-preview bd memories 'permission to publish' --all --format records-json
scripts/memory-preview bd links sample-hffz.18 --json
scripts/memory-preview bd links sample-vd9g.1.6 --json
scripts/memory-preview bd versions adr-0030 --json
scripts/memory-preview bd prime
scripts/memory-preview b9s
```

The final `bd prime` command is expected to refuse the unsupported capability.
In the viewer, use `2` for focus wires, `3` for matrix and `4` for chips. The
matrix supports cell navigation, usage sorting and a problems filter. Retained
version selection is available from an individual Memory.

## Findings and proposed improvements

1. **The graph adds useful work-to-decision references.** A public-demo Issue
   cites the embedded SPA and pairing decisions while following a proposed
   public-mode decision. An activity-view Issue follows a superseded ADR.
   These links expose concrete review questions that separate ADR files do
   not answer. Preserve this distinction between citing context and following
   a requirement.
2. **Search is literal.** The initial evaluation found the consent Memory with
   `explicit consent` but not `permission to publish`. A natural-language
   database question also returned no results. Document this boundary in
   graph-specific help and avoid implying semantic retrieval or automatic
   prime-time injection.
3. **Decision status is unstructured.** b9s reads ADR frontmatter to distinguish
   active, proposed and superseded documents. Searching for a status word also
   finds documents merely mentioning it. Consider a decision Type with explicit
   status and supersession semantics.
4. **Complete graph reads require multiple process starts.** The inventory
   includes Memory-owned Links, but Issue informational Links require traversal.
   Embedded BDP serving is unavailable. b9s improved startup from 13.18 to
   6.89 seconds by traversing only components that can contain unowned Links.
   A complete bulk Link read would simplify this adapter and reduce latency.
5. **Generic traversal returns summaries.** Unowned Link notes require incident
   reads. b9s preserves complete owned records ahead of summaries and obtains
   full incident properties in individual Memory detail. Consider an explicit
   complete-record traversal or a bulk Link inventory.
6. **Versions support exact recall, not full historical replay.** Updating a
   Memory retains its earlier body. A retained Memory carries Links it owned
   at that version. Informational references do not identify the Memory version
   consulted by the author. Consider a supported version reference on citations.
7. **Example Link properties are closed.** The current examples support notes,
   and hierarchy uses a related-Link naming convention in this POC. Production
   hierarchy and decision relations need defined Types rather than conventions.
8. **Machine-readable bounds are useful.** b9s refuses partial inventories and
   incomplete traversals. Preserve explicit completeness indicators and add
   pagination before increasing workspace limits.

## Verification and limits

Real CLI and terminal checks passed 10 tests. Browser checks passed 70 tests
across five configurations plus six checks against the actual POC graph.
Graph optimization compares complete node/edge content with an unpruned read.

This is one local evaluation of a manually seeded snapshot. It does not measure
multi-agent retrieval quality, production reliability or automatic knowledge
maintenance. The initial seed evidence and later citation experiment are in
[memory-poc-evaluation.md](memory-poc-evaluation.md).
