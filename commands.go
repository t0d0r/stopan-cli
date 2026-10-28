package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"stopan-cli/internal/api"
	"stopan-cli/internal/config"
	"stopan-cli/internal/text"
	"stopan-cli/internal/tui"
	"stopan-cli/internal/update"
)

// defaultURL is offered by login when no server is saved yet.
const defaultURL = "https://stopan.linuxfan.org"

var commands = []command{
	{
		name:     "tui",
		synopsis: "[OPTIONS]",
		summary:  "interactive dashboard (the default)",
		about: `
Full-screen dashboard: the ticket queue with its views on top, the list
below, and the ticket with its timeline when one is opened. Notifications
are one key away.

Keys in the list: tab/shift+tab or 1-7 views, j/k move, enter open,
/ search, s sort, [ ] pages, n notifications, r refresh, o open in the
browser, ? help, q quit.
Keys on a ticket: R reply, N internal note (staff), S status,
P priority, A assign to me, U unassign, D done, o browser, esc back.
Replies and notes are written in $EDITOR (default vi).`,
		permute: true,
		setup: func(f *flagDefs) action {
			refresh := f.Duration("r", "refresh", "DURATION", time.Minute, "reload the list this often; 0 turns it off")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				return tui.Run(c, tui.Options{Refresh: *refresh})
			}
		},
	},
	{
		name:     "login",
		synopsis: "[OPTIONS]",
		summary:  "save the server and a personal token",
		about: `
Ask for the server URL and a personal API token, check them with the
server, and save them in ~/.config/stopan/config.json (mode 0600).

Make the token on the Stopan site: account page, "API tokens". A
read-only token is enough for list, show and notifications; replies and
changes need a read-and-write token.

The token is read without echo from the terminal, or from standard input
with --token-stdin, never from the command line, where it would land in
the shell history.`,
		permute: true,
		setup: func(f *flagDefs) action {
			server := f.String("u", "url", "URL", "", "server URL, such as https://stopan.example.org")
			stdin := f.Bool("", "token-stdin", "read the token from standard input")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				return login(e, *server, *stdin)
			}
		},
	},
	{
		name:     "logout",
		synopsis: "[OPTIONS]",
		summary:  "forget the saved token",
		about: `
Remove the token from the config file; the server URL stays. With
--revoke, first revoke the token on the server, so it stops working
everywhere.`,
		permute: true,
		setup: func(f *flagDefs) action {
			revoke := f.Bool("", "revoke", "also revoke the token on the server")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				return logout(e, *revoke)
			}
		},
	},
	{
		name:     "me",
		aliases:  []string{"whoami"},
		synopsis: "[OPTIONS]",
		summary:  "show the token's user and server",
		about:    "Show the user the token acts as, with role and organizations.",
		permute:  true,
		setup: func(f *flagDefs) action {
			asJSON := f.Bool("j", "json", "print the API answer as JSON")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				me, raw, err := c.Me()
				if err != nil {
					return err
				}
				if *asJSON {
					return printRaw(e.out, raw)
				}
				var orgs []string
				for _, o := range me.Organizations {
					orgs = append(orgs, fmt.Sprintf("%s (%d)", o.Name, o.ID))
				}
				tw := tabwriter.NewWriter(e.out, 0, 0, 2, ' ', 0)
				fmt.Fprintf(tw, "server\t%s\n", c.Base)
				fmt.Fprintf(tw, "user\t%s (%d)\n", me.Name, me.ID)
				fmt.Fprintf(tw, "username\t%s\n", orDash(me.Username))
				fmt.Fprintf(tw, "email\t%s\n", orDash(me.Email))
				fmt.Fprintf(tw, "role\t%s\n", me.Role)
				fmt.Fprintf(tw, "language\t%s\n", orDash(me.Language))
				fmt.Fprintf(tw, "organizations\t%s\n", orDash(strings.Join(orgs, ", ")))
				return tw.Flush()
			}
		},
	},
	{
		name:     "list",
		aliases:  []string{"ls", "tickets"},
		synopsis: "[OPTIONS] [VIEW] [SEARCH...]",
		summary:  "list tickets, as the queue or My tickets",
		about: `
List tickets as the queue shows them (staff) or as "My tickets"
(requesters), 25 a page, newest change first. A first word that names a
view chooses it ('stopan list all'); the other words search the subject
and text, or find a ticket number. To search for a view's name, use
--search ('stopan list --search done').

Views for staff: unsolved (default), mine, unassigned, pending,
postponed, done, all. For requesters: open (default), done, all.
Statuses: new, open, pending, postponed, done.
Priorities (staff): low, normal, high, urgent.
Assignee (staff): me, none, or a user id.`,
		permute: true,
		setup: func(f *flagDefs) action {
			view := f.String("v", "view", "VIEW", "", "queue view")
			search := f.String("q", "search", "TEXT", "", "search the subject and text, or find a number")
			status := f.String("s", "status", "STATUS", "", "only this status")
			prio := f.String("p", "priority", "PRIORITY", "", "only this priority")
			assignee := f.String("a", "assignee", "WHO", "", "only tickets of me, none, or a user id")
			org := f.String("o", "org", "ID", "", "only this organization")
			sort := f.String("", "sort", "ORDER", "", "updated, created or priority")
			page := f.Int("", "page", "N", 1, "page number")
			asJSON := f.Bool("j", "json", "print the API answer as JSON")
			return func(e env, args []string) error {
				if len(args) > 0 && *view == "" && isView(args[0]) {
					*view, args = args[0], args[1:]
				}
				if err := oneOf("status", *status, api.Statuses); err != nil {
					return err
				}
				if err := oneOf("priority", *prio, api.Priorities); err != nil {
					return err
				}
				if err := oneOf("sort", *sort, api.Sorts); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				l, raw, err := c.Tickets(api.ListFilter{View: *view, Status: *status, Priority: *prio,
					Assignee: *assignee, Organization: *org, Sort: *sort, Page: *page,
					Query: strings.TrimSpace(strings.Join(append([]string{*search}, args...), " "))})
				if err != nil {
					return err
				}
				if *asJSON {
					return printRaw(e.out, raw)
				}
				return printTickets(e.out, l, time.Now())
			}
		},
	},
	{
		name:     "show",
		aliases:  []string{"view", "cat"},
		synopsis: "[OPTIONS] TICKET",
		summary:  "show a ticket with its timeline",
		about: `
Show one ticket: its fields, the request text and the timeline of
replies, internal notes (staff) and changes, oldest first.`,
		permute: true,
		setup: func(f *flagDefs) action {
			asJSON := f.Bool("j", "json", "print the API answer as JSON")
			return func(e env, args []string) error {
				n, err := oneTicket(args)
				if err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				t, raw, err := c.Ticket(n)
				if err != nil {
					return err
				}
				if *asJSON {
					return printRaw(e.out, raw)
				}
				_, err = io.WriteString(e.out, text.Ticket(t, 0, time.Now()))
				return err
			}
		},
	},
	{
		name:     "new",
		aliases:  []string{"create"},
		synopsis: "[OPTIONS] --subject SUBJECT [TEXT...]",
		summary:  "open a new ticket",
		about: `
Open a ticket. The request text is TEXT, or standard input when it is not
a terminal, or else written in $EDITOR. Requesters with more than one
organization, and staff, choose the organization with --org (an id or a
name).`,
		permute: true,
		setup: func(f *flagDefs) action {
			subject := f.String("t", "subject", "SUBJECT", "", "the ticket's subject")
			org := f.String("o", "org", "ORG", "", "organization id or name")
			prio := f.String("p", "priority", "PRIORITY", "", "priority (staff)")
			assignee := f.String("a", "assignee", "WHO", "", "assignee: me or a user id (staff)")
			requester := f.Int("", "requester", "ID", 0, "requester's user id (staff)")
			md := f.Bool("m", "markdown", "render the text as Markdown")
			asJSON := f.Bool("j", "json", "print the API answer as JSON")
			return func(e env, args []string) error {
				if strings.TrimSpace(*subject) == "" {
					return usagef("--subject is required")
				}
				if err := oneOf("priority", *prio, api.Priorities); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				nt := api.NewTicket{Subject: *subject, Markdown: *md, Priority: *prio, RequesterID: int64(*requester)}
				if *org != "" || *assignee == "me" {
					me, _, err := c.Me()
					if err != nil {
						return err
					}
					if nt.OrganizationID, err = findOrg(me, *org); err != nil {
						return err
					}
					if *assignee == "me" {
						nt.AssigneeID = me.ID
					}
				}
				if *assignee != "" && *assignee != "me" {
					id, err := strconv.ParseInt(*assignee, 10, 64)
					if err != nil {
						return usagef("--assignee must be me or a user id")
					}
					nt.AssigneeID = id
				}
				if nt.Description, err = body(e, args, "# New ticket: "+*subject); err != nil {
					return err
				}
				t, raw, err := c.CreateTicket(nt)
				if err != nil {
					return err
				}
				if *asJSON {
					return printRaw(e.out, raw)
				}
				fmt.Fprintf(e.out, "created #%d %s\n%s\n", t.Number, t.Subject, t.URL)
				return nil
			}
		},
	},
	commentCommand("reply", "add a public reply", false),
	commentCommand("note", "add an internal note (staff)", true),
	{
		name:     "set",
		synopsis: "[OPTIONS] TICKET...",
		summary:  "change status, priority or assignee (staff)",
		about: `
Change one or more tickets, as the fields on the ticket page do. Each
ticket is changed on its own; a failure is reported and the rest go on.

Statuses: new, open, pending, postponed, done (only the moves the
ticket's status allows). Priorities: low, normal, high, urgent.
Assignee: me, none, or a user id.`,
		permute: true,
		setup: func(f *flagDefs) action {
			status := f.String("s", "status", "STATUS", "", "new status")
			prio := f.String("p", "priority", "PRIORITY", "", "new priority")
			assignee := f.String("a", "assignee", "WHO", "", "me, none, or a user id")
			return func(e env, args []string) error {
				nums, err := tickets(args)
				if err != nil {
					return err
				}
				ch := api.Change{Status: *status, Priority: *prio, Assignee: *assignee}
				if ch.Empty() {
					return usagef("nothing to change; give --status, --priority or --assignee")
				}
				if err := oneOf("status", ch.Status, api.Statuses); err != nil {
					return err
				}
				if err := oneOf("priority", ch.Priority, api.Priorities); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				if ch.Assignee == "me" {
					me, _, err := c.Me()
					if err != nil {
						return err
					}
					ch.Assignee = strconv.FormatInt(me.ID, 10)
				} else if ch.Assignee != "" && ch.Assignee != "none" {
					if _, err := strconv.ParseInt(ch.Assignee, 10, 64); err != nil {
						return usagef("--assignee must be me, none or a user id")
					}
				}
				return each(e, nums, func(n int64) (string, error) {
					t, _, err := c.UpdateTicket(n, ch)
					return fmt.Sprintf("#%d %s, %s, %s", t.Number, t.Status, t.Priority, userName(t.Assignee)), err
				})
			}
		},
	},
	{
		name:     "done",
		aliases:  []string{"resolve"},
		synopsis: "TICKET...",
		summary:  "mark tickets as done",
		about: `
Mark tickets as done: the requester's "Mark as done", or for staff the
status done.`,
		permute: true,
		setup: func(f *flagDefs) action {
			return func(e env, args []string) error {
				nums, err := tickets(args)
				if err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				me, _, err := c.Me()
				if err != nil {
					return err
				}
				return each(e, nums, func(n int64) (string, error) {
					var t api.Ticket
					var err error
					if me.Staff() {
						t, _, err = c.UpdateTicket(n, api.Change{Status: "done"})
					} else {
						t, _, err = c.Resolve(n)
					}
					return fmt.Sprintf("#%d %s", t.Number, t.Status), err
				})
			}
		},
	},
	{
		name:     "open",
		aliases:  []string{"browse"},
		synopsis: "TICKET",
		summary:  "open a ticket in the browser",
		about:    "Open the ticket's page on the Stopan site in the default browser.",
		permute:  true,
		setup: func(f *flagDefs) action {
			return func(e env, args []string) error {
				n, err := oneTicket(args)
				if err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				t, _, err := c.Ticket(n)
				if err != nil {
					return err
				}
				return browse(t.URL)
			}
		},
	},
	{
		name:     "notifications",
		aliases:  []string{"notif", "inbox"},
		synopsis: "[OPTIONS]",
		summary:  "list your notifications",
		about:    "List your notifications, newest first, 50 a page. Unread ones are marked with '*'.",
		permute:  true,
		setup: func(f *flagDefs) action {
			unread := f.Bool("u", "unread", "only unread notifications")
			page := f.Int("", "page", "N", 1, "page number")
			asJSON := f.Bool("j", "json", "print the API answer as JSON")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				l, raw, err := c.Notifications(*page)
				if err != nil {
					return err
				}
				if *asJSON {
					return printRaw(e.out, raw)
				}
				return printNotifications(e.out, l, *unread, time.Now())
			}
		},
	},
	{
		name:     "read",
		synopsis: "[OPTIONS] ID...",
		summary:  "mark notifications as read",
		about:    "Mark notifications as read by id (see 'stopan notifications'), or all of them with --all.",
		permute:  true,
		setup: func(f *flagDefs) action {
			all := f.Bool("a", "all", "mark all notifications as read")
			return func(e env, args []string) error {
				if *all == (len(args) > 0) {
					return usagef("give notification ids or --all")
				}
				c, err := client()
				if err != nil {
					return err
				}
				if *all {
					return c.ReadAllNotifications()
				}
				for _, a := range args {
					id, err := strconv.ParseInt(a, 10, 64)
					if err != nil {
						return usagef("%q is not a notification id", a)
					}
					if err := c.ReadNotification(id); err != nil {
						return fmt.Errorf("notification %d: %w", id, err)
					}
				}
				return nil
			}
		},
	},
	{
		name:     "tokens",
		synopsis: "[OPTIONS]",
		summary:  "list your API tokens",
		about: `
List your API tokens (never the secrets). New tokens are made on the
account page of the Stopan site; the API cannot make them.`,
		permute: true,
		setup: func(f *flagDefs) action {
			asJSON := f.Bool("j", "json", "print the API answer as JSON")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				ts, raw, err := c.Tokens()
				if err != nil {
					return err
				}
				if *asJSON {
					return printRaw(e.out, raw)
				}
				return printTokens(e.out, ts, tokenHint(c.Token))
			}
		},
	},
	{
		name:     "revoke",
		synopsis: "ID...",
		summary:  "revoke API tokens",
		about: `
Revoke your API tokens by id (see 'stopan tokens'). Revoking the token in
use is allowed; later commands then fail until the next login.`,
		permute: true,
		setup: func(f *flagDefs) action {
			return func(e env, args []string) error {
				if len(args) == 0 {
					return usagef("give token ids")
				}
				c, err := client()
				if err != nil {
					return err
				}
				for _, a := range args {
					id, err := strconv.ParseInt(a, 10, 64)
					if err != nil {
						return usagef("%q is not a token id", a)
					}
					if err := c.RevokeToken(id); err != nil {
						return fmt.Errorf("token %d: %w", id, err)
					}
					fmt.Fprintf(e.out, "revoked token %d\n", id)
				}
				return nil
			}
		},
	},
	{
		name:     "update",
		synopsis: "[OPTIONS]",
		summary:  "update stopan to the latest release",
		about: `
Look up the latest release on GitHub and install it the way stopan was
installed: brew upgrade for Homebrew, the release's .deb through apt for
the Debian package, otherwise the binary is replaced with the one from the
release tarball. Downloads are checked against the release's SHA256SUMS.
Asks for confirmation unless --yes is given. A build from source is not
replaced.`,
		permute: true,
		setup: func(f *flagDefs) action {
			check := f.Bool("c", "check", "only report whether an update is available")
			yes := f.Bool("y", "yes", "do not ask for confirmation")
			return func(e env, args []string) error {
				if err := noArgs(args); err != nil {
					return err
				}
				return selfUpdate(e, *check, *yes)
			}
		},
	},
}

