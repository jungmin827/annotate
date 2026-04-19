package news

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"annotate/internal/store"
)

type Item struct {
	UUID        string `json:"uuid"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Publisher   string `json:"publisher"`
	PublishedAt int64  `json:"providerPublishTime"`
}

// Fetch fetches up to maxCount news items for ticker from Yahoo Finance.
// baseURL is injectable for tests; pass "" to use the real endpoint.
func Fetch(ticker, baseURL string, maxCount int) ([]Item, error) {
	if baseURL == "" {
		baseURL = "https://query1.finance.yahoo.com"
	}
	url := fmt.Sprintf(
		"%s/v1/finance/search?q=%s&newsCount=%d&enableFuzzyQuery=false&quotesCount=0",
		baseURL, ticker, maxCount,
	)

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		News []Item `json:"news"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return payload.News, nil
}

// FetchAndStore fetches news for ticker and saves to the database.
// Duplicate UUIDs are silently ignored (INSERT OR IGNORE).
func FetchAndStore(ticker string, db *store.DB, baseURL string) error {
	items, err := Fetch(ticker, baseURL, 5)
	if err != nil {
		return err
	}
	fetchedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	for _, item := range items {
		if err := db.InsertNewsItem(store.NewsItem{
			ID:          item.UUID,
			Ticker:      ticker,
			Title:       item.Title,
			Link:        item.Link,
			Publisher:   item.Publisher,
			PublishedAt: item.PublishedAt,
			FetchedAt:   fetchedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}
