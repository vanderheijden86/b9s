# Memory POC seed and retrieval evaluation

The sections through "Evidence and next design review" preserve the initial
2026-10-02 evaluation. The final section records the expanded snapshot and
current viewer checks. Initial timings describe the earlier eager viewer.

[Watch the three-minute feature proof](videos/memory-epic--feat-bd-87qs-memory-beads.mp4).

Task: `bd-db5q.5`. Evaluated on 2026-10-02, using the pinned preview documented in [memory-poc.md](memory-poc.md).
This is a local CLI evaluation in one agent session. It is not a multi-agent retrieval benchmark.

## What was seeded

| Data | Count | Provenance |
|---|---:|---|
| ADR Memories | 30 | Every numbered document in this clone's `docs/adr`, preserved in full |
| Instruction Memories | 2 | Summaries of POC isolation and explicit consent for GitHub writes |
| Issue snapshots | 5 | Selected from a fresh export of 1,388 b9s Issues |
| Informational Links | 12 | Source citations, policy references, and five ADR supersession references |
| Blocking Dependencies | 2 | Source relationships between the selected setup, seed, and UI tasks |

The graph contains 37 Beads. It is a one-way snapshot, not a synchronized copy of b9s.
The ADR set contains 22 active, five superseded, and three proposed decisions. Each Memory title includes its source status.
Each ADR body contains a provenance header followed by the complete original Markdown, including front matter and diagrams.
No source ADR was edited.

Source checkout revision: `5583534e33d521b41d9f1376ae93bb597c3ad025`.
The ignored local directory `.memory-preview/seed-db5q.5/` contains the original export, source hashes, manifests, command outputs, and evaluation evidence.
The export's SHA-256 is `c90c0f8664029355cd7d252ab33c9d187cc8b5361b96ca192b46b19279554fa1`.
The export is an Issue export, not a full Dolt backup. It excludes legacy Memories and infrastructure records by default.

## Issue mapping and omitted data

| Original b9s ID | Preview canonical path | State in snapshot |
|---|---|---|
| `bd-hffz.2` | `beads/sample-hffz.2` | closed |
| `bd-db5q.2` | `beads/sample-db5q.2` | closed |
| `bd-db5q.4` | `beads/sample-db5q.4` | closed |
| `bd-db5q.5` | `beads/sample-db5q.5` | open, with original `in_progress` recorded in notes |
| `bd-db5q.6` | `beads/sample-db5q.6` | open |

Titles identify snapshots. Description, priority, Issue classification, and original notes were copied.
Notes also record the source ID and source status. `external_ref` carries `source:b9s:SOURCE-ID`.
The preview creates a separate internal Issue ID in addition to its canonical Bead path. Use the canonical path for graph commands.

Three closed states and their close reasons were recreated through the preview CLI.
The in-progress source task was deliberately not claimed in the experimental store. Its source status remains in the snapshot notes.
Original timestamps, owner, author, and lifecycle history were not replayed. New preview records have local creation attribution and timestamps.
The original values remain in the retained export. These five selected Issues have no exported comments.
Parent-child relationships were not converted into blocking Dependencies. Their parent Issues were outside the selected sample.
The two source blocking relationships were recreated with their original direction: seed depends on setup, and UI depends on seed.
Graph mode has no `bd import` route. This conversion used supported CLI operations and did not write the graph tables directly.

## What retrieval found and missed

| Query or action | Observed result |
|---|---|
| Recall each Memory by canonical ID | All 32 bodies matched the seed text exactly |
| Search `embedded` or `EMBEDDED` | Same six results, including unrelated uses of the word |
| Search `embedded Dolt` | ADRs 0025, 0029, and 0030 |
| Search `select-only` | ADR 0007 |
| Search `control socket` | ADR 0023 |
| Search `explicit consent` | GitHub consent Memory |
| Search `permission to publish` | No results despite the related consent Memory |
| Ask `why does b9s avoid opening the database directly` | No results despite ADR 0025 |
| Search `fuzzy` | ADRs 0001 and 0002, both superseded, plus active ADR 0004 |
| Search `superseded` | Seven results, including active ADRs that mention the word |
| Search `proposed` | Five results, including documents that mention the word in their body |
| Run `bd prime` | Refused with exit 5, `capability_unavailable` |

This preview searches literal text in titles and bodies. It does not infer synonyms or answer natural-language questions.
Status words are searchable text, not structured filters. A matching superseded decision must not become current guidance merely because it matched.
Provenance at the beginning of a body can dominate the 160-character search excerpt when the filename matches the query.
Agents need explicit search, source-status inspection, and recall. The legacy help text about prime-time injection does not apply to this graph workspace.

## Links, versions, and the viewer

