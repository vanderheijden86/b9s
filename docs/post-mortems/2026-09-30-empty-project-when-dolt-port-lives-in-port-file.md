# Post-mortem: b9s shows an empty project when bd's Dolt port lives only in the port file

- **Date reported:** 2026-09-30, on Discord
- **Affected versions:** v1.3.0 and earlier
- **Fixed in:** [v1.3.1](https://github.com/vanderheijden86/b9s/releases/tag/v1.3.1)
- **Beads:** bd-2glh (JSONL fallback), bd-ptiw (hint text), bd-pvrr (port resolution), bd-vb0j (release)
- **Find the fix:** `git log --grep=bd-pvrr`
- **Status of the diagnosis:** likely cause. The reporter has not yet confirmed their `.beads` contents.

## Summary

A user started b9s in a Beads project and saw no issues. The only message was:

```
Warning: skipping invalid issue on line 1: issue ID cannot be empty
No issues to display.
```

The same project showed its issues in another Beads viewer that reads through bd. Two defects combined. b9s dialled the wrong Dolt port, and when that connection failed it opened a `.jsonl` file that holds no issues. The second defect hid the first: the user saw an empty project instead of a refused connection.

## Impact

- Projects in Dolt **server** mode whose `metadata.json` names no port. That is the normal result of letting bd start its own server.
- The project opened with zero issues and gave no hint that the server was the problem.
- Projects in **embedded** mode were not affected. b9s reads them through `bd export` and reports any failure directly.

## What happened

```
 .beads/
 ├── metadata.json       { "dolt_mode": "server", ... }     no port
 ├── dolt-server.port    52817                              the port bd's server listens on
 └── <journal>.jsonl     {"...": ...}                       a bd file, not issues

 b9s v1.3.0 starts
     │
     ▼
 metadata.json has no port ──▶ assume 3306
     │
     ▼
 dial 127.0.0.1:3306 ──▶ connection refused
     │
     ▼
 fall back to local JSONL: FindJSONLPath takes any non-empty .jsonl
     │
     ▼
 <journal>.jsonl, line 1 has no id ──▶ "skipping invalid issue on line 1"
     │
     ▼
 zero issues ──▶ "No issues to display."    the refused connection is never shown
```

## Root cause

1. **Port resolution did not match bd.** bd starts a per-project server on a free port and records it only in `.beads/dolt-server.port`. bd reads the port from the environment, then that file, then the Dolt server's `listener.port`, then `dolt.port` in `config.yaml`, and last from `metadata.json`. b9s read only `metadata.json` and defaulted to 3306.
2. **The JSONL fallback trusted any file name.** `loader.FindJSONLPath` accepted any non-empty `.jsonl` in `.beads` when no canonical export existed. bd keeps other journals there, whose records are not issues.
3. **A fallback that loads nothing counted as a success.** Because a file opened, `OpenProject` did not report the Dolt failure that came before it.

The trigger was a normal bd setup. The root cause was that b9s kept its own copy of bd's address rules, and that copy was out of date.

## Fix (v1.3.1)

```
 b9s v1.3.1 starts
     │
     ▼
 resolveDoltAddress: env ▶ dolt-server.port ▶ listener.port ▶ config.yaml ▶ metadata.json
     │
     ▼
 dial 127.0.0.1:52817
     │
     ├── server up ──▶ issues shown
     │
     └── refused ──▶ fall back only to a file whose first record is a valid issue
                         │
                         ├── export found ──▶ export shown, with a note that the server failed
                         │
                         └── none ──▶ "Cannot reach the Dolt server at 127.0.0.1:52817"
                                      Try: check that the address is the Dolt server's
                                           and that this computer can reach it
                                           test the connection with bd: bd list
```

| Change | Code |
|---|---|
| Resolve host and port with bd's precedence, including the remote-host and shared-server rules | `internal/datasource/dolt_address.go`, called from `discoverDoltSources` |
| Take a non-canonical `.jsonl` only when its first record passes the loader's own issue rule | `startsWithIssueRecord` and `acceptIssue` in `pkg/loader/loader.go` |
| Name the address in the server-down and timeout hints, without assuming how it is reached | `OpenFailure.Try` in `internal/datasource/open.go` |

## Guards

Each defect has a test that fails if it returns:

- `TestResolveDoltAddressFollowsBDPrecedence` and `TestDiscoverDoltSourcesDialsThePortBDRecorded` (`internal/datasource/dolt_address_test.go`) fix bd's precedence as a table.
- `TestFindJSONLPath_SkipsUnknownFileWithoutIssueRecords` and `TestFindJSONLPath_SkipsUnknownFileWhoseRecordsTheLoaderRejects` (`pkg/loader/loader_test.go`) reject journals that are not issues.
- `TestOpenProjectReportsUnreachableServerBesideNonIssueJournal` (`internal/datasource/open_test.go`) reproduces the report end to end: a refused server plus a journal must give "server down", never an empty project.
- `TestUnreachableServerHintsNameTheAddressNotATunnel` keeps the hints free of assumptions about one network setup.

## What is still open

- b9s does not start a stopped server. When `dolt-server.port` is stale, b9s now reports the address it tried, and `bd list` starts the server again.
- The address rules are still a copy of bd's. If bd changes its precedence, the table test documents what b9s expects, but it cannot detect the change in bd.

## Workaround for v1.3.0

v1.3.0 reads the port only from `metadata.json`. Copy the port bd recorded into it:

```bash
jq --argjson port "$(cat .beads/dolt-server.port)" '.dolt_server_port = $port' \
  .beads/metadata.json > .beads/metadata.json.tmp && mv .beads/metadata.json.tmp .beads/metadata.json
```

bd warns that `dolt_server_port` is deprecated, and the value goes stale when bd restarts its server on a new port. Remove it after upgrading to v1.3.1.
