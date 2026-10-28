package tui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"

	"stopan-cli/internal/api"
)

type recorder struct {
	mu   sync.Mutex
	reqs []string
}

func (rc *recorder) has(s string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for _, r := range rc.reqs {
		if strings.Contains(r, s) {
			return true
		}
	}
	return false
}

func newTestModel(t *testing.T, role string) (model, *recorder) {
	t.Helper()
	rc := &recorder{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rc.mu.Lock()
		rc.reqs = append(rc.reqs, r.Method+" "+r.URL.RequestURI()+" "+string(b))
		rc.mu.Unlock()
		ticket := `{"number":1001,"subject":"Printer on fire","status":"open","priority":"urgent","organization":{"id":1,"name":"Acme"},
			"updated_at":"2026-10-28T11:00:00Z","description":"It smokes.","next_statuses":["pending","postponed","done"],"timeline":[]}`
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/me":
			io.WriteString(w, `{"user":{"id":7,"name":"Ben","role":"`+role+`"}}`)
		case "GET /api/v1/tickets":
			io.WriteString(w, `{"tickets":[`+ticket+`,{"number":1002,"subject":"VPN","status":"new","priority":"normal","organization":{"id":1,"name":"Acme"}}],
				"view":"unsolved","page":1,"total_pages":1,"total_count":2}`)
		case "GET /api/v1/tickets/1001", "PATCH /api/v1/tickets/1001", "POST /api/v1/tickets/1001/resolve":
			io.WriteString(w, `{"ticket":`+ticket+`}`)
		case "GET /api/v1/notifications":
			io.WriteString(w, `{"notifications":[{"id":4,"kind":"replied","read":false,"actor":{"id":1,"name":"Ana"},"ticket":{"number":1001,"subject":"Printer on fire"}}],"page":1,"total_count":1,"unread_count":1}`)
		case "POST /api/v1/notifications/4/read":
			io.WriteString(w, `{"notification":{"id":4,"read":true}}`)
		default:
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"code":"not_found","message":"Not found"}}`)
		}
	}))
	t.Cleanup(ts.Close)
	c, err := api.New(ts.URL, "stopan_test", "test")
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(c, Options{})
	m.search.Cursor.SetMode(cursor.CursorStatic)
	m.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 20})
	return feed(t, m, m.Init()), rc
}

func step(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, cmd := m.Update(msg)
	return feed(t, next.(model), cmd)
}

// feed runs a command and every command it leads to, like the program
// loop, but synchronously. Commands that suspend the terminal are not run.
func feed(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch msg := msg.(type) {
	case nil:
		return m
	case tea.BatchMsg:
		for _, c := range msg {
			m = feed(t, m, c)
		}
		return m
	case tea.QuitMsg:
		return m
	}
	// The editor would suspend the terminal; the cursor blinks forever.
	if n := fmt.Sprintf("%T", msg); strings.Contains(n, "exec") || strings.HasPrefix(n, "cursor.") {
		return m
	}
	return step(t, m, msg)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestListAndViews(t *testing.T) {
	m, rc := newTestModel(t, "agent")
	v := m.View()
	for _, want := range []string{"1 unsolved", "7 all", "#1001", "Printer on fire", "#1002", "inbox 1", "2 tickets"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q in\n%s", want, v)
		}
	}
	if wide := step(t, m, tea.WindowSizeMsg{Width: 160, Height: 20}).View(); !strings.Contains(wide, "Ben @ 127.0.0.1") {
		t.Errorf("a wide header names the user and server:\n%s", wide)
	}
	m = step(t, m, key("tab"))
	if m.views[m.view] != "mine" || !rc.has("view=mine") {
		t.Errorf("tab should load the mine view, view=%d", m.view)
	}
	m = step(t, m, key("7"))
	if !rc.has("view=all") {
		t.Error("7 should load the all view")
	}
	m = step(t, m, key("/"))
	for _, r := range "vpn" {
		m = step(t, m, key(string(r)))
	}
	m = step(t, m, key("enter"))
	if m.query != "vpn" || !rc.has("q=vpn") {
		t.Errorf("search query = %q", m.query)
	}
}

func TestRequesterViews(t *testing.T) {
	m, _ := newTestModel(t, "requester")
	if strings.Join(m.views, ",") != "open,done,all" {
		t.Errorf("views = %v", m.views)
	}
	m = step(t, m, key("enter"))
	m = step(t, m, key("S"))
	if m.mode != normal || !strings.Contains(m.status, "only staff") {
		t.Errorf("requester status change: mode %d, %q", m.mode, m.status)
	}
}

func TestOpenTicketAndPickStatus(t *testing.T) {
	m, rc := newTestModel(t, "agent")
	m = step(t, m, key("enter"))
	if m.screen != ticketScreen || !strings.Contains(m.View(), "It smokes.") {
		t.Fatalf("enter should open the ticket:\n%s", m.View())
	}
	m = step(t, m, key("S"))
	if m.mode != picking || !strings.Contains(m.View(), "status:") {
		t.Fatalf("S should open the status picker:\n%s", m.View())
	}
	m = step(t, m, key("right"))
	m = step(t, m, key("enter"))
	if !rc.has(`PATCH /api/v1/tickets/1001 {"ticket":{"status":"postponed"}}`) {
		t.Errorf("picker did not send the status; requests: %v", rc.reqs)
	}
	m = step(t, m, key("D"))
	if m.mode != confirming {
		t.Fatal("D should ask first")
	}
	m = step(t, m, key("n"))
	if rc.has(`"status":"done"`) {
		t.Error("answering n still marked it done")
	}
	m = step(t, m, key("esc"))
	if m.screen != listScreen {
		t.Error("esc should go back to the list")
	}
}

func TestInbox(t *testing.T) {
	m, rc := newTestModel(t, "agent")
	m = step(t, m, key("n"))
	if m.screen != notifScreen || !strings.Contains(m.View(), "Ana replied on #1001") {
		t.Fatalf("inbox:\n%s", m.View())
	}
	m = step(t, m, key("enter"))
	if !rc.has("POST /api/v1/notifications/4/read") || m.screen != ticketScreen {
		t.Errorf("enter should mark read and open the ticket (screen %d)", m.screen)
	}
}

func TestEditedAsksBeforeSending(t *testing.T) {
	m, rc := newTestModel(t, "agent")
	m = step(t, m, key("enter"))
	m.composing = "note"
	m = step(t, m, editedMsg{text: "Bring water."})
	if m.mode != confirming || !strings.Contains(m.question, "Send note to #1001") {
		t.Fatalf("question = %q", m.question)
	}
	m = step(t, m, key("y"))
	if !rc.has(`POST /api/v1/tickets/1001/comments {"comment":{"body":"Bring water.","internal":true}}`) {
		t.Errorf("note not sent: %v", rc.reqs)
	}
}

func TestEditorFile(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true --flag")
	cmd, path, err := editorCommand("Reply to #1\nsecond line")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "true" || cmd.Args[1] != "--flag" || cmd.Args[2] != path {
		t.Errorf("args = %v", cmd.Args)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 || filepath.Ext(path) != ".md" {
		t.Errorf("temp file %s mode %v", path, fi.Mode().Perm())
	}
	os.WriteFile(path, []byte("Hello\n# dropped\nthere\n\n"), 0o600)
	s, err := readEdited(path)
	if err != nil || s != "Hello\nthere" {
		t.Errorf("text = %q, %v", s, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("temp file left behind")
	}
}
