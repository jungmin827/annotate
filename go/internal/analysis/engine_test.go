package analysis_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"annotate/internal/analysis"
	"annotate/internal/market"
	"annotate/internal/store"
)

var testTrade = store.Trade{
	ID:       "test_001",
	Ticker:   "005930",
	Name:     "삼성전자",
	Market:   "KRX",
	Action:   "buy",
	Price:    70000,
	Quantity: 10,
	Date:     "2024-01-15",
	Reason:   "반도체 사이클 회복 기대",
	Status:   "holding",
}

var testSummary = market.Summary{
	Date:        "2024-01-15",
	Close:       70500,
	RSI14:       62.3,
	PctChange:   0.8,
	VolumeRatio: 1.2,
	Signal:      "특이사항없음",
}

func TestBuildAnalysisPrompt(t *testing.T) {
	prompt := analysis.BuildAnalysisPrompt(testTrade, testSummary)
	if !strings.Contains(prompt, "삼성전자") {
		t.Error("prompt missing 종목명")
	}
	if !strings.Contains(prompt, "반도체 사이클") {
		t.Error("prompt missing reason")
	}
	if !strings.Contains(prompt, "62.3") {
		t.Error("prompt missing RSI")
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	stats := ComputeStats_helper()
	prompt := analysis.BuildReviewPrompt(sampleTrades, stats)
	if !strings.Contains(prompt, "패턴") {
		t.Error("prompt missing 패턴")
	}
}

func ComputeStats_helper() analysis.Stats {
	return analysis.ComputeStats(sampleTrades)
}

func makeClaudeServer(responseText string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": responseText},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestCallClaude(t *testing.T) {
	srv := makeClaudeServer("분석 결과입니다.")
	defer srv.Close()

	got, err := analysis.CallClaude("test-key", "test prompt", srv.URL)
	if err != nil {
		t.Fatalf("CallClaude: %v", err)
	}
	if got != "분석 결과입니다." {
		t.Errorf("unexpected response: %q", got)
	}
}
