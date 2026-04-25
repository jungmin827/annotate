# Phase 3: 뉴스 크롤러 + 백그라운드 스케줄러 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Yahoo Finance에서 보유 종목 뉴스를 수집해 SQLite에 캐시하고, 백그라운드 스케줄러가 주기적으로 갱신하며, `POST /brief` HTTP 엔드포인트가 Claude 요약을 HTMX 프래그먼트로 반환한다.

**Architecture:** 뉴스 데이터는 `news` SQLite 테이블에 UUID 기준으로 중복 없이 캐시된다. 스케줄러는 서버 시작 시 goroutine으로 실행되어 설정된 간격(기본 1시간)으로 보유 종목 뉴스를 갱신한다. `/brief` 엔드포인트는 1시간 이상 된 뉴스를 즉시 갱신하고 Claude API로 3줄 요약한 뒤 HTMX 프래그먼트를 반환한다.

**Tech Stack:** Go, Yahoo Finance v1/finance/search API, modernc.org/sqlite, HTMX, Claude API (claude-sonnet-4-6)

---

## 파일 구조

```
go/
├── internal/
│   ├── news/
│   │   ├── crawler.go          ← 신규: Yahoo Finance 뉴스 수집 + DB 저장
│   │   └── crawler_test.go     ← 신규
│   ├── scheduler/
│   │   ├── jobs.go             ← 신규: 백그라운드 뉴스 갱신 스케줄러
│   │   └── jobs_test.go        ← 신규
│   ├── store/
│   │   ├── db.go               ← 수정: news 테이블 스키마 추가
│   │   ├── db_test.go          ← 수정: news 테이블 존재 확인 추가
│   │   ├── news.go             ← 신규: news CRUD
│   │   └── news_test.go        ← 신규
│   └── handler/
│       ├── handler.go          ← 수정: /brief 라우트 + SetNewsBaseURL
│       ├── handler_test.go     ← 수정: TestBrief 추가
│       └── templates/
│           ├── brief_result.html ← 신규: HTMX 뉴스 브리핑 프래그먼트
│           └── dashboard.html    ← 수정: 뉴스 브리핑 섹션 추가
└── cmd/
    └── server/
        └── main.go             ← 수정: 스케줄러 시작
```

---

## Task 1: news 테이블 스키마 + store 레이어

**Files:**
- Modify: `go/internal/store/db.go`
- Modify: `go/internal/store/db_test.go`
- Create: `go/internal/store/news.go`
- Create: `go/internal/store/news_test.go`

- [ ] **Step 1: 실패하는 테스트 작성**

`go/internal/store/news_test.go`:
```go
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
```

- [ ] **Step 2: 테스트 실행하여 실패 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/store/... -run "TestInsert|TestGetLatest" -v
```

Expected: `store.NewsItem undefined`

- [ ] **Step 3: db.go 스키마에 news 테이블 추가**

`go/internal/store/db.go`의 `schema` const를 아래로 교체:

```go
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
```

- [ ] **Step 4: news.go 구현**

`go/internal/store/news.go`:
```go
package store

import "database/sql"

type NewsItem struct {
	ID          string
	Ticker      string
	Title       string
	Link        string
	Publisher   string
	PublishedAt int64
	FetchedAt   string
}

func (db *DB) InsertNewsItem(n NewsItem) error {
	_, err := db.SQL.Exec(`
		INSERT OR IGNORE INTO news (id, ticker, title, link, publisher, published_at, fetched_at)
		VALUES (?,?,?,?,?,?,?)`,
		n.ID, n.Ticker, n.Title, n.Link, n.Publisher, n.PublishedAt, n.FetchedAt,
	)
	return err
}

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
```

- [ ] **Step 5: db_test.go에 news 테이블 확인 추가**

`go/internal/store/db_test.go`의 `TestOpen_CreatesSchema` 함수 끝(닫는 `}` 전)에 추가:

```go
	err = db.SQL.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='news'",
	).Scan(&name)
	if err != nil {
		t.Fatalf("news table not found: %v", err)
	}
