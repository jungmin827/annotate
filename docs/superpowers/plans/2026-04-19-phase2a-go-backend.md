# Stock-Ops Phase 2a: Go 코어 백엔드 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Python 기반 Phase 1 로직을 Go로 포팅하여 SQLite 데이터 레이어, Yahoo Finance HTTP 클라이언트, 기술적 지표 계산, Claude API 연동을 구현한다.

**Architecture:** `go/` 디렉토리 아래 독립 Go 모듈로 시작한다. 기존 Python 코드와 공존하며 `data/annotate.db`를 데이터 저장소로 사용한다. 외부 의존성은 SQLite 드라이버(`modernc.org/sqlite`) 하나만 사용하고 나머지는 표준 라이브러리로 구현한다.

**Tech Stack:** Go 1.22+, modernc.org/sqlite (pure Go, CGO 불필요), net/http, database/sql, encoding/json, testing + httptest

---

## 파일 구조

```
annotate/
└── go/
    ├── go.mod
    ├── go.sum
    ├── cmd/
    │   └── migrate/
    │       └── main.go          ← trades.json → SQLite 마이그레이션 CLI
    ├── internal/
    │   ├── store/
    │   │   ├── db.go            ← SQLite 연결, 스키마 초기화
    │   │   └── trades.go        ← 매매 CRUD + 조회
    │   ├── market/
    │   │   ├── fetcher.go       ← Yahoo Finance v8 API HTTP 클라이언트
    │   │   └── indicators.go    ← RSI(14), 거래량비율, 등락률 계산
    │   └── analysis/
    │       ├── engine.go        ← Claude API 호출 (net/http)
    │       └── pattern.go       ← 통계 집계 (PnL, 시장별 분포)
    └── internal/
        └── store/
            └── db_test.go       ← DB 테스트 (tmp 파일 사용)
```

실제 테스트 파일 위치:
```
go/
├── internal/store/db_test.go
├── internal/store/trades_test.go
├── internal/market/fetcher_test.go
├── internal/market/indicators_test.go
├── internal/analysis/pattern_test.go
└── internal/analysis/engine_test.go
```

---

## Task 1: Go 모듈 초기화

**Files:**
- Create: `go/go.mod`
- Create: `go/internal/store/db.go` (스켈레톤)

- [ ] **Step 1: go/ 디렉토리 생성 및 모듈 초기화**

```bash
mkdir -p go/cmd/migrate
mkdir -p go/internal/store
mkdir -p go/internal/market
mkdir -p go/internal/analysis
cd go
go mod init annotate
```

`go/go.mod` 결과:
```
module annotate

go 1.22
```

- [ ] **Step 2: modernc.org/sqlite 의존성 추가**

```bash
cd go
go get modernc.org/sqlite@v1.29.0
```

Expected: `go.sum` 생성, `go.mod`에 require 추가됨

- [ ] **Step 3: db.go 작성 (DB 타입 + Open + applySchema)**

`go/internal/store/db.go`:
```go
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps sql.DB with domain methods.
type DB struct {
	SQL *sql.DB // exported so tests can run raw queries when needed
}

// Open opens (or creates) the SQLite database at the given path and applies the schema.
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

// Close closes the underlying database.
func (db *DB) Close() error { return db.SQL.Close() }

func applySchema(db *sql.DB) error {
	_, err := db.Exec(`
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
`)
	return err
}
```

- [ ] **Step 4: db_test.go 작성 및 실행**

`go/internal/store/db_test.go`:
```go
package store_test

import (
	"os"
	"testing"

	"annotate/internal/store"
)

func TestOpen_CreatesSchema(t *testing.T) {
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	db, err := store.Open(f.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// trades 테이블 존재 확인 (db.SQL로 raw 쿼리 가능)
	var name string
	err = db.SQL.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='trades'`).Scan(&name)
	if err != nil {
		t.Fatalf("trades table not found: %v", err)
	}
	if name != "trades" {
		t.Errorf("got %q, want 'trades'", name)
	}

	// market_cache 테이블 존재 확인
	err = db.SQL.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='market_cache'`).Scan(&name)
	if err != nil {
		t.Fatalf("market_cache table not found: %v", err)
	}
}
```

```bash
cd go && go test ./internal/store/ -v -run TestOpen
```

Expected: `PASS`

- [ ] **Step 5: 커밋**

```bash
cd .. && git add go/
git commit -m "feat(go): initialize Go module with SQLite schema"
```

---

## Task 2: 매매 데이터 CRUD (trades.go)

**Files:**
- Create: `go/internal/store/trades.go`
- Create: `go/internal/store/trades_test.go`

- [ ] **Step 1: Trade 타입 정의 및 CRUD 실패 테스트 작성**

