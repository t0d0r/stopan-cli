// Package tui is the full-screen dashboard: the ticket queue with its
// views, one ticket with its timeline, and the notifications.
package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"stopan-cli/internal/api"
	"stopan-cli/internal/text"
)

// Options are the dashboard's settings from the command line.
type Options struct {
	Refresh time.Duration // 0: no automatic reload
}

// Run starts the dashboard and returns when it quits.
func Run(c *api.Client, o Options) error {
	m := newModel(c, o)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type screen int

const (
	listScreen screen = iota
	ticketScreen
	notifScreen
)

// mode is what keys go to: the screen, the search line, a picker, or a
// yes/no question.
type mode int

const (
	normal mode = iota
	searching
	picking
	confirming
	helping
)

type model struct {
	c    *api.Client
	opts Options
	now  func() time.Time

	width, height int
	screen        screen
	mode          mode

	me      *api.Me
	views   []string
	view    int
	sort    string
	query   string
	page    int
	list    api.TicketList
	cursor  int
	loading bool

	ticket *api.Ticket
	vp     viewport.Model

	notifs      api.NotificationList
	notifCursor int
	unread      int

	search textinput.Model

	// picker: one value of a field for the open ticket.
	pickField  string
	pickValues []string
	pickIndex  int

	// composing is "reply" or "note" while the editor is open.
	composing string

	// confirm: a question and what happens on "y".
	question string
	onYes    tea.Cmd

	status  string // last message in the footer
	isError bool
}

func newModel(c *api.Client, o Options) model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "subject, text or #number"
	ti.CharLimit = 200
	return model{c: c, opts: o, now: time.Now, sort: "updated", page: 1, search: ti, vp: viewport.New(0, 0), loading: true}
}

// Messages from the API calls.

type meMsg struct {
	me  api.Me
	err error
}
type listMsg struct {
	list api.TicketList
	err  error
}
type ticketMsg struct {
	t   api.Ticket
	err error
}
type notifMsg struct {
	list api.NotificationList
	err  error
}
type doneMsg struct {
	info   string
	err    error
	ticket int64 // reload this ticket afterwards, if not 0
}
type tickMsg time.Time

func (m model) Init() tea.Cmd {
	return tea.Batch(m.loadMe(), m.tick())
}

func (m model) tick() tea.Cmd {
	if m.opts.Refresh <= 0 {
		return nil
	}
	return tea.Tick(m.opts.Refresh, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) loadMe() tea.Cmd {
	c := m.c
	return func() tea.Msg {
		me, _, err := c.Me()
		return meMsg{me, err}
	}
}

func (m model) filter() api.ListFilter {
	f := api.ListFilter{Sort: m.sort, Page: m.page, Query: m.query}
	if m.view < len(m.views) {
		f.View = m.views[m.view]
	}
	return f
}

func (m model) loadList() tea.Cmd {
	c, f := m.c, m.filter()
	return func() tea.Msg {
		l, _, err := c.Tickets(f)
		return listMsg{l, err}
	}
}

func (m model) loadTicket(n int64) tea.Cmd {
	c := m.c
	return func() tea.Msg {
		t, _, err := c.Ticket(n)
		return ticketMsg{t, err}
	}
}

func (m model) loadNotifs() tea.Cmd {
	c := m.c
	return func() tea.Msg {
		l, _, err := c.Notifications(1)
		return notifMsg{l, err}
	}
}

// act runs a change and reports it; the ticket is reloaded afterwards.
func (m model) act(n int64, info string, fn func() error) tea.Cmd {
	return func() tea.Msg {
		return doneMsg{info: info, err: fn(), ticket: n}
	}
}

func (m *model) setStatus(s string, isErr bool) {
	m.status, m.isError = s, isErr
}

// clearError drops an error message once a later load worked.
func (m *model) clearError() {
	if m.isError {
		m.status, m.isError = "", false
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeViewport()
		return m, nil

	case meMsg:
		if msg.err != nil {
			m.loading = false
			m.setStatus("r retries; cannot reach the server: "+msg.err.Error(), true)
			return m, nil
		}
		m.me = &msg.me
		m.clearError()
		m.views = api.RequesterView
		if msg.me.Staff() {
			m.views = api.StaffViews
		}
		return m, tea.Batch(m.loadList(), m.loadNotifs())

	case listMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
			return m, nil
		}
		m.list = msg.list
		m.clearError()
		m.cursor = min(m.cursor, max(len(m.list.Tickets)-1, 0))
		return m, nil

	case ticketMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
			return m, nil
		}
		keep := m.ticket != nil && m.ticket.Number == msg.t.Number
		m.ticket = &msg.t
		m.screen = ticketScreen
		m.renderTicket(keep)
		return m, nil

	case notifMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
			return m, nil
		}
		m.notifs = msg.list
		m.unread = msg.list.UnreadCount
		m.notifCursor = min(m.notifCursor, max(len(m.notifs.Notifications)-1, 0))
		return m, nil

	case doneMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus(msg.info, false)
		}
		cmds := []tea.Cmd{m.loadList(), m.loadNotifs()}
		if msg.ticket != 0 && m.screen == ticketScreen {
			cmds = append(cmds, m.loadTicket(msg.ticket))
		}
		return m, tea.Batch(cmds...)

	case editedMsg:
		return m.edited(msg)

	case tickMsg:
		cmds := []tea.Cmd{m.tick()}
		if m.me != nil && m.mode == normal {
			cmds = append(cmds, m.loadNotifs())
			if m.screen == listScreen {
				cmds = append(cmds, m.loadList())
			}
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case searching:
		return m.searchKey(k)
	case picking:
		return m.pickKey(k)
	case confirming:
		return m.confirmKey(k)
	case helping:
		m.mode = normal
		return m, nil
	}
	if k.String() == "?" {
		m.mode = helping
		return m, nil
	}
	switch m.screen {
	case ticketScreen:
		return m.ticketKey(k)
	case notifScreen:
		return m.notifKey(k)
	}
	return m.listKey(k)
}

