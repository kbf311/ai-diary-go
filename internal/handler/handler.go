package handler

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"ai-diary-go/internal/dateutil"
	"ai-diary-go/internal/db"
	"ai-diary-go/internal/logger"
)

// Handler は HTTP リクエストを処理します。
type Handler struct {
	DB        *db.DB
	Logger    *logger.Logger
	Templates map[string]*template.Template
}

// New はテンプレートをパースして Handler を初期化します。
func New(database *db.DB, log *logger.Logger, templateDir string) (*Handler, error) {
	funcMap := template.FuncMap{
		"formatDateJapanese":       dateutil.FormatDateJapanese,
		"getWeekdayJapanese":       dateutil.GetWeekdayJapanese,
		"formatWeeklyDateJapanese": dateutil.FormatWeeklyDateJapanese,
		"formatISO":                dateutil.FormatISO,
		"isToday": func(t time.Time) bool {
			return dateutil.FormatISO(t) == dateutil.FormatISO(dateutil.TodayJST())
		},
	}

	pages := []string{
		"dashboard.html",
		"journal.html",
		"milestones.html",
		"config.html",
		"summary.html",
		"export.html",
	}

	basePath := filepath.Join(templateDir, "base.html")
	tmpls := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		pagePath := filepath.Join(templateDir, page)
		t, err := template.New("base.html").Funcs(funcMap).ParseFiles(basePath, pagePath)
		if err != nil {
			return nil, err
		}
		tmpls[page] = t
	}

	return &Handler{
		DB:        database,
		Logger:    log,
		Templates: tmpls,
	}, nil
}

// RegisterRoutes はすべてのルートを ServeMux に登録します。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 静的ファイル
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// ページルート
	mux.HandleFunc("GET /{$}", h.Dashboard)
	mux.HandleFunc("GET /dashboard", h.Dashboard)
	mux.HandleFunc("GET /journal", h.Journal)
	mux.HandleFunc("GET /milestones", h.Milestones)
	mux.HandleFunc("GET /config", h.ConfigPage)
	mux.HandleFunc("POST /config", h.ConfigUpdate)
	mux.HandleFunc("GET /summary", h.Summary)
	mux.HandleFunc("GET /export", h.ExportPage)

	// API ルート
	mux.HandleFunc("GET /api/prompt/{type}", h.GetPromptAPI)
	mux.HandleFunc("GET /api/journal/{date}", h.GetJournalAPI)
	mux.HandleFunc("POST /api/journal/{date}", h.UpdateJournalAPI)
	mux.HandleFunc("POST /api/weekly/{id}", h.UpdateWeeklyAPI)
	mux.HandleFunc("POST /api/monthly/{id}", h.UpdateMonthlyAPI)
	mux.HandleFunc("POST /api/yearly/{id}", h.UpdateYearlyAPI)
	mux.HandleFunc("GET /api/export", h.ExportAPI)
	mux.HandleFunc("GET /api/search", h.SearchAPI)
}

func (h *Handler) render(w http.ResponseWriter, page string, data any) {
	t, ok := h.Templates[page]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base.html", data); err != nil {
		h.Logger.Error("テンプレートの描画中にエラーが発生しました (%s): %v", page, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
