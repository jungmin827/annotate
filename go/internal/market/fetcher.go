package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OHLCV struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

func NormalizeTicker(ticker, market string) string {
	if market == "KRX" {
		return ticker + ".KS"
	}
	return ticker
}

// FetchOHLCV fetches OHLCV rows from Yahoo Finance v8 chart API.
// baseURL is injectable for tests; pass "" to use the real Yahoo endpoint.
func FetchOHLCV(ticker, mkt, start, end, baseURL string) ([]OHLCV, error) {
	yf := NormalizeTicker(ticker, mkt)

	startT, err := time.Parse("2006-01-02", start)
	if err != nil {
		return nil, fmt.Errorf("parse start: %w", err)
	}
	endT, err := time.Parse("2006-01-02", end)
	if err != nil {
		return nil, fmt.Errorf("parse end: %w", err)
	}

	if baseURL == "" {
		baseURL = "https://query1.finance.yahoo.com"
	}
	url := fmt.Sprintf(
		"%s/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d",
		baseURL, yf, startT.Unix(), endT.Unix(),
	)

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []float64 `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error any `json:"error"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(payload.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty result for %s", ticker)
	}

	res := payload.Chart.Result[0]
	q := res.Indicators.Quote[0]
	var rows []OHLCV
	for i, ts := range res.Timestamp {
		rows = append(rows, OHLCV{
			Date:   time.Unix(ts, 0).UTC().Format("2006-01-02"),
			Open:   q.Open[i],
			High:   q.High[i],
			Low:    q.Low[i],
			Close:  q.Close[i],
			Volume: q.Volume[i],
		})
	}
	return rows, nil
}
