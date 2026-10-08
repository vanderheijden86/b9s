# Native Memory views: TUI wires and web graph

Tasks: `bd-y6c5`, `bd-e5u3.21`. Open an HTML file directly. Assets are local and need no server
or internet connection. Each file starts with the TUI design. The **TUI / Web**
buttons switch surfaces while retaining the selection. The separate graph study
starts in Web mode. The priority designs are **01 TUI focus wires** and **04 Web
relationship graph**. Matrix and chips remain reference studies.

| Mockup | TUI | Web |
|---|---|---|
| [01 TUI focus wires](01-focus.html) | Issue/Memory rail, labelled branch wires, exact Link inspector | Focused neighbourhood companion |
| [02 Link matrix](02-matrix.html) | Keyboard cell navigation and typed Link marks | Sticky row labels, descriptive column titles, clickable cells |
| [03 Memory library](03-library.html) | Issue rows with typed Memory chips | Body-first Memory cards with record inspector |
| [04 Web relationship graph](04-graph.html) | Not proposed for the terminal | Stable node positions, selected connections, all four Link Types |

These form one consistent view family, rather than three incompatible schemas.
The web relationship graph replaces the crowded constellation. Matrix answers “what links to
what”. Library answers “what does this Memory say”. All use the same illustrative
native records and retain the distinction between Issues, Memories and Links.

## Installed Types and direction

Checked with `scripts/memory-preview bd types --json` against the isolated
`b9s-memory-poc` workspace. It reports **six Types: two Bead Types and four Link
Types**. These are installed preview definitions, not user-defined Types.

Bead Types: `preview-issue-v2` and `preview-memory-v2`.

| Link Type | Visible label / wire | Allowed endpoints | Meaning of source → target |
|---|---|---|---|
| `example-follows` | follows / solid | Issue or Memory → Issue or Memory | Source follows a policy or model described by target |
| `example-cites` | cites / dashed | Issue or Memory → Issue or Memory | Source cites target as supporting context |
| `preview-related-v2` | related / dotted | Issue or Memory → Issue or Memory | Stored informational association, no scheduling effect |
| `preview-blocks-v1` | depends on / heavy | Issue → Issue only | Source depends on target. Target blocks source |

The blocking Type's name does not change its stored direction. Drawing a source
→ target arrow labelled “blocks” would reverse its meaning. Its properties are
empty, and the source Issue owns it. The three informational Types allow an
optional note. A source Memory owns its informational Links; a source Issue does
not. Colour identifies the Link Type, never Memory health or lifecycle.

The default selected Issue has examples of all four Link Types. The additional
related edge and all content are illustrative records, not writes to the workspace.
Select a Memory to see incoming Issue references and outgoing Memory Links.
There is no blocking dependency on a Memory.

The general graph can contain informational cycles. The web design therefore
does not promise a DAG or impose a dependency hierarchy on every relationship.

## Native data boundary

The pinned preview's `graphstore.Record` has identity, Type, revision, version,
properties, owned Links and attribution. Its Memory properties are **title and
body only**. A Link has identity, Type, source, target, version, properties and
attribution. The installed informational example Types permit a note.

- Display a Memory's title and body as stored. Do not extract ADR number, date,
  lifecycle state, replacement target or decision health from the body.
- Display actual Link Types and direction. `example-follows` and `example-cites`
  are installed preview Types, not a universal built-in decision taxonomy.
- Do not manufacture `supersedes` or `child-of` Links from document text or ID
  conventions. A stored related Link remains related.
- Incoming counts, outgoing counts and “unlinked” are facts computed from the
  complete Link inventory. They are neutral, not quality judgments.
- Issue status and priority belong to Issue records. They never colour a Memory
  or imply that the Issue follows acceptable guidance.
- “Current” and “retained” identify exact record versions, not editorial status.
  Historical owned Links do not claim to be a complete historical neighbourhood.
- Attribution's own `status` field describes the attribution claim. It is not a
  Memory lifecycle field. The raw inspector preserves that native distinction.

The fixture intentionally contains ordinary prose. ADR text remains valid Memory
content, but no view needs an ADR parser. Real titles containing status words
must still be shown verbatim, without turning those words into badges or colours.

