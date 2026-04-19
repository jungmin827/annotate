package store

import "database/sql"

// NewsItem represents a single news article cached for a ticker.
type NewsItem struct {
	ID          string
	Ticker      string
	Title       string
	Link        string
	Publisher   string
	PublishedAt int64
	FetchedAt   string
}

// InsertNewsItem inserts a news item, silently ignoring duplicates by ID.
func (db *DB) InsertNewsItem(n NewsItem) error {
	_, err := db.SQL.Exec(`
		INSERT OR IGNORE INTO news (id, ticker, title, link, publisher, published_at, fetched_at)
		VALUES (?,?,?,?,?,?,?)`,
		n.ID, n.Ticker, n.Title, n.Link, n.Publisher, n.PublishedAt, n.FetchedAt,
	)
	return err
}

// GetNewsForTicker returns up to limit news items for the given ticker,
// ordered by published_at descending.
func (db *DB) GetNewsForTicker(ticker string, limit int) ([]NewsItem, error) {
	rows, err := db.SQL.Query(`
		SELECT id, ticker, title, link, publisher, published_at, fetched_at
		FROM news WHERE ticker = ?
		ORDER BY published_at DESC LIMIT ?`, ticker, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []NewsItem
	for rows.Next() {
		var n NewsItem
		if err := rows.Scan(&n.ID, &n.Ticker, &n.Title, &n.Link, &n.Publisher, &n.PublishedAt, &n.FetchedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return items, rows.Err()
}

// GetLatestNewsFetchedAt returns the most recent fetched_at value for the
// given ticker, or an empty string if no records exist.
func (db *DB) GetLatestNewsFetchedAt(ticker string) (string, error) {
	var t string
	err := db.SQL.QueryRow(
		`SELECT fetched_at FROM news WHERE ticker=? ORDER BY fetched_at DESC LIMIT 1`, ticker,
	).Scan(&t)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return t, err
}
