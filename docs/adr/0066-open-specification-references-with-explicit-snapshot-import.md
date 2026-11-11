---
type: ADR
id: "0066"
title: "Open specification references with explicit snapshot import"
status: proposed
date: 2026-11-11
superseded_by: null
---

## Context

[GitHub #18](https://github.com/vanderheijden86/b9s/issues/18) describes plans
decomposed into Beads issues with `--spec-id`. The producer and b9s run on
different VMs, so the producer cannot run `b9s attach` during decomposition.
The reporter asks for resolvable artifacts without a local or S3 backend, or
for copying an artifact into a backend when opening it. The issue body and
complete comments response were read on 2026-11-11. There were zero comments
and no screenshot or image references in that content.

The [Beads create reference](https://beads.gascity.com/cli-reference/create)
describes `--spec-id` as a specification link. Its graph example uses
`"spec_id": "bd-spec1"`. This does not establish a transport protocol, a
filesystem root, or that every value is a URL. Preserve the value literally.
An opaque document identifier must not become a guessed local filename.

Current `main`, inspected at `c2fccef8b537ed034ebd360ea7cf31c02250e083`,
does **not** support this request:

- [`model.Issue`](../../pkg/model/types.go) has no `spec_id` field.
  [`loader.ParseIssues`](../../pkg/loader/loader.go) discards it on JSONL read.
  The full and lean [Dolt readers](../../internal/datasource/dolt.go) and
  [SQLite reader](../../internal/datasource/sqlite.go) do not select it.
- The [TUI attachment picker](../../pkg/ui/attachments.go) and
  [`attach.List`](../../internal/attach/list.go) collect only
  `beads-attachment/v1` reference comments. They do not resolve spec links.
- [Store selection](../../internal/blobstore/open.go) supports local and S3.
  The TUI can list comments without a store, but opening needs one. The
  `b9s attach` CLI requires configured storage even to list.
- A read-only probe of that checkout parsed four spec-only JSONL issues:
  a relative path, an absolute producer path, an HTTPS URL and an opaque ID.
  Each produced one issue, omitted `spec_id` when serialized, and returned
  zero entries from `Model.AttachmentList()`. This confirms the missing
  behavior without changing the feature or adding a product test.

[ADR 0024](0024-attach-files-through-reference-comments.md) remains the
contract for stored attachments. This proposal adds external references
alongside them. It does not change their format, detach fold, garbage
collection, or Beads schema. The
[architecture](../ARCHITECTURE.md) continues to describe shipped behavior.

## Decision

**Preserve `spec_id` and present it as a distinct Specification reference
beside stored attachments. Open a reachable reference only on an explicit
user action, without requiring a blob backend. Offer a separate explicit
“Import specification snapshot” action through the existing attachment
backend. Viewing or opening must never silently import.**

This is a proposed policy and implementation contract. This task changes
documentation only. It introduces no resolver, transport, migration or UI.
The complexity is in reference identity, host boundaries and trust, rather
than bulk data movement or a durable workflow. Small shared resolver code
and the existing attachment pipeline fit those contracts.

### Reference interpretation and cross-VM access

| Input | Planned behavior |
| --- | --- |
| Empty value | No Specification entry. |
| Explicit relative path, such as `./plan.md` or `docs/plan.md` | Resolve under the operator's trusted root for the active project, never the process working directory. |
| Absolute native path | Require containment in a trusted root, or an explicit producer-root to local-root mapping for that project. |
| Absolute HTTPS URL | Offer explicit navigation in the user's browser. No server fetch is needed for this action. |
| Opaque ID, including `bd-spec1` | Show the original value with an unsupported-resolution reason. Do not guess a path or follow another issue. |
| Other URI schemes, `file:` URLs, malformed URLs or ambiguous bare names | Show the value with an unsupported-resolution reason. A bare filename can be made explicit with `./`. |

For example, a producer may write `/workspace/project/docs/plan.md`.
On the b9s VM, an operator may map `/workspace/project` to an existing,
trusted `/srv/projects/project` checkout or read-only mount. The remaining
suffix is resolved beneath that root with containment checks. Without that
mapping, b9s reports that the producer path is unavailable on this host.
It must not search other projects, contact the producer, run SSH, clone a
repository, or treat a matching filename as proof of identity.

Alternatively, the producer can publish a reachable HTTPS document and
record that URL in `spec_id`. This needs no b9s installation on the producer
and no attachment store. Browser authentication belongs to the user's
browser session. b9s does not inherit it for imports.

Roots and mappings belong to trusted operator configuration, scoped by
project identity. Issue content, repository metadata and the database may
not grant filesystem access. A catalog-only project with no configured
root cannot resolve relative paths. A mapping can select different bytes
on different VMs or branches. Opening shows the local working copy, not
proof of the producer's revision. Use an immutable published URL or import
a reviewed snapshot when that distinction matters.

### Safe opening

- Loading, rendering, reloading and listing references perform no filesystem
  reads of the target, network requests, imports, or automatic opener calls.
  Keep the raw spec value separate from the resolved target and its origin.
- Check local targets against the trusted root after normalization and
  symlink resolution. Reject escapes, devices, sockets, FIFOs and directories.
  Use a confined open and the opened descriptor for previews or imports,
  so a symlink swap cannot escape between checking and reading.
- Render literal text safely. Reject control characters, NULs, URL userinfo,
  scheme-relative URLs and unsupported schemes before any action. Do not
  expand environment variables, `~`, shell syntax or command templates.
- Reuse the content and extension opener allowlist and literal argv from
  [`attachments.go`](../../pkg/ui/attachments.go). A path must not reach
  the OS opener solely because it came from `spec_id`. Preserve
  `B9S_TEST_MODE` and no-browser behavior in tests.
- HTTPS navigation uses an explicit link with a visible origin and safe
  browser link attributes. A browser's `file:` URL cannot open a server's
  filesystem. The web UI must report local references as unavailable unless
  a separately authorized, authenticated server preview is implemented.
- Scope asynchronous work to the active project and selected spec value.
  Cancel on project change and discard stale replies. Bound resolver and
  opener operations. Failures stay visible on the reference and leave
  ordinary issue browsing available. Retry requires another user action.

### Explicit snapshot import

Import requires a configured backend and the user's explicit import action.
It reads the same confined regular local file, or downloads a credential-free
HTTPS resource through a separate guarded importer. A browser login link
may open successfully while import is unavailable. Do not fall back from a
failed open to import.

The HTTPS importer must require an operator-approved exact origin for the
active project. Enforce HTTPS, TLS certificate checks, a bounded redirect
count within that origin, and URL checks on every redirect. Reject loopback,
private, link-local, unspecified, multicast and other special-use addresses,
including cloud metadata targets and IPv4-mapped IPv6 forms. Check DNS
results and pin an approved address for each connection to prevent rebinding.
Disable ambient proxies and do not forward cookies, Authorization headers,
URL credentials or attachment-store credentials. Do not log query secrets.
Use local fixture transports for tests, never an exception in production
policy that an issue can select.

Bound the complete import by the existing two-minute attachment command
deadline, with shorter connection and response deadlines. Limit streamed,
decoded bytes to the configured attachment maximum regardless of the
declared length. Validate names and sniff MIME from bytes. Stage a private
snapshot, hash those same bytes, and use the existing upload-before-comment
ordering. The final mutation uses stock `bd comments add` through `bdrun`.
Check project identity and spec value before the mutation, and retain the
local-with-shared-Dolt refusal from `blobstore.Open`.

The stored attachment is an immutable snapshot. Leave `spec_id` unchanged
and label the imported item as a snapshot rather than a live mirror. Before
a sequential retry, reload reference comments and reuse a live identical
hash rather than append another reference. Concurrent importers can still
append duplicate comments without a cross-process transaction. The existing
fold yields one live item per hash. Do not claim exactly-once mutation.

A failed upload creates no reference. A failed comment or crash after
upload may leave an orphan, handled by the existing GC grace period.
Import must surface that failure and offer a bounded manual retry.
Content-addressed storage deduplicates identical bytes. Import performs no
background refresh and never overwrites a previous snapshot when the source
changes. Detach applies to snapshots only, not to `spec_id`.

## Options considered

| Option | Benefits | Costs and limits |
| --- | --- | --- |
| Direct references plus explicit snapshot import (chosen) | No producer-side b9s requirement. Reachable links work without storage. Operator chooses live source or retained bytes. Existing attachment contract remains usable. | Needs reference classification and host trust rules. An inaccessible producer file still needs publication or a mount. |
| Direct references only | Least storage and no write when opening. HTTPS uses existing browser authentication. | Source can move or change. No offline snapshot. Producer-only files remain inaccessible. |
| Copy into local or S3 storage on first view/open | Reuses existing attachment opening and retains bytes after the first successful copy. | Cannot read a missing producer file. A view becomes a write, needs credentials and may fail for read-only users. Hidden copies become stale. A local copy remains confined to one VM. |
| Require producers to use `b9s attach` | One existing attachment path and explicit upload ownership. | Fails the reporter's separate-VM workflow and requires b9s plus storage access at decomposition time. |
| Automatically resolve remote filesystem paths over SSH | Could access producer-only files with credentials. | A path contains no host or authorization contract. Introduces remote command execution and credential management outside this feature. |

## Consequences

No new backend is needed to open an HTTPS specification or an accessible,
trusted file. Copying on demand cannot by itself solve cross-VM reachability.
A local imported snapshot is accessible only where that backend is mounted.
Use the existing S3 backend for shared snapshot bytes and keep the local
backend's shared-Dolt guard.

Readers must preserve `spec_id` across JSONL, SQLite, Dolt full and lean
queries, first-page and delta reads, detail hydration, clones and relevant
web API types. Older schemas without the optional column remain readable.
A missing optional column must not conceal an unrelated query failure.
No migration is needed. Opaque IDs remain visible until their publisher
provides a documented resolution contract.

The resolver uses a closed classification in code. Per-issue spec values
remain data, and host roots, mappings and origin grants remain operator
configuration. No persistent import job or background retry machine is
introduced. The state-machine checklist therefore applies to the bounded
foreground operation: one owner, explicit user events, deadlines with visible
failure, cancellation on project change, and idempotent snapshot reuse.
Reference failures do not change the issue lifecycle.

Revisit this decision if Beads defines resolvable spec identifiers or a
native artifact transport, or if authenticated publication becomes necessary.
Do not add arbitrary scheme handlers as configuration shortcuts.

## Follow-up tasks

These tickets were filed with `discovered-from:bd-teri.13` and deferred for
implementation after design review. The intended hierarchy is feature
`bd-cbs3` under epic `bd-teri`, with the three tasks beneath the feature.
The lane CLI rejected `--parent`, so the dispatcher must assign that hierarchy.
Execute the tasks in the order below. This ADR adds no task dependencies.

- **bd-cbs3, feature:** Open specification references across b9s hosts.
- **bd-psgc:** Preserve `spec_id` in every issue reader. Test-first fixtures
  must cover paths, HTTPS, opaque IDs, empty values, absent columns, cloning,
  hydration and reload parity without target I/O.
- **bd-0alf:** Open specification references without a blob backend.
  Add the shared resolver and TUI entry, with root and symlink containment,
  unavailable cross-VM paths, literal openers, project-switch cancellation,
  read-only use and bounded PTY coverage. Document two VM roots and HTTPS
  publication. Browser-local paths must remain unavailable.
- **bd-6oqw:** Import a reachable specification snapshot explicitly.
  Cover address and redirect policy, DNS rebinding, byte and time limits,
  source races, MIME checks, backend guards, upload/comment failures,
  sequential retry and concurrent duplicate folds. Document local versus
  shared snapshot storage and prove the flow with local fixtures.

## Advice

The existing architecture and ADR 0024 informed this proposal. No reporter
feedback beyond the issue body was available. Security and cross-VM examples
above are design requirements for the follow-up tasks, not claims of shipped
or tested resolver behavior.
