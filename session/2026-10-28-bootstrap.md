# 2026-10-28 - bootstrap

## Done

- New project stopan-cli: a Go command `stopan` for the Stopan service
  desk's JSON API (../stopan/specs/api.md), with a Bubble Tea dashboard,
  self-update and release packaging modelled on lazyai.
- internal/api: client for /api/v1 (me, tickets list/show/create/update/
  resolve, comments, notifications, tokens), JSON error decoding,
  HTTPS-only URLs (http only for localhost), same-host redirects only.
- internal/config: ~/.config/stopan/config.json (0600, dir 0700), atomic
  save, world-readable file refused, STOPAN_URL / STOPAN_TOKEN override.
- Commands: tui (default), login, logout [--revoke], me, list, show,
  new, reply, note, set, done, open, notifications, read, tokens, revoke,
  update; --json on the reading commands; texts from args, stdin or
  $EDITOR (specs/cli.md).
- Dashboard: view tabs, search, sort, pages, ticket view with timeline,
  reply and note in $EDITOR with a y/N question, status and priority
  pickers, assign to me / unassign, done, inbox, ? help, auto refresh
  (specs/tui.md).
- internal/update copied from lazyai and renamed (release
  t0d0r/stopan-cli, formula t0d0r/tap/stopan-cli, binary stopan).
- Makefile from lazyai without test-linux; make dist builds the four
  tarballs (checked with VERSION=v0.0.0-test); make deb not run yet
  (needs docker). Formula source in packaging/homebrew/stopan-cli.rb.
- Checked in tmux at 100 columns against a local fake API: list, ticket
  view, status picker, inbox, help, and the CLI list/inbox/usage error.
  Fixed from that: the inbox count stays when the header is narrow; an
  error in the footer clears after the next successful load; the
  "r retries" hint comes before the error text so it is not cut.

## Decisions

- Separate repository; command `stopan`, everything else `stopan-cli`
  (specs/overview.md "Names").
- No CLI framework, same command table and flag parsing as lazyai.
- Token only from a hidden prompt, stdin or STOPAN_TOKEN; no --token
  option (shell history, ps).
- Replies and notes in $EDITOR, not an in-dashboard text area.
- Default server offered by login: the production site named in
  ../stopan/specs/deploy.md.

## Open

- Not yet run against a real server with a real token.
- GitHub repository t0d0r/stopan-cli does not exist yet; no remote, no
  release, formula not in the tap (sha256 is a placeholder).
- Not in v0.1: attachments, editing, bulk, sharing, translations,
  reports, admin endpoints, profiles, completions (todo/todo.md Later).

## Files

main.go, flags.go, commands.go, cli_test.go,
internal/{api,config,text,tui,update}/*, Makefile, README.md, LICENSE,
AGENTS.md, .gitignore, packaging/homebrew/stopan-cli.rb, specs/*,
todo/todo.md.

## Run and verify

- `make check` (gofmt, vet, tests against fake servers, no network).
- `make run`, or `make install` then `stopan login` with a token from
  the account page of the site, then `stopan list`, `stopan`.
- `make dist VERSION=vX.Y.Z` for release tarballs.

## Next step

Log in to the real server with a read-only token and try list, show and
the dashboard; then a read-and-write token on a test ticket for reply,
note and set. After that, create the GitHub repository and cut v0.1.0
(specs/release.md).

## Release v0.1.0 (in progress)

- make check passes; annotated tag v0.1.0 on 0dfb5d8; make dist and
  make deb VERSION=v0.1.0 on the tag: four tarballs, two .deb files and
  SHA256SUMS in bin/dist. gitleaks over the history: no leaks.
- The arm64 .deb installs in debian:latest (Version 0.1.0, Recommends
  ca-certificates); `stopan --version` prints v0.1.0, an unknown command
  exits 2, plain http to a remote host is refused.
- Not done yet: the GitHub repository t0d0r/stopan-cli, pushing main
  and the tag, the GitHub release, the formula in t0d0r/homebrew-tap.

Next step: create the public repository, push main and v0.1.0, publish
the release with bin/dist/*, then put the archive's sha256 in
packaging/homebrew/stopan-cli.rb and the tap (specs/release.md, Steps
4-6).

## list VIEW

- `stopan list all` works like `stopan list --view all`: a first word
  that is a view name is the view, the rest is the search; `-q/--search`
  searches for a word that is also a view name; `-v` wins over the word
  (specs/cli.md "list"). Reason: `list` showed only the default
  `unsolved` view (new, open, pending), and switching views needed `-v`.
- Checked against the local dev server: `list all` 43 tickets on two
  pages, `list postponed` 9.
- The v0.1.0 tag was not published yet, so it moves to the commit with
  this change and the release files are built again.
