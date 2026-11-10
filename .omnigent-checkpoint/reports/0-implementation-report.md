# bd-hffz.12 implementation report

Implementation complete. Delivery is not approved. No delivery commit, delivery push, PR, AUR submission, or issue state change occurred.

## Git preflight

- Shared repository: `/home/omnigent/workspace/b9s`.
- Actual branch: `omnigent/bd-hffz.12`.
- HEAD: `3a5ed086abe347af005c06efc3fe3f2c7661af12`.
- Fetched `origin/main`: `3a5ed086abe347af005c06efc3fe3f2c7661af12`.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git rev-parse --verify origin/main`: ref exists.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-hffz.12`: exit 2, absent.
- No local delivery branch existed. Created from fetched `origin/main`, not bootstrap HEAD.
- `git merge-base --is-ancestor origin/main HEAD`: exit 0.
- Required commits: none. No prerequisite ancestry checks were needed against base or HEAD.
- `bd show bd-hffz.11`: CLOSED, close reason `v1.3.0 published, brew on 1.3.0`. `bd dep list bd-hffz.12` reports that dependency closed.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-hffz.12 --base origin/main`: no checkpoint existed.
- Evidence: `/tmp/bd-hffz.12/preflight.txt`.

## Changed files

- `packaging/arch/PKGBUILD`: b9s-bin 1.3.5, x86_64 and aarch64 release archives, pinned SHA-256, glibc dependency, optional beads and xdg-utils, versioned b9s provider/conflict, binary/license/README installation. Strip/debug transforms disabled so the release binary remains intact.
- `packaging/arch/.SRCINFO`: generated using Arch's actual makepkg, matched byte for byte.
- `packaging/arch/README.md`: fresh-machine build/install commands, separate bd prerequisite, package-managed upgrades, release refresh checks, AUR publication status.
- `README.md`: Arch installation link.

No Go or web source changed. This is package configuration and documentation. There is no application logic or bug change requiring a red/green regression. The actual package build/install and payload checks exercise the configuration. No ADR decision changed.

## AUR and release evidence

`timeout 30s curl --fail --silent --show-error 'https://aur.archlinux.org/rpc/v5/info?arg[]=b9s-bin&arg[]=b9s&arg[]=beads&arg[]=beads-bin' -o /tmp/bd-hffz.12/aur-info.json` exited 0 before edits. Neither requested b9s name appears. Existing beads and beads-bin both provide bd. Names can be registered later. No AUR upload occurred.

`timeout 30s gh api repos/vanderheijden86/b9s/releases/latest` exited 0, returning v1.3.5. Both Linux archives downloaded from that release. Computed archive hashes equal its published checksums:

- amd64: `2097463963a19821336402560b31fbda3cc52ec4a12fd85354cc91bccb6c57ae`.
- arm64: `04997bac05c7e40c4f2c1b62a3fda4e5e3b3a9acd1263212d3ce18b3bed4624d`.

Archives contain b9s, LICENSE and README.md. ELF headers identify x86-64 and AArch64. The x86-64 executable uses `/lib64/ld-linux-x86-64.so.2`, supplied by glibc. The aarch64 archive has no ELF interpreter. Installed x86-64 startup succeeded using the fresh Arch runtime.

Evidence: `release.json`, `aur-info.json`, `checksums.txt`, `release-compatibility.txt`, and `package-payloads.json`, all under `/tmp/bd-hffz.12/`.

## Required Go checks

All commands ran from `/home/omnigent/workspace/b9s`. Dependency installation preceded checks and used committed go.mod/go.sum. No lockfile changed. No baseline comparison ran.

| Exact command | Exit | Measured result | Evidence |
| --- | --- | --- | --- |
| `timeout 240s go mod download` | 0 | Dependency download completed | `dependencies.log` |
| `timeout 300s env GOMAXPROCS=2 GOFLAGS=-p=2 go build ./...` | 0 | 1 command check passed, 0 failed, 0 skipped | `go-build.log` |
| `timeout 180s env GOMAXPROCS=2 GOFLAGS=-p=2 go vet ./...` | 0 | 1 command check passed, 0 failed, 0 skipped | `go-vet.log` |
| `timeout 720s env GOMAXPROCS=2 GOFLAGS='-p=2 -v' make test-stable` | 0 | 2124 top-level tests passed, 0 failed, 18 skipped, 0 interrupted, 0 unrun, across 22 packages | `test-stable.log`, `stable-evidence.json` |

Including nested tests and fuzz seeds, the verbose stable report contains 3759 passes, 0 failures and 20 skips. These are separate counting units, not extra tests to add to the top-level total. No flaky outcomes occurred. Counts were measured only after command exit. The Makefile retains its 10-minute package deadline. Existing bounded helper behavior was not changed.

Top-level skips are opt-in real-bd capability probes (1), S3 tests (3), tests requiring an explicitly configured Dolt DSN (7), Memory preview/research tests (4), and PERF_TEST stress tests (3). Two additional nested skips require absent external fixture files. These are optional environment checks. The Makefile also excludes its declared host-sensitive tests and tests/e2e. No required packaging check was skipped.

## Disposable Arch commands and results

No Docker, Podman or PRoot was installed initially. Docker/Podman sockets were absent. `unshare -Ur true` returned `Operation not permitted`. Bubblewrap also reported no permission to create a namespace. A child `PTRACE_TRACEME` check succeeded.

Downloaded PRoot 5.5.0 from the upstream release and checked SHA-256 against its SHA256SUMS:
`db185300a9daabbce86aca7632d8c7fdc4249f6f153171954013b70c5f491ec0`.

Downloaded the official Arch 2026.11.01 x86_64 bootstrap and matched its published SHA-256:
`655bc99849b80859173fa39338a6c910185e952173f6a1a61526cfa34ab8339c`.

The host lacks the zstd executable. Python's installed zstandard module streamed the archive into two distinct fresh roots. Extraction used tarfile's data filter with absolute symlinks converted into equivalent relative links inside the guest root. The root filesystem is fresh Arch, under PRoot on the Pod's kernel, not a booted VM.

Build root: `/tmp/bd-hffz.12/arch-root/root.x86_64`.
Install root: `/tmp/bd-hffz.12/arch-install-root/root.x86_64`.

Build-root setup ran `pacman-key --init` and `pacman-key --populate archlinux` under a 180-second outer deadline. Stopping that root's gpg-agent allowed PRoot to exit normally with code 0. Package signatures remained required.

Two `pacman -Syu --needed --noconfirm base-devel` attempts exited 1 with `Could not resolve host: geo.mirror.pkgbuild.com`. Direct curl/getent succeeded. `pacman --debug` showed its Landlock/seccomp downloader sandbox active under PRoot. The setup succeeded after disabling only pacman's downloader sandbox through the documented command flag. This is an environment workaround, not a PKGBUILD setting. No browser dependency checks or signature checks were disabled.

Successful setup command, exit 0, outer deadline 300 seconds:

```sh
timeout 300s /tmp/bd-hffz.12/proot -S /tmp/bd-hffz.12/arch-root/root.x86_64 -w / /bin/bash -c 'pacman --disable-sandbox -Syu --needed --noconfirm base-devel; result=$?; gpgconf --homedir /etc/pacman.d/gnupg --kill gpg-agent; exit "$result"'
```

Arch setup evidence: `arch-keyring.log`, `arch-dependencies.log`, `arch-dependencies-retry.log`, `pacman-debug.log`, `arch-dependencies-proot.log`. The logs retain normal unbooted-root systemd hook skips. The setup command exited 0.

Native x86_64 source verification and package build, exit 0, two command checks passed, 0 failed, 0 skipped:

```sh
timeout 180s /tmp/bd-hffz.12/proot -i 1000:1000 -r /tmp/bd-hffz.12/arch-root/root.x86_64 -b /dev -b /proc -b /etc/resolv.conf -b /tmp -w /tmp/bd-hffz.12/arch-build /usr/bin/env -i HOME=/tmp/bd-hffz.12/arch-home PATH=/usr/bin TERM=xterm-256color /bin/bash -c 'makepkg --verifysource && makepkg'
```

Evidence: `arch-build.log`.
Package: `/tmp/bd-hffz.12/arch-build/b9s-bin-1.3.5-1-x86_64.pkg.tar.zst`.

Aarch64 source verification and package assembly, exit 0, two command checks passed, 0 failed, 0 skipped:

```sh
timeout 180s /tmp/bd-hffz.12/proot -i 1000:1000 -r /tmp/bd-hffz.12/arch-root/root.x86_64 -b /dev -b /proc -b /etc/resolv.conf -b /tmp -w /tmp/bd-hffz.12/arch-build-arm64 /usr/bin/env -i HOME=/tmp/bd-hffz.12/arch-home PATH=/usr/bin TERM=xterm-256color /bin/bash -c 'makepkg --config /tmp/bd-hffz.12/makepkg-aarch64.conf --verifysource && makepkg --config /tmp/bd-hffz.12/makepkg-aarch64.conf'
```

The config contains `. /etc/makepkg.conf` followed by `CARCH=aarch64`. No compilation occurs. This checks architecture-specific source selection, checksum and package assembly without claiming ARM execution.

Evidence: `arch-build-arm64.log`.
Package: `/tmp/bd-hffz.12/arch-build-arm64/b9s-bin-1.3.5-1-aarch64.pkg.tar.zst`.

Fresh install root contained no b9s or bd and did not receive base-devel. Installation/integrity command, exit 0, two command checks passed, 0 failed, 0 skipped:

```sh
timeout 60s /tmp/bd-hffz.12/proot -S /tmp/bd-hffz.12/arch-install-root/root.x86_64 -w / /bin/bash -c 'pacman -U --noconfirm /tmp/bd-hffz.12/arch-build/b9s-bin-1.3.5-1-x86_64.pkg.tar.zst && pacman -Qkk b9s-bin'
```

`pacman -Qkk` reports `b9s-bin: 10 total files, 0 altered files`. Missing sync-database warnings are expected in this untouched bootstrap, because local package installation requires none. Evidence: `arch-install.log`.

Both package payload checks passed (2 passed, 0 failed, 0 skipped). They read each generated package, check `.PKGINFO` architecture/version/name and compare packaged binary/license bytes to that architecture's downloaded release. Evidence: `package-payloads.json`.

## Installed CLI smoke

Final results: 5 checks passed, 0 failed, 0 skipped, 0 interrupted, 0 unrun, 0 flaky. Evidence: `cli-final-evidence.json` contains the exact guest command argv, isolated environment and per-check detail.

- Installed binary, license and README exactly match release bytes. Executable mode is 0755. bd is absent.
- `b9s --version` exits 0 and prints `b9s v1.3.5 (3a5ed086)` and the full current commit URL.
- `b9s --help` exits 0 and includes the version flag description.
- `script --quiet --return --command /usr/bin/b9s /dev/null`, with B9S_TUI_AUTOCLOSE_MS=1200 and a JSONL-only fixture, exits 0 and renders `Root`.
- `b9s web --no-token --listen 127.0.0.1:<allocated-port>` serves the fixture's A and B issues through `/api/snapshot`, the embedded index and its `app.js` asset (129669 bytes). Loopback HTTP checks were bounded to 12 seconds, with request deadlines and process cleanup. No browser process ran.

All smoke subprocesses had 15-second deadlines, except the web startup's explicit 12-second deadline. Environment was cleared with env -i, with private HOME/XDG_CONFIG_HOME, B9S_TEST_MODE=1 and B9S_NO_BROWSER=1. No shared database was accessed or changed.

Initial smoke attempt reported 3 passes and 1 failure because the temporary HTTP probe invented an `/assets/` path restriction. The product's HTML references `app.js`. The corrected probe reads the actual script src, requires it to remain same-origin, then downloads it. The earlier passing CLI checks were retained. This is a corrected probe, not a flaky product test or a suppressed product error. Both reports are retained: `cli-smoke.json` and `web-smoke-corrected.json`. Checkpoint comparison correctly refused reuse of non-passing/skipped evidence (exit 2, compatible=false). No completed Go suite was repeated.

## Self-review, durability and handoff

Bash syntax, generated .SRCINFO match and Git whitespace checks passed. Evidence: `implementation.txt`. Self-review found no remaining package defect. An independent reviewer/tester verdict is pending Polly's sequence and is not asserted here.

Unapproved checkpoint pushes succeeded to `refs/heads/omnigent-checkpoints/bd-hffz.12`. They do not change delivery HEAD or its index. All new files were named explicitly. Final checkpoint save retains this report and selected scrubbed evidence. No delivery branch push occurred.

No required-check failures remain. No discovered issues were filed. No unrelated work was changed.

Residual limits: aarch64 was assembled and checked, not executed on native ARM. PRoot uses the host kernel and needs the pacman downloader workaround during build-root setup. Beads is deliberately optional for read-only JSONL/SQLite use and must be installed separately for writes or embedded Dolt. No issue-writing behavior was exercised without bd. AUR availability is an observation, not a reservation.