```

- [ ] **Step 6: 전체 store 테스트 통과 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/store/... -v
```

Expected: 기존 테스트 포함 전체 PASS (TestOpen_CreatesSchema, TestInsertAndGetTradeByID 등 + 신규 4개)

- [ ] **Step 7: 커밋**

```bash
git add go/internal/store/news.go go/internal/store/news_test.go go/internal/store/db.go go/internal/store/db_test.go
git commit -m "feat: add news table schema and store CRUD"
```

---

## Task 2: Yahoo Finance 뉴스 크롤러

**Files:**
- Create: `go/internal/news/crawler.go`
- Create: `go/internal/news/crawler_test.go`

- [ ] **Step 1: 실패하는 테스트 작성**

`go/internal/news/crawler_test.go`:
```go
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
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/news/... -v
```

Expected: `cannot find package "annotate/internal/news"`

- [ ] **Step 3: crawler.go 구현**

`go/internal/news/crawler.go`:
```go
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
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/news/... -v
```

Expected: 4 tests PASS

- [ ] **Step 5: 커밋**

```bash
git add go/internal/news/crawler.go go/internal/news/crawler_test.go
git commit -m "feat: add Yahoo Finance news crawler with DB persistence"
```

---

## Task 3: POST /brief 엔드포인트 + 대시보드 UI

**Files:**
- Create: `go/internal/handler/templates/brief_result.html`
- Modify: `go/internal/handler/templates/dashboard.html`
- Modify: `go/internal/handler/handler.go`
- Modify: `go/internal/handler/handler_test.go`

- [ ] **Step 1: brief_result.html 작성**

`go/internal/handler/templates/brief_result.html`:
```html
<div style="margin-top:.75rem;padding:1rem;background:#fffbf0;border-radius:6px;border-left:3px solid #f39c12">
  <div style="font-size:.75rem;color:#666;margin-bottom:.75rem">📰 보유 종목 뉴스 브리핑 — {{.Date}}</div>
  {{if .Briefs}}
    {{range .Briefs}}
    <div style="margin-bottom:1.25rem">
      <div style="font-size:.8rem;font-weight:600;color:#333;margin-bottom:.3rem">{{.Name}} ({{.Ticker}})</div>
      <div style="font-size:.875rem;line-height:1.8;white-space:pre-wrap">{{.Summary}}</div>
    </div>
    {{end}}
  {{else}}
  <div style="color:#888;font-size:.875rem">보유 중인 종목이 없습니다.</div>
  {{end}}
  <div style="font-size:.7rem;color:#aaa;margin-top:.5rem">※ 이 브리핑은 투자 자문이 아닙니다.</div>
</div>
```

- [ ] **Step 2: dashboard.html에 뉴스 브리핑 섹션 추가**

`go/internal/handler/templates/dashboard.html`의 패턴 분석 `</div>` 닫는 태그 직후, `</div>` (container) 닫기 전에 삽입:

```html
  <div class="section">
    <h2>뉴스 브리핑</h2>
    <button class="btn btn-secondary"
      hx-post="/brief"
      hx-target="#brief-result"
      hx-swap="innerHTML"
      hx-indicator="#brief-ind">
      보유 종목 뉴스 브리핑
    </button>
    <span id="brief-ind" class="htmx-indicator"> 수집 중...</span>
    <div id="brief-result"></div>
  </div>
```

- [ ] **Step 3: 실패하는 핸들러 테스트 작성**

`go/internal/handler/handler_test.go`의 import 블록을 아래로 교체:

```go
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
```

`handler_test.go` 파일 끝에 추가:

```go
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
```

