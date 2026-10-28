# Release

Same process as lazyai (../lazyai/specs/release.md); the differences
are the names and that there is no Linux container test (stopan has no
system dependencies to check).

## Artifacts

Each release is an annotated tag `vX.Y.Z` on main and a GitHub release
of t0d0r/stopan-cli with:

- `stopan-cli-vX.Y.Z-OS-ARCH.tar.gz` for linux/amd64, linux/arm64,
  darwin/amd64 and darwin/arm64, each with one directory of the same
  name holding `stopan`, `LICENSE` and `README.md`.
- `stopan-cli_X.Y.Z_ARCH.deb` for amd64 and arm64.
- `SHA256SUMS` over all of them, in `sha256sum` format.

`make dist` builds the tarballs into `bin/dist` with `CGO_ENABLED=0
-trimpath -ldflags "-s -w -X main.version=VERSION"` (static, no build
paths, tar entries owned by root). `make deb` (after `make dist`, same
VERSION) wraps each Linux binary in a .deb with `dpkg-deb
--root-owner-group` in a `debian:latest` container.

Debian package: `stopan-cli`, `/usr/bin/stopan`,
`/usr/share/doc/stopan-cli/{copyright,changelog.gz}`, a lintian override
for `statically-linked-binary`; no Depends; Recommends
`ca-certificates` (HTTPS); Suggests `xdg-utils` (the browser key).

## Homebrew

Formula `stopan-cli` in the tap t0d0r/homebrew-tap
(`brew install t0d0r/tap/stopan-cli`). The source of the formula is
kept here in `packaging/homebrew/stopan-cli.rb` and copied to
`Formula/stopan-cli.rb` in the tap for each release, with `url` and
`sha256` of the tag's source archive
(`https://github.com/t0d0r/stopan-cli/archive/refs/tags/vX.Y.Z.tar.gz`).
It builds from source with Go and installs `bin/stopan`, stamped with
the version; its test checks `--version`, `--help` and the usage error
exit status 2. It works on macOS and Linux (no `depends_on :macos`, no
runtime dependencies).

## Steps

1. `make check` passes; README install examples name the new version.
2. Clean tree; `git tag -a vX.Y.Z -m "stopan-cli vX.Y.Z"` on main.
3. `make dist` on the tag (VERSION exactly `vX.Y.Z`), then
   `make deb VERSION=vX.Y.Z`.
4. `gitleaks git` finds nothing; push main and the tag (only when the
   operator asks).
5. `gh release create vX.Y.Z bin/dist/*.tar.gz bin/dist/*.deb
   bin/dist/SHA256SUMS --title vX.Y.Z --notes ...`; download the files
   again and check them with `sha256sum -c SHA256SUMS`.
6. Homebrew: put the archive's sha256 in the formula, copy it to the
   tap, `brew style`, `brew audit --formula t0d0r/tap/stopan-cli`,
   `brew install --build-from-source`, `brew test`; push the tap.

The GitHub repository t0d0r/stopan-cli must exist and be public before
the first release, since `stopan update` and Homebrew download from it
without credentials.

## Security

Release assets are not signed; `SHA256SUMS` only protects against
corrupted downloads. Run `gitleaks git` before every push: tests and
docs use invented names, `example.org` hosts and `stopan_test`-style
fake tokens only.
