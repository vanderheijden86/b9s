# Attachments in Comments Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Attach files to Beads issues from b9s, with the bytes in an S3-compatible
store and a reference in an ordinary Beads comment, so stock `bd` keeps working.

**Architecture:** ADR 0024. b9s hashes and uploads a file, then runs
`bd comments add` with a versioned reference line. Every data source already loads
comments; b9s parses reference lines out of them, lists attachments in the detail
pane, and opens a file by download or presigned URL. No Beads schema change.

**Tech stack:** Go 1.25, aws-sdk-go-v2 (S3), bubbletea/huh, the existing `bd` write path.

## Table of contents

- [Starting point](#starting-point)
- [Reference format](#reference-format)
- [Configuration](#configuration)
- [Task 1: blob store package in b9s](#task-1-blob-store-package-in-b9s)
- [Task 2: reference format parser](#task-2-reference-format-parser)
- [Task 3: attachment config and store factory](#task-3-attachment-config-and-store-factory)
- [Task 4: b9s attach subcommand](#task-4-b9s-attach-subcommand)
- [Task 5: detail pane shows and opens attachments](#task-5-detail-pane-shows-and-opens-attachments)
- [Task 6: add from the TUI](#task-6-add-from-the-tui)
- [Task 7: garbage collection](#task-7-garbage-collection)
- [Task 8: infrastructure](#task-8-infrastructure)
- [Task 9: web image and E2E](#task-9-web-image-and-e2e)
- [Task 10: attachments table reader (deferred)](#task-10-attachments-table-reader-deferred)
- [Out of scope](#out-of-scope)

## Starting point

The Beads fork branch `feat/attachments-object-store`
(`vanderheijden86/beads`) holds a finished, reviewed package
`internal/attachments/blobstore`: `Store` interface, `Key()`, `Local` and `S3`
backends, a shared contract test, and `scripts/attachments-minio.sh` (runs
`adobe/s3mock:3.11.0` on `127.0.0.1:19000`). It imports only the standard library
and aws-sdk-go-v2. Task 1 copies it; the rest of the fork branch is parked.

## Reference format

A reference comment has a human line, then one machine line. Only the machine line
is parsed; the human line is for stock `bd show`.

```
📎 screenshot.png (image/png, 118 KB)
beads-attachment/v1 sha256=<64 hex> size=<bytes> type=<mime> name=<percent-encoded>
```

```
📎 detached screenshot.png
beads-attachment-detach/v1 sha256=<64 hex>
```

Rules:

- The machine line starts at column 0 with the exact marker and `/v1`. Fields are
  space-separated `key=value`, order-free, each key at most once. Unknown keys are
  ignored, so v1 readers accept later additive fields.
- `sha256` is 64 lowercase hex. `size` is a decimal int64 >= 0. `type` is a MIME
  type that `mime.ParseMediaType` accepts. `name` is `url.PathEscape`d and decodes
  to a base name with no `/`, `\` or control characters.
- A line that starts with the marker but breaks a rule is ignored and reported to
  the debug log. It never crashes the reader and never yields a partial reference.
- An unknown version (`/v2`) is ignored by a v1 reader.
- The attachment list of an issue is the fold of its comments in `created_at`
  order (id as tie-break): an attach adds or replaces by hash, a detach removes by
  hash. Attaching a hash again after a detach shows it again.
- The blob key is `blobstore.Key(prefix, database, "sha256", hash)`. The comment
  never names a bucket, endpoint or prefix.

## Configuration

`~/.config/b9s/config.yaml`, new section:

```yaml
attachments:
  backend: s3              # s3 | local
  max_bytes: 26214400      # 25 MiB
  gc_grace: 24h
  local_dir: ""            # local backend only; default <beads dir>/attachments
  prefix: osenco            # the workspace; default is the project's database name; applies to both backends
  local_with_dolt_server: false # required true to use the local backend when the source is a Dolt server
  s3:
    endpoint: https://nbg1.your-objectstorage.com
    region: nbg1
    bucket: osenco-beads-attachments
    path_style: false
    url_ttl: 15m            # at most 168h (7 days): the S3 presign limit
    credential_command: "" # prints two lines: access key id, secret
```

Secrets never go in the file: `backend`, `max_bytes`, `gc_grace`, `local_dir`,
`prefix`, `local_with_dolt_server` and `s3` are the only keys an `attachments`
mapping accepts, and `endpoint`, `region`, `bucket`, `path_style`, `url_ttl` and
`credential_command` are the only keys under `s3`; any other key, or a YAML alias
or merge node anywhere in the section, fails to load. Secrets come from
`B9S_ATTACHMENTS_S3_ACCESS_KEY_ID` and `B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY`, or
from `credential_command`. The database segment of the key is the Dolt database
name, or the project name for SQLite and JSONL sources. The `local` backend is
refused for a Dolt server source unless `local_with_dolt_server: true` is set:
b9s's shared Dolt server is reached through an SSH tunnel and looks like loopback
locally even when other operators share it, so a host-based loopback check cannot
tell the two apart.

## Task 1: blob store package in b9s

**Files:** create `internal/blobstore/` from the fork package (blobstore.go,
local.go, s3.go and their tests); create `scripts/attachments-s3.sh` from the
fork's `attachments-minio.sh`; modify `go.mod`/`go.sum`.

- Copy the files at fork commit `013392226`; change only the package import path.
- The gated S3 test keeps `BEADS_TEST_S3_ENDPOINT`, renamed `B9S_TEST_S3_ENDPOINT`.
- Tests: the whole contract suite, once without and once with the S3 test server.
- Commit: `feat(attach): content-addressed blob store with local and S3 backends`.

## Task 2: reference format parser

**Files:** create `internal/attachref/attachref.go` and `attachref_test.go`.

API:

```go
type Ref struct {
    SHA256 string
    Size   int64
    Type   string
    Name   string
}
func Format(r Ref) (string, error)          // human line + machine line
func FormatDetach(r Ref) (string, error)
func Parse(text string) (refs []Ref, detaches []string)
func Collect(comments []*model.Comment) []Attachment // fold in time order
type Attachment struct { Ref; CommentID string; AddedBy string; AddedAt time.Time }
```

Tests: round trip for names with spaces, `%`, unicode and quotes; every rule in
[Reference format](#reference-format) has a rejecting case; fold order with detach
and re-attach; a comment with ordinary text around the machine line; `FuzzParse`
that asserts Parse never panics and every returned Ref passes validation.

Commit: `feat(attach): versioned attachment reference format`.

## Task 3: attachment config and store factory

**Files:** modify `pkg/config/config.go` (+ tests); create
`internal/blobstore/open.go` (`Open(cfg, source) (Store, KeyFunc, error)`).

Tests: defaults; secrets from env and from `credential_command` (fake command
script in `t.TempDir()`); the local backend refused for a Dolt server source
without `local_with_dolt_server: true`; `max_bytes` and `url_ttl` parsing; a
config file with an unknown key, or a YAML alias or merge node, under
`attachments` is rejected with a clear error.

Commit: `feat(attach): attachment settings and blob store selection`.

## Task 4: b9s attach subcommand

**Files:** create `cmd/b9s/attach.go` (+ tests), dispatched like `ctl` in `main.go`.

```
b9s attach <issue-id> <file>...          upload, then bd comments add
b9s attach --detach <issue-id> <sha256>  bd comments add with a detach line
b9s attach list <issue-id> [--json]
b9s attach get <issue-id> <sha256|name> [-o path]
b9s attach url <issue-id> <sha256|name>  presigned URL (S3 only)
```

- Order: hash while streaming to a temp file, reject above `max_bytes`, sniff MIME
  (`http.DetectContentType`, extension as tie-break), `Put`, then `bd comments add`.
- `bd` runs through the same resolver and environment as `IssueWriter`, in the
  project's directory, so it writes to the same database b9s reads.
- `get` verifies the sha256 of the downloaded bytes before it writes the output.
- Tests use the local backend and a fake `bd` on `PATH` that records its argv.

Commit: `feat(attach): b9s attach uploads files and writes reference comments`.

## Task 5: detail pane shows and opens attachments

**Files:** modify the detail rendering in `pkg/ui/` (find where comments render),
`pkg/ui/model.go` for the key, README key table.

- Attachments render as a list above the comments: name, type, size, author, age.
- Reference comments render as their human line only; the machine line is hidden.
- A key (check `model.go` for a free one; `A` if free) opens a picker over the
  issue's attachments. Enter downloads to a temp dir and opens with the system
  opener; in the web image (`B9S_WEB=1`) it prints an OSC 8 presigned link instead.
- `B9S_TEST_MODE` suppresses the opener. Test accessors expose the attachment list.

Commit: `feat(ui): list and open attachments in the detail pane`.

## Task 6: add from the TUI

A huh form asks for one or more paths (tab completion if huh supports it),
runs the Task 4 code path in a `tea.Cmd`, and shows progress and errors in the
status line. Test through the model with a fake `bd` and the local backend.

Commit: `feat(ui): attach files from the detail pane`.

## Task 7: garbage collection

`b9s attach gc [--dry-run] [--grace 24h]`: list blobs under `prefix/database`,
collect every referenced hash from all comments of the database (detached ones
count as unreferenced), and delete blobs that are unreferenced and older than the
grace period. Default is `--dry-run`; `--apply` deletes. Test with the local
backend and fixed clocks.

Commit: `feat(attach): collect unreferenced blobs`.

## Task 8: infrastructure

Beads task `bd-t8j5.8`: bucket `osenco-beads-attachments` on Hetzner Object
Storage, one key pair per workspace scoped to its prefix, secrets in the workspace
`.envrc`, a weekly `b9s attach gc --apply`, and a smoke test of every Store method
and a presigned download against the real bucket.

## Task 9: web image and E2E

Beads task `bd-t8j5.11`: the web image passes the attachment env through the login
proxy, and a PTY E2E test attaches a file, sees it in the detail pane and opens it
(with `B9S_TEST_MODE`), against the local backend.

## Task 10: attachments table reader (deferred)

Only when upstream Beads merges an `attachments` table: the Dolt and SQLite readers
check `information_schema` for it, read its rows, and `Collect` merges them with
reference comments by hash. Deferred with no date.

## Out of scope

- Changes to Beads or `bd`.
- Thumbnails or inline image rendering in the terminal.
- Encryption beyond what the bucket provides.
