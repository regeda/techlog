package service

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

const (
	defaultSearchLimit  = 20
	maxSearchLimit      = 100
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

// SearchParams selects events. Zero values mean "no constraint".
type SearchParams struct {
	Query  string
	System string
	Types  []string
	Since  time.Time
	Until  time.Time
	Limit  int
}

// ParseTime accepts an RFC 3339 timestamp or a YYYY-MM-DD date. A bare date is
// the start of that day in UTC, or its last millisecond when endOfDay is set.
func ParseTime(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not an RFC 3339 timestamp or a YYYY-MM-DD date", raw)
	}
	if endOfDay {
		d = d.Add(24*time.Hour - time.Millisecond)
	}
	return d, nil
}

// SearchParamsFromQuery reads q, system, type (repeatable or comma-separated),
// since, until and limit. Problems are *BadRequestError.
func SearchParamsFromQuery(v url.Values) (SearchParams, error) {
	p := SearchParams{Query: strings.TrimSpace(v.Get("q")), System: v.Get("system")}
	for _, raw := range v["type"] {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				p.Types = append(p.Types, t)
			}
		}
	}
	var err error
	if raw := v.Get("since"); raw != "" {
		if p.Since, err = ParseTime(raw, false); err != nil {
			return SearchParams{}, &BadRequestError{Msg: "invalid since: " + err.Error()}
		}
	}
	if raw := v.Get("until"); raw != "" {
		if p.Until, err = ParseTime(raw, true); err != nil {
			return SearchParams{}, &BadRequestError{Msg: "invalid until: " + err.Error()}
		}
	}
	if raw := v.Get("limit"); raw != "" {
		if p.Limit, err = strconv.Atoi(raw); err != nil {
			return SearchParams{}, &BadRequestError{Msg: fmt.Sprintf("invalid limit: %q is not an integer", raw)}
		}
	}
	return p, nil
}

func clamp(n, def, max int) int {
	switch {
	case n <= 0:
		return def
	case n > max:
		return max
	}
	return n
}

// Search returns events newest first. The result is never nil.
func (s *Service) Search(ctx context.Context, p SearchParams) ([]event.Event, error) {
	events, err := s.store.Search(ctx, store.Filter{
		Text: p.Query, System: p.System, Types: p.Types, Since: p.Since, Until: p.Until,
		Limit: clamp(p.Limit, defaultSearchLimit, maxSearchLimit),
	})
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []event.Event{}
	}
	return events, nil
}

// History returns the latest events of one system, oldest first. An unknown
// system has an empty history; it is not an error.
func (s *Service) History(ctx context.Context, system string, since time.Time, limit int) ([]event.Event, error) {
	if system == "" {
		return nil, &BadRequestError{Msg: "system is required"}
	}
	events, err := s.store.Search(ctx, store.Filter{
		System: system, Since: since, Limit: clamp(limit, defaultHistoryLimit, maxHistoryLimit),
	})
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []event.Event{}
	}
	slices.Reverse(events)
	return events, nil
}

func (s *Service) Systems(ctx context.Context) ([]string, error) {
	systems, err := s.store.Systems(ctx)
	if err != nil {
		return nil, err
	}
	if systems == nil {
		systems = []string{}
	}
	return systems, nil
}
