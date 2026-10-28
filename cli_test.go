package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fake is a Stopan API with one agent, one ticket and one notification.
// It records the requests it gets.
type fake struct {
	mu   sync.Mutex
	reqs []string          // "METHOD /path?query"
	body map[string]string // last JSON body by "METHOD /path"
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	key := r.Method + " " + r.URL.Path
	f.reqs = append(f.reqs, key+"?"+r.URL.RawQuery)
	f.body[key] = string(b)
	f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer stopan_test" {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"code":"unauthorized","message":"Invalid token"}}`)
		return
	}
	ticket := `{"number":1001,"reference":"#1001","subject":"Printer on fire","status":"open","priority":"high",
		"organization":{"id":1,"name":"Acme"},"requester":{"id":1,"name":"Ana","role":"requester"},
		"assignee":null,"created_at":"2026-10-28T10:00:00Z","updated_at":"2026-10-28T11:00:00Z",
		"url":"https://desk.example.org/tickets/1001","description":"It smokes.","next_statuses":["pending","done"],
		"timeline":[{"id":1,"action":"replied","actor":{"id":7,"name":"Ben","role":"agent"},"created_at":"2026-10-28T11:00:00Z",
		"comment":{"id":3,"body":"On my way.","author":{"id":7,"name":"Ben","role":"agent"},"created_at":"2026-10-28T11:00:00Z"}}]}`
	switch key {
	case "GET /api/v1/me":
		io.WriteString(w, `{"user":{"id":7,"name":"Ben","role":"agent","email":"ben@example.org","organizations":[{"id":1,"name":"Acme"},{"id":2,"name":"Beta Corp"}]}}`)
	case "GET /api/v1/tickets":
		io.WriteString(w, `{"tickets":[`+ticket+`],"view":"unsolved","page":1,"total_pages":1,"total_count":1}`)
	case "GET /api/v1/tickets/1001", "PATCH /api/v1/tickets/1001", "POST /api/v1/tickets":
		io.WriteString(w, `{"ticket":`+ticket+`}`)
	case "POST /api/v1/tickets/1001/comments":
		var in struct {
			Comment struct {
				Internal bool `json:"internal"`
			} `json:"comment"`
		}
		json.Unmarshal(b, &in)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"comment": map[string]any{"id": 9, "internal": in.Comment.Internal}})
	case "GET /api/v1/notifications":
		io.WriteString(w, `{"notifications":[{"id":4,"kind":"replied","read":false,"actor":{"id":1,"name":"Ana"},
			"created_at":"2026-10-28T11:00:00Z","ticket":{"number":1001,"subject":"Printer on fire"}}],"page":1,"total_count":1,"unread_count":1}`)
	case "GET /api/v1/tokens":
		io.WriteString(w, `{"tokens":[{"id":3,"name":"laptop","hint":"test","read_only":false,"created_at":"2026-10-27T10:00:00Z"}]}`)
	case "DELETE /api/v1/tokens/3", "POST /api/v1/notifications/read_all":
		w.WriteHeader(204)
	default:
		w.WriteHeader(404)
		io.WriteString(w, `{"error":{"code":"not_found","message":"Not found"}}`)
	}
}

func (f *fake) last(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.body[key]
}

func setup(t *testing.T) *fake {
	t.Helper()
	f := &fake{body: map[string]string{}}
	ts := httptest.NewServer(f)
	t.Cleanup(ts.Close)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STOPAN_URL", ts.URL)
	t.Setenv("STOPAN_TOKEN", "stopan_test")
	return f
}

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, env{strings.NewReader(stdin), &out, &errb})
	return code, out.String(), errb.String()
}

func TestListShowMe(t *testing.T) {
	f := setup(t)
	code, out, errs := runCLI(t, "", "list", "--view", "mine", "printer")
	if code != 0 || !strings.Contains(out, "#1001") || !strings.Contains(out, "Printer on fire") || !strings.Contains(out, "1 tickets") {
		t.Fatalf("list: %d %q %q", code, out, errs)
	}
	if got := f.reqs[0]; got != "GET /api/v1/tickets?q=printer&view=mine" {
		t.Errorf("request = %s", got)
	}
	code, out, _ = runCLI(t, "", "show", "#1001")
	if code != 0 || !strings.Contains(out, "It smokes.") || !strings.Contains(out, "Ben replied") {
		t.Errorf("show: %d %q", code, out)
	}
	code, out, _ = runCLI(t, "", "show", "1001", "--json")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Errorf("show --json: %d %q", code, out)
	}
	code, out, _ = runCLI(t, "", "whoami")
	if code != 0 || !strings.Contains(out, "Acme (1), Beta Corp (2)") {
		t.Errorf("me: %d %q", code, out)
	}
}

