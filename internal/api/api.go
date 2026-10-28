// Package api is a client for the Stopan JSON API under /api/v1 (the
// server's specs/api.md). Every request carries the personal token as
// "Authorization: Bearer"; errors come back as *Error with the server's
// code and message.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxBody limits a JSON answer; a ticket with a long timeline is far less.
const maxBody = 32 << 20

// Client talks to one Stopan server as the token's user.
type Client struct {
	Base      string // https://host, without /api/v1
	Token     string
	HTTP      *http.Client
	UserAgent string
}

// New checks the server URL and returns a client for it.
func New(base, token, userAgent string) (*Client, error) {
	base, err := NormalizeURL(base)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("no token; run 'stopan login'")
	}
	return &Client{Base: base, Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}, UserAgent: userAgent}, nil
}

// NormalizeURL accepts "host", "https://host" or "https://host/", and
// returns "https://host". Plain http is refused except for the local
// machine, so a token never crosses a network in clear text.
func NormalizeURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("no server URL; run 'stopan login'")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%q is not a server URL", s)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%q: give only the scheme, host and path", s)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLocal(u.Hostname()) {
			return "", fmt.Errorf("%q: plain http is only allowed for localhost; use https", s)
		}
	default:
		return "", fmt.Errorf("%q: the scheme must be https", s)
	}
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/api/v1")
	return strings.TrimSuffix(u.String(), "/"), nil
}

func isLocal(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Error is an answer other than 2xx.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string][]string
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	var parts []string
	keys := make([]string, 0, len(e.Details))
	for k := range e.Details {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+": "+strings.Join(e.Details[k], ", "))
	}
	if len(parts) > 0 && !strings.Contains(msg, parts[0]) {
		msg += " (" + strings.Join(parts, "; ") + ")"
	}
	if e.Code == "" {
		return fmt.Sprintf("%d: %s", e.Status, msg)
	}
	return fmt.Sprintf("%s: %s", e.Code, msg)
}

// IsCode tells whether err is an API error with that code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

// Do sends one request. body, when not nil, is sent as JSON. The raw
// answer is returned for --json output and decoded into out when out is
// not nil.
func (c *Client) Do(method, path string, query url.Values, body, out any) ([]byte, error) {
	u := c.Base + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	hc := *c.HTTP
	// The token goes with every redirect Go follows to the same host; a
	// redirect elsewhere, or down to http, is refused.
	hc.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if r.URL.Host != via[0].URL.Host || (via[0].URL.Scheme == "https" && r.URL.Scheme != "https") {
			return fmt.Errorf("refusing redirect to %s", r.URL.Redacted())
		}
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return nil
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBody {
		return nil, fmt.Errorf("%s %s: answer larger than %d MiB", method, path, maxBody>>20)
	}
	if resp.StatusCode/100 != 2 {
		return raw, decodeError(resp, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, fmt.Errorf("%s %s: %w", method, path, err)
		}
	}
	return raw, nil
}

