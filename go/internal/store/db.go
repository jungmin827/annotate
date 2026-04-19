package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB. SQL is exported so test files in the store_test package
// can execute raw queries directly.
type DB struct {
	SQL *sql.DB
}

// Open opens (or creates) the SQLite database at path and applies the schema.
func Open(path string) (*DB, error) {
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := applySchema(raw); err != nil {
		raw.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{SQL: raw}, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.SQL.Close()
}

func applySchema(raw *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS trades (
	id             TEXT PRIMARY KEY,
	ticker         TEXT NOT NULL,
	name           TEXT NOT NULL,
	market         TEXT NOT NULL,
	action         TEXT NOT NULL,
	price          REAL NOT NULL,
	quantity       INTEGER NOT NULL,
	date           TEXT NOT NULL,
	time           TEXT,
	reason         TEXT NOT NULL,
	status         TEXT NOT NULL,
	linked_sell_id TEXT,
	linked_buy_id  TEXT
);

CREATE TABLE IF NOT EXISTS market_cache (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	ticker  TEXT NOT NULL,
	date    TEXT NOT NULL,
	open    REAL,
	high    REAL,
	low     REAL,
	close   REAL,
	volume  INTEGER,
	UNIQUE(ticker, date)
);

CREATE TABLE IF NOT EXISTS news (
	id           TEXT PRIMARY KEY,
	ticker       TEXT NOT NULL,
	title        TEXT NOT NULL,
	link         TEXT NOT NULL,
	publisher    TEXT,
	published_at INTEGER,
	fetched_at   TEXT NOT NULL
);
`
	_, err := raw.Exec(schema)
	return err
}
