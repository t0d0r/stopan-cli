# Todo

## Done

- [x] project skeleton: Makefile, AGENTS.md, specs, todo, session
- [x] API client with HTTPS-only URLs, Bearer token, JSON errors
      (specs/auth.md)
- [x] config file 0600 with STOPAN_URL / STOPAN_TOKEN overrides; login,
      logout --revoke, tokens, revoke (specs/auth.md)
- [x] commands: me, list, show, new, reply, note, set, done, open,
      notifications, read, all with --json where they read (specs/cli.md)
- [x] dashboard: views, search, sort, pages, ticket view, reply and note
      in $EDITOR, status and priority pickers, assign, done, inbox
      (specs/tui.md)
- [x] stopan update from lazyai: homebrew, deb, binary (specs/update.md)
- [x] make dist / make deb and the Homebrew formula source
      (specs/release.md)

## Next

- [ ] try against the real server with a real token: login, list, show,
      reply in a test ticket, the dashboard in a real terminal (light and
      dark theme, 80 columns)
- [x] create the public GitHub repository t0d0r/stopan-cli and push
      (specs/release.md)
- [x] first release v0.1.0: tag, make dist, make deb, gh release,
      formula in t0d0r/homebrew-tap (specs/release.md)
- [x] check stopan update end to end after the second release (binary,
      deb, homebrew) (specs/update.md)

- [x] profiles: several servers or accounts in one config, -P, profiles,
      use, logout --forget, STOPAN_PROFILE (specs/auth.md)
- [x] views in the list help; `stopan list VIEW` (specs/cli.md)
- [x] release v0.2.0 with profiles and the list views (specs/release.md)

- [x] shell completions for bash, zsh and fish, installed by brew, the
      .deb and the tarballs (specs/completion.md)
- [x] README section "Why Stopan?"
- [ ] release v0.3.0 with the completions; the formula in the tap gets
      generate_completions_from_executable only then (specs/release.md)

## Later

- [ ] attachments: upload with new/reply (`--file`), download
      (`stopan get TICKET FILE`) (needs a spec)
- [ ] edit the description and comments; task list check boxes;
      revisions (needs a spec)
- [ ] bulk changes with `PATCH /tickets/bulk` for `set` on many tickets
- [ ] share / unshare, translations (staff)
- [ ] staff extras: reports, stages, people profiles, deleted tickets
- [ ] admin commands: users, organizations, memberships, settings,
      messages
- [ ] dashboard: mouse, the board by stages, filters by priority and
      assignee
