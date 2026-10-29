# Update

`stopan update` replaces the running stopan with the latest GitHub
release of t0d0r/stopan-cli, in the way it was installed. The design and
the code (internal/update) come from lazyai (../lazyai/specs/update.md),
renamed; behavior is the same.

```
stopan update [--check] [--yes]
```

- `--check`, `-c`: report the installed and the latest version, change
  nothing.
- `--yes`, `-y`: do not ask for confirmation.

## Latest release

The latest release is the tag that
`https://github.com/t0d0r/stopan-cli/releases/latest` redirects to
(`.../releases/tag/vX.Y.Z`). stopan reads the `Location` header and does
not follow it.

Decision: the redirect, not the GitHub API. Reason: no JSON and no API
rate limit for unauthenticated users. GitHub may still answer with the
previous tag for a minute or two after a release (seen with lazyai);
running `update` again a little later finds it.

## Versions

A release version is `vX.Y.Z` or `X.Y.Z` (Homebrew builds without the
`v`). Versions compare by number: major, then minor, then patch.

- Installed version equal to or newer than the latest: print
  `stopan VERSION is up to date` and exit 0.
- Installed version is not a release version (`dev`, or `git describe`
  output such as `v0.1.0-2-gabc1234` or `v0.1.0-dirty`): a build from
  source. `--check` reports the latest version; `update` refuses and
  says to update the source (`git pull`, `make install`). Exit 1.
- Otherwise print `update available: OLD -> NEW`. With `--check`, exit 0
  here. Without it, ask `Update stopan OLD -> NEW (METHOD)? [y/N]` unless
  `--yes`; an answer other than `y` changes nothing and exits 0.

## Install methods

The method follows from the path of the running executable after
resolving symlinks.

| method   | detected by                                   | update                                        |
|----------|-----------------------------------------------|-----------------------------------------------|
| homebrew | path contains `/Cellar/stopan-cli/`           | run `brew upgrade t0d0r/tap/stopan-cli`       |
| deb      | Linux, and `dpkg-query -S PATH` succeeds      | download and check the .deb, `apt install` it |
| binary   | anything else                                 | download and check the tarball, replace PATH  |

- homebrew: brew builds from the tap formula, so a release the tap has
  not picked up yet is reported by brew as up to date.
- deb: downloads `stopan-cli_X.Y.Z_ARCH.deb` and `SHA256SUMS` into a new
  temporary directory (mode 0755, file 0644, so apt's sandbox user can
  read it), checks the sum, then runs `sudo apt-get install -y FILE`
  (without sudo when already root). The directory is removed afterwards.
- binary: downloads `stopan-cli-vX.Y.Z-OS-ARCH.tar.gz` and `SHA256SUMS`,
  checks the sum, takes the regular file `stopan-cli-vX.Y.Z-OS-ARCH/stopan`
  from the tarball, writes it to a temporary file next to PATH with mode
  0755 and renames it over PATH. When the directory is not writable, the
  update fails and names the directory.

OS and ARCH are the ones stopan was built for (`runtime.GOOS`,
`runtime.GOARCH`). A platform without a release asset fails with a
message.

Done: `updated stopan OLD -> NEW (PATH)` for the binary method; brew and
apt print their own output.

## Limits

- Downloads start at github.com over HTTPS and follow GitHub's redirect
  to its asset host; a redirect to anything other than HTTPS is refused.
  Each request has a 2 minute timeout and each file at most 64 MiB.
- A file missing from `SHA256SUMS` or with a different sum stops the
  update before anything is installed.

## Decisions

- No automatic update check. stopan talks to the Stopan server it is
  logged in to and, only when `update` runs, to github.com. Reason: no
  surprise network traffic, and the same rule as lazyai.
- The TUI has no update action; `stopan update` is run from a shell.
- A source build is never replaced.

## Security

`SHA256SUMS` comes from the same release as the files, so the check
catches corrupted or truncated downloads, not a compromised release page.
TLS to github.com authenticates the download. Release assets are not
signed (see release.md). The update never touches the config file or the
token.

## Acceptance

- Unit tests (internal/update): version parsing and comparison, the
  latest-release redirect, checksum mismatch and missing entry, tarball
  extraction (only the expected regular file), replacing a file in a
  temporary directory, method detection by path, a full binary update
  against a local server.
- By hand after the first two releases: a binary built as the older
  release updates itself; `--check` on a current version says it is up
  to date; a dev build refuses; `brew upgrade` through the tap.

Checked from the published v0.1.0 to v0.2.0: binary method from the
darwin-arm64 tarball in a temporary directory; deb method as root in
debian:latest (arm64), apt unpacked 0.2.0 over 0.1.0; homebrew method on
macOS (`stopan update -y` ran brew upgrade, brew test passes). Right
after publishing, releases/latest still answered v0.1.0 to stopan for
about 40 seconds while curl already got v0.2.0, as seen with lazyai.
