// Package ui is Techlog's read-only web UI: server-rendered pages with
// embedded templates and CSS, no JavaScript and no build step.
package ui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const (
	eventsPageSize   = 50 // events per page on the list
	historyPageLimit = 200
)

var funcs = template.FuncMap{
	"fmtTime": func(s string) string {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return s
		}
		return t.UTC().Format("2006-01-02 15:04")
	},
	// typeClass maps "incident.opened" to "incident", the CSS colour family.
	"typeClass": func(t string) string {
		family, _, _ := strings.Cut(t, ".")
		return family
	},
	"pathEscape": url.PathEscape,
	"toJSON": func(v any) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err.Error()
		}
		return string(b)
	},
}

type server struct {
	svc  *service.Service
	tmpl *template.Template
}

// Handler serves the UI. Mount it at "/".
func Handler(svc *service.Service) http.Handler {
	s := &server{
		svc:  svc,
		tmpl: template.Must(template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.events)
	mux.HandleFunc("GET /events/{id}", s.event)
	mux.HandleFunc("GET /systems/{system}", s.system)
	mux.HandleFunc("GET /story/{id}", s.story)
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	return mux
}

type eventsPage struct {
	Title                         string
	Q, System, Type, Since, Until string
	Systems, Types                []string
	Events                        []event.Event
	Error                         string

	// Paging. From and To number the events shown (1-based); the URLs keep
	// every filter and change only the page.
	Page, From, To   int
	HasPrev, HasNext bool
	PrevURL, NextURL string
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := eventsPage{
		Title: "Events", Q: q.Get("q"), System: q.Get("system"), Type: q.Get("type"),
		Since: q.Get("since"), Until: q.Get("until"),
		Types: event.Types, Events: []event.Event{},
	}
	systems, err := s.svc.Systems(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page.Systems = systems

	status := http.StatusOK
	pageNo, pageErr := parsePage(q.Get("page"))
	p, err := service.SearchParamsFromQuery(q)
	if err == nil {
		err = pageErr
	}
	var bad *service.BadRequestError
	switch {
	case errors.As(err, &bad):
		page.Error = bad.Msg
		status = http.StatusBadRequest
	case err != nil:
		s.fail(w, r, err)
		return
	default:
		// Ask for one event more than a page holds: it says whether a next page exists.
		p.Limit = eventsPageSize + 1
		p.Offset = (pageNo - 1) * eventsPageSize
		events, err := s.svc.Search(r.Context(), p)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if page.HasNext = len(events) > eventsPageSize; page.HasNext {
			events = events[:eventsPageSize]
		}
		page.Events = events
		page.Page, page.From, page.To = pageNo, p.Offset+1, p.Offset+len(events)
		page.HasPrev = pageNo > 1
		page.PrevURL, page.NextURL = pageURL(q, pageNo-1), pageURL(q, pageNo+1)
	}
	s.render(w, status, "events.html", page)
}

// parsePage reads the 1-based page number; an absent value is page 1.
func parsePage(raw string) (int, error) {
	if raw == "" {
		return 1, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, &service.BadRequestError{Msg: fmt.Sprintf("invalid page: %q is not a positive integer", raw)}
	}
	return n, nil
}

// pageURL is the list URL for page n with the current filters kept.
func pageURL(q url.Values, n int) string {
	v := url.Values{}
	for k, vs := range q {
		v[k] = vs
	}
	v.Set("page", strconv.Itoa(n))
	return "/?" + v.Encode()
}

// linkRow is one link in the event page, with the other event's type and summary.
type linkRow struct {
	ID, Rel, Basis, Rationale, Type, Summary string
}

type eventPage struct {
	Title              string
	Event              event.Event
	Outgoing, Incoming []linkRow
}

func (s *server) event(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "event.html", eventPage{
		Title:    d.Event.ID,
		Event:    d.Event,
		Outgoing: s.rows(r.Context(), d.Outgoing, false),
		Incoming: s.rows(r.Context(), d.Incoming, true),
	})
}

// rows describes the event at the far end of each link. A lookup failure only
// leaves the type and summary blank.
func (s *server) rows(ctx context.Context, links []event.Link, incoming bool) []linkRow {
	rows := make([]linkRow, 0, len(links))
	for _, l := range links {
		id := l.Target
		if incoming {
			id = l.Source
		}
		row := linkRow{ID: id, Rel: l.Rel, Basis: l.Basis, Rationale: l.Rationale}
		if d, err := s.svc.Get(ctx, id); err == nil {
			row.Type, row.Summary = d.Event.Type, d.Event.Summary
		}
		rows = append(rows, row)
	}
	return rows
}

type systemPage struct {
	Title, System string
	Events        []event.Event
}

func (s *server) system(w http.ResponseWriter, r *http.Request) {
	system := r.PathValue("system")
	events, err := s.svc.History(r.Context(), system, time.Time{}, historyPageLimit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "system.html", systemPage{Title: system, System: system, Events: events})
}

type storyItem struct {
	Event event.Event
	Edges []event.Link // links that leave this event, within the story
	Root  bool
}

type storyPage struct {
	Title, Root string
	Items       []storyItem
}

func (s *server) story(w http.ResponseWriter, r *http.Request) {
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth")) // invalid or missing: the service default
	st, err := s.svc.Story(r.Context(), r.PathValue("id"), depth)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := map[string][]event.Link{}
	for _, l := range st.Edges {
		out[l.Source] = append(out[l.Source], l)
	}
	page := storyPage{Title: "Story " + st.Root, Root: st.Root, Items: make([]storyItem, len(st.Events))}
	for i, ev := range st.Events {
		page.Items[i] = storyItem{Event: ev, Edges: out[ev.ID], Root: ev.ID == st.Root}
	}
	s.render(w, http.StatusOK, "story.html", page)
}

// render executes into a buffer first, so a template error cannot leave a
// half-written page.
func (s *server) render(w http.ResponseWriter, status int, name string, data any) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("ui: render", "template", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		slog.Error("ui: write response", "err", err)
	}
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	slog.Error("ui: internal error", "path", r.URL.Path, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
