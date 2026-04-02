# Migrating Beads from Embedded Dolt to Server Mode

This guide walks through migrating a beads project from embedded Dolt (the default) to an external Dolt SQL server, preserving all issues, dependencies, comments, and memories.

## Table of Contents

- [Background](#background)
- [Prerequisites](#prerequisites)
- [Step 1: Export a JSONL Backup](#step-1-export-a-jsonl-backup)
- [Step 2: Re-initialize with Server Mode](#step-2-re-initialize-with-server-mode)
- [Step 3: Verify the Migration](#step-3-verify-the-migration)
- [Step 4: Configure Remotes (Optional)](#step-4-configure-remotes-optional)
- [Step 5: Update Environment](#step-5-update-environment)
- [Shared Server Mode (Alternative)](#shared-server-mode-alternative)
- [Rollback](#rollback)
- [Troubleshooting](#troubleshooting)
- [Reference: What Changes on Disk](#reference-what-changes-on-disk)

## Background

Beads supports two Dolt modes:

| | Embedded | Server |
|---|---|---|
| **How it runs** | In-process inside `bd` | External `dolt sql-server` (MySQL protocol) |
| **Concurrency** | Single writer | Multiple concurrent writers |
| **Network** | None | TCP on configurable host/port |
| **Use case** | Solo developer | Multi-agent, team, or remote access |
| **Data location** | `.beads/dolt/` | `.beads/dolt/` (same, accessed via TCP) |

Server mode is required when multiple agents or processes need to read/write issues concurrently, or when you want to access the database from a remote host.

## Prerequisites

- `bd` CLI v0.56 or later (`bd --version` to check)
- The target Dolt server running and accessible, OR willingness to let `bd` auto-manage a local server
- `BEADS_DOLT_PASSWORD` environment variable set if the server requires authentication

## Step 1: Export a JSONL Backup

Before touching anything, export your current issues to a portable JSONL file:

```bash
bd export -o .beads/issues.jsonl
```

This writes every issue (with labels, dependencies, and comment counts) to `.beads/issues.jsonl`. Memories are included by default.

Verify the export looks right:

```bash
wc -l .beads/issues.jsonl   # Should match your issue count
head -1 .beads/issues.jsonl | jq .  # Spot-check one record
```

> **Keep this file.** It is your safety net for rollback.

## Step 2: Re-initialize with Server Mode

### Option A: Local server (bd auto-manages it)

Re-init the project, importing from the JSONL you just exported:

```bash
bd init --from-jsonl --server --force
```

This will:
1. Start a local Dolt SQL server (auto-managed by `bd`)
2. Create a database named after your issue prefix
3. Import all issues from `.beads/issues.jsonl`
4. Write `.beads/metadata.json` with `"dolt_mode": "server"`

The `--force` flag is needed because the project is already initialized. `bd` will prompt for confirmation.

### Option B: Remote / existing server

Point to an external server by specifying connection details:

```bash
bd init --from-jsonl --server \
  --server-host=osen.co \
  --server-port=3306 \
  --server-user=root \
  --database=myproject \
  --force
```

If the database does not exist on the server, `bd init` creates it. If it already exists (e.g., set up by an orchestrator), pass `--database=<name>` and `bd` will use it as-is.

### What gets written to metadata.json

After either option, `.beads/metadata.json` will look like:

```json
{
  "database": "dolt",
  "backend": "dolt",
  "dolt_mode": "server",
  "dolt_server_host": "127.0.0.1",
  "dolt_server_port": 3307,
  "dolt_server_user": "root",
  "dolt_database": "myproject",
  "project_id": "<your-project-uuid>"
}
```

Both `bd` and `b9s` read this file, so they always agree on connection details.

## Step 3: Verify the Migration

Run through these checks to confirm everything transferred:

```bash
# 1. Confirm bd connects to the server
bd dolt show

# 2. Test the connection explicitly
bd dolt test

# 3. List issues and compare count to your backup
bd list | wc -l

# 4. Spot-check a specific issue
bd show <any-issue-id>

# 5. Verify dependencies survived
bd dep list <issue-with-deps>

# 6. If using b9s, launch it and confirm it reads from Dolt
b9s
# Bottom status bar should show: dolt://host:port/database
```

## Step 4: Configure Remotes (Optional)

If you want to replicate your Dolt database to a remote (DoltHub, another server, etc.):

```bash
# Add a remote
bd dolt remote add origin https://doltremoteapi.dolthub.com/<user>/<repo>

# Push to it
bd dolt push

# Later, pull changes from collaborators
bd dolt pull
```

For auto-push after every write, set:

```bash
bd config set dolt.auto-push auto
```

## Step 5: Update Environment

Set `BEADS_DOLT_PASSWORD` in your shell profile if the server requires authentication:

```bash
# ~/.zshrc or ~/.bashrc
export BEADS_DOLT_PASSWORD="your-password-here"
```

For CI/CD or agent environments, inject this as a secret.

## Shared Server Mode (Alternative)

If you manage multiple beads projects on one machine, `--shared-server` runs a single Dolt server at `~/.beads/shared-server/` shared across all projects:

```bash
bd init --from-jsonl --shared-server --force
```

Each project gets its own database on the shared server, identified by the issue prefix. This reduces resource usage compared to one server per project.

## Rollback

If the migration fails or you need to go back to embedded mode:

```bash
# Re-init in embedded mode from your backup JSONL
bd init --from-jsonl --force
```

Without `--server`, `bd init` defaults to embedded mode. Your `.beads/issues.jsonl` backup from Step 1 is the source.

## Troubleshooting

### `bd dolt test` fails with "connection refused"

The server is not running or not listening on the expected port.

```bash
# Check if a server is running
bd dolt status

# Start it manually
bd dolt start

# Verify the port
lsof -i :3307
```

### `bd init --from-jsonl` says "no issues found"

The export file is missing or empty:

```bash
ls -la .beads/issues.jsonl
```

If the file is gone, you can re-export from the embedded database (if it still exists):

```bash
bd export -o .beads/issues.jsonl
```

### b9s shows "JSONL" as the source instead of Dolt

Check `.beads/metadata.json` has `"dolt_mode": "server"` and the host/port are correct. b9s only connects to Dolt in server mode (embedded mode has no TCP socket for b9s to connect to).

```bash
cat .beads/metadata.json | jq .dolt_mode
# Should output: "server"
```

### Port conflict (another service on 3306/3307)

Override the port:

```bash
bd dolt set port 3308
bd dolt start
```

### "dolt_server_port in metadata.json is deprecated"

Remove the `dolt_server_port` field from `.beads/metadata.json`. The port is now read from `.beads/dolt-server.port` (a file written by the server on startup). This prevents cross-project port conflicts.

## Reference: What Changes on Disk

| File | Embedded | Server |
|---|---|---|
| `.beads/metadata.json` | `dolt_mode: "embedded"` | `dolt_mode: "server"` + host/port/user/database |
| `.beads/dolt/` | Present (data dir) | Present (data dir, accessed via TCP) |
| `.beads/dolt-server.pid` | Absent | PID of running server |
| `.beads/dolt-server.port` | Absent | Actual port the server bound to |
| `.beads/issues.jsonl` | Optional (legacy/export) | Optional (only if `export.auto: true`) |
| `.beads/config.yaml` | Dolt connection note | Same (connection is in metadata.json) |
