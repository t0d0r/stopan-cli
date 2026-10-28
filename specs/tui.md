# Dashboard

`stopan` (or `stopan tui`) starts a full-screen dashboard in the
alternate screen. `-r/--refresh DURATION` (default `1m`, `0` off) reloads
the list and the unread count while the list is shown and nothing is
being typed or asked.

## Layout

```
stopan Ben @ desk.example.org   1 unsolved  2 mine  3 unassigned ...   inbox 3
 #1001   open      urgent   1h Ben            Printer on fire
 #1002   new       normal   2d -              VPN
 ...
page 1/3, 57 tickets, sort updated                                    ? help
```

- Header: name, user and server (without the scheme), the view tabs
  (staff: unsolved, mine, unassigned, pending, postponed, done, all;
  requesters: open, done, all, numbered 1 to 7), and the unread
  notification count on the right (`inbox N`, highlighted when N > 0).
  When the line is too narrow, the user and server go first, then the
  end of the tabs; the inbox count always stays.
- List rows: number, status, priority, age of the last change, the
  assignee (staff) or the requester (requester), subject cut to the
  width. Urgent and high rows are colored; the selected row is reversed.
- Footer: page, count, sort and search; or the last message (errors in
  red); or the input line, picker or question.
- Colors adapt to light and dark terminals (lipgloss AdaptiveColor).

## Screens and keys

List:

| key | action |
|---|---|
| `tab`/`l`/right, `shift+tab`/`h`/left, `1`-`7` | change the view (page 1) |
| `j`/`k`, arrows, `g`/`G` | move |
| `enter` | open the ticket |
| `/` | search line (enter applies, esc cancels); `esc` in the list clears the search |
| `s` | next sort: updated, created, priority |
| `[` `]`, PgUp/PgDn | pages |
| `n` | notifications |
| `r` | reload (also retries `/me` after a failed start) |
| `o` | open the ticket page in the browser |
| `?` | help (any key closes it) |
| `q`, `ctrl+c` | quit |

Ticket (the text of `stopan show`, wrapped to the width, in a scrolling
viewport; `j`/`k`, arrows, PgUp/PgDn, space scroll):

| key | action |
|---|---|
| `R` | reply: `$EDITOR` opens (the dashboard is suspended); after saving, `Send reply to #N (first words)? [y/N]` |
| `N` | internal note, the same way (staff only) |
| `S` | status picker with the ticket's `next_statuses` (staff) |
| `P` | priority picker (staff) |
| `A` / `U` | assign to me / unassign, after a y/N question (staff) |
| `D` | mark as done after a y/N question (staff: status done; requester: resolve) |
| `r` | reload the ticket |
| `o` | browser |
| `esc`, `q`, backspace | back to the list (reloaded) |

Pickers: left/right (or `h`/`l`, tab) choose, `enter` sets, `esc`
cancels; the current value is preselected. Requesters get a message
instead of a picker.

Notifications: newest first, unread marked `*` and highlighted. `enter`
marks the notification read and opens its ticket; `a` marks all read
after a question; `r` reloads; `esc`/`n`/`q` back.

After every change the list, the unread count and the open ticket are
reloaded, and the footer says what happened or shows the server's error.

## Decisions

- Writing happens in `$EDITOR`, not in a text area inside the
  dashboard. Reason: replies are often several paragraphs; people's own
  editor handles that best; and the code stays small.
- Every write from the dashboard asks y/N first, except picking a
  status or priority, where `enter` on the picker is the confirmation.
- No mouse support yet.

## Acceptance

Model tests against a fake API (internal/tui/tui_test.go): the first
screen shows tabs, rows and the inbox count; tab and `7` load other
views; `/` search sends `q`; requesters get three views and no status
picker; `enter` opens a ticket; `S`, right, `enter` sends the next
status; `D` then `n` sends nothing; the inbox opens a ticket and marks
the notification read; an edited note is sent only after `y`; the editor
command and temporary file (0600, `#` lines dropped, file removed).

By hand: run against a real server in a real terminal (light and dark
theme, 80 columns and wide), reply through vi, and the browser key.