func decodeError(resp *http.Response, raw []byte) error {
	e := &Error{Status: resp.StatusCode}
	var wrap struct {
		Error struct {
			Code    string              `json:"code"`
			Message string              `json:"message"`
			Details map[string][]string `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &wrap) == nil {
		e.Code, e.Message, e.Details = wrap.Error.Code, wrap.Error.Message, wrap.Error.Details
	}
	if e.Code == "rate_limited" {
		if s := resp.Header.Get("Retry-After"); s != "" {
			e.Message += " (retry after " + s + "s)"
		}
	}
	return e
}

// Representations (the server's specs/api.md "Representations").

type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Email string `json:"email,omitempty"`
}

// Staff tells whether the user is an agent or an administrator.
func (u User) Staff() bool { return u.Role == "agent" || u.Role == "admin" }

type Ref struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Me struct {
	User
	Username         string `json:"username"`
	Language         string `json:"language"`
	Deactivated      bool   `json:"deactivated"`
	NotifyNewTickets bool   `json:"notify_new_tickets"`
	Organizations    []Ref  `json:"organizations"`
}

type File struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
	URL         string `json:"url"`
}

type Comment struct {
	ID        int64      `json:"id"`
	Body      string     `json:"body"`
	Internal  bool       `json:"internal"`
	Markdown  bool       `json:"markdown"`
	Source    string     `json:"source"`
	Author    *User      `json:"author"`
	CreatedAt time.Time  `json:"created_at"`
	EditedAt  *time.Time `json:"edited_at"`
	Files     []File     `json:"files"`
}

type Event struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	From      any       `json:"from"`
	To        any       `json:"to"`
	Internal  bool      `json:"internal"`
	Actor     *User     `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
	Comment   *Comment  `json:"comment"`
}

type Ticket struct {
	Number       int64      `json:"number"`
	Reference    string     `json:"reference"`
	Subject      string     `json:"subject"`
	Status       string     `json:"status"`
	Priority     string     `json:"priority"`
	Source       string     `json:"source"`
	Markdown     bool       `json:"markdown"`
	Stage        *Ref       `json:"stage"`
	Organization Ref        `json:"organization"`
	Requester    *User      `json:"requester"`
	Assignee     *User      `json:"assignee"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DoneAt       *time.Time `json:"done_at"`
	EditedAt     *time.Time `json:"edited_at"`
	URL          string     `json:"url"`

	// Only in Ticket (one ticket), not in the list.
	Description  string   `json:"description"`
	Files        []File   `json:"files"`
	Shared       bool     `json:"shared"`
	ShareURL     string   `json:"share_url"`
	NextStatuses []string `json:"next_statuses"`
	Timeline     []Event  `json:"timeline"`
}

type TicketList struct {
	Tickets    []Ticket `json:"tickets"`
	View       string   `json:"view"`
	Page       int      `json:"page"`
	TotalPages int      `json:"total_pages"`
	TotalCount int      `json:"total_count"`
}

type Notification struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	Actor     *User     `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
	Ticket    *struct {
		Number  int64  `json:"number"`
		Subject string `json:"subject"`
	} `json:"ticket"`
}

type NotificationList struct {
	Notifications []Notification `json:"notifications"`
	Page          int            `json:"page"`
	TotalCount    int            `json:"total_count"`
	UnreadCount   int            `json:"unread_count"`
}

type Token struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Hint       string     `json:"hint"`
	ReadOnly   bool       `json:"read_only"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// Values the server accepts (the server's specs/tickets.md, queues.md).
var (
	Statuses      = []string{"new", "open", "pending", "postponed", "done"}
	Priorities    = []string{"low", "normal", "high", "urgent"}
	StaffViews    = []string{"unsolved", "mine", "unassigned", "pending", "postponed", "done", "all"}
	RequesterView = []string{"open", "done", "all"}
	Sorts         = []string{"updated", "created", "priority"}
)

// Calls. Each returns the raw JSON too, for --json.

func (c *Client) Me() (Me, []byte, error) {
	var w struct {
		User Me `json:"user"`
	}
	raw, err := c.Do("GET", "/me", nil, nil, &w)
	return w.User, raw, err
}

// ListFilter is the query of GET /tickets.
type ListFilter struct {
	View, Status, Priority, Assignee, Organization, Query, Sort string
	Page                                                        int
}

func (f ListFilter) values() url.Values {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("view", f.View)
	set("status", f.Status)
	set("priority", f.Priority)
	set("assignee", f.Assignee)
	set("organization", f.Organization)
	set("q", f.Query)
	set("sort", f.Sort)
	if f.Page > 1 {
		q.Set("page", strconv.Itoa(f.Page))
	}
	return q
}

func (c *Client) Tickets(f ListFilter) (TicketList, []byte, error) {
	var l TicketList
	raw, err := c.Do("GET", "/tickets", f.values(), nil, &l)
	return l, raw, err
}

func ticketPath(n int64) string { return "/tickets/" + strconv.FormatInt(n, 10) }

func (c *Client) do1(method, path string, body any) (Ticket, []byte, error) {
	var w struct {
		Ticket Ticket `json:"ticket"`
	}
	raw, err := c.Do(method, path, nil, body, &w)
	return w.Ticket, raw, err
}

func (c *Client) Ticket(n int64) (Ticket, []byte, error) { return c.do1("GET", ticketPath(n), nil) }

// NewTicket is the body of POST /tickets; zero fields are left out.
type NewTicket struct {
	Subject        string `json:"subject"`
	Description    string `json:"description"`
	Markdown       bool   `json:"markdown,omitempty"`
	OrganizationID int64  `json:"organization_id,omitempty"`
	RequesterID    int64  `json:"requester_id,omitempty"`
	Priority       string `json:"priority,omitempty"`
	AssigneeID     int64  `json:"assignee_id,omitempty"`
}

func (c *Client) CreateTicket(t NewTicket) (Ticket, []byte, error) {
	return c.do1("POST", "/tickets", map[string]any{"ticket": t})
}

// Change is a staff change of one ticket. Assignee is "" (no change),
// "none" (unassign) or a user id.
type Change struct {
	Status, Priority, Assignee string
}

func (ch Change) Empty() bool { return ch == Change{} }

func (c *Client) UpdateTicket(n int64, ch Change) (Ticket, []byte, error) {
	t := map[string]any{}
	if ch.Status != "" {
		t["status"] = ch.Status
	}
	if ch.Priority != "" {
		t["priority"] = ch.Priority
	}
	switch ch.Assignee {
	case "":
	case "none":
		t["assignee_id"] = nil
	default:
		t["assignee_id"] = ch.Assignee
	}
	return c.do1("PATCH", ticketPath(n), map[string]any{"ticket": t})
}

// Resolve is the requester's "Mark as done".
func (c *Client) Resolve(n int64) (Ticket, []byte, error) {
	return c.do1("POST", ticketPath(n)+"/resolve", nil)
}

// NewComment is a reply, or an internal note for staff. Status (staff)
// changes the ticket's status with the reply.
type NewComment struct {
	Body     string `json:"body"`
	Internal bool   `json:"internal,omitempty"`
	Markdown bool   `json:"markdown,omitempty"`
	Status   string `json:"status,omitempty"`
}

func (c *Client) AddComment(n int64, cm NewComment) (Comment, []byte, error) {
	var w struct {
		Comment Comment `json:"comment"`
	}
	raw, err := c.Do("POST", ticketPath(n)+"/comments", nil, map[string]any{"comment": cm}, &w)
	return w.Comment, raw, err
}

func (c *Client) Notifications(page int) (NotificationList, []byte, error) {
	q := url.Values{}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	var l NotificationList
	raw, err := c.Do("GET", "/notifications", q, nil, &l)
	return l, raw, err
}

func (c *Client) ReadNotification(id int64) error {
	_, err := c.Do("POST", "/notifications/"+strconv.FormatInt(id, 10)+"/read", nil, nil, nil)
	return err
}

func (c *Client) ReadAllNotifications() error {
	_, err := c.Do("POST", "/notifications/read_all", nil, nil, nil)
	return err
}

func (c *Client) Tokens() ([]Token, []byte, error) {
	var w struct {
		Tokens []Token `json:"tokens"`
	}
	raw, err := c.Do("GET", "/tokens", nil, nil, &w)
	return w.Tokens, raw, err
}

func (c *Client) RevokeToken(id int64) error {
	_, err := c.Do("DELETE", "/tokens/"+strconv.FormatInt(id, 10), nil, nil, nil)
	return err
}
