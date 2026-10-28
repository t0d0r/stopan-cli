package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	ok := map[string]string{
		"stopan.example.org":                "https://stopan.example.org",
		"https://stopan.example.org/":       "https://stopan.example.org",
		"https://stopan.example.org/api/v1": "https://stopan.example.org",
		"https://example.org/desk/":         "https://example.org/desk",
		"http://localhost:3000":             "http://localhost:3000",
		"http://127.0.0.1:3000/":            "http://127.0.0.1:3000",
	}
	for in, want := range ok {
		got, err := NormalizeURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "http://stopan.example.org", "ftp://x", "https://u:p@x.org", "https://x.org/?a=1"} {
		if got, err := NormalizeURL(in); err == nil {
			t.Errorf("NormalizeURL(%q) = %q, want an error", in, got)
		}
	}
}

func server(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	c, err := New(ts.URL, "stopan_secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBearerAndDecode(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer stopan_secret" || r.URL.Path != "/api/v1/me" {
			http.Error(w, "bad", 400)
			return
		}
		if r.URL.RawQuery != "" {
			t.Errorf("token or other data in the query: %q", r.URL.RawQuery)
		}
		io.WriteString(w, `{"user":{"id":7,"name":"Ana","role":"agent","organizations":[{"id":1,"name":"Acme"}]}}`)
	})
	me, raw, err := c.Me()
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != 7 || !me.Staff() || len(me.Organizations) != 1 || !strings.Contains(string(raw), `"Ana"`) {
		t.Errorf("me = %+v", me)
	}
}

func TestErrors(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/tickets/1":
			w.WriteHeader(401)
			io.WriteString(w, `{"error":{"code":"unauthorized","message":"Invalid token"}}`)
		case "/api/v1/tickets/2":
			w.WriteHeader(422)
			io.WriteString(w, `{"error":{"code":"invalid","message":"Status is not allowed","details":{"status":["is not allowed"]}}}`)
		case "/api/v1/tickets/3":
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(429)
			io.WriteString(w, `{"error":{"code":"rate_limited","message":"Too many requests"}}`)
		default:
			w.WriteHeader(502)
			io.WriteString(w, "<html>bad gateway</html>")
		}
	})
	_, _, err := c.Ticket(1)
	if !IsCode(err, "unauthorized") || err.Error() != "unauthorized: Invalid token" {
		t.Errorf("401: %v", err)
	}
	_, _, err = c.Ticket(2)
	if !IsCode(err, "invalid") || !strings.Contains(err.Error(), "status: is not allowed") {
		t.Errorf("422: %v", err)
	}
	_, _, err = c.Ticket(3)
	if !strings.Contains(err.Error(), "retry after 60s") {
		t.Errorf("429: %v", err)
	}
	_, _, err = c.Ticket(4)
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Errorf("502: %v", err)
	}
}

func TestUpdateTicketBody(t *testing.T) {
	var got map[string]map[string]any
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s %s", r.Method, r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		io.WriteString(w, `{"ticket":{"number":5,"status":"pending"}}`)
	})
	if _, _, err := c.UpdateTicket(5, Change{Status: "pending", Assignee: "none"}); err != nil {
		t.Fatal(err)
	}
	tk := got["ticket"]
	if v, ok := tk["assignee_id"]; !ok || v != nil {
		t.Errorf("assignee none should send null: %v", tk)
	}
	if tk["status"] != "pending" || tk["priority"] != nil {
		t.Errorf("body = %v", tk)
	}
}

func TestListQuery(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("view") != "mine" || q.Get("q") != "printer jam" || q.Get("page") != "2" || q.Has("status") {
			t.Errorf("query = %v", q)
		}
		io.WriteString(w, `{"tickets":[{"number":1001,"subject":"Printer"}],"view":"mine","page":2,"total_pages":2,"total_count":26}`)
	})
	l, _, err := c.Tickets(ListFilter{View: "mine", Query: "printer jam", Page: 2})
	if err != nil || len(l.Tickets) != 1 || l.TotalCount != 26 {
		t.Fatalf("list = %+v, %v", l, err)
	}
}

func TestRedirectElsewhereRefused(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("token followed a redirect to another host: %s", r.Header.Get("Authorization"))
	}))
	defer other.Close()
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+"/x", http.StatusFound)
	})
	if _, _, err := c.Me(); err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Errorf("err = %v", err)
	}
}
