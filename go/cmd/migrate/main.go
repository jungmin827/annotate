package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"annotate/internal/store"
)

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

type jsonFile struct {
	Trades []jsonTrade `json:"trades"`
}

func run(tradesJSON, dbPath string) error {
	data, err := os.ReadFile(tradesJSON)
	if err != nil {
		return fmt.Errorf("read %s: %w", tradesJSON, err)
	}

	var jf jsonFile
	if err := json.Unmarshal(data, &jf); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	inserted := 0
	for _, jt := range jf.Trades {
		t := store.Trade{
			ID:       jt.ID,
			Ticker:   jt.Ticker,
			Name:     jt.Name,
			Market:   jt.Market,
			Action:   jt.Action,
			Price:    jt.Price,
			Quantity: jt.Quantity,
			Date:     jt.Date,
			Time:     jt.Time,
			Reason:   jt.Reason,
			Status:   jt.Status,
		}
		if jt.LinkedSellID != nil {
			t.LinkedSellID = *jt.LinkedSellID
		}
		if jt.LinkedBuyID != nil {
			t.LinkedBuyID = *jt.LinkedBuyID
		}
		if err := db.InsertTrade(t); err != nil {
			return fmt.Errorf("insert %s: %w", t.ID, err)
		}
		inserted++
	}

	fmt.Printf("migrated %d trades → %s\n", inserted, dbPath)
	return nil
}

func main() {
	root := filepath.Join("..", "..")
	tradesJSON := filepath.Join(root, "data", "trades.json")
	dbPath := filepath.Join(root, "data", "annotate.db")

	if len(os.Args) == 3 {
		tradesJSON = os.Args[1]
		dbPath = os.Args[2]
	}

	if err := run(tradesJSON, dbPath); err != nil {
		log.Fatal(err)
	}
}
