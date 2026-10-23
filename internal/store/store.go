// Package store defines the storage contract for Techlog events. Backends live
// in subpackages and are verified by the suite in package storetest.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/regeda/techlog/internal/event"
)

var (
	// ErrNotFound means an event (or a link endpoint) does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means an event with that id already exists.
	ErrConflict = errors.New("conflict")
	// ErrDuplicate means the same actor already recorded an event with that
	// idempotency key. Insert returns a *DuplicateError that matches it.
	ErrDuplicate = errors.New("duplicate idempotency key")
)

// DuplicateError is returned by Insert when (actor id, idempotency key) is
// already stored. ExistingID is the event that holds the key.
type DuplicateError struct{ ExistingID string }

func (e *DuplicateError) Error() string {
	return "duplicate idempotency key: already recorded as " + e.ExistingID
}

func (e *DuplicateError) Is(target error) bool { return target == ErrDuplicate }

// Filter selects events. Zero values mean "no constraint", except Limit, where
// a value <= 0 means 100, and Offset, where a value <= 0 means 0. Results are
// newest first unless Ascending is set; Offset skips that many from the start.
type Filter struct {
	Since     time.Time
	Until     time.Time
	Text      string
	System    string
	Types     []string
	Limit     int
	Offset    int
	Ascending bool
}

// Store persists immutable events and the links between them. There is no
// update or delete.
type Store interface {
	// Insert stores an event and the links in e.Links. It returns ErrConflict
	// if the id exists, ErrDuplicate (as a *DuplicateError) if the actor
	// already used the idempotency key, and ErrNotFound if a link target does
	// not exist; in every case nothing is stored.
	Insert(ctx context.Context, e event.Event) error
	// AddLink stores a link between two existing events. Adding an identical
	// link again is a no-op. It returns ErrNotFound if either end is missing.
	AddLink(ctx context.Context, l event.Link) error
	// Get returns one event with its outgoing links merged into Links.
	Get(ctx context.Context, id string) (event.Event, error)
	Search(ctx context.Context, f Filter) ([]event.Event, error)
	// Links returns every link where id is the source or the target.
	Links(ctx context.Context, id string) ([]event.Link, error)
	// Systems returns the distinct subject systems, sorted.
	Systems(ctx context.Context) ([]string, error)
	Close() error
}