- [ ] **Step 4: 테스트 실패 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/handler/... -run TestBrief -v
```

Expected: `h.SetNewsBaseURL undefined` 또는 route not found

- [ ] **Step 5: handler.go 전체 교체**

`go/internal/handler/handler.go`:
```go
package handler

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"annotate/internal/analysis"
	"annotate/internal/market"
	"annotate/internal/news"
	"annotate/internal/store"
)

//go:embed templates
var templateFS embed.FS

type Handler struct {
	db            *store.DB
	apiKey        string
	claudeBaseURL string
	newsBaseURL   string
	tmpl          *template.Template
	mux           *http.ServeMux
}

func New(db *store.DB, apiKey string) *Handler {
	h := &Handler{
		db:     db,
		apiKey: apiKey,
	}
	h.tmpl = template.Must(template.ParseFS(templateFS, "templates/*.html"))
	h.mux = http.NewServeMux()
	h.mux.HandleFunc("GET /", h.dashboard)
	h.mux.HandleFunc("GET /trades", h.tradesList)
	h.mux.HandleFunc("POST /analyze/{id}", h.analyze)
	h.mux.HandleFunc("POST /review", h.review)
	h.mux.HandleFunc("POST /brief", h.brief)
	return h
}

func (h *Handler) SetClaudeBaseURL(url string) { h.claudeBaseURL = url }
func (h *Handler) SetNewsBaseURL(url string)   { h.newsBaseURL = url }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	trades, err := h.db.GetAllTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	holdings, _ := h.db.GetHoldingTrades()
	stats := analysis.ComputeStats(trades)

	avgPnL, avgHold := 0.0, 0.0
	if len(stats.ClosedTrades) > 0 {
		for _, ct := range stats.ClosedTrades {
			avgPnL += ct.PnLPct
			avgHold += float64(ct.HoldDays)
		}
		n := float64(len(stats.ClosedTrades))
		avgPnL /= n
		avgHold /= n
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "dashboard.html", map[string]any{
		"Holdings": holdings,
		"Stats":    stats,
		"AvgPnL":   avgPnL,
		"AvgHold":  avgHold,
	})
}

func (h *Handler) tradesList(w http.ResponseWriter, r *http.Request) {
	trades, err := h.db.GetAllTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "trades.html", map[string]any{
		"Trades": trades,
	})
}

func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	trade, err := h.db.GetTradeByID(id)
	if err != nil {
		http.Error(w, "trade not found", http.StatusNotFound)
		return
	}

	summary := h.fetchSummary(trade)
	prompt := analysis.BuildAnalysisPrompt(trade, summary)
	result, err := analysis.CallClaude(h.apiKey, prompt, h.claudeBaseURL)
	if err != nil {
		http.Error(w, "analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "analyze_result.html", map[string]any{
		"Trade":  trade,
		"Result": result,
	})
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	trades, err := h.db.GetAllTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stats := analysis.ComputeStats(trades)
	prompt := analysis.BuildReviewPrompt(trades, stats)
	result, err := analysis.CallClaude(h.apiKey, prompt, h.claudeBaseURL)
	if err != nil {
		http.Error(w, "review failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "review_result.html", map[string]any{
		"Result": result,
	})
}

type briefItem struct {
	Name    string
	Ticker  string
	Summary string
}

func (h *Handler) brief(w http.ResponseWriter, r *http.Request) {
	holdings, err := h.db.GetHoldingTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seen := map[string]bool{}
	var briefs []briefItem
	for _, t := range holdings {
		if seen[t.Ticker] {
			continue
		}
		seen[t.Ticker] = true

		fetchedAt, _ := h.db.GetLatestNewsFetchedAt(t.Ticker)
		if newsIsStale(fetchedAt) {
			_ = news.FetchAndStore(t.Ticker, h.db, h.newsBaseURL)
		}

		items, _ := h.db.GetNewsForTicker(t.Ticker, 5)
		summary := h.summarizeNews(t.Name, items)
		briefs = append(briefs, briefItem{Name: t.Name, Ticker: t.Ticker, Summary: summary})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "brief_result.html", map[string]any{
		"Briefs": briefs,
		"Date":   time.Now().Format("2006-01-02"),
	})
}

