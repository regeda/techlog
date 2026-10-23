package service

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/regeda/techlog/internal/event"
)

const (
	defaultStoryDepth = 10
	maxStoryDepth     = 50
	maxStoryNodes     = 200
)

// Story is the set of events connected to a root event through links, in
// either direction, with the links between them.
type Story struct {
	Root   string        `json:"root"`
	Events []event.Event `json:"events"` // oldest first
	Edges  []event.Link  `json:"edges"`
}

// Story walks links breadth-first from id, at most depth hops (default 10, max
// 50) and at most 200 events. Cycles are safe: each event is visited once.
func (s *Service) Story(ctx context.Context, id string, depth int) (Story, error) {
	if depth <= 0 {
		depth = defaultStoryDepth
	}
	depth = min(depth, maxStoryDepth)

	root, err := s.store.Get(ctx, id)
	if err != nil {
		return Story{}, err
	}
	seen, edges, err := s.walk(ctx, id, root, depth)
	if err != nil {
		return Story{}, err
	}
	return buildStory(id, seen, edges), nil
}

// walk visits the events reachable from the root, up to depth hops. It returns
// the events seen and every link met on the way.
func (s *Service) walk(ctx context.Context, id string, root event.Event, depth int) (map[string]event.Event, map[event.Link]struct{}, error) {
	seen := map[string]event.Event{id: root}
	edges := map[event.Link]struct{}{}
	frontier := []string{id}
	for hop := 0; hop < depth && len(frontier) > 0; hop++ {
		var next []string
		for _, cur := range frontier {
			found, err := s.expand(ctx, cur, seen, edges)
			if err != nil {
				return nil, nil, err
			}
			next = append(next, found...)
		}
		frontier = next
	}
	return seen, edges, nil
}

// expand records the links of cur and loads the events they lead to that are
// not seen yet. It returns the ids it added to seen.
func (s *Service) expand(ctx context.Context, cur string, seen map[string]event.Event, edges map[event.Link]struct{}) ([]string, error) {
	links, err := s.store.Links(ctx, cur)
	if err != nil {
		return nil, err
	}
	var added []string
	for _, l := range links {
		edges[l] = struct{}{}
		other := l.Target
		if other == cur {
			other = l.Source
		}
		if _, ok := seen[other]; ok || len(seen) >= maxStoryNodes {
			continue
		}
		ev, err := s.store.Get(ctx, other)
		if err != nil {
			return nil, err
		}
		seen[other] = ev
		added = append(added, other)
	}
	return added, nil
}

// buildStory orders the events oldest first and keeps the links whose two
// ends were both seen.
func buildStory(root string, seen map[string]event.Event, edges map[event.Link]struct{}) Story {
	st := Story{Root: root, Events: make([]event.Event, 0, len(seen)), Edges: []event.Link{}}
	for _, ev := range seen {
		st.Events = append(st.Events, ev)
	}
	slices.SortFunc(st.Events, func(a, b event.Event) int {
		if c := eventTime(a).Compare(eventTime(b)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	for l := range edges {
		_, hasSource := seen[l.Source]
		_, hasTarget := seen[l.Target]
		if hasSource && hasTarget {
			st.Edges = append(st.Edges, l)
		}
	}
	slices.SortFunc(st.Edges, func(a, b event.Link) int {
		return strings.Compare(a.Source+a.Target+a.Rel+a.Basis, b.Source+b.Target+b.Rel+b.Basis)
	})
	return st
}

// eventTime is the parsed time of e. Stored events were validated, so a parse
// failure leaves the zero time, which sorts first.
func eventTime(e event.Event) time.Time {
	t, err := e.ParsedTime()
	if err != nil {
		return time.Time{}
	}
	return t
}
