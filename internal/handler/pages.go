package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"ai-diary-go/internal/constants"
	"ai-diary-go/internal/dateutil"
	"ai-diary-go/internal/model"
)

// DailyAlert はダッシュボードのデイリーアラート表示用データです。
type DailyAlert struct {
	DateStr string
	DateJP  string
}

// SummaryAlert は週次・月次・年次アラート表示用データです。
type SummaryAlert struct {
	ID     int64
	DateJP string
}

// DashboardData はダッシュボード画面用のテンプレートデータです。
type DashboardData struct {
	Endpoint      string
	DailyAlerts   []DailyAlert
	WeeklyAlerts  []SummaryAlert
	MonthlyAlerts []SummaryAlert
	YearlyAlerts  []SummaryAlert
}

// Dashboard はダッシュボード画面を描画します。
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	data := DashboardData{
		Endpoint:      "dashboard",
		DailyAlerts:   h.getDailyAlerts(),
		WeeklyAlerts:  h.getWeeklyAlerts(),
		MonthlyAlerts: h.getMonthlyAlerts(),
		YearlyAlerts:  h.getYearlyAlerts(),
	}
	h.render(w, "dashboard.html", data)
}

func (h *Handler) getDailyAlerts() []DailyAlert {
	startDate, err := h.DB.GetStartDate()
	if err != nil {
		h.Logger.Error("デイリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	today := dateutil.TodayJST()
	logs, err := h.DB.GetDailyAlerts(startDate, today, constants.MaxDailyAlerts)
	if err != nil {
		h.Logger.Error("デイリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	alerts := make([]DailyAlert, 0, len(logs))
	for _, l := range logs {
		alerts = append(alerts, DailyAlert{
			DateStr: dateutil.FormatISO(l.Date),
			DateJP:  dateutil.FormatDateJapanese(l.Date),
		})
	}
	return alerts
}

func (h *Handler) getWeeklyAlerts() []SummaryAlert {
	startDate, err := h.DB.GetStartDate()
	if err != nil {
		h.Logger.Error("ウィークリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	today := dateutil.TodayJST()
	currentWeekMonday := dateutil.GetMondayOfWeek(today)
	logs, err := h.DB.GetWeeklyAlerts(dateutil.GetMondayOfWeek(startDate), currentWeekMonday, constants.MaxDailyAlerts)
	if err != nil {
		h.Logger.Error("ウィークリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	alerts := make([]SummaryAlert, 0, len(logs))
	for _, l := range logs {
		alerts = append(alerts, SummaryAlert{
			ID:     l.ID,
			DateJP: dateutil.FormatWeeklyDateJapanese(l.StartDate, l.WeekNumber),
		})
	}
	return alerts
}

func (h *Handler) getMonthlyAlerts() []SummaryAlert {
	startDate, err := h.DB.GetStartDate()
	if err != nil {
		h.Logger.Error("マンスリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	today := dateutil.TodayJST()
	currentMonthFirst := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, dateutil.JST)
	startMonthFirst := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, dateutil.JST)
	logs, err := h.DB.GetMonthlyAlerts(startMonthFirst, currentMonthFirst, constants.MaxDailyAlerts)
	if err != nil {
		h.Logger.Error("マンスリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	alerts := make([]SummaryAlert, 0, len(logs))
	for _, l := range logs {
		alerts = append(alerts, SummaryAlert{
			ID:     l.ID,
			DateJP: fmt.Sprintf("%d年%d月", l.Year, l.Month),
		})
	}
	return alerts
}

func (h *Handler) getYearlyAlerts() []SummaryAlert {
	startDate, err := h.DB.GetStartDate()
	if err != nil {
		h.Logger.Error("イヤリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	today := dateutil.TodayJST()
	currentYearFirst := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, dateutil.JST)
	startYearFirst := time.Date(startDate.Year(), 1, 1, 0, 0, 0, 0, dateutil.JST)
	logs, err := h.DB.GetYearlyAlerts(startYearFirst, currentYearFirst, constants.MaxDailyAlerts)
	if err != nil {
		h.Logger.Error("イヤリーアラートの取得中にエラーが発生しました: %v", err)
		return nil
	}
	alerts := make([]SummaryAlert, 0, len(logs))
	for _, l := range logs {
		alerts = append(alerts, SummaryAlert{
			ID:     l.ID,
			DateJP: fmt.Sprintf("%d年", l.Year),
		})
	}
	return alerts
}

// Journal はデイリー日記画面を描画します。
func (h *Handler) Journal(w http.ResponseWriter, r *http.Request) {
	h.render(w, "journal.html", map[string]any{"Endpoint": "journal"})
}

// Milestones は日記検索画面を描画します。
func (h *Handler) Milestones(w http.ResponseWriter, r *http.Request) {
	h.render(w, "milestones.html", map[string]any{"Endpoint": "milestones"})
}

// ConfigPage はプロンプト設定画面を描画します。
func (h *Handler) ConfigPage(w http.ResponseWriter, r *http.Request) {
	getTemplate := func(logType string) string {
		p, err := h.DB.GetPromptByType(logType)
		if err != nil || p == nil {
			return ""
		}
		return p.Template
	}

	h.render(w, "config.html", map[string]any{
		"Endpoint":      "config",
		"DailyPrompt":   getTemplate("daily"),
		"WeeklyPrompt":  getTemplate("weekly"),
		"MonthlyPrompt": getTemplate("monthly"),
		"YearlyPrompt":  getTemplate("yearly"),
	})
}

// ConfigUpdate はプロンプト設定を保存します。
func (h *Handler) ConfigUpdate(w http.ResponseWriter, r *http.Request) {
	prompts := []struct {
		logType  string
		template string
	}{
		{"daily", r.FormValue("daily_prompt")},
		{"weekly", r.FormValue("weekly_prompt")},
		{"monthly", r.FormValue("monthly_prompt")},
		{"yearly", r.FormValue("yearly_prompt")},
	}

	for _, p := range prompts {
		if err := h.DB.UpsertPrompt(p.logType, p.template); err != nil {
			h.Logger.Error("プロンプト設定の更新中にエラーが発生しました (%s): %v", p.logType, err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "プロンプト設定の保存に失敗しました",
			})
			return
		}
	}

	h.Logger.Info("プロンプト設定を更新しました")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "プロンプト設定を保存しました",
	})
}

// ExportPage はエクスポート画面を描画します。
func (h *Handler) ExportPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "export.html", map[string]any{"Endpoint": "export"})
}

// WeekDayItem は週次要約画面の1日分の表示データです。
type WeekDayItem struct {
	Date    time.Time
	Content string
}

type weekDayJS struct {
	Date    string  `json:"date"`
	Content *string `json:"content"`
}

type monthWeekJS struct {
	StartDate  string  `json:"start_date"`
	WeekNumber int     `json:"week_number"`
	Content    *string `json:"content"`
}

// SummaryData は要約作成画面用のテンプレートデータです。
type SummaryData struct {
	Endpoint      string
	Error         string
	Type          string
	WeeklyLog     *model.WeeklyLog
	WeekData      []WeekDayItem
	WeekTitle     string
	MonthlyLog    *model.MonthlyLog
	WeeklyLogs    []model.WeeklyLog
	MonthTitle    string
	YearlyLog     *model.YearlyLog
	MonthlyLogs   []model.MonthlyLog
	YearTitle     string
	WeekDataJSON  template.JS
	MonthDataJSON template.JS
}

// Summary は要約作成画面を描画します。
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	typeParam := r.URL.Query().Get("type")
	idParam := r.URL.Query().Get("id")

	baseData := SummaryData{
		Endpoint:      "summary",
		WeekDataJSON:  template.JS("[]"),
		MonthDataJSON: template.JS("[]"),
	}

	if idParam == "" || (typeParam != "weekly" && typeParam != "monthly" && typeParam != "yearly") {
		baseData.Error = "パラメータが不正です"
		h.render(w, "summary.html", baseData)
		return
	}

	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		baseData.Error = "無効なIDです"
		h.render(w, "summary.html", baseData)
		return
	}

	switch typeParam {
	case "weekly":
		weeklyLog, err := h.DB.GetWeeklyLogByID(id)
		if err != nil {
			h.Logger.Error("要約作成画面の表示中にエラーが発生しました: %v", err)
			baseData.Error = "データの取得中にエラーが発生しました"
			h.render(w, "summary.html", baseData)
			return
		}
		if weeklyLog == nil {
			baseData.Error = "週次ログが見つかりませんでした"
			h.render(w, "summary.html", baseData)
			return
		}

		startDate := weeklyLog.StartDate
		endDate := startDate.AddDate(0, 0, 6)
		dailyLogs, err := h.DB.GetDailyLogsBetween(startDate, endDate)
		if err != nil {
			h.Logger.Error("要約作成画面の表示中にエラーが発生しました: %v", err)
			baseData.Error = "データの取得中にエラーが発生しました"
			h.render(w, "summary.html", baseData)
			return
		}

		dailyMap := make(map[string]string, len(dailyLogs))
		for _, dl := range dailyLogs {
			dailyMap[dateutil.FormatISO(dl.Date)] = dl.Content
		}

		weekData := make([]WeekDayItem, 0, 7)
		jsItems := make([]weekDayJS, 0, 7)
		for i := 0; i < 7; i++ {
			cur := startDate.AddDate(0, 0, i)
			iso := dateutil.FormatISO(cur)
			c := dailyMap[iso]
			weekData = append(weekData, WeekDayItem{Date: cur, Content: c})
			var ptr *string
			if c != "" {
				val := c
				ptr = &val
			}
			jsItems = append(jsItems, weekDayJS{Date: iso, Content: ptr})
		}

		jsBytes, _ := json.Marshal(jsItems)
		baseData.Type = "weekly"
		baseData.WeeklyLog = weeklyLog
		baseData.WeekData = weekData
		baseData.WeekTitle = dateutil.FormatWeeklyDateJapanese(startDate, weeklyLog.WeekNumber)
		baseData.WeekDataJSON = template.JS(jsBytes)
		h.render(w, "summary.html", baseData)

	case "monthly":
		monthlyLog, err := h.DB.GetMonthlyLogByID(id)
		if err != nil {
			h.Logger.Error("要約作成画面の表示中にエラーが発生しました: %v", err)
			baseData.Error = "データの取得中にエラーが発生しました"
			h.render(w, "summary.html", baseData)
			return
		}
		if monthlyLog == nil {
			baseData.Error = "月次ログが見つかりませんでした"
			h.render(w, "summary.html", baseData)
			return
		}

		weeklyLogs, err := h.DB.GetWeeklyLogsByYearMonth(monthlyLog.Year, monthlyLog.Month)
		if err != nil {
			h.Logger.Error("要約作成画面の表示中にエラーが発生しました: %v", err)
			baseData.Error = "データの取得中にエラーが発生しました"
			h.render(w, "summary.html", baseData)
			return
		}

		jsItems := make([]monthWeekJS, 0, len(weeklyLogs))
		for _, wl := range weeklyLogs {
			var ptr *string
			if wl.Content != "" {
				val := wl.Content
				ptr = &val
			}
			jsItems = append(jsItems, monthWeekJS{
				StartDate:  dateutil.FormatISO(wl.StartDate),
				WeekNumber: wl.WeekNumber,
				Content:    ptr,
			})
		}
		jsBytes, _ := json.Marshal(jsItems)

		baseData.Type = "monthly"
		baseData.MonthlyLog = monthlyLog
		baseData.WeeklyLogs = weeklyLogs
		baseData.MonthTitle = fmt.Sprintf("%d年%d月", monthlyLog.Year, monthlyLog.Month)
		baseData.MonthDataJSON = template.JS(jsBytes)
		h.render(w, "summary.html", baseData)

	case "yearly":
		yearlyLog, err := h.DB.GetYearlyLogByID(id)
		if err != nil {
			h.Logger.Error("要約作成画面の表示中にエラーが発生しました: %v", err)
			baseData.Error = "データの取得中にエラーが発生しました"
			h.render(w, "summary.html", baseData)
			return
		}
		if yearlyLog == nil {
			baseData.Error = "年次ログが見つかりませんでした"
			h.render(w, "summary.html", baseData)
			return
		}

		monthlyLogs, err := h.DB.GetMonthlyLogsByYear(yearlyLog.Year)
		if err != nil {
			h.Logger.Error("要約作成画面の表示中にエラーが発生しました: %v", err)
			baseData.Error = "データの取得中にエラーが発生しました"
			h.render(w, "summary.html", baseData)
			return
		}

		baseData.Type = "yearly"
		baseData.YearlyLog = yearlyLog
		baseData.MonthlyLogs = monthlyLogs
		baseData.YearTitle = fmt.Sprintf("%d年", yearlyLog.Year)
		h.render(w, "summary.html", baseData)
	}
}