// commentCommand is reply or note: the same command with or without the
// internal flag.
func commentCommand(name, summary string, internal bool) command {
	what := "a public reply, which the requester sees and gets by email"
	if internal {
		what = "an internal note, which only staff see"
	}
	return command{
		name:     name,
		synopsis: "[OPTIONS] TICKET [TEXT...]",
		summary:  summary,
		about: fmt.Sprintf(`
Add %s. The text is TEXT, or standard input when it is not a terminal,
or else written in $EDITOR (default vi). Lines starting with '#' are
dropped from the editor's text. Staff may change the status with the
same message (--status).`, what),
		permute: true,
		setup: func(f *flagDefs) action {
			status := f.String("s", "status", "STATUS", "", "also change the status (staff)")
			md := f.Bool("m", "markdown", "render the text as Markdown")
			return func(e env, args []string) error {
				if len(args) == 0 {
					return usagef("missing TICKET")
				}
				n, err := ticketNumber(args[0])
				if err != nil {
					return err
				}
				if err := oneOf("status", *status, api.Statuses); err != nil {
					return err
				}
				c, err := client()
				if err != nil {
					return err
				}
				text, err := body(e, args[1:], fmt.Sprintf("# %s to ticket #%d", strings.ToUpper(name[:1])+name[1:], n))
				if err != nil {
					return err
				}
				cm, _, err := c.AddComment(n, api.NewComment{Body: text, Internal: internal, Markdown: *md, Status: *status})
				if err != nil {
					return err
				}
				kind := "reply"
				if cm.Internal {
					kind = "note"
				}
				fmt.Fprintf(e.out, "added %s %d to #%d\n", kind, cm.ID, n)
				return nil
			}
		},
	}
}