`go/internal/store/trades_test.go`:
```go
package store_test

import (
	"os"
	"testing"

	"annotate/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := store.Open(f.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsertAndGetTrade(t *testing.T) {
	db := openTestDB(t)

	trade := store.Trade{
		ID:       "test_001",
		Ticker:   "005930",
		Name:     "삼성전자",
		Market:   "KRX",
		Action:   "buy",
		Price:    70000,
		Quantity: 10,
		Date:     "2024-01-15",
		Reason:   "외국인 순매수",
		Status:   "holding",
	}

	if err := db.InsertTrade(trade); err != nil {
		t.Fatalf("InsertTrade: %v", err)
	}

	got, err := db.GetTradeByID("test_001")
	if err != nil {
		t.Fatalf("GetTradeByID: %v", err)
	}

	if got.Ticker != "005930" {
		t.Errorf("Ticker: got %q, want %q", got.Ticker, "005930")
	}
	if got.Price != 70000 {
		t.Errorf("Price: got %v, want 70000", got.Price)
	}
}

func TestGetAllTrades(t *testing.T) {
	db := openTestDB(t)

	trades := []store.Trade{
		{ID: "t1", Ticker: "005930", Name: "삼성전자", Market: "KRX", Action: "buy", Price: 70000, Quantity: 10, Date: "2024-01-15", Reason: "test", Status: "holding"},
		{ID: "t2", Ticker: "NVDA", Name: "NVIDIA", Market: "NASDAQ", Action: "buy", Price: 500.0, Quantity: 5, Date: "2024-02-01", Reason: "AI", Status: "holding"},
	}
	for _, tr := range trades {
		if err := db.InsertTrade(tr); err != nil {
			t.Fatal(err)
		}
	}

	all, err := db.GetAllTrades()
	if err != nil {
		t.Fatalf("GetAllTrades: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("got %d trades, want 2", len(all))
	}
}

func TestGetHoldingTrades(t *testing.T) {
	db := openTestDB(t)

	db.InsertTrade(store.Trade{ID: "h1", Ticker: "005930", Name: "삼성전자", Market: "KRX", Action: "buy", Price: 70000, Quantity: 10, Date: "2024-01-15", Reason: "r", Status: "holding"})
	db.InsertTrade(store.Trade{ID: "c1", Ticker: "NVDA", Name: "NVIDIA", Market: "NASDAQ", Action: "buy", Price: 500, Quantity: 5, Date: "2024-01-16", Reason: "r", Status: "closed"})

	holding, err := db.GetHoldingTrades()
	if err != nil {
		t.Fatalf("GetHoldingTrades: %v", err)
	}
	if len(holding) != 1 {
		t.Errorf("got %d holding, want 1", len(holding))
	}
	if holding[0].Status != "holding" {
		t.Errorf("status: got %q, want 'holding'", holding[0].Status)
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd go && go test ./internal/store/ -v -run TestInsert 2>&1 | head -20
```

Expected: `undefined: store.Trade` 또는 `undefined: store.DB`

- [ ] **Step 3: trades.go 구현**

`go/internal/store/trades.go`:
```go
package store

import (
	"database/sql"
	"fmt"
)

// Trade represents a single buy or sell record.
type Trade struct {
	ID           string
	Ticker       string
	Name         string
	Market       string
	Action       string
	Price        float64
	Quantity     int
	Date         string
	Time         string
	Reason       string
	Status       string
	LinkedSellID string
	LinkedBuyID  string
}

// InsertTrade inserts a new trade. Returns error if ID already exists.
func (db *DB) InsertTrade(t Trade) error {
	_, err := db.SQL.Exec(`
INSERT INTO trades (id, ticker, name, market, action, price, quantity, date, time, reason, status, linked_sell_id, linked_buy_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Ticker, t.Name, t.Market, t.Action, t.Price, t.Quantity,
		t.Date, nullString(t.Time), t.Reason, t.Status,
		nullString(t.LinkedSellID), nullString(t.LinkedBuyID),
	)
	if err != nil {
		return fmt.Errorf("InsertTrade: %w", err)
	}
	return nil
}

// GetTradeByID returns the trade with the given ID, or error if not found.
func (db *DB) GetTradeByID(id string) (Trade, error) {
	row := db.SQL.QueryRow(`SELECT id, ticker, name, market, action, price, quantity, date, COALESCE(time,''), reason, status, COALESCE(linked_sell_id,''), COALESCE(linked_buy_id,'') FROM trades WHERE id = ?`, id)
	return scanTrade(row)
}

