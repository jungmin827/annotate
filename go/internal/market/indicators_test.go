package market_test

import (
	"math"
	"testing"

	"annotate/internal/market"
)

func makeSampleRows() []market.OHLCV {
	closes := []float64{70000, 71000, 72000, 71500, 73000, 74000, 73500, 75000, 74500, 76000, 75500, 77000, 76500, 78000, 77500}
	volumes := []float64{1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6, 1e6}
	rows := make([]market.OHLCV, len(closes))
	for i, c := range closes {
		rows[i] = market.OHLCV{Close: c, Volume: volumes[i]}
	}
	return rows
}

func TestCalculateIndicators_Columns(t *testing.T) {
	rows := makeSampleRows()
	result := market.CalculateIndicators(rows)
	last := result[len(result)-1]
	if math.IsNaN(last.RSI14) {
		t.Error("RSI14 is NaN on last row")
	}
	if math.IsNaN(last.PctChange) {
		t.Error("PctChange is NaN on last row")
	}
	if math.IsNaN(last.VolumeRatio) {
		t.Error("VolumeRatio is NaN on last row")
	}
}

func TestCalculateIndicators_RSIRange(t *testing.T) {
	rows := makeSampleRows()
	result := market.CalculateIndicators(rows)
	for _, r := range result {
		if math.IsNaN(r.RSI14) {
			continue
		}
		if r.RSI14 < 0 || r.RSI14 > 100 {
			t.Errorf("RSI14 out of range: %v", r.RSI14)
		}
	}
}

func TestSummarizeOnDate(t *testing.T) {
	rows := makeSampleRows()
	for i := range rows {
		rows[i].Date = []string{
			"2024-01-10", "2024-01-11", "2024-01-12", "2024-01-13", "2024-01-14",
			"2024-01-15", "2024-01-16", "2024-01-17", "2024-01-18", "2024-01-19",
			"2024-01-20", "2024-01-21", "2024-01-22", "2024-01-23", "2024-01-24",
		}[i]
	}
	result := market.CalculateIndicators(rows)
	s, err := market.SummarizeOnDate(result, "2024-01-24")
	if err != nil {
		t.Fatalf("SummarizeOnDate: %v", err)
	}
	if s.Close != 77500 {
		t.Errorf("unexpected close: %v", s.Close)
	}
	if s.Signal == "" {
		t.Error("signal should not be empty")
	}
}
