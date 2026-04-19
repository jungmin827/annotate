package scheduler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"annotate/internal/scheduler"
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

func TestScheduler_FetchesNewsOnStart(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"news": []map[string]any{
				{"uuid": "sched_001", "title": "Scheduled News", "link": "https://example.com", "publisher": "AP", "providerPublishTime": 1713456789},
			},
		})
	}))
	defer srv.Close()

	db := openTestDB(t)
	_ = db.InsertTrade(store.Trade{
		ID: "t1", Ticker: "NVDA", Name: "NVIDIA", Market: "NASDAQ",
		Action: "buy", Price: 875.5, Quantity: 5,
		Date: "2025-03-15", Reason: "AI", Status: "holding",
	})

	s := scheduler.New(db, 10*time.Second, srv.URL)
	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()

	items, err := db.GetNewsForTicker("NVDA", 10)
	if err != nil {
		t.Fatalf("GetNewsForTicker: %v", err)
	}
	if len(items) == 0 {
		t.Error("expected news items to be stored after scheduler start")
	}
	if callCount == 0 {
		t.Error("expected Yahoo Finance to be called at least once")
	}
}

func TestScheduler_StopsCleanly(t *testing.T) {
	db := openTestDB(t)
	s := scheduler.New(db, 10*time.Second, "")
	s.Start()
	done := make(chan struct{})
	go func() {
		s.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop() did not return within 1 second")
	}
}
