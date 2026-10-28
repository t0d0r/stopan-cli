package text

import (
	"strings"
	"testing"
	"time"

	"stopan-cli/internal/api"
)

func TestAgo(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	cases := map[time.Duration]string{
		10 * time.Second: "now",
		5 * time.Minute:  "5m",
		3 * time.Hour:    "3h",
		50 * time.Hour:   "2d",
	}
	for d, want := range cases {
		if got := Ago(now.Add(-d), now); got != want {
			t.Errorf("Ago(-%v) = %q, want %q", d, got, want)
		}
	}
	if got := Ago(time.Time{}, now); got != "-" {
		t.Errorf("zero = %q", got)
	}
}

func TestCut(t *testing.T) {
	if got := Cut("a\nb   c", 10); got != "a b c" {
		t.Errorf("got %q", got)
	}
	if got := Cut("abcdefghij", 6); got != "abc..." {
		t.Errorf("got %q", got)
	}
	if got := Cut("Принтерът гори", 9); got != "Принте..." {
		t.Errorf("got %q", got)
	}
}

func TestTicket(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	agent := &api.User{ID: 2, Name: "Ben", Role: "agent"}
	tk := api.Ticket{
		Number: 1001, Subject: "Printer on fire", Status: "open", Priority: "high",
		Organization: api.Ref{ID: 1, Name: "Acme"},
		Requester:    &api.User{ID: 1, Name: "Ana", Email: "ana@example.org"},
		Description:  "It smokes.",
		CreatedAt:    now.Add(-2 * time.Hour), UpdatedAt: now,
		Timeline: []api.Event{
			{Action: "created", Actor: agent},
			{Action: "replied", Actor: agent, CreatedAt: now, Comment: &api.Comment{Body: "On my way."}},
			{Action: "noted", Actor: agent, Internal: true, Comment: &api.Comment{Body: "Bring water."}},
			{Action: "status_changed", Actor: agent, From: "new", To: "open"},
		},
	}
	out := Ticket(tk, 0, now)
	for _, want := range []string{
		"#1001  Printer on fire", "Ana <ana@example.org>", "assignee      -", "  It smokes.",
		"Ben replied", "  On my way.", "internal note  [internal]", "status new -> open",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "opened the ticket") {
		t.Errorf("the created event repeats the header:\n%s", out)
	}
}

func TestNotification(t *testing.T) {
	n := api.Notification{Kind: "replied", Actor: &api.User{Name: "Ana"}}
	n.Ticket = &struct {
		Number  int64  `json:"number"`
		Subject string `json:"subject"`
	}{12, "VPN"}
	if got := Notification(n); got != "Ana replied on #12 VPN" {
		t.Errorf("got %q", got)
	}
}
