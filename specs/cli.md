# Commands

```
stopan [COMMAND] [OPTIONS] [ARGS...]
```

Without a COMMAND (or with only options) the dashboard starts (tui.md).
Unix conventions as in lazyai: `-h/--help` everywhere, `help COMMAND`,
`-V/--version`, long and short options, options may follow arguments,
`--` ends options. Exit status 0 success, 1 failure (including every API
error), 2 usage error (`stopan CMD: MESSAGE` and
`Run 'stopan CMD --help' for usage.`).

TICKET is a ticket number, with or without `#` (`1001`, `#1001`).

Every command takes `-P/--profile NAME`, also before the command
(`stopan -P dev list`, `stopan list -P dev`, `--profile=dev`), to use
that profile instead of the current one (auth.md "Profiles"). A bad
name is a usage error (exit 2); a profile that does not exist fails with
exit 1.

Every command that reads takes `-j/--json` and then prints the API's
answer, indented, instead of the table.

API errors print `stopan: CODE: MESSAGE`, with validation details
(`invalid: Status is not allowed (status: is not allowed)`).

| command | aliases | API | what |
|---|---|---|---|
| `tui` | | | dashboard; `-r/--refresh DURATION` (default 1m, 0 off) |
| `login` | | `GET /me` | auth.md |
| `logout` | | `GET /tokens`, `DELETE /tokens/:id` | auth.md; `--forget` removes the profile |
| `profiles` | | | list the profiles, `*` on the one in use (auth.md) |
| `use NAME` | | | make a profile current (auth.md) |
| `me` | `whoami` | `GET /me` | server, user, username, email, role, language, organizations |
| `list [VIEW] [SEARCH...]` | `ls`, `tickets` | `GET /tickets` | see below |
| `show TICKET` | `view`, `cat` | `GET /tickets/:n` | fields, request text, timeline |
| `new --subject S [TEXT...]` | `create` | `POST /tickets` | see below |
| `reply TICKET [TEXT...]` | | `POST /tickets/:n/comments` | public reply |
| `note TICKET [TEXT...]` | | `POST /tickets/:n/comments` | internal note (`internal: true`) |
| `set TICKET...` | | `PATCH /tickets/:n` | staff: status, priority, assignee |
| `done TICKET...` | `resolve` | staff `PATCH status=done`, requester `POST /resolve` | mark as done |
| `open TICKET` | `browse` | `GET /tickets/:n` | open the ticket's `url` with `open` (macOS) or `xdg-open` |
| `notifications` | `notif`, `inbox` | `GET /notifications` | `-u/--unread`, `--page N` |
| `read ID...` / `read --all` | | `POST /notifications/:id/read`, `read_all` | mark read |
| `tokens` | | `GET /tokens` | auth.md |
| `revoke ID...` | | `DELETE /tokens/:id` | auth.md |
| `update` | | GitHub | update.md |

## list

`stopan list all` is `stopan list --view all`: when `--view` is not
given and the first word after the options is a view name (staff:
unsolved, mine, unassigned, pending, postponed, done, all; requesters:
open, done, all), it is the view. The other words are the search. To
search for a view's name, use `-q/--search TEXT`, which is joined in
front of the remaining words. A view the user does not have is left to
the server, which then uses its default view.

`stopan --help` names the views in a VIEW paragraph; `stopan help list`
lists each view with what it holds, the requester views and examples
(`stopan list mine` for the tickets assigned to you); the `--view`
option's help names the staff views.

Decision: a view as a plain word, not only `-v`. Reason: switching views
is the most common use of `list`; `-v` stays for scripts.

Options: `-v/--view`, `-q/--search`, `-s/--status`, `-p/--priority`, `-a/--assignee`
(`me`, `none` or a user id), `-o/--org ID`, `--sort`
(`updated|created|priority`), `--page N`. Words after the options are
the search (`q`). Status, priority and sort names are checked before the
request (usage error); views and the rest are left to the server, which
falls back to its default for unknown values.

Output: a table `TICKET STATUS PRIO UPDATED REQUESTER ASSIGNEE SUBJECT`
(UPDATED as `now`, `5m`, `3h`, `2d`, then the date; names cut to 20 and
subjects to 70 characters), then `VIEW: page P of N, C tickets`.

## show

```
#1001  Printer on fire

  status        open
  priority      high
  organization  Acme
  requester     Ana <ana@example.org>
  assignee      -
  ...
  page          https://.../tickets/1001

  It smokes.

-- 2026-10-28 11:00  Ben replied
  On my way.
-- 2026-10-28 11:05  Ben added an internal note  [internal]
  Bring water.
-- 2026-10-28 11:06  Ben changed the status open -> pending
```

The `created` event is left out (the header says it). Files are listed
as `[file] NAME (TYPE, SIZE)`. Times are local.

## Texts: new, reply, note

The text is, in order: the arguments joined with spaces; else standard
input when it is not a terminal (at most 1 MiB); else `$VISUAL` or
`$EDITOR` (default `vi`, may hold options such as `code -w`) on a new
0600 temporary file `stopan-*.md`, whose `#` lines (a header naming the
ticket) are dropped. An empty text sends nothing and fails with
`empty text; nothing sent`. The file is removed afterwards.

`reply` and `note`: `-s/--status STATUS` (staff: change the status with
the message), `-m/--markdown`. A requester's note is turned into a
public reply by the server (../stopan/specs/api.md).

`new`: `-t/--subject` (required), `-o/--org ID|NAME` (a name matches the
user's organizations from `/me`, exact first, then a unique substring),
`-p/--priority`, `-a/--assignee me|ID`, `--requester ID` (staff),
`-m/--markdown`. Prints `created #N SUBJECT` and the page URL.

## set, done

`set` needs at least one of `-s/--status`, `-p/--priority`,
`-a/--assignee` (`me` is looked up with `/me`; `none` sends
`assignee_id: null`). Several tickets are changed one by one; each
success prints one line, each failure `stopan: #N: ERROR`, and the exit
status is 1 when any failed.

`done` asks `/me` once: staff set the status `done`, requesters use
"Mark as done" (`POST /resolve`).

## Acceptance

CLI tests against a fake API (cli_test.go): list query and table, a
view as the first word (`list all`, `list done printer jam`), `--search`
for a view's name, `-v` taking precedence (`list -v mine all` searches
"all"), show
text and `--json`, `whoami`, reply from arguments with `--status`, note
from standard input with `internal: true`, `set -a me -p urgent` body,
`done` as staff, `new` with an organization name and `-a me`, a wrong
token exits 1 with the server message, usage errors exit 2, `--version`,
`help reply`, the inbox table and `read --all`.
