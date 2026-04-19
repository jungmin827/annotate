package store

import (
	"database/sql"
	"fmt"
)

type Trade struct {
	ID           string
	Ticker       string
	Name         string
	Market       string
	Action       string
	Price        float64
	Quantity     int
	Date         string
	Time         string
	Reason       string
	Status       string
	LinkedSellID string
	LinkedBuyID  string
}

func (db *DB) InsertTrade(t Trade) error {
	_, err := db.SQL.Exec(`
		INSERT OR IGNORE INTO trades
		(id, ticker, name, market, action, price, quantity, date, time, reason, status, linked_sell_id, linked_buy_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Ticker, t.Name, t.Market, t.Action, t.Price, t.Quantity,
		t.Date, t.Time, t.Reason, t.Status, t.LinkedSellID, t.LinkedBuyID,
	)
	return err
}

func (db *DB) GetTradeByID(id string) (Trade, error) {
	row := db.SQL.QueryRow(`SELECT id,ticker,name,market,action,price,quantity,date,time,reason,status,linked_sell_id,linked_buy_id FROM trades WHERE id=?`, id)
	return scanTrade(row)
}

func (db *DB) GetAllTrades() ([]Trade, error) {
	rows, err := db.SQL.Query(`SELECT id,ticker,name,market,action,price,quantity,date,time,reason,status,linked_sell_id,linked_buy_id FROM trades ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTrades(rows)
}

func (db *DB) GetHoldingTrades() ([]Trade, error) {
	rows, err := db.SQL.Query(`SELECT id,ticker,name,market,action,price,quantity,date,time,reason,status,linked_sell_id,linked_buy_id FROM trades WHERE status='holding' ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTrades(rows)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTrade(s scanner) (Trade, error) {
	var t Trade
	err := s.Scan(&t.ID, &t.Ticker, &t.Name, &t.Market, &t.Action, &t.Price, &t.Quantity,
		&t.Date, &t.Time, &t.Reason, &t.Status, &t.LinkedSellID, &t.LinkedBuyID)
	if err == sql.ErrNoRows {
		return Trade{}, fmt.Errorf("trade not found")
	}
	return t, err
}

func collectTrades(rows *sql.Rows) ([]Trade, error) {
	var out []Trade
	for rows.Next() {
		t, err := scanTrade(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