func (m model) listKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.list.Tickets)
	switch s := k.String(); s {
	case "q":
		return m, tea.Quit
	case "j", "down":
		m.cursor = min(m.cursor+1, max(n-1, 0))
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = max(n-1, 0)
	case "tab", "l", "right":
		return m.switchView(m.view + 1)
	case "shift+tab", "h", "left":
		return m.switchView(m.view - 1 + len(m.views))
	case "1", "2", "3", "4", "5", "6", "7":
		i, _ := strconv.Atoi(s)
		if i <= len(m.views) {
			return m.switchView(i - 1)
		}
	case "]", "pgdown":
		if m.page < m.list.TotalPages {
			m.page++
			m.cursor = 0
			return m, m.loadList()
		}
	case "[", "pgup":
		if m.page > 1 {
			m.page--
			m.cursor = 0
			return m, m.loadList()
		}
	case "s":
		i := slices.Index(api.Sorts, m.sort)
		m.sort = api.Sorts[(i+1)%len(api.Sorts)]
		m.page, m.cursor = 1, 0
		m.setStatus("sorted by "+m.sort, false)
		return m, m.loadList()
	case "/":
		m.mode = searching
		m.search.SetValue(m.query)
		m.search.CursorEnd()
		return m, m.search.Focus()
	case "esc":
		if m.query != "" {
			m.query, m.page, m.cursor = "", 1, 0
			m.setStatus("search cleared", false)
			return m, m.loadList()
		}
	case "r":
		if m.me == nil {
			return m, m.loadMe()
		}
		m.setStatus("reloaded", false)
		return m, tea.Batch(m.loadList(), m.loadNotifs())
	case "n":
		m.screen = notifScreen
		return m, m.loadNotifs()
	case "enter":
		if t, ok := m.selected(); ok {
			m.loading = true
			return m, m.loadTicket(t.Number)
		}
	case "o":
		if t, ok := m.selected(); ok {
			return m, browse(t.URL)
		}
	}
	return m, nil
}

func (m model) selected() (api.Ticket, bool) {
	if m.cursor < len(m.list.Tickets) {
		return m.list.Tickets[m.cursor], true
	}
	return api.Ticket{}, false
}

func (m model) switchView(i int) (tea.Model, tea.Cmd) {
	if len(m.views) == 0 {
		return m, nil
	}
	m.view = i % len(m.views)
	m.page, m.cursor = 1, 0
	return m, m.loadList()
}

func (m model) searchKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "enter":
		m.mode = normal
		m.search.Blur()
		m.query = strings.TrimSpace(m.search.Value())
		m.page, m.cursor = 1, 0
		return m, m.loadList()
	case "esc":
		m.mode = normal
		m.search.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(k)
	return m, cmd
}