func newsIsStale(fetchedAt string) bool {
	if fetchedAt == "" {
		return true
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", fetchedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > time.Hour
}

func (h *Handler) summarizeNews(name string, items []store.NewsItem) string {
	if len(items) == 0 {
		return "최근 뉴스를 찾을 수 없습니다."
	}
	var lines string
	for _, item := range items {
		lines += "- " + item.Title + "\n"
	}
	prompt := fmt.Sprintf(
		"다음은 %s의 최근 뉴스 헤드라인입니다.\n투자 추천 없이, 사실 기반으로 3줄 이내로 요약하세요.\n\n%s\n형식:\n1. [핵심 내용 1]\n2. [핵심 내용 2]\n3. [핵심 내용 3]",
		name, lines,
	)
	result, err := analysis.CallClaude(h.apiKey, prompt, h.claudeBaseURL)
	if err != nil {
		return "요약 실패: " + err.Error()
	}
	return result
}

func (h *Handler) fetchSummary(trade store.Trade) market.Summary {
	t, err := time.Parse("2006-01-02", trade.Date)
	if err != nil {
		return market.Summary{}
	}
	start := t.AddDate(0, 0, -30).Format("2006-01-02")
	end := t.AddDate(0, 0, 5).Format("2006-01-02")

	rows, err := market.FetchOHLCV(trade.Ticker, trade.Market, start, end, "")
	if err != nil || len(rows) < 15 {
		return market.Summary{Signal: "데이터없음"}
	}
	indicators := market.CalculateIndicators(rows)
	summary, err := market.SummarizeOnDate(indicators, trade.Date)
	if err != nil {
		return market.Summary{Signal: "데이터없음"}
	}
	return summary
}

func newlineToBreak(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}
```

- [ ] **Step 6: 핸들러 테스트 통과 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/handler/... -v
```

Expected: 기존 5개 + TestBrief_NoHoldings + TestBrief_WithHolding = 7개 PASS

- [ ] **Step 7: 커밋**

```bash
git add go/internal/handler/handler.go go/internal/handler/handler_test.go go/internal/handler/templates/brief_result.html go/internal/handler/templates/dashboard.html
git commit -m "feat: add /brief endpoint with news fetch and Claude summarization"
```

---

## Task 4: 백그라운드 스케줄러

**Files:**
- Create: `go/internal/scheduler/jobs.go`
- Create: `go/internal/scheduler/jobs_test.go`
- Modify: `go/cmd/server/main.go`

- [ ] **Step 1: 실패하는 테스트 작성**

`go/internal/scheduler/jobs_test.go`:
```go
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
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/scheduler/... -v
```

Expected: `cannot find package "annotate/internal/scheduler"`

- [ ] **Step 3: jobs.go 구현**

`go/internal/scheduler/jobs.go`:
```go
package scheduler

import (
	"log"
	"time"

	"annotate/internal/news"
	"annotate/internal/store"
)

type Scheduler struct {
	db          *store.DB
	interval    time.Duration
	newsBaseURL string
	stop        chan struct{}
}

// New creates a scheduler that fetches news for holding stocks at the given interval.
// newsBaseURL is injectable for tests; pass "" to use the real Yahoo Finance endpoint.
func New(db *store.DB, interval time.Duration, newsBaseURL string) *Scheduler {
	return &Scheduler{
		db:          db,
		interval:    interval,
		newsBaseURL: newsBaseURL,
		stop:        make(chan struct{}),
	}
}

// Start launches the background goroutine. It performs an immediate fetch,
// then repeats at the configured interval.
func (s *Scheduler) Start() {
	go s.run()
}

// Stop signals the scheduler to exit. It returns immediately; the goroutine
// exits asynchronously after the current fetch completes.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) run() {
	s.fetchNews()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.fetchNews()
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) fetchNews() {
	holdings, err := s.db.GetHoldingTrades()
	if err != nil {
		log.Printf("[scheduler] get holdings: %v", err)
		return
	}
	seen := map[string]bool{}
	for _, t := range holdings {
		if seen[t.Ticker] {
			continue
		}
		seen[t.Ticker] = true
		if err := news.FetchAndStore(t.Ticker, s.db, s.newsBaseURL); err != nil {
			log.Printf("[scheduler] fetch news %s: %v", t.Ticker, err)
		} else {
			log.Printf("[scheduler] fetched news for %s", t.Ticker)
		}
	}
}
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd /Users/parkjungmin/annotate/go
go test ./internal/scheduler/... -v
```

Expected: 2 tests PASS

- [ ] **Step 5: server/main.go에 스케줄러 연동**

`go/cmd/server/main.go` 전체 교체:
```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"annotate/internal/handler"
	"annotate/internal/scheduler"
	"annotate/internal/store"
)

func main() {
	_, src, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(src), "..", "..", "..")

	dbPath := filepath.Join(root, "data", "annotate.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.SQL.Close()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "warning: ANTHROPIC_API_KEY not set; analysis endpoints will fail")
	}

	sched := scheduler.New(db, time.Hour, "")
	sched.Start()
	defer sched.Stop()

	h := handler.New(db, apiKey)
	addr := ":8080"
	fmt.Printf("Stock-Ops server listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, h))
}
```

- [ ] **Step 6: 전체 빌드 확인**

```bash
cd /Users/parkjungmin/annotate/go
go build ./...
```

Expected: 에러 없이 빌드 완료

- [ ] **Step 7: 커밋**

```bash
git add go/internal/scheduler/jobs.go go/internal/scheduler/jobs_test.go go/cmd/server/main.go
git commit -m "feat: add background scheduler for periodic news fetching"
```

---

## Task 5: 전체 통합 확인

**Files:** 없음 (검증만)

- [ ] **Step 1: 전체 테스트 실행**

```bash
cd /Users/parkjungmin/annotate/go
go test ./... -v
```

Expected: 모든 패키지 PASS (store, market, analysis, news, handler, scheduler)

- [ ] **Step 2: 서버 실행 + 뉴스 브리핑 동작 확인**

```bash
cd /Users/parkjungmin/annotate/go
ANTHROPIC_API_KEY=<your_key> go run ./cmd/server
```

브라우저에서 `http://localhost:8080` 접속 후:
1. 대시보드 하단에 "뉴스 브리핑" 섹션이 표시되는지 확인
2. "보유 종목 뉴스 브리핑" 버튼 클릭 → HTMX로 결과 로드 확인 (NVIDIA, 삼성전자 각 3줄 요약)
3. 서버 로그에 `[scheduler] fetched news for NVDA` 출력 확인

- [ ] **Step 3: 최종 커밋**

```bash
git add .
git commit -m "feat: Phase 3 complete — news crawler, scheduler, /brief endpoint"
```

---

## 검증 체크리스트

| 항목 | 확인 명령 |
|------|-----------|
| news 테이블 스키마 | `go test ./internal/store/... -run TestOpen_CreatesSchema` |
| News CRUD | `go test ./internal/store/... -run "TestInsert\|TestGetLatest"` |
| 뉴스 크롤러 | `go test ./internal/news/... -v` |
| `/brief` 엔드포인트 | `go test ./internal/handler/... -run TestBrief` |
| 스케줄러 | `go test ./internal/scheduler/... -v` |
| 전체 빌드 | `go build ./...` |
| 전체 테스트 | `go test ./... -v` |
| 실서버 동작 | 브라우저에서 뉴스 브리핑 버튼 클릭 확인 |
