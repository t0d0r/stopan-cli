# Overview

## Goal

`stopan` is a command line tool and terminal dashboard for a Stopan
service desk (../stopan). It works through the server's JSON API
(`/api/v1`, ../stopan/specs/api.md) with a personal token, so a person
can list, read, answer and change tickets, and read notifications,
without a browser. It is one static Go binary, updated with
`stopan update` and installed with Homebrew, a .deb or a tarball, like
lazyai (../lazyai).

## Names

| what                                   | name                       |
|----------------------------------------|----------------------------|
| project directory, GitHub repository   | `stopan-cli`               |
| Homebrew formula, .deb package         | `stopan-cli`               |
| release tarballs                       | `stopan-cli-vX.Y.Z-OS-ARCH.tar.gz` |
| the command                            | `stopan`                   |
| Go module                              | `stopan-cli`               |

Decision: the command is `stopan`, not `stopan-cli`. Reason: it is typed
often; the project and package names say "cli" to tell them apart from
the server repository.

## Scope (v0.1)

- Account: `login`, `logout`, `me`, `tokens`, `revoke`, and profiles
  for several servers or accounts: `profiles`, `use`, `-P` (auth.md).
- Tickets: `list`, `show`, `new`, `reply`, `note`, `set`, `done`, `open`
  (cli.md).
- Notifications: `notifications`, `read`.
- Dashboard: `stopan` with no command (tui.md).
- Self-update: `update` (update.md); release tarballs, .deb and the
  Homebrew formula (release.md).
- Shell completion for bash, zsh and fish: `completion` (completion.md).

## Not yet (later items in todo/todo.md)

- File uploads and downloads (`ticket[files][]`, attachments).
- Editing descriptions and comments, task list check boxes, revisions.
- Bulk changes (`PATCH /tickets/bulk`), sharing, translations.
- Staff extras: reports, stages, people profiles, deleted tickets.
- Admin endpoints: users, organizations, memberships, settings,
  messages.

## Decisions

- **Separate repository.** The CLI has its own release cycle, toolchain
  (Go) and users; the server is Ruby on Rails. It depends only on the
  documented API, not on server code.
- **Go, no CLI framework.** Same structure as lazyai: `main.go` with a
  command table, GNU-style options from `flags.go`, exit status 0, 1, 2.
  Reason: one binary, no runtime, and the same habits in both projects.
- **Bubble Tea** (bubbletea, bubbles, lipgloss) for the dashboard, the
  same libraries as lazyai.
- **The server decides.** The client checks only the shape of input
  (status and priority names, numbers). Rights, allowed status moves and
  validation come from the server's answer; its error message is shown
  as is (in the user's language).
- **`--json`** on every command that reads prints the API answer, so
  scripts get exactly what the server sent.
- **Texts for replies and notes** come from the arguments, standard
  input, or `$EDITOR`; the dashboard always uses the editor and asks
  before sending.

## Security Note

The token acts as its user (same rights as the web app). It is kept in a
file only the user can read, never on the command line, and only sent
over HTTPS (plain HTTP only to localhost for development). See auth.md.
