package service

import (
	"fmt"
	"strings"

	"github.com/regeda/techlog/internal/event"
)

// InvalidError reports that a submitted event or link failed validation. Each
// Problem names the JSON Pointer path of the offending field.
type InvalidError struct {
	Problems []event.Problem
}

func (e *InvalidError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Path + ": " + p.Message
	}
	return "invalid: " + strings.Join(parts, "; ")
}

// BadRequestError reports a malformed request (a bad date, limit, or body).
type BadRequestError struct {
	Msg string
}

func (e *BadRequestError) Error() string { return e.Msg }

// DuplicateError reports that the actor already recorded an event with this
// idempotency key. ExistingID is that event.
type DuplicateError struct {
	Key        string
	ActorID    string
	ExistingID string
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("duplicate: idempotency_key %q for actor %q was already recorded as %s",
		e.Key, e.ActorID, e.ExistingID)
}