func noArgs(args []string) error {
	if len(args) > 0 {
		return usagef("unexpected argument %q", args[0])
	}
	return nil
}

func oneOf(name, v string, allowed []string) error {
	if v != "" && !slices.Contains(allowed, v) {
		return usagef("%s must be one of %s", name, strings.Join(allowed, ", "))
	}
	return nil
}

// isView tells whether s names a view of staff or of requesters; the
// server falls back to its default for a view the user does not have.
func isView(s string) bool {
	return slices.Contains(api.StaffViews, s) || slices.Contains(api.RequesterView, s)
}

func ticketNumber(s string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimPrefix(s, "#"), 10, 64)
	if err != nil || n <= 0 {
		return 0, usagef("%q is not a ticket number", s)
	}
	return n, nil
}

func oneTicket(args []string) (int64, error) {
	switch len(args) {
	case 0:
		return 0, usagef("missing TICKET")
	case 1:
		return ticketNumber(args[0])
	}
	return 0, usagef("unexpected argument %q", args[1])
}

func tickets(args []string) ([]int64, error) {
	if len(args) == 0 {
		return nil, usagef("missing TICKET")
	}
	var nums []int64
	for _, a := range args {
		n, err := ticketNumber(a)
		if err != nil {
			return nil, err
		}
		nums = append(nums, n)
	}
	return nums, nil
}

