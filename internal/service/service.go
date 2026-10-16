// Package service holds Techlog's use cases. The HTTP API, the MCP server and
// the web UI call only this package, never a store directly.
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

// Provenance methods applied when a submission does not state one.
const (
	MethodHuman = "human_entered"   // events submitted through the HTTP API
	MethodAgent = "agent_submitted" // events submitted through MCP
)

const maxIDAttempts = 5

// DefaultClockSkew is how far ahead of the server clock an event time may be.
const DefaultClockSkew = 5 * time.Second

type Service struct {
	store     store.Store
	validator *event.Validator
	now       func() time.Time
	skew      time.Duration
	entropy   io.Reader
}

type Option func(*Service)

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithClockSkew sets how far ahead of the clock an event time may be. Zero
// allows none; a negative value makes New fail.
func WithClockSkew(d time.Duration) Option { return func(s *Service) { s.skew = d } }

func New(st store.Store, opts ...Option) (*Service, error) {
	v, err := event.NewValidator()
	if err != nil {
		return nil, err
	}
	s := &Service{store: st, validator: v, now: time.Now, skew: DefaultClockSkew, entropy: rand.Reader}
	for _, o := range opts {
		o(s)
	}
	if s.skew < 0 {
		return nil, fmt.Errorf("clock skew must not be negative, got %s", s.skew)
	}
	return s, nil
}

// Record stores a new event. doc is the event without an id (a supplied id is
// ignored). method is the provenance method to use when doc has none. The
// event is validated against the JSON Schema, then link targets are checked; a
// second event with the same actor and idempotency_key is rejected with a
// *DuplicateError.
func (s *Service) Record(ctx context.Context, doc map[string]any, method string) (event.Event, error) {
	doc, err := event.Normalize(doc)
	if err != nil {
		return event.Event{}, &BadRequestError{Msg: "event is not valid JSON: " + err.Error()}
	}
	now := s.now()
	stamped := isBlank(doc["time"])
	applyDefaults(doc, method, now)
	id, err := event.NewID(now, s.entropy)
	if err != nil {
		return event.Event{}, err
	}
	doc["id"] = id

	ev, err := s.prepare(ctx, doc, now, stamped)
	if err != nil {
		return event.Event{}, err
	}
	for attempt := 0; ; attempt++ {
		err = s.store.Insert(ctx, ev)
		if !errors.Is(err, store.ErrConflict) || attempt == maxIDAttempts-1 {
			break
		}
		newID, idErr := event.NewID(s.now(), s.entropy)
		if idErr != nil {
			return event.Event{}, idErr
		}
		ev.ID = newID
	}
	var dup *store.DuplicateError
	if errors.As(err, &dup) {
		return event.Event{}, &DuplicateError{Key: ev.IdempotencyKey, ActorID: ev.Actor.ID, ExistingID: dup.ExistingID}
	}
	if err != nil {
		return event.Event{}, err
	}
	return s.store.Get(ctx, ev.ID)
}

// applyDefaults fills what the caller may omit. It never invents a basis for a
// caused_by link: that must be stated.
func applyDefaults(doc map[string]any, method string, now time.Time) {
	delete(doc, "id")
	if isBlank(doc["schema_version"]) {
		doc["schema_version"] = event.SchemaVersion
	}
	if isBlank(doc["time"]) {
		doc["time"] = now.UTC().Format(time.RFC3339)
	}
	if doc["provenance"] == nil {
		doc["provenance"] = map[string]any{}
	}
	if prov, ok := doc["provenance"].(map[string]any); ok {
		if isBlank(prov["method"]) {
			prov["method"] = method
		}
		if isBlank(prov["recorded_by"]) {
			if actor, ok := doc["actor"].(map[string]any); ok {
				if id, ok := actor["id"].(string); ok && id != "" {
					prov["recorded_by"] = id
				}
			}
		}
	}
	if links, ok := doc["links"].([]any); ok {
		for _, raw := range links {
			if l, ok := raw.(map[string]any); ok && isBlank(l["basis"]) && l["rel"] != "caused_by" {
				l["basis"] = "confirmed"
			}
		}
	}
}

