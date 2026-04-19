package store_test

import (
	"path/filepath"
	"testing"

	"annotate/internal/store"
)

func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

var sampleTrade = store.Trade{
	ID:       "trade_001",
	Ticker:   "005930",
	Name:     "삼성전자",
	Market:   "KRX",
	Action:   "buy",
	Price:    72000,
	Quantity: 10,
	Date:     "2025-04-10",
	Time:     "10:23",
	Reason:   "AI 반도체 수요 증가",
	Status:   "holding",
}

func TestInsertAndGetTradeByID(t *testing.T) {
	db := openTestDB(t)
	if err := db.InsertTrade(sampleTrade); err != nil {
		t.Fatalf("InsertTrade: %v", err)
	}
	got, err := db.GetTradeByID("trade_001")
	if err != nil {
		t.Fatalf("GetTradeByID: %v", err)
	}
	if got.Name != "삼성전자" || got.Price != 72000 {
		t.Errorf("unexpected trade: %+v", got)
	}
}

func TestGetTradeByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := db.GetTradeByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing trade")
	}
}

func TestGetAllTrades(t *testing.T) {
	db := openTestDB(t)
	db.InsertTrade(sampleTrade)
	trades, err := db.GetAllTrades()
	if err != nil {
		t.Fatalf("GetAllTrades: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
}

func TestGetHoldingTrades(t *testing.T) {
	db := openTestDB(t)
	db.InsertTrade(sampleTrade)
	closed := sampleTrade
	closed.ID = "trade_002"
	closed.Status = "closed"
	db.InsertTrade(closed)

	holding, err := db.GetHoldingTrades()
	if err != nil {
		t.Fatalf("GetHoldingTrades: %v", err)
	}
	if len(holding) != 1 || holding[0].ID != "trade_001" {
		t.Errorf("unexpected holding trades: %+v", holding)
	}
}