// each runs fn for every ticket, prints its result line, and reports
// failures together at the end.
func each(e env, nums []int64, fn func(int64) (string, error)) error {
	failed := 0
	for _, n := range nums {
		line, err := fn(n)
		if err != nil {
			fmt.Fprintf(e.err, "stopan: #%d: %v\n", n, err)
			failed++
			continue
		}
		fmt.Fprintln(e.out, line)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d tickets failed", failed, len(nums))
	}
	return nil
}

func findOrg(me api.Me, org string) (int64, error) {
	if org == "" {
		return 0, nil
	}
	if id, err := strconv.ParseInt(org, 10, 64); err == nil {
		return id, nil
	}
	var found []api.Ref
	for _, o := range me.Organizations {
		if strings.EqualFold(o.Name, org) {
			return o.ID, nil
		}
		if strings.Contains(strings.ToLower(o.Name), strings.ToLower(org)) {
			found = append(found, o)
		}
	}
	if len(found) == 1 {
		return found[0].ID, nil
	}
	if len(found) > 1 {
		return 0, usagef("%q matches %d organizations; use the id", org, len(found))
	}
	return 0, usagef("no organization of yours matches %q", org)
}

// client loads the config and returns an API client.
func client() (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return api.New(cfg.URL, cfg.Token, "stopan-cli/"+version)
}