// GetAllTrades returns all trades ordered by date.
func (db *DB) GetAllTrades() ([]Trade, error) {
	rows, err := db.SQL.Query(`SELECT id, ticker, name, market, action, price, quantity, date, COALESCE(time,''), reason, status, COALESCE(linked_sell_id,''), COALESCE(linked_buy_id,'') FROM trades ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrades(rows)
}

// GetHoldingTrades returns all trades with status='holding'.
func (db *DB) GetHoldingTrades() ([]Trade, error) {
	rows, err := db.SQL.Query(`SELECT id, ticker, name, market, action, price, quantity, date, COALESCE(time,''), reason, status, COALESCE(linked_sell_id,''), COALESCE(linked_buy_id,'') FROM trades WHERE status='holding' ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrades(rows)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTrade(s scanner) (Trade, error) {
	var t Trade
	err := s.Scan(&t.ID, &t.Ticker, &t.Name, &t.Market, &t.Action, &t.Price, &t.Quantity, &t.Date, &t.Time, &t.Reason, &t.Status, &t.LinkedSellID, &t.LinkedBuyID)
	return t, err
}

func scanTrades(rows *sql.Rows) ([]Trade, error) {
	var trades []Trade
	for rows.Next() {
		t, err := scanTrade(rows)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd go && go test ./internal/store/ -v
```

Expected: 4개 테스트 모두 PASS (`TestOpen_CreatesSchema`, `TestInsertAndGetTrade`, `TestGetAllTrades`, `TestGetHoldingTrades`)

- [ ] **Step 5: 커밋**

```bash
cd .. && git add go/
git commit -m "feat(go): add SQLite trade CRUD layer"
```

---

## Task 3: trades.json → SQLite 마이그레이션

**Files:**
- Create: `go/cmd/migrate/main.go`

- [ ] **Step 1: migrate/main.go 작성**

`go/cmd/migrate/main.go`:
```go
// migrate reads data/trades.json and writes all trades into data/annotate.db.
// Safe to run multiple times — duplicate IDs are skipped.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"annotate/internal/store"
)

type jsonTrades struct {
	Trades []jsonTrade `json:"trades"`
}

type jsonTrade struct {
	ID           string  `json:"id"`
	Ticker       string  `json:"ticker"`
	Name         string  `json:"name"`
	Market       string  `json:"market"`
	Action       string  `json:"action"`
	Price        float64 `json:"price"`
	Quantity     int     `json:"quantity"`
	Date         string  `json:"date"`
	Time         string  `json:"time"`
	Reason       string  `json:"reason"`
	Status       string  `json:"status"`
	LinkedSellID *string `json:"linked_sell_id"`
	LinkedBuyID  *string `json:"linked_buy_id"`
}

func main() {
	root := projectRoot()
	jsonPath := filepath.Join(root, "data", "trades.json")
	dbPath := filepath.Join(root, "data", "annotate.db")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Fatalf("read trades.json: %v", err)
	}

	var jt jsonTrades
	if err := json.Unmarshal(data, &jt); err != nil {
		log.Fatalf("parse trades.json: %v", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	inserted, skipped := 0, 0
	for _, jtr := range jt.Trades {
		t := store.Trade{
			ID:       jtr.ID,
			Ticker:   jtr.Ticker,
			Name:     jtr.Name,
			Market:   jtr.Market,
			Action:   jtr.Action,
			Price:    jtr.Price,
			Quantity: jtr.Quantity,
			Date:     jtr.Date,
			Time:     jtr.Time,
			Reason:   jtr.Reason,
			Status:   jtr.Status,
		}
		if jtr.LinkedSellID != nil {
			t.LinkedSellID = *jtr.LinkedSellID
		}
		if jtr.LinkedBuyID != nil {
			t.LinkedBuyID = *jtr.LinkedBuyID
		}

		if err := db.InsertTrade(t); err != nil {
			// UNIQUE constraint 위반 = 이미 존재 → skip
			skipped++
			continue
		}
		inserted++
	}

	fmt.Printf("마이그레이션 완료: %d건 추가, %d건 건너뜀\n", inserted, skipped)
}

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	// go/cmd/migrate/main.go → 3단계 위가 루트
	return filepath.Join(filepath.Dir(filename), "..", "..", "..")
}
```

- [ ] **Step 2: 마이그레이션 실행 확인**

```bash
cd go && go run ./cmd/migrate/
```

Expected:
```
마이그레이션 완료: 2건 추가, 0건 건너뜀
```

- [ ] **Step 3: 재실행 시 중복 건너뜀 확인**

```bash
cd go && go run ./cmd/migrate/
```

Expected:
```
마이그레이션 완료: 0건 추가, 2건 건너뜀
```

- [ ] **Step 4: 커밋**

```bash
cd .. && git add go/cmd/migrate/
git commit -m "feat(go): add JSON to SQLite migration tool"
```

---

## Task 4: Yahoo Finance HTTP 클라이언트 (fetcher.go)

**Files:**
- Create: `go/internal/market/fetcher.go`
- Create: `go/internal/market/fetcher_test.go`

- [ ] **Step 1: OHLCV 타입 및 실패 테스트 작성**

`go/internal/market/fetcher_test.go`:
```go
package market_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"annotate/internal/market"
)

// Yahoo Finance v8 응답 최소 구조 모킹
func makeYFResponse(closes []float64, timestamps []int64) []byte {
	type result struct {
		Timestamp  []int64 `json:"timestamp"`
		Indicators struct {
			Quote []struct {
				Open   []float64 `json:"open"`
				High   []float64 `json:"high"`
				Low    []float64 `json:"low"`
				Close  []float64 `json:"close"`
				Volume []int64   `json:"volume"`
			} `json:"quote"`
		} `json:"indicators"`
	}
	type chart struct {
		Result []result `json:"result"`
	}
	type response struct {
		Chart chart `json:"chart"`
	}

	opens := make([]float64, len(closes))
	highs := make([]float64, len(closes))
	lows := make([]float64, len(closes))
	volumes := make([]int64, len(closes))
	for i, c := range closes {
		opens[i] = c - 100
		highs[i] = c + 200
		lows[i] = c - 200
		volumes[i] = 1000000
	}

	r := response{Chart: chart{Result: []result{{
		Timestamp: timestamps,
		Indicators: struct {
			Quote []struct {
				Open   []float64 `json:"open"`
				High   []float64 `json:"high"`
				Low    []float64 `json:"low"`
				Close  []float64 `json:"close"`
				Volume []int64   `json:"volume"`
			} `json:"quote"`
		}{Quote: []struct {
			Open   []float64 `json:"open"`
			High   []float64 `json:"high"`
			Low    []float64 `json:"low"`
			Close  []float64 `json:"close"`
			Volume []int64   `json:"volume"`
		}{{Open: opens, High: highs, Low: lows, Close: closes, Volume: volumes}}},
	}}}}

	b, _ := json.Marshal(r)
	return b
}

func TestFetchOHLCV_ReturnsRows(t *testing.T) {
	closes := []float64{70000, 71000, 72000, 73000, 74000}
	timestamps := []int64{1704672000, 1704758400, 1704844800, 1704931200, 1705017600}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeYFResponse(closes, timestamps))
	}))
	defer srv.Close()

	rows, err := market.FetchOHLCV("005930.KS", "2024-01-08", "2024-01-12", srv.URL)
	if err != nil {
		t.Fatalf("FetchOHLCV: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("got %d rows, want 5", len(rows))
	}
	if rows[0].Close != 70000 {
		t.Errorf("Close[0]: got %v, want 70000", rows[0].Close)
	}
}