func isBlank(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

// prepare validates doc (which already has an id) and decodes it. It reports
// schema problems and unresolvable link targets together.
func (s *Service) prepare(ctx context.Context, doc map[string]any, now time.Time, stamped bool) (event.Event, error) {
	if err := s.validator.Validate(doc); err != nil {
		var ve *event.ValidationError
		if errors.As(err, &ve) {
			return event.Event{}, &InvalidError{Problems: ve.Problems}
		}
		return event.Event{}, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return event.Event{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var ev event.Event
	if err := dec.Decode(&ev); err != nil {
		return event.Event{}, fmt.Errorf("decode validated event: %w", err)
	}

	var problems []event.Problem
	evTime, evErr := ev.ParsedTime()
	if evErr != nil {
		problems = append(problems, event.Problem{Path: "/time", Message: evErr.Error()})
	} else {
		if !stamped && evTime.After(now.Add(s.skew)) {
			problems = append(problems, event.Problem{
				Path:    "/time",
				Message: fmt.Sprintf("time %s is in the future, beyond the allowed clock skew of %s", ev.Time, s.skew),
			})
		}
		ev.Time = evTime.UTC().Format(time.RFC3339Nano)
	}
	var latestTarget time.Time
	latestIdx := -1
	for i, l := range ev.Links {
		target, err := s.store.Get(ctx, l.Target)
		if errors.Is(err, store.ErrNotFound) {
			problems = append(problems, event.Problem{
				Path:    fmt.Sprintf("/links/%d/target", i),
				Message: fmt.Sprintf("event %s does not exist", l.Target),
			})
			continue
		} else if err != nil {
			return event.Event{}, err
		}
		targetTime, terr := target.ParsedTime()
		if terr != nil || evErr != nil {
			continue
		}
		if targetTime.After(latestTarget) {
			latestTarget, latestIdx = targetTime, i
		}
		if !stamped && evTime.Before(targetTime) {
			problems = append(problems, event.Problem{
				Path:    fmt.Sprintf("/links/%d", i),
				Message: fmt.Sprintf("time %s is earlier than %s, the time of linked event %s", ev.Time, target.Time, l.Target),
			})
		}
	}
	if stamped && evErr == nil && latestTarget.After(now.Add(s.skew)) {
		// A stamp is never pushed past the clock plus the skew: the target is
		// out of range and the event cannot follow it.
		problems = append(problems, event.Problem{
			Path:    fmt.Sprintf("/links/%d", latestIdx),
			Message: fmt.Sprintf("linked event %s is dated %s, too far ahead of the clock for a server-stamped time", ev.Links[latestIdx].Target, latestTarget.UTC().Format(time.RFC3339Nano)),
		})
	} else if stamped && evErr == nil && latestTarget.After(evTime) {
		ev.Time = latestTarget.UTC().Format(time.RFC3339Nano)
	}
	if len(problems) > 0 {
		return event.Event{}, &InvalidError{Problems: problems}
	}
	return ev, nil
}

// LinkInput describes a link to add between two existing events.
type LinkInput struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Rel       string `json:"rel"`
	Basis     string `json:"basis,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

// Link adds a link. Basis defaults to "confirmed" except for caused_by, where
// it must be stated. An identical link added twice is a no-op.
func (s *Service) Link(ctx context.Context, in LinkInput) (event.Link, error) {
	if in.Basis == "" && in.Rel != "caused_by" {
		in.Basis = "confirmed"
	}
	doc := map[string]any{"rel": in.Rel, "target": in.Target}
	if in.Basis != "" {
		doc["basis"] = in.Basis
	}
	var problems []event.Problem
	if err := s.validator.ValidateLink(doc); err != nil {
		var ve *event.ValidationError
		if !errors.As(err, &ve) {
			return event.Link{}, err
		}
		problems = append(problems, ve.Problems...)
	}
	switch in.Source {
	case "":
		problems = append(problems, event.Problem{Path: "/source", Message: "source is required"})
	case in.Target:
		problems = append(problems, event.Problem{Path: "/target", Message: "an event cannot link to itself"})
	}
	if len(problems) == 0 {
		for _, end := range []struct{ path, id string }{{"/source", in.Source}, {"/target", in.Target}} {
			if _, err := s.store.Get(ctx, end.id); errors.Is(err, store.ErrNotFound) {
				problems = append(problems, event.Problem{Path: end.path, Message: fmt.Sprintf("event %s does not exist", end.id)})
			} else if err != nil {
				return event.Link{}, err
			}
		}
	}
	if len(problems) > 0 {
		return event.Link{}, &InvalidError{Problems: problems}
	}
	l := event.Link{Source: in.Source, Target: in.Target, Rel: in.Rel, Basis: in.Basis, Rationale: in.Rationale}
	if err := s.store.AddLink(ctx, l); err != nil {
		return event.Link{}, err
	}
	return l, nil
}

// Detail is an event with the links on either side of it.
type Detail struct {
	Event    event.Event  `json:"event"`
	Outgoing []event.Link `json:"outgoing"`
	Incoming []event.Link `json:"incoming"`
}

// Get returns one event and its links. A missing event is store.ErrNotFound.
func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	ev, err := s.store.Get(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	links, err := s.store.Links(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Event: ev, Outgoing: []event.Link{}, Incoming: []event.Link{}}
	for _, l := range links {
		if l.Source == id {
			d.Outgoing = append(d.Outgoing, l)
		}
		if l.Target == id {
			d.Incoming = append(d.Incoming, l)
		}
	}
	return d, nil
}

// Import stores events that already carry ids (used to load the seed story). It
// validates them like Record but keeps their ids, and skips ids and idempotency keys that exist.
// Documents must be ordered so every link target comes before its source.
func (s *Service) Import(ctx context.Context, docs []map[string]any) (inserted, skipped int, err error) {
	for _, raw := range docs {
		doc, err := event.Normalize(raw)
		if err != nil {
			return inserted, skipped, &BadRequestError{Msg: "event is not valid JSON: " + err.Error()}
		}
		ev, err := s.prepare(ctx, doc, s.now(), false)
		if err != nil {
			return inserted, skipped, err
		}
		switch err := s.store.Insert(ctx, ev); {
		case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrDuplicate):
			skipped++
		case err != nil:
			return inserted, skipped, err
		default:
			inserted++
		}
	}
	return inserted, skipped, nil
}