// body is the text of a ticket or a comment: the arguments, else
// standard input when it is not a terminal, else the editor.
func body(e env, args []string, header string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	if f, ok := e.in.(*os.File); !ok || !term.IsTerminal(int(f.Fd())) {
		b, err := io.ReadAll(io.LimitReader(e.in, 1<<20))
		if err != nil {
			return "", err
		}
		return nonEmpty(string(b))
	}
	s, err := tui.Edit(header)
	if err != nil {
		return "", err
	}
	return nonEmpty(s)
}

func nonEmpty(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty text; nothing sent")
	}
	return s, nil
}

func login(e env, server string, tokenStdin bool) error {
	saved, err := config.FileOnly()
	if err != nil {
		return err
	}
	in := bufio.NewReader(e.in)
	if server == "" {
		def := saved.URL
		if def == "" {
			def = defaultURL
		}
		if tokenStdin {
			server = def
		} else {
			fmt.Fprintf(e.out, "Server URL [%s]: ", def)
			line, err := in.ReadString('\n')
			if err != nil && line == "" {
				return errors.New("no server URL given")
			}
			if server = strings.TrimSpace(line); server == "" {
				server = def
			}
		}
	}
	base, err := api.NormalizeURL(server)
	if err != nil {
		return err
	}
	token, err := readToken(e, in, tokenStdin)
	if err != nil {
		return err
	}
	c, err := api.New(base, token, "stopan-cli/"+version)
	if err != nil {
		return err
	}
	me, _, err := c.Me()
	if err != nil {
		return fmt.Errorf("checking the token with %s: %w", base, err)
	}
	path, err := config.Save(config.Config{URL: base, Token: token})
	if err != nil {
		return err
	}
	fmt.Fprintf(e.out, "logged in to %s as %s (%s); saved in %s\n", base, me.Name, me.Role, path)
	return nil
}

