package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Sighting struct {
	ID           int64
	OrganismName string
	FoundAt      time.Time
}

type Store interface {
	RecordSighting(ctx context.Context, name string) (count int, err error)
	GetSightings(ctx context.Context) ([]Sighting, error)
	Close() error
}

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sightings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organism_name TEXT NOT NULL,
			found_at DATETIME NOT NULL
		)
	`)
	return err
}

func (s *SQLiteStore) RecordSighting(ctx context.Context, name string) (int, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sightings (organism_name, found_at) VALUES (?, ?)",
		name, now,
	)
	if err != nil {
		return 0, err
	}

	var count int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sightings WHERE LOWER(organism_name) = LOWER(?)",
		name,
	).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *SQLiteStore) GetSightings(ctx context.Context) ([]Sighting, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, organism_name, found_at FROM sightings ORDER BY found_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sightings []Sighting
	for rows.Next() {
		var sg Sighting
		var foundAtStr string
		if err := rows.Scan(&sg.ID, &sg.OrganismName, &foundAtStr); err != nil {
			return nil, err
		}
		sg.FoundAt, _ = time.Parse("2006-01-02 15:04:05+00:00", foundAtStr)
		if sg.FoundAt.IsZero() {
			sg.FoundAt, _ = time.Parse("2006-01-02T15:04:05Z", foundAtStr)
		}
		if sg.FoundAt.IsZero() {
			sg.FoundAt, _ = time.Parse(time.RFC3339, foundAtStr)
		}
		sightings = append(sightings, sg)
	}
	return sightings, rows.Err()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// NormalizeName lowercases and trims an organism name for comparison.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
