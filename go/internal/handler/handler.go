package handler

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	"annotate/internal/analysis"
	"annotate/internal/market"
	"annotate/internal/store"
)

//go:embed templates
var templateFS embed.FS

type Handler struct {
	db            *store.DB
	apiKey        string
	claudeBaseURL string
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
	return h
}

func (h *Handler) SetClaudeBaseURL(url string) { h.claudeBaseURL = url }

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

// newlineToBreak converts \n to <br> for HTML display.
func newlineToBreak(s string) template.HTML {
	escaped := template.HTMLEscapeString(s)
	return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
}