func TestReplyNoteSet(t *testing.T) {
	f := setup(t)
	if code, out, errs := runCLI(t, "", "reply", "1001", "On", "my", "way", "--status", "pending"); code != 0 || !strings.Contains(out, "added reply 9 to #1001") {
		t.Fatalf("reply: %d %q %q", code, out, errs)
	}
	if got := f.last("POST /api/v1/tickets/1001/comments"); got != `{"comment":{"body":"On my way","status":"pending"}}` {
		t.Errorf("reply body = %s", got)
	}
	if code, out, _ := runCLI(t, "Bring water.\n", "note", "1001"); code != 0 || !strings.Contains(out, "added note") {
		t.Errorf("note from stdin: %d %q", code, out)
	}
	if got := f.last("POST /api/v1/tickets/1001/comments"); got != `{"comment":{"body":"Bring water.","internal":true}}` {
		t.Errorf("note body = %s", got)
	}
	if code, _, errs := runCLI(t, "", "set", "1001", "-a", "me", "-p", "urgent"); code != 0 {
		t.Errorf("set: %d %q", code, errs)
	}
	if got := f.last("PATCH /api/v1/tickets/1001"); got != `{"ticket":{"assignee_id":"7","priority":"urgent"}}` {
		t.Errorf("set body = %s", got)
	}
	if code, _, _ := runCLI(t, "", "done", "1001"); code != 0 || f.last("PATCH /api/v1/tickets/1001") != `{"ticket":{"status":"done"}}` {
		t.Errorf("done as staff should set the status: %d %s", code, f.last("PATCH /api/v1/tickets/1001"))
	}
}

func TestNew(t *testing.T) {
	f := setup(t)
	code, out, errs := runCLI(t, "", "new", "-t", "VPN down", "--org", "beta", "-a", "me", "Cannot", "connect")
	if code != 0 || !strings.Contains(out, "created #1001") {
		t.Fatalf("new: %d %q %q", code, out, errs)
	}
	want := `{"ticket":{"subject":"VPN down","description":"Cannot connect","organization_id":2,"assignee_id":7}}`
	if got := f.last("POST /api/v1/tickets"); got != want {
		t.Errorf("body = %s\nwant   %s", got, want)
	}
}

func TestErrorsAndUsage(t *testing.T) {
	setup(t)
	t.Setenv("STOPAN_TOKEN", "stopan_wrong")
	if code, _, errs := runCLI(t, "", "list"); code != 1 || !strings.Contains(errs, "unauthorized: Invalid token") {
		t.Errorf("bad token: %d %q", code, errs)
	}
	for _, args := range [][]string{
		{"show"}, {"show", "abc"}, {"set", "1"}, {"list", "--status", "closed"}, {"nope"}, {"read"}, {"new", "x"},
	} {
		if code, _, errs := runCLI(t, "", args...); code != 2 || !strings.Contains(errs, "Run 'stopan") {
			t.Errorf("%v: exit %d, %q", args, code, errs)
		}
	}
	if code, out, _ := runCLI(t, "", "--version"); code != 0 || out != "stopan dev\n" {
		t.Errorf("version: %q", out)
	}
	if code, out, _ := runCLI(t, "", "help", "reply"); code != 0 || !strings.Contains(out, "Usage: stopan reply") {
		t.Errorf("help reply: %q", out)
	}
}

func TestLoginLogout(t *testing.T) {
	setup(t)
	url := os.Getenv("STOPAN_URL")
	t.Setenv("STOPAN_URL", "")
	t.Setenv("STOPAN_TOKEN", "")
	code, out, errs := runCLI(t, "stopan_test\n", "login", "--url", url, "--token-stdin")
	if code != 0 || !strings.Contains(out, "as Ben (agent)") {
		t.Fatalf("login: %d %q %q", code, out, errs)
	}
	p := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "stopan", "config.json")
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"token": "stopan_test"`) {
		t.Errorf("config = %s", data)
	}
	if code, out, _ := runCLI(t, "", "tokens"); code != 0 || !strings.Contains(out, "laptop (this one)") {
		t.Errorf("tokens: %d %q", code, out)
	}
	if code, out, errs := runCLI(t, "", "logout", "--revoke"); code != 0 || !strings.Contains(out, "revoked") {
		t.Errorf("logout: %d %q %q", code, out, errs)
	}
	data, _ = os.ReadFile(p)
	if strings.Contains(string(data), "stopan_test") || !strings.Contains(string(data), url) {
		t.Errorf("after logout: %s", data)
	}
	if code, _, errs := runCLI(t, "wrong\n", "login", "--url", url, "--token-stdin"); code != 1 || !strings.Contains(errs, "unauthorized") {
		t.Errorf("wrong token: %d %q", code, errs)
	}
	if code, _, errs := runCLI(t, "x\n", "login", "--url", "http://desk.example.org", "--token-stdin"); code != 1 || !strings.Contains(errs, "use https") {
		t.Errorf("plain http: %d %q", code, errs)
	}
}

func TestNotifications(t *testing.T) {
	setup(t)
	code, out, _ := runCLI(t, "", "inbox")
	if code != 0 || !strings.Contains(out, "*  4") || !strings.Contains(out, "Ana replied on #1001") {
		t.Errorf("inbox: %d %q", code, out)
	}
	if code, _, _ := runCLI(t, "", "read", "--all"); code != 0 {
		t.Errorf("read --all: %d", code)
	}
}
