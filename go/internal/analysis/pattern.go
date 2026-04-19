package analysis

import (
	"math"
	"time"

	"annotate/internal/store"
)

type ClosedTrade struct {
	BuyID    string
	SellID   string
	Ticker   string
	PnLPct   float64
	HoldDays int
}

type Stats struct {
	TotalTrades  int
	BuyCount     int
	SellCount    int
	HoldingCount int
	ClosedTrades []ClosedTrade
	Markets      map[string]int
}

func ComputeStats(trades []store.Trade) Stats {
	s := Stats{
		TotalTrades: len(trades),
		Markets:     make(map[string]int),
	}

	buyMap := make(map[string]store.Trade)
	for _, t := range trades {
		switch t.Action {
		case "buy":
			s.BuyCount++
			buyMap[t.ID] = t
			s.Markets[t.Market]++
		case "sell":
			s.SellCount++
		}
		if t.Status == "holding" {
			s.HoldingCount++
		}
	}

	for _, t := range trades {
		if t.Action != "sell" || t.LinkedBuyID == "" {
			continue
		}
		buy, ok := buyMap[t.LinkedBuyID]
		if !ok {
			continue
		}
		pnl := (t.Price - buy.Price) / buy.Price * 100
		buyDate, _ := time.Parse("2006-01-02", buy.Date)
		sellDate, _ := time.Parse("2006-01-02", t.Date)
		days := int(math.Round(sellDate.Sub(buyDate).Hours() / 24))
		s.ClosedTrades = append(s.ClosedTrades, ClosedTrade{
			BuyID:    buy.ID,
			SellID:   t.ID,
			Ticker:   t.Ticker,
			PnLPct:   math.Round(pnl*100) / 100,
			HoldDays: days,
		})
	}

	return s
}
