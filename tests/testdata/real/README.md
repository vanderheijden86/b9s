# b9s project snapshot

`b9s.jsonl` is a sanitized `bd export` of the b9s issue database.
`b9s.count` records its exact issue count as a decimal integer with a newline.
Pod load assertions must read this file rather than hard-code a count.

The snapshot keeps the exported issue IDs, issue types, statuses, priorities,
dates, estimates, labels and every typed dependency. This includes closed work
and epic → feature → task hierarchies for tree, board and graph checks.
It is an issue fixture, not a full Dolt backup.

From the repository root, with Go and the project's `bd` connection available:

```sh
go run ./scripts/refresh-sample
go test -count=1 -timeout=60s ./scripts/refresh-sample
```

The refresh command runs `bd -C . export --readonly` with a 60-second deadline.
To name the source and destination explicitly:

```sh
go run ./scripts/refresh-sample \
  -source /absolute/path/to/b9s \
  -output /absolute/path/to/b9s/tests/testdata/real/b9s.jsonl
```

The destination directory must exist. Refresh updates both the JSONL and its
sibling `.count` file. Review and version both together. Empty, malformed or
duplicate records, wrong dependency sources and missing targets fail before
either file is written. A failed file write can leave the pair inconsistent.
The fixture test detects this and the refresh must succeed before use.

The sanitizer uses a closed field projection. It omits assignees, owners,
creators, all comments and attachment references, descriptions, notes, designs,
acceptance criteria, close reasons, external references and metadata. Unknown
fields are omitted too. Retained titles and labels redact email addresses,
URLs (including S3 and file URLs) and filesystem paths. Dispatch and lane labels
are omitted. Review retained display text before publishing a refreshed snapshot.

No test needs a live database. The focused suite uses synthetic export input,
a fake `bd` and the committed fixture. It checks privacy, JSONL parsing,
dependency integrity, nested hierarchy, deterministic output and the exact count
loaded through b9s's JSONL loader. Refresh reads the source database only.
