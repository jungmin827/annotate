package market_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"annotate/internal/market"
)

func TestNormalizeTicker(t *testing.T) {
	cases := []struct {
		ticker, mkt, want string
	}{
		{"005930", "KRX", "005930.KS"},
		{"NVDA", "NASDAQ", "NVDA"},
		{"AAPL", "NYSE", "AAPL"},
	}
	for _, c := range cases {
		if got := market.NormalizeTicker(c.ticker, c.mkt); got != c.want {
			t.Errorf("NormalizeTicker(%q,%q)=%q want %q", c.ticker, c.mkt, got, c.want)
		}
	}
}

func makeYFResponse() map[string]any {
	timestamps := []int64{1704931200, 1705017600, 1705104000}
	return map[string]any{
		"chart": map[string]any{
			"result": []any{
				map[string]any{
					"timestamp": timestamps,
					"indicators": map[string]any{
						"quote": []any{
							map[string]any{
								"open":   []float64{70000, 71000, 72000},
								"high":   []float64{71000, 72000, 73000},
								"low":    []float64{69000, 70000, 71000},
								"close":  []float64{70500, 71500, 72500},
								"volume": []float64{1000000, 1200000, 900000},
							},
						},
					},
				},
			},
			"error": nil,
		},
	}
}

func TestFetchOHLCV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeYFResponse())
	}))
	defer srv.Close()

	rows, err := market.FetchOHLCV("NVDA", "NASDAQ", "2024-01-11", "2024-01-13", srv.URL)
	if err != nil {
		t.Fatalf("FetchOHLCV: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Close != 70500 {
		t.Errorf("unexpected close: %v", rows[0].Close)
	}
}
