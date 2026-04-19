package store_test

import (
	"path/filepath"
	"testing"

	"annotate/internal/store"
)

func TestOpen_CreatesSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var name string
	err = db.SQL.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='trades'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("trades table not found: %v", err)
	}

	err = db.SQL.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='market_cache'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("market_cache table not found: %v", err)
	}

	err = db.SQL.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='news'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("news table not found: %v", err)
	}
}