func (m model) ticketKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	t := m.ticket
	staff := m.me != nil && m.me.Staff()
	switch k.String() {
	case "q", "esc", "backspace":
		m.screen = listScreen
		return m, m.loadList()
	case "r":
		return m, m.loadTicket(t.Number)
	case "R":
		m.composing = "reply"
		return m, editInTUI(fmt.Sprintf("Reply to #%d %s\nThe requester sees this reply and gets it by email.", t.Number, t.Subject))
	case "N":
		if !staff {
			m.setStatus("internal notes are for staff", true)
			return m, nil
		}
		m.composing = "note"
		return m, editInTUI(fmt.Sprintf("Internal note on #%d %s\nOnly staff see this note.", t.Number, t.Subject))
	case "S":
		if !staff {
			m.setStatus("only staff change the status; D marks it done", true)
			return m, nil
		}
		if len(t.NextStatuses) == 0 {
			m.setStatus("no status change is allowed from "+t.Status, true)
			return m, nil
		}
		return m.pick("status", t.NextStatuses), nil
	case "P":
		if !staff {
			m.setStatus("only staff change the priority", true)
			return m, nil
		}
		return m.pick("priority", api.Priorities), nil
	case "A":
		if !staff {
			return m, nil
		}
		me := strconv.FormatInt(m.me.ID, 10)
		return m.ask(fmt.Sprintf("Assign #%d to yourself?", t.Number),
			m.act(t.Number, fmt.Sprintf("#%d assigned to you", t.Number), func() error {
				_, _, err := m.c.UpdateTicket(t.Number, api.Change{Assignee: me})
				return err
			})), nil
	case "U":
		if !staff {
			return m, nil
		}
		return m.ask(fmt.Sprintf("Unassign #%d?", t.Number),
			m.act(t.Number, fmt.Sprintf("#%d unassigned", t.Number), func() error {
				_, _, err := m.c.UpdateTicket(t.Number, api.Change{Assignee: "none"})
				return err
			})), nil
	case "D":
		return m.ask(fmt.Sprintf("Mark #%d as done?", t.Number),
			m.act(t.Number, fmt.Sprintf("#%d done", t.Number), func() error {
				var err error
				if staff {
					_, _, err = m.c.UpdateTicket(t.Number, api.Change{Status: "done"})
				} else {
					_, _, err = m.c.Resolve(t.Number)
				}
				return err
			})), nil
	case "o":
		return m, browse(t.URL)
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(k)
	return m, cmd
}

// edited asks before sending the editor's text as a reply or a note.
func (m model) edited(msg editedMsg) (tea.Model, tea.Cmd) {
	kind := m.composing
	m.composing = ""
	if msg.err != nil {
		m.setStatus(msg.err.Error(), true)
		return m, nil
	}
	if msg.text == "" || m.ticket == nil || kind == "" {
		m.setStatus("empty text; nothing sent", false)
		return m, nil
	}
	n, body := m.ticket.Number, msg.text
	q := fmt.Sprintf("Send %s to #%d (%s)?", kind, n, text.Cut(body, max(m.width/3, 20)))
	return m.ask(q, m.act(n, fmt.Sprintf("%s added to #%d", kind, n), func() error {
		_, _, err := m.c.AddComment(n, api.NewComment{Body: body, Internal: kind == "note"})
		return err
	})), nil
}

func (m model) pick(field string, values []string) model {
	m.mode = picking
	m.pickField, m.pickValues = field, values
	m.pickIndex = 0
	if m.ticket != nil {
		cur := m.ticket.Status
		if field == "priority" {
			cur = m.ticket.Priority
		}
		if i := slices.Index(values, cur); i >= 0 {
			m.pickIndex = i
		}
	}
	return m
}

func (m model) pickKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "q":
		m.mode = normal
	case "left", "h", "shift+tab", "up", "k":
		m.pickIndex = (m.pickIndex - 1 + len(m.pickValues)) % len(m.pickValues)
	case "right", "l", "tab", "down", "j":
		m.pickIndex = (m.pickIndex + 1) % len(m.pickValues)
	case "enter":
		m.mode = normal
		t, v, field := m.ticket, m.pickValues[m.pickIndex], m.pickField
		ch := api.Change{Status: v}
		if field == "priority" {
			ch = api.Change{Priority: v}
		}
		return m, m.act(t.Number, fmt.Sprintf("#%d %s %s", t.Number, field, v), func() error {
			_, _, err := m.c.UpdateTicket(t.Number, ch)
			return err
		})
	}
	return m, nil
}

func (m model) ask(q string, yes tea.Cmd) model {
	m.mode, m.question, m.onYes = confirming, q, yes
	return m
}

func (m model) confirmKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = normal
	if k.String() == "y" || k.String() == "Y" {
		cmd := m.onYes
		m.onYes = nil
		return m, cmd
	}
	m.onYes = nil
	m.setStatus("cancelled", false)
	return m, nil
}