## Changes to the implemented views

| Existing surface | Proposed change |
|---|---|
| Memory list | Title, canonical ID, Type, version, incoming/outgoing counts. Literal title/body search. |
| Focus wires | Terminal first. Select any Issue or Memory from the rail. Branch wires show full Type labels and target titles. Separate outgoing and incoming sections. |
| Matrix | Issue rows and Memory columns. Use native titles and IDs. Cells contain stored Type marks, with separate Link records in the inspector. |
| Matrix sort/filter | Title or incoming count. Actual Link Type. No active/proposed order and no problems filter. |
| Chips | Type + target title. Neutral “No outgoing Memory Links”. No health marks or inferred replacement badges. |
| Web constellation | Fixed Issue/Memory cards with persistent coordinates. Highlight selected connections, filter all four Types, inspect exact edge records. |
| Detail pane | Body, Links, exact versions and fields. Notes and ownership come from complete records. |
| Main Issue tree/detail | Rename DECISIONS to MEMORY LINKS. Show native relations/counts. No decision-health ranking. |

This task changes mockups only. Implementing this model requires a superseding
ADR and a deliberate removal of `Health`, frontmatter-derived fields and inferred
supersession from the product adapter. Active ADRs are not rewritten by this design.

## Spectroscope patterns to reuse

Inspected local Spectroscope revision `6ba2bf9`:

| Source | Reuse in b9s |
|---|---|
| `src/lib/viewport.ts` | Anchor-preserving `zoomAt`, `fitToContent` and one viewport transform. This prototype adapts the zoom formula directly. |
| `src/ui/visualisation.ts`, SVG layer setup | Separate routes, nodes and transient emphasis. Fixed node positions in the focused view. |
| `src/ui/visualisation.ts`, focus handlers | Explicit selection, keyboard activation and neighbour emphasis. |
| `src/ui/app.css`, focus rules | Short opacity transitions and distinct selected outlines. |
| `src/ui/packet-inspector.ts` | A persistent inspector that presents the underlying record fields. |

The graph uses Memory rectangles and rounded Issue cards with readable
titles. Link paths have stable endpoints. Zoom and pan change one parent
transform. Selecting a node keeps every coordinate and the viewport fixed in the
web graph. Unrelated connections are dimmed, with their labels hidden. **Trace selected**
animates direction once, on request. It does not imply packets, work execution,
record freshness or a stream of changes. Reduced-motion preferences disable it.

## Optional live updates

Both surfaces expose **Refresh** and optional **Follow changes**. The outer
**Demo: incoming change** button simulates a revised native record.

- With follow disabled, the pending change waits for Refresh.
- With follow enabled, the version/body changes and selection stays in place.
- Existing node positions and the viewport remain stable across a record update.
- A real implementation should update by canonical ID and version, coalesce
  bursts, and show refresh errors without replacing the last complete snapshot.
- No continuous force simulation, activity particles or arbitrary “live” states.

Transport and invalidation remain an investigation, tracked under **`bd-mu5r`**.
Direct Dolt might avoid CLI startup/open costs. Dropping ADR parsing removes an
unwanted semantic layer, but its time cost must be measured separately.

## Interaction checks

The mockups are static local artifacts. Repository policy leaves opening mockup
files to the user. DOM-level checks exercise both surfaces without opening a
browser: selection, search, matrix cells and keyboard movement, Type filters,
versions, empty states, viewport controls and optional update delivery.

The focused regression suite is `mockups.test.cjs`. Install its DOM dependency
outside the repository, then run with a hard timeout:

```sh
npm install --prefix /tmp/b9s-native-mockup-checks jsdom --no-audit --no-fund
NODE_PATH=/tmp/b9s-native-mockup-checks/node_modules node --test --test-timeout=10000 docs/mockups/native-memories/mockups.test.cjs
```

It checks installed Type identities, dependency endpoint/property/ownership
rules, all four visible wire examples, terminal filters/navigation, graph edge
inspection, stable coordinates, viewport preservation and retained snapshots.

No application code or databases are changed by the mockups. No external assets,
fonts, telemetry or network requests are used. These checks do not replace a
browser layout review after the user selects a direction.