func TestNormalizeTicker_KRX(t *testing.T) {
	got := market.NormalizeTicker("005930", "KRX")
	if got != "005930.KS" {
		t.Errorf("got %q, want '005930.KS'", got)
	}
}

func TestNormalizeTicker_NASDAQ(t *testing.T) {
	got := market.NormalizeTicker("NVDA", "NASDAQ")
	if got != "NVDA" {
		t.Errorf("got %q, want 'NVDA'", got)
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd go && go test ./internal/market/ -v -run TestFetch 2>&1 | head -10
```

Expected: `undefined: market.FetchOHLCV`

- [ ] **Step 3: fetcher.go 구현**

`go/internal/market/fetcher.go`:
```go
// Package market provides Yahoo Finance OHLCV data fetching.
package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OHLCV represents one day of price data.
type OHLCV struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// NormalizeTicker converts a raw ticker to Yahoo Finance format.
// KRX tickers get ".KS" suffix; others are returned as-is.
func NormalizeTicker(ticker, market string) string {
	if market == "KRX" {
		return ticker + ".KS"
	}
	return ticker
}

// yahooBase is the Yahoo Finance v8 chart endpoint base URL.
// Override in tests by passing a custom baseURL.
const yahooBase = "https://query1.finance.yahoo.com"

// FetchOHLCV fetches daily OHLCV data from Yahoo Finance.
// baseURL can be overridden in tests (pass "" to use default).
func FetchOHLCV(yfTicker, startDate, endDate, baseURL string) ([]OHLCV, error) {
	if baseURL == "" {
		baseURL = yahooBase
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("parse start: %w", err)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, fmt.Errorf("parse end: %w", err)
	}

	url := fmt.Sprintf(
		"%s/v8/finance/chart/%s?interval=1d&period1=%d&period2=%d&events=history",
		baseURL, yfTicker, start.Unix(), end.Unix(),
	)

	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parseYFResponse(body)
}

type yfResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func parseYFResponse(body []byte) ([]OHLCV, error) {
	var yr yfResponse
	if err := json.Unmarshal(body, &yr); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if yr.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo error %s: %s", yr.Chart.Error.Code, yr.Chart.Error.Description)
	}
	if len(yr.Chart.Result) == 0 {
		return nil, fmt.Errorf("no result in response")
	}

	res := yr.Chart.Result[0]
	if len(res.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no quote data")
	}
	q := res.Indicators.Quote[0]
	n := len(res.Timestamp)

	rows := make([]OHLCV, 0, n)
	for i := 0; i < n; i++ {
		if i >= len(q.Close) || q.Close[i] == 0 {
			continue
		}
		date := time.Unix(res.Timestamp[i], 0).UTC().Format("2006-01-02")
		rows = append(rows, OHLCV{
			Date:   date,
			Open:   safeIdx(q.Open, i),
			High:   safeIdx(q.High, i),
			Low:    safeIdx(q.Low, i),
			Close:  q.Close[i],
			Volume: safeIdxInt(q.Volume, i),
		})
	}
	return rows, nil
}

func safeIdx(s []float64, i int) float64 {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func safeIdxInt(s []int64, i int) int64 {
	if i < len(s) {
		return s[i]
	}
	return 0
}
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd go && go test ./internal/market/ -v -run TestFetch
cd go && go test ./internal/market/ -v -run TestNormalize
```

Expected: 3개 모두 PASS

- [ ] **Step 5: 커밋**

```bash
cd .. && git add go/internal/market/fetcher.go go/internal/market/fetcher_test.go
git commit -m "feat(go): add Yahoo Finance OHLCV HTTP client"
```

---

## Task 5: 기술적 지표 계산 (indicators.go)

**Files:**
- Create: `go/internal/market/indicators.go`
- Create: `go/internal/market/indicators_test.go`

- [ ] **Step 1: 실패 테스트 작성**

`go/internal/market/indicators_test.go`:
```go
package market_test

import (
	"testing"

	"annotate/internal/market"
)

func makeSampleOHLCV() []market.OHLCV {
	closes := []float64{70000, 71000, 72000, 71500, 73000, 74000, 73500, 75000, 74500, 76000}
	rows := make([]market.OHLCV, len(closes))
	for i, c := range closes {
		rows[i] = market.OHLCV{
			Date:   fmt.Sprintf("2024-01-%02d", i+10),
			Open:   c - 100,
			High:   c + 200,
			Low:    c - 200,
			Close:  c,
			Volume: 1_000_000,
		}
	}
	return rows
}

func TestCalculateIndicators_HasRequiredFields(t *testing.T) {
	rows := makeSampleOHLCV()
	result := market.CalculateIndicators(rows)

	for _, r := range result {
		// pct_change와 volume_ratio는 모든 행에 존재해야 함 (일부는 NaN)
		_ = r.PctChange
		_ = r.VolumeRatio
		_ = r.RSI14
	}
	// RSI는 최소 period+1개 데이터 후 유효
	if len(result) != len(rows) {
		t.Errorf("result length %d != input length %d", len(result), len(rows))
	}
}

func TestRSI_Range(t *testing.T) {
	rows := makeSampleOHLCV()
	result := market.CalculateIndicators(rows)

	for i, r := range result {
		if r.RSI14 == 0 {
			continue // RSI 계산 전 구간 (NaN으로 처리)
		}
		if r.RSI14 < 0 || r.RSI14 > 100 {
			t.Errorf("row %d: RSI14=%v out of [0,100]", i, r.RSI14)
		}
	}
}

func TestVolumeRatio_FirstFourAreZero(t *testing.T) {
	rows := makeSampleOHLCV()
	result := market.CalculateIndicators(rows)

	// 처음 4개는 5일 평균 계산 불가 → 0
	for i := 0; i < 4; i++ {
		if result[i].VolumeRatio != 0 {
			t.Errorf("row %d: VolumeRatio should be 0, got %v", i, result[i].VolumeRatio)
		}
	}
	// 5번째(인덱스 4)부터 유효
	if result[4].VolumeRatio == 0 {
		t.Errorf("row 4: VolumeRatio should be non-zero")
	}
}

func TestSummarizeOnDate_ReturnsClosestDate(t *testing.T) {
	rows := makeSampleOHLCV()
	result := market.CalculateIndicators(rows)

	summary := market.SummarizeOnDate(result, "2024-01-14")
	if summary.Date == "" {
		t.Error("SummarizeOnDate returned empty Date")
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd go && go test ./internal/market/ -v -run TestRSI 2>&1 | head -10
```

Expected: `undefined: market.CalculateIndicators`

- [ ] **Step 3: indicators.go 구현**

`go/internal/market/indicators.go`:
```go
package market

import "fmt"

// WithIndicators extends OHLCV with computed technical indicators.
type WithIndicators struct {
	OHLCV
	RSI14       float64 // 0 means not yet calculable
	PctChange   float64 // % change from previous close
	VolumeRatio float64 // volume / 5-day avg volume; 0 means insufficient data
	Cumulative5d float64 // 5-day cumulative return %
}

// Summary is a human-readable snapshot of a single day's indicators.
type Summary struct {
	Date         string
	Close        float64
	RSI14        float64
	PctChange    float64
	VolumeRatio  float64
	Cumulative5d float64
	Signal       string // e.g. "과매수(RSI>70), 급등추격(+3%↑)"
}

// CalculateIndicators computes RSI(14), pct_change, volume_ratio, cumulative_5d
// for every row. Rows where data is insufficient are set to zero.
func CalculateIndicators(rows []OHLCV) []WithIndicators {
	n := len(rows)
	result := make([]WithIndicators, n)
	for i, r := range rows {
		result[i].OHLCV = r
	}

	// pct_change
	for i := 1; i < n; i++ {
		prev := rows[i-1].Close
		if prev != 0 {
			result[i].PctChange = (rows[i].Close - prev) / prev * 100
		}
	}

	// cumulative_5d
	for i := 5; i < n; i++ {
		prev := rows[i-5].Close
		if prev != 0 {
			result[i].Cumulative5d = (rows[i].Close - prev) / prev * 100
		}
	}

	// volume_ratio (5일 이동평균)
	for i := 4; i < n; i++ {
		var sum float64
		for j := i - 4; j <= i; j++ {
			sum += float64(rows[j].Volume)
		}
		avg := sum / 5
		if avg != 0 {
			result[i].VolumeRatio = float64(rows[i].Volume) / avg
		}
	}

	// RSI(14) — Wilder's smoothing
	if n > 14 {
		gains := make([]float64, n)
		losses := make([]float64, n)
		for i := 1; i < n; i++ {
			ch := rows[i].Close - rows[i-1].Close
			if ch > 0 {
				gains[i] = ch
			} else {
				losses[i] = -ch
			}
		}
		var avgG, avgL float64
		for i := 1; i <= 14; i++ {
			avgG += gains[i]
			avgL += losses[i]
		}
		avgG /= 14
		avgL /= 14
		result[14].RSI14 = wilder(avgG, avgL)

		for i := 15; i < n; i++ {
			avgG = (avgG*13 + gains[i]) / 14
			avgL = (avgL*13 + losses[i]) / 14
			result[i].RSI14 = wilder(avgG, avgL)
		}
	}

	return result
}

func wilder(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// SummarizeOnDate finds the row closest to targetDate and returns a Summary.
func SummarizeOnDate(rows []WithIndicators, targetDate string) Summary {
	if len(rows) == 0 {
		return Summary{}
	}
	best := 0
	for i, r := range rows {
		if r.Date == targetDate {
			best = i
			break
		}
		// 날짜 문자열 비교 (YYYY-MM-DD는 사전순 = 날짜순)
		if abs(r.Date, targetDate) < abs(rows[best].Date, targetDate) {
			best = i
		}
	}
	r := rows[best]

	var signals []string
	if r.RSI14 > 70 {
		signals = append(signals, "과매수(RSI>70)")
	}
	if r.PctChange > 3 {
		signals = append(signals, "급등추격(+3%↑)")
	}
	if r.VolumeRatio > 3 {
		signals = append(signals, "거래량급증(3배↑)")
	}
	signal := "특이사항없음"
	if len(signals) > 0 {
		signal = joinSignals(signals)
	}

	return Summary{
		Date:         r.Date,
		Close:        r.Close,
		RSI14:        round2(r.RSI14),
		PctChange:    round2(r.PctChange),
		VolumeRatio:  round2(r.VolumeRatio),
		Cumulative5d: round2(r.Cumulative5d),
		Signal:       signal,
	}
}

func abs(a, b string) string {
	if a < b {
		return b
	}
	return a
}

func joinSignals(ss []string) string {
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}

func round2(f float64) float64 {
	return float64(int(f*100)) / 100
}

// Sprintf helper for test file
var _ = fmt.Sprintf
```

테스트 파일에서 `fmt` import가 필요하므로 수정:

`go/internal/market/indicators_test.go` 상단:
```go
package market_test

import (
	"fmt"
	"testing"

	"annotate/internal/market"
)
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd go && go test ./internal/market/ -v
```

Expected: 6개 모두 PASS

- [ ] **Step 5: 커밋**

```bash
cd .. && git add go/internal/market/indicators.go go/internal/market/indicators_test.go
git commit -m "feat(go): add RSI/volume/pct-change indicator calculations"
```

---

## Task 6: 패턴 통계 집계 (pattern.go)

**Files:**
- Create: `go/internal/analysis/pattern.go`
- Create: `go/internal/analysis/pattern_test.go`

- [ ] **Step 1: 실패 테스트 작성**

`go/internal/analysis/pattern_test.go`:
```go
package analysis_test

import (
	"testing"

	"annotate/internal/analysis"
	"annotate/internal/store"
)

func sampleTrades() []store.Trade {
	return []store.Trade{
		{ID: "t1", Ticker: "005930", Name: "삼성전자", Market: "KRX", Action: "buy", Price: 70000, Quantity: 10, Date: "2024-01-15", Reason: "외국인 순매수", Status: "closed", LinkedSellID: "t2"},
		{ID: "t2", Ticker: "005930", Name: "삼성전자", Market: "KRX", Action: "sell", Price: 77000, Quantity: 10, Date: "2024-02-20", Reason: "목표가 달성", Status: "closed", LinkedBuyID: "t1"},
		{ID: "t3", Ticker: "NVDA", Name: "NVIDIA", Market: "NASDAQ", Action: "buy", Price: 500.0, Quantity: 5, Date: "2024-03-01", Reason: "AI 수혜주", Status: "holding"},
	}
}

func TestComputeStats_Basic(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades())
	if stats.TotalTrades != 3 {
		t.Errorf("TotalTrades: got %d, want 3", stats.TotalTrades)
	}
	if stats.BuyCount != 2 {
		t.Errorf("BuyCount: got %d, want 2", stats.BuyCount)
	}
	if stats.HoldingCount != 1 {
		t.Errorf("HoldingCount: got %d, want 1", stats.HoldingCount)
	}
}

func TestComputeStats_PnL(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades())
	if len(stats.ClosedTrades) != 1 {
		t.Fatalf("ClosedTrades: got %d, want 1", len(stats.ClosedTrades))
	}
	pnl := stats.ClosedTrades[0].PnLPct
	if pnl < 9.9 || pnl > 10.1 {
		t.Errorf("PnLPct: got %.2f, want ~10.0", pnl)
	}
}

func TestComputeStats_HoldDays(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades())
	holdDays := stats.ClosedTrades[0].HoldDays
	// 2024-01-15 → 2024-02-20 = 36일
	if holdDays != 36 {
		t.Errorf("HoldDays: got %d, want 36", holdDays)
	}
}

func TestComputeStats_Markets(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades())
	if stats.Markets["KRX"] != 1 {
		t.Errorf("Markets[KRX]: got %d, want 1", stats.Markets["KRX"])
	}
	if stats.Markets["NASDAQ"] != 1 {
		t.Errorf("Markets[NASDAQ]: got %d, want 1", stats.Markets["NASDAQ"])
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd go && go test ./internal/analysis/ -v -run TestCompute 2>&1 | head -10
```

Expected: `undefined: analysis.ComputeStats`

- [ ] **Step 3: pattern.go 구현**

`go/internal/analysis/pattern.go`:
```go
// Package analysis provides trade statistics and Claude API integration.
package analysis

import (
	"time"

	"annotate/internal/store"
)

// ClosedTrade represents a matched buy→sell pair with computed PnL.
type ClosedTrade struct {
	BuyID    string
	SellID   string
	Ticker   string
	PnLPct   float64
	HoldDays int
}

// Stats is the aggregate summary of all trades.
type Stats struct {
	TotalTrades  int
	BuyCount     int
	SellCount    int
	HoldingCount int
	ClosedTrades []ClosedTrade
	Markets      map[string]int // market → buy count
}

// ComputeStats aggregates trades into Stats.
func ComputeStats(trades []store.Trade) Stats {
	buyMap := map[string]store.Trade{}
	for _, t := range trades {
		if t.Action == "buy" {
			buyMap[t.ID] = t
		}
	}

	var closed []ClosedTrade
	for _, t := range trades {
		if t.Action == "sell" && t.LinkedBuyID != "" {
			if buy, ok := buyMap[t.LinkedBuyID]; ok {
				pnl := (t.Price - buy.Price) / buy.Price * 100
				buyDate, _ := time.Parse("2006-01-02", buy.Date)
				sellDate, _ := time.Parse("2006-01-02", t.Date)
				hold := int(sellDate.Sub(buyDate).Hours() / 24)
				closed = append(closed, ClosedTrade{
					BuyID:    buy.ID,
					SellID:   t.ID,
					Ticker:   t.Ticker,
					PnLPct:   round2(pnl),
					HoldDays: hold,
				})
			}
		}
	}

	markets := map[string]int{}
	buyCount, sellCount, holdCount := 0, 0, 0
	for _, t := range trades {
		switch t.Action {
		case "buy":
			buyCount++
			markets[t.Market]++
		case "sell":
			sellCount++
		}
		if t.Status == "holding" {
			holdCount++
		}
	}

	return Stats{
		TotalTrades:  len(trades),
		BuyCount:     buyCount,
		SellCount:    sellCount,
		HoldingCount: holdCount,
		ClosedTrades: closed,
		Markets:      markets,
	}
}

func round2(f float64) float64 {
	return float64(int(f*100)) / 100
}
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd go && go test ./internal/analysis/ -v -run TestCompute
```

Expected: 4개 PASS

- [ ] **Step 5: 커밋**

```bash
cd .. && git add go/internal/analysis/pattern.go go/internal/analysis/pattern_test.go
git commit -m "feat(go): add trade statistics aggregation"
```

---

## Task 7: Claude API 연동 (engine.go)

**Files:**
- Create: `go/internal/analysis/engine.go`
- Create: `go/internal/analysis/engine_test.go`

- [ ] **Step 1: 실패 테스트 작성**

`go/internal/analysis/engine_test.go`:
```go
package analysis_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"annotate/internal/analysis"
	"annotate/internal/market"
	"annotate/internal/store"
)

func makeMockClaude(responseText string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": responseText},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

var sampleTrade = store.Trade{
	ID:       "test_001",
	Ticker:   "005930",
	Name:     "삼성전자",
	Market:   "KRX",
	Action:   "buy",
	Price:    70000,
	Quantity: 10,
	Date:     "2024-01-15",
	Reason:   "외국인 순매수",
	Status:   "holding",
}

var sampleSummary = market.Summary{
	Date:        "2024-01-15",
	Close:       70000,
	RSI14:       62.3,
	PctChange:   0.8,
	VolumeRatio: 1.2,
	Signal:      "특이사항없음",
}

func TestCallClaude_ReturnsText(t *testing.T) {
	srv := makeMockClaude("근거 충분. RSI 62.3은 정상 구간입니다.")
	defer srv.Close()

	result, err := analysis.CallClaude(
		"test-api-key",
		"claude-sonnet-4-6",
		"분석 프롬프트 테스트",
		srv.URL,
	)
	if err != nil {
		t.Fatalf("CallClaude: %v", err)
	}
	if result == "" {
		t.Error("CallClaude returned empty string")
	}
}

func TestBuildAnalysisPrompt_NoBuySellAdvice(t *testing.T) {
	prompt := analysis.BuildAnalysisPrompt(sampleTrade, sampleSummary, []string{})
	forbidden := []string{"사세요", "파세요", "반드시", "무조건"}
	for _, f := range forbidden {
		if contains(prompt, f) {
			t.Errorf("prompt contains forbidden phrase: %q", f)
		}
	}
}

func TestBuildAnalysisPrompt_ContainsTradeInfo(t *testing.T) {
	prompt := analysis.BuildAnalysisPrompt(sampleTrade, sampleSummary, []string{})
	for _, expect := range []string{"005930", "삼성전자", "62.3", "외국인 순매수"} {
		if !contains(prompt, expect) {
			t.Errorf("prompt missing %q", expect)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: 테스트 실패 확인**

```bash
cd go && go test ./internal/analysis/ -v -run TestCallClaude 2>&1 | head -10
```

Expected: `undefined: analysis.CallClaude`

- [ ] **Step 3: engine.go 구현**

`go/internal/analysis/engine.go`:
```go
package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"annotate/internal/market"
	"annotate/internal/store"
)

// anthropicBase is the Anthropic API endpoint.
const anthropicBase = "https://api.anthropic.com"

// CallClaude sends a prompt to the Claude API and returns the response text.
// baseURL overrides the API endpoint (use "" for production, test server URL for tests).
func CallClaude(apiKey, model, prompt, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = anthropicBase
	}

	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	})

	req, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("claude error: %s", result.Error.Message)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty content from claude")
	}
	return result.Content[0].Text, nil
}

// BuildAnalysisPrompt constructs the analysis prompt for a single trade.
func BuildAnalysisPrompt(trade store.Trade, summary market.Summary, principles []string) string {
	ptext := "(원칙 없음)"
	if len(principles) > 0 {
		ptext = ""
		for _, p := range principles {
			ptext += "- " + p + "\n"
		}
	}

	return fmt.Sprintf(`당신은 투자자의 매매 기록을 분석하는 파트너입니다.
매수/매도 추천은 절대 하지 않습니다. 과거 데이터를 바탕으로 기술적 상태와 매수 근거의 타당성만 평가합니다.

## 매매 정보
- 종목: %s (%s, %s)
- 행동: %s
- 가격: %.0f
- 날짜: %s
- 매수 이유: %s

## 매수 시점 기술적 상태
- 종가: %.0f
- RSI(14): %.1f
- 전일 대비 등락률: %.2f%%
- 거래량/5일평균: %.2f배
- 신호: %s

## 투자자 원칙
%s

## 분석 요청
1. 매수 시점의 기술적 상태를 평가하세요.
2. 매수 이유가 실제 시장 상황과 일치하는지 평가하세요.
3. 근거 타당성을 "근거 충분", "근거 약함", "근거 부족" 중 하나로 판단하세요.
4. 이 매매에서 개선할 수 있는 점을 1~2문장으로 제시하세요.

확정적 예측이나 매수/매도 추천 표현은 사용하지 마세요.`,
		trade.Name, trade.Ticker, trade.Market,
		trade.Action, trade.Price, trade.Date, trade.Reason,
		summary.Close, summary.RSI14, summary.PctChange, summary.VolumeRatio, summary.Signal,
		ptext,
	)
}

// BuildReviewPrompt constructs the pattern analysis prompt.
func BuildReviewPrompt(trades []store.Trade, stats Stats, principles []string) string {
	var reasons string
	for _, t := range trades {
		if t.Action == "buy" && t.Reason != "" {
			reasons += "- " + t.Reason + "\n"
		}
	}

	ptext := "(원칙 없음)"
	if len(principles) > 0 {
		ptext = ""
		for _, p := range principles {
			ptext += "- " + p + "\n"
		}
	}

	pnlSummary := "완료된 매매 없음"
	if len(stats.ClosedTrades) > 0 {
		var totalPnL float64
		var totalDays int
		for _, c := range stats.ClosedTrades {
			totalPnL += c.PnLPct
			totalDays += c.HoldDays
		}
		n := float64(len(stats.ClosedTrades))
		pnlSummary = fmt.Sprintf("평균 수익률: %.1f%%, 평균 보유기간: %.0f일", totalPnL/n, float64(totalDays)/n)
	}

	return fmt.Sprintf(`당신은 투자자의 매매 패턴을 분석하는 파트너입니다.
매수/매도 추천은 절대 하지 않습니다.

## 전체 매매 통계
- 총 매매: %d건 (매수 %d건, 매도 %d건)
- 현재 보유: %d건
- 완료 건 성과: %s

## 매수 이유 전체 목록
%s

## 투자자 원칙
%s

## 패턴 분석 요청
3~5개의 패턴 문장을 작성하세요: "패턴상 당신은 [행동]합니다. [N건 중 N건에서 확인됨]"

확정적 예측 표현은 사용하지 마세요. 관찰된 패턴만 기술하세요.`,
		stats.TotalTrades, stats.BuyCount, stats.SellCount, stats.HoldingCount, pnlSummary,
		reasons, ptext,
	)
}
```

- [ ] **Step 4: 테스트 통과 확인**

```bash
cd go && go test ./internal/analysis/ -v
```

Expected: 6개 모두 PASS

- [ ] **Step 5: 커밋**

```bash
cd .. && git add go/internal/analysis/engine.go go/internal/analysis/engine_test.go
git commit -m "feat(go): add Claude API client and prompt builders"
```

---

## Task 8: 전체 통합 테스트 및 빌드 확인

- [ ] **Step 1: 전체 Go 테스트 실행**

```bash
cd go && go test ./... -v 2>&1 | tail -30
```

Expected: 모든 패키지 PASS, 실패 없음

- [ ] **Step 2: 빌드 확인**

```bash
cd go && go build ./...
```

Expected: 오류 없이 완료

- [ ] **Step 3: 마이그레이션 CLI 최종 확인**

```bash
cd go && go run ./cmd/migrate/ && ls -la ../data/annotate.db
```

Expected: `annotate.db` 파일 생성, trades 데이터 포함

- [ ] **Step 4: 최종 커밋**

```bash
cd .. && git add go/
git commit -m "feat(go): complete Phase 2a Go backend core

- SQLite store: trades CRUD, market cache schema
- Migration: trades.json → annotate.db
- Market: Yahoo Finance v8 HTTP client with mock test
- Indicators: RSI(14), volume_ratio, pct_change (pure Go)
- Analysis: Claude API client + prompt builders (analysis/why/review)
- Pattern: PnL statistics, market distribution"
```

---

## 검증 체크리스트

| 항목 | 검증 방법 |
|------|-----------|
| Go 모듈 | `cd go && go build ./...` |
| SQLite 스키마 | `go test ./internal/store/ -v` |
| Yahoo Finance 파싱 | `go test ./internal/market/ -v -run TestFetch` |
| RSI 범위 | `go test ./internal/market/ -v -run TestRSI` |
| PnL 계산 | `go test ./internal/analysis/ -v -run TestCompute` |
| Claude API | `go test ./internal/analysis/ -v -run TestCallClaude` |
| 전체 | `go test ./... -v` |
| 마이그레이션 | `go run ./cmd/migrate/` → `data/annotate.db` 생성 확인 |

---

## Phase 2b 예고

Phase 2a 완료 후 Phase 2b (웹 UI)를 별도 계획으로 진행:
- `cmd/server/main.go` — Go HTTP 서버 (`:8080`)
- `internal/web/handler.go` — 대시보드, 분석, 리뷰 핸들러
- `internal/web/templates/` — HTMX + Chart.js 템플릿
- 기존 Python `streamlit run dashboard/app.py` 대체