func (m model) notifKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	ns := m.notifs.Notifications
	switch k.String() {
	case "q", "esc", "n":
		m.screen = listScreen
		return m, nil
	case "j", "down":
		m.notifCursor = min(m.notifCursor+1, max(len(ns)-1, 0))
	case "k", "up":
		m.notifCursor = max(m.notifCursor-1, 0)
	case "r":
		return m, m.loadNotifs()
	case "a":
		return m.ask("Mark all notifications as read?", m.act(0, "all notifications read", m.c.ReadAllNotifications)), nil
	case "enter":
		if m.notifCursor >= len(ns) {
			return m, nil
		}
		n := ns[m.notifCursor]
		var cmds []tea.Cmd
		if !n.Read {
			id := n.ID
			cmds = append(cmds, m.act(0, "", func() error { return m.c.ReadNotification(id) }))
		}
		if n.Ticket != nil {
			m.loading = true
			cmds = append(cmds, m.loadTicket(n.Ticket.Number))
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func browse(url string) tea.Cmd {
	return func() tea.Msg {
		if url == "" {
			return doneMsg{err: fmt.Errorf("the server sent no page URL")}
		}
		name := "xdg-open"
		if runtime.GOOS == "darwin" {
			name = "open"
		}
		if err := exec.Command(name, url).Run(); err != nil {
			return doneMsg{err: err}
		}
		return doneMsg{info: "opened " + url}
	}
}

// Layout.

var (
	accent   = lipgloss.AdaptiveColor{Light: "#7a4d00", Dark: "#ffcf5c"}
	faint    = lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#8a8a8a"}
	danger   = lipgloss.AdaptiveColor{Light: "#b00020", Dark: "#ff6b6b"}
	titleSt  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	tabSt    = lipgloss.NewStyle().Padding(0, 1).Foreground(faint)
	tabOnSt  = lipgloss.NewStyle().Padding(0, 1).Bold(true).Reverse(true)
	selSt    = lipgloss.NewStyle().Reverse(true)
	faintSt  = lipgloss.NewStyle().Foreground(faint)
	errSt    = lipgloss.NewStyle().Foreground(danger)
	unreadSt = lipgloss.NewStyle().Bold(true).Foreground(accent)
	prioSt   = map[string]lipgloss.Style{
		"urgent": lipgloss.NewStyle().Bold(true).Foreground(danger),
		"high":   lipgloss.NewStyle().Foreground(danger),
		"low":    faintSt,
	}
)

func (m *model) resizeViewport() {
	m.vp.Width = m.width
	m.vp.Height = max(m.height-3, 1)
	if m.ticket != nil {
		m.renderTicket(true)
	}
}

func (m *model) renderTicket(keepOffset bool) {
	off := m.vp.YOffset
	m.vp.SetContent(text.Ticket(*m.ticket, m.width-1, m.now()))
	if keepOffset {
		m.vp.SetYOffset(off)
	} else {
		m.vp.GotoTop()
	}
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	body := m.listView()
	switch {
	case m.mode == helping:
		body = helpText
	case m.screen == ticketScreen && m.ticket != nil:
		body = m.vp.View()
	case m.screen == notifScreen:
		body = m.notifView()
	}
	body = lipgloss.NewStyle().Height(m.height - 2).MaxHeight(m.height - 2).Render(body)
	return lipgloss.JoinVertical(lipgloss.Left, m.header(), body, m.footer())
}

func (m model) header() string {
	left := titleSt.Render("stopan")
	if m.me != nil {
		left += faintSt.Render(" " + m.me.Name + " @ " + m.c.Base[strings.Index(m.c.Base, "://")+3:])
	}
	var mid string
	switch m.screen {
	case listScreen:
		for i, v := range m.views {
			st := tabSt
			if i == m.view {
				st = tabOnSt
			}
			mid += st.Render(fmt.Sprintf("%d %s", i+1, v))
		}
	case ticketScreen:
		if m.ticket != nil {
			mid = " " + titleSt.Render(fmt.Sprintf("#%d", m.ticket.Number)) + " " + text.Cut(m.ticket.Subject, max(m.width/2, 10))
		}
	case notifScreen:
		mid = " " + titleSt.Render("notifications")
	}
	right := faintSt.Render("inbox 0")
	if m.unread > 0 {
		right = unreadSt.Render(fmt.Sprintf("inbox %d", m.unread))
	}
	// The inbox count always shows: the server goes first, then the
	// end of the tabs.
	room := m.width - lipgloss.Width(right) - 1
	line := left + "  " + mid
	if lipgloss.Width(line) > room {
		line = titleSt.Render("stopan") + "  " + mid
	}
	if lipgloss.Width(line) > room {
		line = ansi.Truncate(line, max(room, 0), "")
	}
	return line + strings.Repeat(" ", max(m.width-lipgloss.Width(line)-lipgloss.Width(right), 1)) + right
}

func (m model) listView() string {
	if m.loading && len(m.list.Tickets) == 0 {
		return faintSt.Render("  loading...")
	}
	if len(m.list.Tickets) == 0 {
		return faintSt.Render("  no tickets in this view")
	}
	now := m.now()
	staff := m.me != nil && m.me.Staff()
	var rows []string
	start := 0
	visible := max(m.height-3, 1)
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	for i := start; i < len(m.list.Tickets) && i < start+visible; i++ {
		t := m.list.Tickets[i]
		who := userName(t.Requester)
		if staff {
			who = userName(t.Assignee)
		}
		fixed := fmt.Sprintf(" %-7s %-9s %-6s %4s %-14s ", fmt.Sprintf("#%d", t.Number), t.Status, t.Priority,
			text.Ago(t.UpdatedAt, now), text.Cut(who, 14))
		line := fixed + text.Cut(t.Subject, max(m.width-lipgloss.Width(fixed)-1, 5))
		switch {
		case i == m.cursor:
			line = selSt.Render(padRight(line, m.width))
		case prioSt[t.Priority].String() != "":
			st := prioSt[t.Priority]
			line = st.Render(line)
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func (m model) notifView() string {
	ns := m.notifs.Notifications
	if len(ns) == 0 {
		return faintSt.Render("  no notifications")
	}
	now := m.now()
	var rows []string
	visible := max(m.height-3, 1)
	start := 0
	if m.notifCursor >= visible {
		start = m.notifCursor - visible + 1
	}
	for i := start; i < len(ns) && i < start+visible; i++ {
		n := ns[i]
		mark := " "
		if !n.Read {
			mark = "*"
		}
		fixed := fmt.Sprintf(" %s %4s  ", mark, text.Ago(n.CreatedAt, now))
		line := fixed + text.Cut(text.Notification(n), max(m.width-lipgloss.Width(fixed)-1, 5))
		switch {
		case i == m.notifCursor:
			line = selSt.Render(padRight(line, m.width))
		case !n.Read:
			line = unreadSt.Render(line)
		}
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func (m model) footer() string {
	var s string
	switch m.mode {
	case searching:
		return m.search.View()
	case picking:
		parts := []string{m.pickField + ":"}
		for i, v := range m.pickValues {
			if i == m.pickIndex {
				parts = append(parts, selSt.Render(" "+v+" "))
			} else {
				parts = append(parts, " "+v+" ")
			}
		}
		return strings.Join(parts, " ") + faintSt.Render("   enter set, esc cancel")
	case confirming:
		return titleSt.Render(m.question) + " [y/N]"
	}
	switch {
	case m.status != "" && m.isError:
		s = errSt.Render(m.status)
	case m.status != "":
		s = m.status
	case m.screen == listScreen:
		s = fmt.Sprintf("page %d/%d, %d tickets, sort %s", max(m.list.Page, 1), max(m.list.TotalPages, 1), m.list.TotalCount, m.sort)
		if m.query != "" {
			s += ", search " + strconv.Quote(m.query)
		}
		s = faintSt.Render(s)
	case m.screen == ticketScreen:
		s = faintSt.Render(fmt.Sprintf("%3.f%%  R reply  N note  S status  P priority  A assign me  D done  o browser  esc back", m.vp.ScrollPercent()*100))
	case m.screen == notifScreen:
		s = faintSt.Render("enter open  a read all  r reload  esc back")
	}
	s = text.Cut(s, max(m.width-8, 1))
	return s + strings.Repeat(" ", max(m.width-lipgloss.Width(s)-6, 1)) + faintSt.Render("? help")
}

func padRight(s string, w int) string {
	if n := lipgloss.Width(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func userName(u *api.User) string {
	if u == nil {
		return "-"
	}
	return u.Name
}

const helpText = `
  List                          Ticket
  tab / shift+tab   next view   R        reply (in $EDITOR)
  1-7               view        N        internal note (staff)
  j k / arrows      move        S        status (staff)
  enter             open        P        priority (staff)
  /                 search      A / U    assign to me / unassign
  esc               clear       D        mark as done
  s                 sort        j k      scroll
  [ ]               pages       r        reload
  n                 inbox       o        open in the browser
  r                 reload      esc      back to the list
  o                 browser
  q                 quit        Inbox: enter open, a read all

  Any key closes this help.`
