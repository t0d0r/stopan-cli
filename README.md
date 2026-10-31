# stopan-cli

`stopan` is the command line and terminal dashboard for a
[Stopan](https://github.com/t0d0r/stopan) service desk. It lists, reads,
answers and changes tickets, and reads notifications, through the
server's JSON API with your personal token. One static binary for macOS
and Linux.

## Why Stopan?

"Stopan" (Стопан) comes from Bulgarian folklore, where it represents a
guardian spirit that protects the home and its inhabitants.

The name reflects the application's role as a Service Desk - a central
point for support, incident management, and service requests, helping
maintain stability, reliability, and continuity of IT services.

## Install

```sh
brew install t0d0r/tap/stopan-cli
```

Or download a static binary or a .deb from the
[releases](https://github.com/t0d0r/stopan-cli/releases) (`uname -m`:
`x86_64` is amd64, `aarch64` is arm64):

```sh
V=v0.2.0 A=linux-amd64
curl -LO https://github.com/t0d0r/stopan-cli/releases/download/$V/stopan-cli-$V-$A.tar.gz
curl -LO https://github.com/t0d0r/stopan-cli/releases/download/$V/SHA256SUMS
sha256sum -c SHA256SUMS --ignore-missing
tar -xzf stopan-cli-$V-$A.tar.gz
install -m 0755 stopan-cli-$V-$A/stopan ~/.local/bin/
```

From source (Go 1.24 or newer): `make install` puts `stopan` in
`~/.local/bin`.

Later updates: `stopan update` (it uses brew, apt or replaces the
binary, the way stopan was installed).

## Log in

Make a token on the Stopan site: account page, "API tokens". A read-only
token is enough to read; replies and changes need read and write.

```sh
stopan login                 # asks for the server URL and the token (not shown)
stopan me
```

The token is saved in `~/.config/stopan/config.json` (mode 0600).

Several servers or accounts are profiles:

```sh
stopan login -P dev          # log in to a second server as the profile "dev"
stopan profiles              # list them; * marks the one in use
stopan use dev               # switch
stopan -P prod list          # one command with another profile
stopan logout -P dev --forget
```

`STOPAN_PROFILE` chooses a profile; `STOPAN_URL` and `STOPAN_TOKEN`
override its server and token, for scripts and CI.

## Use

```sh
stopan                                   # dashboard
stopan list                              # the queue (staff) or My tickets
stopan list all                          # a view: unsolved (default), mine, unassigned,
                                         # pending, postponed, done, all
stopan list mine -s open printer         # view, status and a search
stopan show 1001
stopan reply 1001 "On my way."           # or from stdin, or in $EDITOR
stopan note 1001                         # internal note (staff), in $EDITOR
stopan set 1001 1002 -a me -p high       # staff: assignee, priority, status
stopan done 1001
stopan new -t "VPN down" -o Acme < details.txt
stopan inbox                             # notifications; stopan read --all
stopan list --json | jq '.tickets[].number'
```

`stopan help` lists every command; `stopan help COMMAND` shows its
options.

## Shell completion

Homebrew and the .deb install completions for bash, zsh and fish. From
a tarball or a source build, add them yourself:

```sh
stopan completion fish > ~/.config/fish/completions/stopan.fish
stopan completion zsh > "${fpath[1]}/_stopan"     # then restart zsh
echo 'source <(stopan completion bash)' >> ~/.bashrc
```

They complete commands, options, views (`stopan list <TAB>`), statuses,
priorities and your profile names (`stopan -P <TAB>`).

Dashboard keys: `tab` or `1`-`7` views, `j`/`k` move, `enter` open, `/`
search, `n` notifications, `R` reply, `N` note, `S` status, `P`
priority, `A` assign to me, `D` done, `o` browser, `?` help, `q` quit.

## Development

```sh
make help     # targets
make check    # gofmt, go vet, tests (fake API server, no network)
make run      # build and start the dashboard
make dist     # release tarballs; make deb for the .deb files
```

Specs are in `specs/`, the work list in `todo/todo.md`.

## Security

The token acts as you, with your rights. It is sent only over HTTPS
(plain HTTP only to localhost), never in URLs, and stored only in a file
that only you can read. Prefer a read-only token with an expiry for
scripts, and revoke a token that may have leaked with `stopan revoke ID`
or on the account page.

## License

BSD 2-Clause, see LICENSE.