func readToken(e env, in *bufio.Reader, fromStdin bool) (string, error) {
	f, isFile := e.in.(*os.File)
	if !fromStdin && isFile && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(e.out, "Token (not shown): ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(e.out)
		if err != nil {
			return "", err
		}
		return checkToken(string(b))
	}
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no token given")
	}
	return checkToken(line)
}

func checkToken(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("no token given")
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return "", errors.New("the token must be one word")
	}
	return s, nil
}

func logout(e env, revoke bool) error {
	saved, err := config.FileOnly()
	if err != nil {
		return err
	}
	if revoke {
		c, err := client()
		if err != nil {
			return err
		}
		if err := revokeCurrent(c); err != nil {
			return err
		}
		fmt.Fprintln(e.out, "revoked the token on", c.Base)
	}
	if saved.Token == "" {
		fmt.Fprintln(e.out, "no saved token")
		return nil
	}
	saved.Token = ""
	if _, err := config.Save(saved); err != nil {
		return err
	}
	fmt.Fprintln(e.out, "removed the saved token")
	if os.Getenv("STOPAN_TOKEN") != "" {
		fmt.Fprintln(e.out, "STOPAN_TOKEN is still set in the environment")
	}
	return nil
}

// revokeCurrent finds the token in use among the user's tokens by its
// last four characters and revokes it.
func revokeCurrent(c *api.Client) error {
	ts, _, err := c.Tokens()
	if err != nil {
		return err
	}
	hint := tokenHint(c.Token)
	var match []api.Token
	for _, t := range ts {
		if t.Hint == hint {
			match = append(match, t)
		}
	}
	if len(match) != 1 {
		return fmt.Errorf("cannot tell which token is in use (%d end in %q); revoke it with 'stopan revoke ID'", len(match), hint)
	}
	return c.RevokeToken(match[0].ID)
}

