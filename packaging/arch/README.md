# Arch Linux package

`PKGBUILD` packages the published b9s v1.3.5 binary as `b9s-bin`. It does not
compile Go or the web UI. The recipe supports `x86_64` and `aarch64`, maps them
to the release's `amd64` and `arm64` archives, and pins each archive's SHA-256.

The AUR RPC returned no package named `b9s` or `b9s-bin` on 2026-11-10.
This repository contains the recipe. It has not been submitted to the AUR.

## Build and install

On a fresh Arch Linux machine, install the standard build tools:

```bash
sudo pacman -Syu --needed base-devel git
git clone https://github.com/vanderheijden86/b9s.git
cd b9s/packaging/arch
makepkg --syncdeps
sudo pacman -U ./b9s-bin-1.3.5-1-*.pkg.tar.zst
b9s --version
b9s --help
```

Run `makepkg` as a regular user. Review the recipe before building it.
The build downloads the release archive and checks its pinned checksum.
The resulting package installs `/usr/bin/b9s`, the MIT license under
`/usr/share/licenses/b9s-bin/`, and the release README under
`/usr/share/doc/b9s-bin/`.

Install [Beads](https://github.com/steveyegge/beads) separately, for example
through the AUR's `beads` or `beads-bin` package. Both provide the `bd` command.
It is optional for viewing JSONL or SQLite issues, but required for every
issue write and for reading embedded Dolt projects. `xdg-utils` supplies
`xdg-open` for links and attachments.

Upgrade through a rebuilt package and `pacman -U`, or through an AUR helper
if the recipe is published there. Do not run `sudo b9s --update`: pacman owns
`/usr/bin/b9s`, and a self-update would bypass its package database.

## Check a release

In a disposable Arch environment with `base-devel` installed:

```bash
makepkg --verifysource
makepkg
makepkg --printsrcinfo > .SRCINFO
sudo pacman -U ./b9s-bin-1.3.5-1-*.pkg.tar.zst
pacman -Qkk b9s-bin
b9s --version
b9s --help
```

When upgrading the recipe, change `pkgver`, reset `pkgrel` to `1`, and
replace both hashes with the matching entries from that release's
`checksums.txt`. Download both archives and compare their hashes before
regenerating `.SRCINFO`. Check the archive contents, native package build,
installation, and CLI behavior again. An `aarch64` runtime check requires
an ARM machine or suitable emulation.
