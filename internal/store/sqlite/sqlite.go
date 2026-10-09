// Package sqlite is the SQLite storage backend, built on the pure-Go
// modernc.org/sqlite driver (no cgo).
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const (
	// _txlock=immediate starts write transactions with BEGIN IMMEDIATE, so the
	// busy timeout applies to taking the lock (a deferred transaction that reads
	// first fails at once with SQLITE_BUSY when another process writes).
	pragmas = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"
	// timeLayout is fixed-width UTC, so the time column sorts and compares as text.
	timeLayout = "2006-01-02T15:04:05.000Z"
)

var memCounter atomic.Int64

// Store implements store.Store on SQLite. It uses a single connection: writes
// are serialised by the pool instead of failing with "database is locked", and
// no method may start a second query while a result set is still open.
type Store struct{ db *sql.DB }

var _ store.Store = (*Store)(nil)

// Open opens (creating it, and its directory, if needed) the database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	return open("file:" + path + "?" + pragmas)
}

// OpenMemory opens a private in-memory database, unique per call. Used by tests.
func OpenMemory() (*Store, error) {
	n := memCounter.Add(1)
	return open(fmt.Sprintf("file:techlogmem%d?mode=memory&cache=shared&%s", n, pragmas))
}

func open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// migrate applies migrations/NNN_*.sql files, in name order, that are newer
// than PRAGMA user_version.
func migrate(db *sql.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	for i, e := range entries {
		version := i + 1
		if version <= current {
			continue
		}
		script, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func ensureEvents(ctx context.Context, q querier, ids ...string) error {
	for _, id := range ids {
		var one int
		err := q.QueryRowContext(ctx, `SELECT 1 FROM events WHERE id = ?`, id).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("event %s: %w", id, store.ErrNotFound)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func insertLink(ctx context.Context, q querier, source, target, rel, basis, rationale string) error {
	_, err := q.ExecContext(ctx,
		`INSERT OR IGNORE INTO links (source, target, rel, basis, rationale, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		source, target, rel, basis, rationale, time.Now().UTC().Format(timeLayout))
	return err
}

func (s *Store) Insert(ctx context.Context, e event.Event) error {
	t, err := e.ParsedTime()
	if err != nil {
		return fmt.Errorf("event time: %w", err)
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	targets := make([]string, len(e.Links))
	for i, l := range e.Links {
		targets[i] = l.Target
	}
	if err := ensureEvents(ctx, tx, targets...); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, type, time, status, system, component, actor_kind, actor_id, summary, body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Type, t.UTC().Format(timeLayout), e.Status, e.Subject.System, e.Subject.Component,
		e.Actor.Kind, e.Actor.ID, e.Summary, string(body))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return store.ErrConflict
		}
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO events_fts (id, summary, system, component, type) VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.Summary, e.Subject.System, e.Subject.Component, e.Type); err != nil {
		return err
	}
	for _, l := range e.Links {
		if err := insertLink(ctx, tx, e.ID, l.Target, l.Rel, l.Basis, ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AddLink(ctx context.Context, l event.Link) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureEvents(ctx, tx, l.Source, l.Target); err != nil {
		return err
	}
	if err := insertLink(ctx, tx, l.Source, l.Target, l.Rel, l.Basis, l.Rationale); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Get(ctx context.Context, id string) (event.Event, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT body FROM events WHERE id = ?`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return event.Event{}, store.ErrNotFound
	}
	if err != nil {
		return event.Event{}, err
	}
	return s.hydrate(ctx, body)
}

// hydrate decodes a stored body and replaces its links with the current
// outgoing rows from the links table (so links added later are included).
func (s *Store) hydrate(ctx context.Context, body string) (event.Event, error) {
	var e event.Event
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		return event.Event{}, fmt.Errorf("decode stored event: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT target, rel, basis FROM links WHERE source = ? ORDER BY rowid`, e.ID)
	if err != nil {
		return event.Event{}, err
	}
	defer rows.Close()
	var links []event.LinkRef
	for rows.Next() {
		var l event.LinkRef
		if err := rows.Scan(&l.Target, &l.Rel, &l.Basis); err != nil {
			return event.Event{}, err
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return event.Event{}, err
	}
	e.Links = links
	return e, nil
}

func (s *Store) Search(ctx context.Context, f store.Filter) ([]event.Event, error) {
	var (
		where []string
		args  []any
	)
	if q := ftsQuery(f.Text); q != "" {
		where = append(where, "e.id IN (SELECT id FROM events_fts WHERE events_fts MATCH ?)")
		args = append(args, q)
	}
	if f.System != "" {
		where = append(where, "e.system = ?")
		args = append(args, f.System)
	}
	if len(f.Types) > 0 {
		where = append(where, "e.type IN ("+strings.TrimSuffix(strings.Repeat("?,", len(f.Types)), ",")+")")
		for _, t := range f.Types {
			args = append(args, t)
		}
	}
	if !f.Since.IsZero() {
		where = append(where, "e.time >= ?")
		args = append(args, f.Since.UTC().Format(timeLayout))
	}
	if !f.Until.IsZero() {
		where = append(where, "e.time <= ?")
		args = append(args, f.Until.UTC().Format(timeLayout))
	}
	dir := "DESC"
	if f.Ascending {
		dir = "ASC"
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query := "SELECT e.body FROM events e"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY e.time " + dir + ", e.id " + dir + " LIMIT ?"
	args = append(args, limit)

	bodies, err := s.queryStrings(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	events := make([]event.Event, 0, len(bodies))
	for _, b := range bodies {
		e, err := s.hydrate(ctx, b)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (s *Store) Links(ctx context.Context, id string) ([]event.Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, target, rel, basis, rationale FROM links WHERE source = ? OR target = ? ORDER BY rowid`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []event.Link
	for rows.Next() {
		var l event.Link
		if err := rows.Scan(&l.Source, &l.Target, &l.Rel, &l.Basis, &l.Rationale); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *Store) Systems(ctx context.Context) ([]string, error) {
	return s.queryStrings(ctx, `SELECT DISTINCT system FROM events ORDER BY system`)
}

// queryStrings runs a single-column text query and closes the result set
// before returning, so the single connection is free for the next query.
func (s *Store) queryStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
