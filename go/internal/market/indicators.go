package market

import (
	"fmt"
	"math"
	"strings"
)

type Indicator struct {
	OHLCV
	RSI14       float64
	PctChange   float64
	VolumeRatio float64
	Cumulative5d float64
}

type Summary struct {
	Date        string
	Close       float64
	RSI14       float64
	PctChange   float64
	VolumeRatio float64
	Cumulative5d float64
	Signal      string
}

// CalculateIndicators computes RSI14, PctChange, VolumeRatio, Cumulative5d for each row.
// Rows with insufficient history get NaN for that field.
func CalculateIndicators(rows []OHLCV) []Indicator {
	n := len(rows)
	out := make([]Indicator, n)
	for i, r := range rows {
		out[i].OHLCV = r
		out[i].RSI14 = math.NaN()
		out[i].PctChange = math.NaN()
		out[i].VolumeRatio = math.NaN()
		out[i].Cumulative5d = math.NaN()
	}

	// PctChange
	for i := 1; i < n; i++ {
		out[i].PctChange = (rows[i].Close - rows[i-1].Close) / rows[i-1].Close * 100
	}

	// VolumeRatio (5-day rolling mean)
	for i := 4; i < n; i++ {
		var sum float64
		for j := i - 4; j <= i; j++ {
			sum += rows[j].Volume
		}
		avg := sum / 5
		if avg != 0 {
			out[i].VolumeRatio = rows[i].Volume / avg
		}
	}

	// Cumulative5d
	for i := 5; i < n; i++ {
		out[i].Cumulative5d = (rows[i].Close - rows[i-5].Close) / rows[i-5].Close * 100
	}

	// RSI14 — Wilder's smoothing
	if n > 14 {
		var gainSum, lossSum float64
		for i := 1; i <= 14; i++ {
			diff := rows[i].Close - rows[i-1].Close
			if diff > 0 {
				gainSum += diff
			} else {
				lossSum -= diff
			}
		}
		avgGain := gainSum / 14
		avgLoss := lossSum / 14
		out[14].RSI14 = rsi(avgGain, avgLoss)

		for i := 15; i < n; i++ {
			diff := rows[i].Close - rows[i-1].Close
			gain, loss := 0.0, 0.0
			if diff > 0 {
				gain = diff
			} else {
				loss = -diff
			}
			avgGain = (avgGain*13 + gain) / 14
			avgLoss = (avgLoss*13 + loss) / 14
			out[i].RSI14 = rsi(avgGain, avgLoss)
		}
	}

	return out
}

func rsi(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

func SummarizeOnDate(rows []Indicator, date string) (Summary, error) {
	for _, r := range rows {
		if r.Date == date {
			return buildSummary(r), nil
		}
	}
	// fallback: last row
	if len(rows) == 0 {
		return Summary{}, fmt.Errorf("no data")
	}
	return buildSummary(rows[len(rows)-1]), nil
}

func buildSummary(r Indicator) Summary {
	var signals []string
	if !math.IsNaN(r.RSI14) && r.RSI14 > 70 {
		signals = append(signals, "과매수(RSI>70)")
	}
	if !math.IsNaN(r.PctChange) && r.PctChange > 3 {
		signals = append(signals, "급등추격(+3%↑)")
	}
	if !math.IsNaN(r.VolumeRatio) && r.VolumeRatio > 3 {
		signals = append(signals, "거래량급증(3배↑)")
	}
	sig := "특이사항없음"
	if len(signals) > 0 {
		sig = strings.Join(signals, ", ")
	}
	return Summary{
		Date:        r.Date,
		Close:       r.Close,
		RSI14:       r.RSI14,
		PctChange:   r.PctChange,
		VolumeRatio: r.VolumeRatio,
		Cumulative5d: r.Cumulative5d,
		Signal:      sig,
	}
}
