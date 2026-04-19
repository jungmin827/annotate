package handler

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"annotate/internal/analysis"
	"annotate/internal/market"
	"annotate/internal/news"
	"annotate/internal/store"
)

//go:embed templates
var templateFS embed.FS

type Handler struct {
	db            *store.DB
	apiKey        string
	claudeBaseURL string
	newsBaseURL   string
	tmpl          *template.Template
	mux           *http.ServeMux
}

func New(db *store.DB, apiKey string) *Handler {
	h := &Handler{
		db:     db,
		apiKey: apiKey,
	}
	h.tmpl = template.Must(template.ParseFS(templateFS, "templates/*.html"))
	h.mux = http.NewServeMux()
	h.mux.HandleFunc("GET /", h.dashboard)
	h.mux.HandleFunc("GET /trades", h.tradesList)
	h.mux.HandleFunc("POST /analyze/{id}", h.analyze)
	h.mux.HandleFunc("POST /review", h.review)
	h.mux.HandleFunc("POST /brief", h.brief)
	return h
}

func (h *Handler) SetClaudeBaseURL(url string) { h.claudeBaseURL = url }
func (h *Handler) SetNewsBaseURL(url string)   { h.newsBaseURL = url }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	trades, err := h.db.GetAllTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	holdings, _ := h.db.GetHoldingTrades()
	stats := analysis.ComputeStats(trades)

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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "dashboard.html", map[string]any{
		"Holdings": holdings,
		"Stats":    stats,
		"AvgPnL":   avgPnL,
		"AvgHold":  avgHold,
	})
}

func (h *Handler) tradesList(w http.ResponseWriter, r *http.Request) {
	trades, err := h.db.GetAllTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "trades.html", map[string]any{
		"Trades": trades,
	})
}

func (h *Handler) analyze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	trade, err := h.db.GetTradeByID(id)
	if err != nil {
		http.Error(w, "trade not found", http.StatusNotFound)
		return
	}

	summary := h.fetchSummary(trade)
	prompt := analysis.BuildAnalysisPrompt(trade, summary)
	result, err := analysis.CallClaude(h.apiKey, prompt, h.claudeBaseURL)
	if err != nil {
		http.Error(w, "analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "analyze_result.html", map[string]any{
		"Trade":  trade,
		"Result": result,
	})
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	trades, err := h.db.GetAllTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stats := analysis.ComputeStats(trades)
	prompt := analysis.BuildReviewPrompt(trades, stats)
	result, err := analysis.CallClaude(h.apiKey, prompt, h.claudeBaseURL)
	if err != nil {
		http.Error(w, "review failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "review_result.html", map[string]any{
		"Result": result,
	})
}

type briefItem struct {
	Name    string
	Ticker  string
	Summary string
}

func (h *Handler) brief(w http.ResponseWriter, r *http.Request) {
	holdings, err := h.db.GetHoldingTrades()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	seen := map[string]bool{}
	var briefs []briefItem
	for _, t := range holdings {
		if seen[t.Ticker] {
			continue
		}
		seen[t.Ticker] = true

		fetchedAt, _ := h.db.GetLatestNewsFetchedAt(t.Ticker)
		if newsIsStale(fetchedAt) {
			_ = news.FetchAndStore(t.Ticker, h.db, h.newsBaseURL)
		}

		items, _ := h.db.GetNewsForTicker(t.Ticker, 5)
		summary := h.summarizeNews(t.Name, items)
		briefs = append(briefs, briefItem{Name: t.Name, Ticker: t.Ticker, Summary: summary})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.ExecuteTemplate(w, "brief_result.html", map[string]any{
		"Briefs": briefs,
		"Date":   time.Now().Format("2006-01-02"),
	})
}

func newsIsStale(fetchedAt string) bool {
	if fetchedAt == "" {
		return true
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", fetchedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > time.Hour
}

func (h *Handler) summarizeNews(name string, items []store.NewsItem) string {
	if len(items) == 0 {
		return "최근 뉴스를 찾을 수 없습니다."
	}
	var lines string
	for _, item := range items {
		lines += "- " + item.Title + "\n"
	}
	prompt := fmt.Sprintf(
		"다음은 %s의 최근 뉴스 헤드라인입니다.\n투자 추천 없이, 사실 기반으로 3줄 이내로 요약하세요.\n\n%s\n형식:\n1. [핵심 내용 1]\n2. [핵심 내용 2]\n3. [핵심 내용 3]",
		name, lines,
	)
	result, err := analysis.CallClaude(h.apiKey, prompt, h.claudeBaseURL)
	if err != nil {
		return "요약 실패: " + err.Error()
	}
	return result
}

func (h *Handler) fetchSummary(trade store.Trade) market.Summary {
	t, err := time.Parse("2006-01-02", trade.Date)
	if err != nil {
		return market.Summary{}
	}
	start := t.AddDate(0, 0, -30).Format("2006-01-02")
	end := t.AddDate(0, 0, 5).Format("2006-01-02")

	rows, err := market.FetchOHLCV(trade.Ticker, trade.Market, start, end, "")
	if err != nil || len(rows) < 15 {
		return market.Summary{Signal: "데이터없음"}
	}
	indicators := market.CalculateIndicators(rows)
	summary, err := market.SummarizeOnDate(indicators, trade.Date)
	if err != nil {
		return market.Summary{Signal: "데이터없음"}
	}
	return summary
}

func newlineToBreak(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}
