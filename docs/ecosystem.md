# Beads ecosystem

This is the compatibility snapshot requested by `bd-ac05.1.1`. The required
versions below name that snapshot, not the latest releases or new minimum
versions for every b9s user. Gas City's bd requirement is an upstream
requirement.

The executable checks ran in the lane Pod. They do **not** establish what is
installed on the operator's laptop, on `big-hetzner`, or in a Gas City city.
Those parts of the installed inventory remain blocked on access.

## Components and stores

“Downloaded” means an upstream release archive was downloaded into a temporary
directory, checked against its published checksum, and its executable run.
It does not mean the executable replaced an installed tool.

| Component | Installed version observed in the Pod | Required version or contract | Store mode | What reads the store | What writes the store |
|---|---|---|---|---|---|
| Released `bd` | Installed `bd --version`: **1.3.0-rc.1**. Separately downloaded **1.3.1** reports that version. Laptop and host versions unknown. | **1.3.1** for this snapshot and Gas City below | Shared SQL server (`server`), `proxied-server`, or ordinary `embedded`, according to workspace metadata | `bd` reads its selected store. b9s reads server stores through SQL and ordinary embedded stores through `bd export`. | `bd` performs issue writes. b9s invokes `bd` for mutations. Gas City invokes `gc bd`. |
| Release-candidate `bd` | No separately installed candidate identified. Downloaded **1.3.2-rc.2** reports that version. | **1.3.2-rc.2** as the candidate under consideration, not a replacement for the released requirement | Shared SQL server, `proxied-server`, or ordinary `embedded`, according to metadata | Same issue read paths. The checked candidate does not provide the graph-preview command dialect. | The candidate `bd` writes only the workspace it is explicitly selected to use. No candidate store was opened in this inventory. |
| Preview `bd-prev` | Unavailable. Neither the shell function nor the preview executable exists in the Pod. No installed preview version or revision checked. | Matching graph-preview CLI and workspace capabilities. No numeric release requirement is established. | **Graph preview**, documented locally as embedded Dolt | b9s reads through the matching preview `bd` CLI after capability checks. It does not open the graph store directly. | The matching preview `bd` owns graph mutations. b9s Memory views are read-only. |
| Gas City `gc` | Not installed on Pod `PATH`. Downloaded **1.5.0** prints `1.5.0`. Actual city installation unknown. | **1.5.0**, which requires **bd 1.3.1** | New cities use bd-owned **`proxied-server`** stores. Existing cities retain their topology until explicitly migrated. | Gas City reaches its ledger through `gc bd`. b9s can read a reachable proxied store with the adapter in this checkout. | Gas City delegates ledger mutations and maintenance to `gc bd`. bd owns the proxy and Dolt process lifecycle for proxied scopes. |
| b9s | Not installed on Pod `PATH`. Downloaded **1.3.3** prints `b9s v1.3.3`. Laptop installation unknown. | **1.3.3** is the requested released baseline. This checkout also contains a proxied-server adapter absent from that release's datasource tree. | Shared SQL server, ordinary embedded, and graph preview. **Proxied-server support described here belongs to this checkout, not an established capability of the checked release.** | b9s reads SQL directly for server sources, `bd export` for ordinary embedded sources, and the preview CLI for graph sources. | b9s issues mutations through `bd`, never direct database writes. Memory views do not mutate the graph. |
| Shared Dolt on `big-hetzner` | **Unknown**. Host name does not resolve in the Pod. No server executable or SQL engine version checked. | No Dolt version specified by this task. Need the host executable version and the live server's engine version. | **Shared SQL server**, specified deployment | b9s SQL readers and each project's `bd` read the configured project database. The catalog account is SELECT-only. | Project-scoped `bd` writers. b9s delegates writes to `bd`. This inventory performed no application-data writes. |
| Operator's local fallback | **Unknown**. No `dolt` executable in Pod `PATH`. Pod loopback **3307** refuses TCP. This says nothing about laptop availability. | Local fallback endpoint **3307**, specified deployment. No Dolt version specified. | **Shared SQL server protocol**, separate local fallback instance, not embedded mode | Local `bd` and b9s when explicitly configured for the fallback | Local project `bd`. The fallback can diverge and requires reconciliation before returning to the shared store. |
| Gas City city/rig store | **Unknown**. No actual city directory, scope metadata, proxy record, or server version checked. | With Gas City **1.5.0** and bd **1.3.1**, inspect the actual scope's metadata before choosing a reader. | **`proxied-server`** for newly created scopes. A legacy city's topology must be checked separately. | `gc bd`, and this checkout's b9s SQL reader through the recorded loopback proxy as `root`, without a password | bd through the proxy, including commands delegated by Gas City or b9s. b9s does not start a proxy. |
| Ordinary embedded project | No embedded project was opened for this inventory. Its engine version is unknown. | A matching `bd` with working `export`. Keep graph-preview workspaces separate. | **Embedded** Dolt, opened inside `bd` | b9s runs `bd export` and parses stdout. It holds no embedded engine handle between reads. | `bd` opens and closes the embedded store per command. |

