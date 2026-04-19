package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"annotate/internal/handler"
	"annotate/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.SQL.Close() })
	return db
}

func seedTrade(t *testing.T, db *store.DB, tr store.Trade) {
	t.Helper()
	if err := db.InsertTrade(tr); err != nil {
		t.Fatalf("insert trade: %v", err)
	}
}

var sample = store.Trade{
	ID: "t1", Ticker: "005930", Name: "삼성전자", Market: "KRX",
	Action: "buy", Price: 70000, Quantity: 10,
	Date: "2024-01-15", Reason: "반도체 사이클", Status: "holding",
}

func TestDashboard_OK(t *testing.T) {
	db := openTestDB(t)
	seedTrade(t, db, sample)

	h := handler.New(db, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "삼성전자") {
		t.Error("dashboard missing trade name")
	}
}

func TestTradesList_OK(t *testing.T) {
	db := openTestDB(t)
	seedTrade(t, db, sample)

	h := handler.New(db, "")
	req := httptest.NewRequest(http.MethodGet, "/trades", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "005930") {
		t.Error("trades page missing ticker")
	}
}

func TestAnalyze_HTMX(t *testing.T) {
	// mock Claude server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"분석 결과입니다."}]}`))
	}))
	defer srv.Close()

	db := openTestDB(t)
	seedTrade(t, db, sample)

	h := handler.New(db, "test-key")
	h.SetClaudeBaseURL(srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/analyze/t1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "분석 결과") {
		t.Error("analyze response missing result text")
	}
}

func TestAnalyze_NotFound(t *testing.T) {
	db := openTestDB(t)
	h := handler.New(db, "")
	req := httptest.NewRequest(http.MethodPost, "/analyze/nonexistent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", rec.Code)
	}
}

func TestReview_HTMX(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"패턴상 당신은 반도체 관련주를 선호합니다."}]}`))
	}))
	defer srv.Close()

	db := openTestDB(t)
	seedTrade(t, db, sample)

	h := handler.New(db, "test-key")
	h.SetClaudeBaseURL(srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/review", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "패턴") {
		t.Error("review response missing pattern text")
	}
}

func TestBrief_NoHoldings(t *testing.T) {
	db := openTestDB(t)
	h := handler.New(db, "")

	req := httptest.NewRequest(http.MethodPost, "/brief", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "보유 중인 종목이 없습니다") {
		t.Error("expected empty holdings message")
	}
}

func TestBrief_WithHolding(t *testing.T) {
	claudeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"1. NVIDIA 실적 호조\n2. AI 수요 증가"}]}`))
	}))
	defer claudeSrv.Close()

	newsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"news": []map[string]any{
				{"uuid": "n1", "title": "NVIDIA Q1 Record", "link": "https://example.com", "publisher": "Reuters", "providerPublishTime": 1713456789},
			},
		})
	}))
	defer newsSrv.Close()

	db := openTestDB(t)
	seedTrade(t, db, store.Trade{
		ID: "t2", Ticker: "NVDA", Name: "NVIDIA", Market: "NASDAQ",
		Action: "buy", Price: 875.5, Quantity: 5,
		Date: "2025-03-15", Reason: "AI 수혜주", Status: "holding",
	})

	h := handler.New(db, "test-key")
	h.SetClaudeBaseURL(claudeSrv.URL)
	h.SetNewsBaseURL(newsSrv.URL)

	req := httptest.NewRequest(http.MethodPost, "/brief", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "NVIDIA") {
		t.Error("brief response missing ticker name")
	}
}
