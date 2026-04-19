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

func BuildAnalysisPrompt(trade store.Trade, summary market.Summary) string {
	return fmt.Sprintf(`당신은 투자자의 매매 기록을 분석하는 파트너입니다.
매수/매도 추천은 절대 하지 않습니다. 과거 데이터를 바탕으로 기술적 상태와 매수 근거의 타당성만 평가합니다.

## 매매 정보
- 종목: %s (%s, %s)
- 행동: %s / 가격: %.2f / 수량: %d / 날짜: %s
- 매수 이유: %s

## 매수 시점 기술적 상태
- 종가: %.2f / RSI(14): %.1f / 전일 대비: %.2f%% / 거래량비율: %.2f배 / 신호: %s

## 분석 요청
1. 매수 시점의 기술적 상태를 평가하세요 (과매수/정상/과매도).
2. 매수 이유(reason)가 실제 시장 상황과 일치하는지 평가하세요.
3. 근거 타당성을 "근거 충분", "근거 약함", "근거 부족" 중 하나로 판단하세요.
4. 개선할 수 있는 점을 1~2문장으로 제시하세요.

확정적 예측("반드시", "무조건")이나 매수/매도 추천 표현은 사용하지 마세요.`,
		trade.Name, trade.Ticker, trade.Market,
		trade.Action, trade.Price, trade.Quantity, trade.Date, trade.Reason,
		summary.Close, summary.RSI14, summary.PctChange, summary.VolumeRatio, summary.Signal,
	)
}

func BuildReviewPrompt(trades []store.Trade, stats Stats) string {
	var reasons string
	for _, t := range trades {
		if t.Action == "buy" && t.Reason != "" {
			reasons += "- " + t.Reason + "\n"
		}
	}
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
	return fmt.Sprintf(`당신은 투자자의 매매 패턴을 분석하는 파트너입니다.
매수/매도 추천은 절대 하지 않습니다.

## 전체 매매 통계
- 총 매매: %d건 (매수 %d건, 매도 %d건) / 현재 보유: %d건
- 완료 건 평균 수익률: %.1f%%, 평균 보유기간: %.0f일

## 매수 이유 전체 목록
%s

## 패턴 분석 요청
3~5개의 패턴 문장을 작성하세요: "패턴상 당신은 [행동/경향]합니다."
확정적 예측 표현은 사용하지 마세요.`,
		stats.TotalTrades, stats.BuyCount, stats.SellCount, stats.HoldingCount,
		avgPnL, avgHold, reasons,
	)
}

// CallClaude sends a prompt to the Claude API and returns the text response.
// baseURL is injectable for tests; pass "" to use the real Anthropic endpoint.
func CallClaude(apiKey, prompt, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	url := baseURL + "/v1/messages"

	body, _ := json.Marshal(map[string]any{
		"model":      "claude-sonnet-4-6",
		"max_tokens": 1024,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return result.Content[0].Text, nil
}
