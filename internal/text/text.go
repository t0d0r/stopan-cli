// Package text turns API records into plain text for the CLI and the
// dashboard, so a ticket reads the same in both.
package text

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"stopan-cli/internal/api"
)

// Ago is a short age: "now", "5m", "3h", "2d", then the date.
func Ago(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return t.Local().Format("2006-01-02")
}

// Stamp is a local date and time.
func Stamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// Cut shortens s to n cells, ending with "...". Line breaks become spaces.
func Cut(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if n <= 0 || ansi.StringWidth(s) <= n {
		return s
	}
	if n <= 3 {
		return ansi.Truncate(s, n, "")
	}
	return ansi.Truncate(s, n, "...")
}

// Wrap breaks s at width cells, at spaces where it can; 0 leaves it.
func Wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Wrap(s, width, "")
}

func name(u *api.User) string {
	if u == nil {
		return "system"
	}
	return u.Name
}

func person(u *api.User) string {
	if u == nil {
		return "-"
	}
	if u.Email != "" {
		return u.Name + " <" + u.Email + ">"
	}
	return u.Name
}

func value(v any) string {
	switch x := v.(type) {
	case nil:
		return "none"
	case float64:
		return fmt.Sprintf("%g", x)
	case string:
		if x == "" {
			return "none"
		}
		return x
	}
	return fmt.Sprint(v)
}

// Event is the one-line heading of a timeline entry.
func Event(e api.Event) string {
	who := name(e.Actor)
	switch e.Action {
	case "created":
		return who + " opened the ticket"
	case "replied":
		return who + " replied"
	case "noted":
		return who + " added an internal note"
	case "status_changed":
		return fmt.Sprintf("%s changed the status %s -> %s", who, value(e.From), value(e.To))
	case "priority_changed":
		return fmt.Sprintf("%s changed the priority %s -> %s", who, value(e.From), value(e.To))
	case "assignee_changed":
		return fmt.Sprintf("%s changed the assignee %s -> %s", who, value(e.From), value(e.To))
	case "stage_changed":
		return fmt.Sprintf("%s moved it to another stage (%s -> %s)", who, value(e.From), value(e.To))
	case "shared":
		return who + " shared the ticket by link"
	case "unshared":
		return who + " stopped sharing the ticket"
	case "deleted":
		return who + " deleted the ticket"
	case "restored":
		return who + " restored the ticket"
	}
	return who + " " + strings.ReplaceAll(e.Action, "_", " ")
}

// Notification is one line for a notification.
func Notification(n api.Notification) string {
	if n.Kind == "message" && n.Message != "" {
		return name(n.Actor) + ": " + n.Message
	}
	what := map[string]string{
		"created":        "opened",
		"replied":        "replied on",
		"noted":          "added a note on",
		"assigned":       "assigned you",
		"status_changed": "changed the status of",
	}[n.Kind]
	if what == "" {
		what = strings.ReplaceAll(n.Kind, "_", " ")
	}
	s := name(n.Actor) + " " + what
	if n.Ticket != nil {
		s += fmt.Sprintf(" #%d %s", n.Ticket.Number, n.Ticket.Subject)
	}
	if n.Kind == "status_changed" && n.Status != "" {
		s += " -> " + n.Status
	}
	return s
}

// Ticket is a ticket with its fields, text and timeline. width wraps
// the texts; 0 leaves them to the terminal.
func Ticket(t api.Ticket, width int, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d  %s\n\n", t.Number, t.Subject)
	row := func(k, v string) { fmt.Fprintf(&b, "  %-13s %s\n", k, v) }
	row("status", t.Status)
	row("priority", t.Priority)
	row("organization", t.Organization.Name)
	if t.Stage != nil {
		row("stage", t.Stage.Name)
	}
	row("requester", person(t.Requester))
	row("assignee", person(t.Assignee))
	if t.Source != "" {
		row("source", t.Source)
	}
	row("created", Stamp(t.CreatedAt)+"  ("+Ago(t.CreatedAt, now)+")")
	row("updated", Stamp(t.UpdatedAt)+"  ("+Ago(t.UpdatedAt, now)+")")
	if t.DoneAt != nil {
		row("done", Stamp(*t.DoneAt))
	}
	if t.ShareURL != "" {
		row("shared", t.ShareURL)
	} else if t.Shared {
		row("shared", "yes")
	}
	if len(t.NextStatuses) > 0 {
		row("next status", strings.Join(t.NextStatuses, ", "))
	}
	row("page", t.URL)
	if t.Description != "" {
		b.WriteString("\n")
		b.WriteString(indent(Wrap(strings.TrimSpace(t.Description), width-2)))
	}
	files(&b, t.Files)
	for _, e := range t.Timeline {
		if e.Action == "created" {
			continue
		}
		b.WriteString("\n")
		head := fmt.Sprintf("-- %s  %s", Stamp(e.CreatedAt), Event(e))
		if e.Internal {
			head += "  [internal]"
		}
		b.WriteString(head + "\n")
		if c := e.Comment; c != nil {
			b.WriteString(indent(Wrap(strings.TrimSpace(c.Body), width-2)))
			if c.EditedAt != nil {
				b.WriteString("  (edited " + Stamp(*c.EditedAt) + ")\n")
			}
			files(&b, c.Files)
		}
	}
	return b.String()
}

func files(b *strings.Builder, fs []api.File) {
	for _, f := range fs {
		fmt.Fprintf(b, "  [file] %s (%s, %s)\n", f.Filename, f.ContentType, Bytes(f.ByteSize))
	}
}

func indent(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// Bytes is a size in B, K, M or G.
func Bytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%dB", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	case n < 1<<30:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
}
