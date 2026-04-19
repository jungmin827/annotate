package scheduler

import (
	"log"
	"time"

	"annotate/internal/news"
	"annotate/internal/store"
)

type Scheduler struct {
	db          *store.DB
	interval    time.Duration
	newsBaseURL string
	stop        chan struct{}
}

// New creates a scheduler that fetches news for holding stocks at the given interval.
// newsBaseURL is injectable for tests; pass "" to use the real Yahoo Finance endpoint.
func New(db *store.DB, interval time.Duration, newsBaseURL string) *Scheduler {
	return &Scheduler{
		db:          db,
		interval:    interval,
		newsBaseURL: newsBaseURL,
		stop:        make(chan struct{}),
	}
}

// Start launches the background goroutine. It performs an immediate fetch,
// then repeats at the configured interval.
func (s *Scheduler) Start() {
	go s.run()
}

// Stop signals the scheduler to exit and waits for the goroutine to finish.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) run() {
	s.fetchNews()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.fetchNews()
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) fetchNews() {
	holdings, err := s.db.GetHoldingTrades()
	if err != nil {
		log.Printf("[scheduler] get holdings: %v", err)
		return
	}
	seen := map[string]bool{}
	for _, t := range holdings {
		if seen[t.Ticker] {
			continue
		}
		seen[t.Ticker] = true
		if err := news.FetchAndStore(t.Ticker, s.db, s.newsBaseURL); err != nil {
			log.Printf("[scheduler] fetch news %s: %v", t.Ticker, err)
		} else {
			log.Printf("[scheduler] fetched news for %s", t.Ticker)
		}
	}
}