func tokenHint(token string) string {
	if len(token) < 4 {
		return token
	}
	return token[len(token)-4:]
}

func browse(url string) error {
	if url == "" {
		return errors.New("the server sent no page URL")
	}
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, url).Run()
}

func selfUpdate(e env, check, yes bool) error {
	c := update.New()
	latest, err := c.Latest()
	if err != nil {
		return err
	}
	current, release := update.Parse(version)
	if !release {
		fmt.Fprintf(e.out, "stopan %s is a build from source; the latest release is %s\n", version, latest)
		if check {
			return nil
		}
		return fmt.Errorf("not replacing a build from source; update it with git pull and make install")
	}
	if current.Compare(latest) >= 0 {
		fmt.Fprintf(e.out, "stopan %s is up to date\n", version)
		return nil
	}
	fmt.Fprintf(e.out, "update available: %s -> %s\n", current, latest)
	if check {
		return nil
	}
	exe, err := update.Executable()
	if err != nil {
		return err
	}
	method := update.Detect(exe, runtime.GOOS, update.DpkgOwns)
	q := fmt.Sprintf("Update stopan %s -> %s (%s)?", current, latest, method)
	if !yes && !confirm(e.out, bufio.NewReader(e.in), q) {
		fmt.Fprintln(e.out, "skipped")
		return nil
	}
	if err := c.Install(method, latest, exe); err != nil {
		return err
	}
	if method == update.Binary {
		fmt.Fprintf(e.out, "updated stopan %s -> %s (%s)\n", current, latest, exe)
	}
	return nil
}

func confirm(w io.Writer, r *bufio.Reader, question string) bool {
	fmt.Fprintf(w, "%s [y/N] ", question)
	line, _ := r.ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(line), "y")
}

// Output.

func printRaw(w io.Writer, raw []byte) error {
	var buf bytes.Buffer
	if len(raw) == 0 {
		raw = []byte("null")
	}
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

func printTickets(w io.Writer, l api.TicketList, now time.Time) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TICKET\tSTATUS\tPRIO\tUPDATED\tREQUESTER\tASSIGNEE\tSUBJECT")
	for _, t := range l.Tickets {
		fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\t%s\t%s\t%s\n", t.Number, t.Status, t.Priority,
			text.Ago(t.UpdatedAt, now), text.Cut(userName(t.Requester), 20), text.Cut(userName(t.Assignee), 20),
			text.Cut(t.Subject, 70))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "%s: page %d of %d, %d tickets\n", l.View, l.Page, l.TotalPages, l.TotalCount)
	return nil
}

func printNotifications(w io.Writer, l api.NotificationList, unreadOnly bool, now time.Time) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, " \tID\tWHEN\tTICKET\tWHAT")
	for _, n := range l.Notifications {
		if unreadOnly && n.Read {
			continue
		}
		mark, ticket := " ", "-"
		if !n.Read {
			mark = "*"
		}
		if n.Ticket != nil {
			ticket = fmt.Sprintf("#%d", n.Ticket.Number)
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", mark, n.ID, text.Ago(n.CreatedAt, now), ticket, text.Cut(text.Notification(n), 90))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "page %d, %d notifications, %d unread\n", l.Page, l.TotalCount, l.UnreadCount)
	return nil
}

func printTokens(w io.Writer, ts []api.Token, current string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tENDS\tACCESS\tCREATED\tLAST USED\tEXPIRES")
	for _, t := range ts {
		access := "read and write"
		if t.ReadOnly {
			access = "read-only"
		}
		name := t.Name
		if t.Hint == current {
			name += " (this one)"
		}
		fmt.Fprintf(tw, "%d\t%s\t...%s\t%s\t%s\t%s\t%s\n", t.ID, name, t.Hint, access,
			t.CreatedAt.Local().Format("2006-01-02"), date(t.LastUsedAt), date(t.ExpiresAt))
	}
	return tw.Flush()
}

func date(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Local().Format("2006-01-02")
}

func userName(u *api.User) string {
	if u == nil {
		return "-"
	}
	return u.Name
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