ADR 0030 has two outgoing `example-cites` Links and one incoming `example-follows` Link from the viewer Issue snapshot.
The b9s viewer shows its full body, version, both arrows, endpoint titles, and Link notes.
The complete viewer also reads all 32 Memories. In this single run, selecting one Memory took 14.734 seconds and listing all took 14.191 seconds.
The adapter loads all incident Links before applying the selected-ID filter. These are local observations, not a performance benchmark.

Five supersession references use `preview-related-v2` plus a note saying the source ADR was superseded by the target.
That example Type does not enforce supersession semantics. It preserves the reference for this evaluation without inventing a production Type contract.
Informational Links do not change Issue scheduling.

Adding Memory-owned Links changed ADR 0030's version. Recalling its original version still returned the exact original body.
This checks retained version selection, not complete historical replay.
Discovery's `ownedLinkCount` describes outgoing owned Links. It does not count incoming references from Issues. Use `bd links ID` for both directions.

## Evidence and next design review

The evaluation completed with **47 passed, zero failed, zero skipped checks**.
`evaluation.json` records each bounded check and command output. `viewer-all.txt` captures the complete rendered view.
An additional comparison of the five captured create responses checked descriptions, priorities, classifications, original notes, and source references against the export.
Each subprocess had a 45-second deadline. This task changed seed data and documentation, not application code.
The initial informational Link call used a bare `--id` and was refused before mutation. The successful calls use `--id links/ID`.

Task `bd-db5q.6` owns the user-reviewed UI design. Evidence from this seed suggests:

- Show ADR status and source references alongside search results.
- Provide full-body recall after compact search results, with clear literal-search behavior.
- Keep incoming and outgoing Links visible, separate from blocking Dependencies.
- Label snapshot Issues and keep source lifecycle metadata distinguishable from preview metadata.
- Offer retained-version selection without presenting it as a complete source history.
- Avoid loading every Memory's Links when the user selects one Memory.

These are inputs to that review. The integrated TUI design has not been finalized.

## Expanded citation experiment and viewer checks, 2026-10-03

Tasks `bd-db5q.11`, `.7`, `.18`, `.19`, and `.21` extend that evaluation. The
isolated snapshot contains **25 Issues, 32 Memories and 42 Links**. Twenty
additional open epic/feature snapshots, eight hierarchy Links and twenty
decision Links were created through the preview CLI. Source data was read only.
Snapshot Issues are not synchronized with the current b9s issue database.

| Work | Persisted reference | What the relationship adds |
|---|---|---|
| Public demo (`sample-hffz.18`) | cites 0019 and 0020, follows proposed 0029 | Separates reuse of the SPA from relaxing pairing and depending on a proposed decision |
| Activity view (`sample-vd9g.1.6`) | follows superseded 0009, cites 0007 and 0025 | Exposes a stale decision reference and identifies shared-server and embedded-read constraints |
| Agenda and flag migration (`sample-36s`, `sample-n0wb`) | no Links | Shows missing decision coverage without inventing a governing ADR |

The activity Link note explicitly says to implement against ADR 0027. The
graph surfaces the contradiction between the recorded target and its advice.
The public-demo Link note identifies proposed ADR 0029 as an acceptance gap.
These relationships connect work to decisions, information that an ADR file
alone does not contain. They remain manually authored claims, not proof that
an implementation conforms to the referenced decision.

The terminal offers list, focus wires, matrix and chips views. The browser
offers a D3 constellation with directed Links, pan/zoom, node selection and a
problem filter. Status comes from Memory frontmatter, not title text. The
Memory browser supports literal search, full bodies and retained versions.

Verification against fresh isolated embedded stores checked changed bodies,
retained owned Links, incoming Issue references, literal search, refused
unknown revisions, ordinary Issue state, printed detail and terminal version
selection. All **10 tests passed**, with no skipped tests. A PTY harness hang
was traced to an unclosed input pipe and fixed before the final run.

Browser checks passed **70 tests across five browser/device configurations**.
The real 57-node/42-Link POC passed **six additional checks**, including directed
markers, body selection, problem filtering, phone rendering and console errors.
No test wrote to the shared Dolt server.

Final review found a race between the browser snapshot and graph response.
The browser refuses a mismatched version. An explicit retry refreshes the
snapshot once before requesting the graph. The regression passed in all five
browser/device configurations.

The complete graph read improved from **13.18 to 6.89 seconds** on this machine.
An unpruned read and the optimized read returned equal node and edge content.
Memory-only components use complete owned records from the inventory. Issue
and unknown-Type components still traverse both directions. The regression
also checks that complete owned Link properties survive deduplication.
See [ADR 0040](adr/0040-traverse-only-components-with-unowned-links.md).

Reproduce with the commands in [testing.md](testing.md). The preview source
remains pinned to `1f3466b5dfcb8c76ecb9ceaf4b847df695131f96`. The launcher selects
that binary and an isolated config directory. Findings for review are in
[memory-preview-findings.md](memory-preview-findings.md). Nothing was published
as part of this evaluation.
