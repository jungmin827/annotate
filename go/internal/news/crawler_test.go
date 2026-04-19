package news_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"annotate/internal/news"
	"annotate/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mockNewsServer(items []map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"news": items})
	}))
}

func TestFetch_ReturnsItems(t *testing.T) {
	srv := mockNewsServer([]map[string]any{
		{"uuid": "id1", "title": "NVIDIA Beats", "link": "https://example.com/1", "publisher": "Reuters", "providerPublishTime": 1713456789},
		{"uuid": "id2", "title": "More News", "link": "https://example.com/2", "publisher": "Bloomberg", "providerPublishTime": 1713456800},
	})
	defer srv.Close()

	items, err := news.Fetch("NVDA", srv.URL, 5)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].UUID != "id1" || items[0].Title != "NVIDIA Beats" {
		t.Errorf("unexpected item: %+v", items[0])
	}
}

func TestFetch_EmptyResponse(t *testing.T) {
	srv := mockNewsServer([]map[string]any{})
	defer srv.Close()

	items, err := news.Fetch("NVDA", srv.URL, 5)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestFetchAndStore_SavesItems(t *testing.T) {
	srv := mockNewsServer([]map[string]any{
		{"uuid": "s1", "title": "Stored News", "link": "https://example.com/s1", "publisher": "AP", "providerPublishTime": 1713456789},
	})
	defer srv.Close()

	db := openTestDB(t)
	if err := news.FetchAndStore("NVDA", db, srv.URL); err != nil {
		t.Fatalf("FetchAndStore: %v", err)
	}

	items, err := db.GetNewsForTicker("NVDA", 10)
	if err != nil {
		t.Fatalf("GetNewsForTicker: %v", err)
	}
	if len(items) != 1 || items[0].ID != "s1" {
		t.Errorf("unexpected stored items: %+v", items)
	}
}

func TestFetchAndStore_IdempotentOnDuplicate(t *testing.T) {
	srv := mockNewsServer([]map[string]any{
		{"uuid": "dup1", "title": "Same News", "link": "https://example.com", "publisher": "AP", "providerPublishTime": 1713456789},
	})
	defer srv.Close()

	db := openTestDB(t)
	if err := news.FetchAndStore("AAPL", db, srv.URL); err != nil {
		t.Fatalf("first FetchAndStore: %v", err)
	}
	if err := news.FetchAndStore("AAPL", db, srv.URL); err != nil {
		t.Fatalf("second FetchAndStore should not error: %v", err)
	}

	items, _ := db.GetNewsForTicker("AAPL", 10)
	if len(items) != 1 {
		t.Errorf("expected 1 item after idempotent call, got %d", len(items))
	}
}
