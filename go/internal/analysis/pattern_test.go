package analysis_test

import (
	"testing"

	"annotate/internal/analysis"
	"annotate/internal/store"
)

var sampleTrades = []store.Trade{
	{ID: "t1", Ticker: "005930", Name: "삼성전자", Market: "KRX", Action: "buy", Price: 70000, Quantity: 10, Date: "2024-01-15", Status: "closed", LinkedSellID: "t2"},
	{ID: "t2", Ticker: "005930", Name: "삼성전자", Market: "KRX", Action: "sell", Price: 77000, Quantity: 10, Date: "2024-02-20", Status: "closed", LinkedBuyID: "t1"},
	{ID: "t3", Ticker: "NVDA", Name: "NVIDIA", Market: "NASDAQ", Action: "buy", Price: 500, Quantity: 5, Date: "2024-03-01", Status: "holding"},
}

func TestComputeStats_Counts(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades)
	if stats.TotalTrades != 3 {
		t.Errorf("TotalTrades=%d want 3", stats.TotalTrades)
	}
	if stats.BuyCount != 2 {
		t.Errorf("BuyCount=%d want 2", stats.BuyCount)
	}
	if stats.SellCount != 1 {
		t.Errorf("SellCount=%d want 1", stats.SellCount)
	}
	if stats.HoldingCount != 1 {
		t.Errorf("HoldingCount=%d want 1", stats.HoldingCount)
	}
}

func TestComputeStats_PnL(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades)
	if len(stats.ClosedTrades) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(stats.ClosedTrades))
	}
	ct := stats.ClosedTrades[0]
	if ct.Ticker != "005930" {
		t.Errorf("ticker=%s want 005930", ct.Ticker)
	}
	// (77000-70000)/70000 * 100 = 10%
	if ct.PnLPct < 9.9 || ct.PnLPct > 10.1 {
		t.Errorf("PnLPct=%v want ~10", ct.PnLPct)
	}
	if ct.HoldDays != 36 {
		t.Errorf("HoldDays=%d want 36", ct.HoldDays)
	}
}

func TestComputeStats_Markets(t *testing.T) {
	stats := analysis.ComputeStats(sampleTrades)
	if stats.Markets["KRX"] != 1 {
		t.Errorf("KRX count=%d want 1", stats.Markets["KRX"])
	}
	if stats.Markets["NASDAQ"] != 1 {
		t.Errorf("NASDAQ count=%d want 1", stats.Markets["NASDAQ"])
	}
}
