package store_test

import (
	"testing"

	"annotate/internal/store"
)

func TestInsertAndGetNewsItems(t *testing.T) {
	db := openTestDB(t)
	item := store.NewsItem{
		ID:          "news_001",
		Ticker:      "NVDA",
		Title:       "NVIDIA Beats Expectations",
		Link:        "https://example.com/nvda",
		Publisher:   "Reuters",
		PublishedAt: 1713456789,
		FetchedAt:   "2025-04-19T09:00:00Z",
	}
	if err := db.InsertNewsItem(item); err != nil {
		t.Fatalf("InsertNewsItem: %v", err)
	}
	items, err := db.GetNewsForTicker("NVDA", 10)
	if err != nil {
		t.Fatalf("GetNewsForTicker: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ID != "news_001" || items[0].Title != "NVIDIA Beats Expectations" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestInsertNewsItem_IgnoreDuplicate(t *testing.T) {
	db := openTestDB(t)
	item := store.NewsItem{
		ID: "dup_001", Ticker: "AAPL", Title: "Title", Link: "https://example.com",
		PublishedAt: 1713456789, FetchedAt: "2025-04-19T09:00:00Z",
	}
	if err := db.InsertNewsItem(item); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := db.InsertNewsItem(item); err != nil {
		t.Fatalf("duplicate should not error: %v", err)
	}
	items, _ := db.GetNewsForTicker("AAPL", 10)
	if len(items) != 1 {
		t.Errorf("expected 1 item after duplicate, got %d", len(items))
	}
}

func TestGetLatestNewsFetchedAt_Missing(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetLatestNewsFetchedAt("MISSING")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestGetLatestNewsFetchedAt_ReturnsLatest(t *testing.T) {
	db := openTestDB(t)
	_ = db.InsertNewsItem(store.NewsItem{
		ID: "n1", Ticker: "TSLA", Title: "T1", Link: "L1",
		PublishedAt: 1713456789, FetchedAt: "2025-04-19T08:00:00Z",
	})
	_ = db.InsertNewsItem(store.NewsItem{
		ID: "n2", Ticker: "TSLA", Title: "T2", Link: "L2",
		PublishedAt: 1713456800, FetchedAt: "2025-04-19T09:00:00Z",
	})
	got, err := db.GetLatestNewsFetchedAt("TSLA")
	if err != nil {
		t.Fatalf("GetLatestNewsFetchedAt: %v", err)
	}
	if got != "2025-04-19T09:00:00Z" {
		t.Errorf("expected 2025-04-19T09:00:00Z, got %s", got)
	}
}
