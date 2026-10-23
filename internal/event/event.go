// Package event holds the Techlog event model (draft 0.1): Go types, id
// generation and JSON Schema validation. It depends on nothing else in the
// module.
package event

import "time"

// SchemaVersion is the only event schema version this server accepts.
const SchemaVersion = "0.1-draft"

// Types lists the 13 event types, in lifecycle order. A test keeps it equal to
// the enum in the embedded schema.
var Types = []string{
	"problem.reported",
	"investigation.completed",
	"proposal.recorded",
	"decision.made",
	"change.implemented",
	"verification.completed",
	"release.deployed",
	"incident.opened",
	"mitigation.applied",
	"rollback.executed",
	"recovery.verified",
	"postmortem.published",
	"action.tracked",
}

type Actor struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	OnBehalfOf string `json:"on_behalf_of,omitempty"`
}

type Subject struct {
	System      string `json:"system"`
	Component   string `json:"component,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type Source struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type Evidence struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Digest string `json:"digest,omitempty"`
}

// LinkRef is a link as it appears inside an event: it points at an earlier event.
type LinkRef struct {
	Rel    string `json:"rel"`
	Target string `json:"target"`
	Basis  string `json:"basis"`
}

type Provenance struct {
	RecordedBy string `json:"recorded_by"`
	Method     string `json:"method"`
	Confidence string `json:"confidence,omitempty"`
}

// Event is one immutable engineering event. Time is kept as the submitted
// RFC 3339 string; use ParsedTime for comparisons.
type Event struct {
	Extensions     map[string]any `json:"extensions,omitempty"`
	Actor          Actor          `json:"actor"`
	Provenance     Provenance     `json:"provenance"`
	Subject        Subject        `json:"subject"`
	Type           string         `json:"type"`
	Time           string         `json:"time"`
	SchemaVersion  string         `json:"schema_version"`
	Summary        string         `json:"summary,omitempty"`
	IdempotencyKey string         `json:"idempotency_key"`
	ID             string         `json:"id"`
	Sources        []Source       `json:"sources,omitempty"`
	Evidence       []Evidence     `json:"evidence,omitempty"`
	Links          []LinkRef      `json:"links,omitempty"`
}

// ParsedTime parses Time as RFC 3339.
func (e Event) ParsedTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.Time)
}

// Link is a stored link between two events. Rationale is only kept for links
// added after the fact; it is not part of the event schema.
type Link struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Rel       string `json:"rel"`
	Basis     string `json:"basis"`
	Rationale string `json:"rationale,omitempty"`
}
