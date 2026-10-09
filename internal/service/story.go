package service

import (
	"context"
	"slices"
	"strings"

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
	seen := map[string]event.Event{id: root}
	edges := map[event.Link]struct{}{}
	frontier := []string{id}
	for hop := 0; hop < depth && len(frontier) > 0; hop++ {
		var next []string
		for _, cur := range frontier {
			links, err := s.store.Links(ctx, cur)
			if err != nil {
				return Story{}, err
			}
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
					return Story{}, err
				}
				seen[other] = ev
				next = append(next, other)
			}
		}
		frontier = next
	}

	st := Story{Root: id, Events: make([]event.Event, 0, len(seen)), Edges: []event.Link{}}
	for _, ev := range seen {
		st.Events = append(st.Events, ev)
	}
	slices.SortFunc(st.Events, func(a, b event.Event) int {
		ta, _ := a.ParsedTime()
		tb, _ := b.ParsedTime()
		if c := ta.Compare(tb); c != 0 {
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
	return st, nil
}
