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
)

// Filter selects events. Zero values mean "no constraint", except Limit, where
// a value <= 0 means 100. Results are newest first unless Ascending is set.
type Filter struct {
	Text      string
	System    string
	Types     []string
	Since     time.Time // inclusive
	Until     time.Time // inclusive
	Limit     int
	Ascending bool
}

// Store persists immutable events and the links between them. There is no
// update or delete.
type Store interface {
	// Insert stores an event and the links in e.Links. It returns ErrConflict
	// if the id exists and ErrNotFound if a link target does not exist; in
	// both cases nothing is stored.
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