These paths follow [the architecture](ARCHITECTURE.md),
[the data IO guide](data-io.md), and the decisions on
[shared read access](adr/0007-read-shared-beads-through-one-select-only-catalog-account.md),
[embedded reads](adr/0025-read-embedded-dolt-through-bd-export.md),
[preview transport](adr/0041-retain-cli-memory-reads-after-transport-probes.md),
and [Memory capability checks](adr/0045-probe-bd-help-before-memory-reads.md).
The [proxied-server adapter](../internal/datasource/proxied_server.go) describes
the discovery rules implemented in this checkout.

## Checked executable evidence

Each version below is copied from a completed executable invocation. The
release commands used downloaded Linux archives, not binaries from another
host. All four release archives matched their upstream checksum files.

```text
Installed Pod tool:
$ bd --version
bd version 1.3.0-rc.1 (9c6a69ec1)

Downloaded release executables, invoked by absolute path:
$ /tmp/bd-ac05.1.1-evidence/bd-release/bd --version
bd version 1.3.1 (c1c4b642a)
$ /tmp/bd-ac05.1.1-evidence/bd-rc-release/bd --version
bd version 1.3.2-rc.2 (53637a74a)
$ /tmp/bd-ac05.1.1-evidence/gas-city-release/gc version
1.5.0
$ /tmp/bd-ac05.1.1-evidence/b9s-release/b9s --version
b9s v1.3.3
```

The release artifacts came from
[Beads v1.3.1](https://github.com/gastownhall/beads/releases/tag/v1.3.1),
[Beads v1.3.2-rc.2](https://github.com/gastownhall/beads/releases/tag/v1.3.2-rc.2),
[Gas City v1.5.0](https://github.com/gastownhall/gascity/releases/tag/v1.5.0),
and [b9s v1.3.3](https://github.com/vanderheijden86/b9s/releases/tag/v1.3.3).
The [b9s release datasource tree](https://github.com/vanderheijden86/b9s/tree/v1.3.3/internal/datasource)
was checked separately from the current checkout.

Both checked Beads release executables reject `versions --help` and
`links --help` as unknown commands. Their `memories --help` succeeds, but
does not advertise `records-json`. That command name alone does not establish
Memory graph support. The preview needs the capability contract in the ADR,
not a release number. See upstream
[Memory Beads proposal #5877](https://github.com/gastownhall/beads/issues/5877)
and [implementation tracker #6535](https://github.com/gastownhall/beads/issues/6535).

## Access still required

The Pod has no `bd-prev`, `gc`, `b9s`, `dolt`, or `ssh` on `PATH`.
The documented preview executable and shell configuration are absent.
A bounded connection attempt to `big-hetzner` returned
`Name or service not known`. A bounded connection attempt to
`127.0.0.1:3307` returned `Connection refused`.

Completing the installed inventory requires read-only access to:

- The operator's laptop shell for installed `bd --version`, the candidate
  executable's `--version`, `bd-prev --version`, `b9s --version`, and the
  fallback `dolt version`.
- `big-hetzner` for `dolt version` and a SELECT-only `DOLT_VERSION()` query
  against the running server. A CLI version alone does not identify a
  separately running server.
- The actual Gas City installation for `gc version`, its selected
  `bd --version`, and the city and rig `.beads/metadata.json`,
  `.beads/proxied_server_client_info.json`, and applicable `proxy.pid`.
  Read the running engine version through the existing endpoint.
- The preview binary and ready workspace metadata. Check the preview's
  `versions`, `links`, and `memories` help from a neutral directory, then
  check workspace readiness with its matching CLI.

These are missing acceptance prerequisites for `bd-ac05.1.1`, not permission
to install, migrate, start, or stop any host service. Downloaded release
evidence does not close those gaps.
