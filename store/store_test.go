package store

import (
	"context"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecordFirstSighting(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	count, err := s.RecordSighting(ctx, "Monarch Butterfly")
	if err != nil {
		t.Fatalf("RecordSighting failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1 for first sighting, got %d", count)
	}
}

func TestRecordRepeatSighting(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.RecordSighting(ctx, "Ladybug")
	s.RecordSighting(ctx, "Ladybug")
	count, err := s.RecordSighting(ctx, "Ladybug")
	if err != nil {
		t.Fatalf("RecordSighting failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3 for third sighting, got %d", count)
	}
}

func TestDifferentOrganismsIndependentCounts(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.RecordSighting(ctx, "Ladybug")
	s.RecordSighting(ctx, "Ladybug")

	count, err := s.RecordSighting(ctx, "Dragonfly")
	if err != nil {
		t.Fatalf("RecordSighting failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1 for new organism, got %d", count)
	}
}

func TestCaseInsensitiveCounting(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.RecordSighting(ctx, "Monarch Butterfly")
	count, err := s.RecordSighting(ctx, "monarch butterfly")
	if err != nil {
		t.Fatalf("RecordSighting failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2 for case-insensitive match, got %d", count)
	}
}

func TestGetSightings(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	s.RecordSighting(ctx, "Ladybug")
	s.RecordSighting(ctx, "Dragonfly")
	s.RecordSighting(ctx, "Ladybug")

	sightings, err := s.GetSightings(ctx)
	if err != nil {
		t.Fatalf("GetSightings failed: %v", err)
	}
	if len(sightings) != 3 {
		t.Errorf("expected 3 sightings, got %d", len(sightings))
	}
	for _, s := range sightings {
		if s.FoundAt.IsZero() {
			t.Error("expected non-zero timestamp")
		}
		if s.OrganismName == "" {
			t.Error("expected non-empty organism name")
		}
	}
}

func TestSightingTimestamp(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	before := time.Now().Add(-time.Second)
	s.RecordSighting(ctx, "Ant")
	after := time.Now().Add(time.Second)

	sightings, _ := s.GetSightings(ctx)
	if len(sightings) != 1 {
		t.Fatalf("expected 1 sighting, got %d", len(sightings))
	}
	ts := sightings[0].FoundAt
	if ts.Before(before) || ts.After(after) {
		t.Errorf("timestamp %v not between %v and %v", ts, before, after)
	}
}
