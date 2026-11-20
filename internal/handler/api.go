package handler

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-diary-go/internal/dateutil"
)

// GetPromptAPI は指定種別のプロンプトテンプレートを返します。
func (h *Handler) GetPromptAPI(w http.ResponseWriter, r *http.Request) {
	logType := r.PathValue("type")
	switch logType {
	case "daily", "weekly", "monthly", "yearly":
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "プロンプトが見つかりませんでした",
		})
		return
	}

	prompt, err := h.DB.GetPromptByType(logType)
	if err != nil {
		h.Logger.Error("プロンプト取得エラー (%s): %v", logType, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "プロンプトの取得に失敗しました",
		})
		return
	}
	if prompt == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "プロンプトが見つかりませんでした",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"template": prompt.Template,
	})
}

// GetJournalAPI は指定日付のデイリー日記データを返します。
func (h *Handler) GetJournalAPI(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	targetDate, err := dateutil.ParseDate(dateStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効な日付形式です",
		})
		return
	}

	dailyLog, err := h.DB.GetDailyLogByDate(targetDate)
	if err != nil {
		h.Logger.Error("日記データ取得エラー (%s): %v", dateStr, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの取得に失敗しました",
		})
		return
	}

	if dailyLog == nil {
		today := dateutil.TodayJST()
		if targetDate.After(today) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"status":    "error",
				"message":   "未来の日付は登録できません",
				"is_future": true,
			})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":    "error",
			"message":   "データが見つかりませんでした",
			"is_future": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"content": dailyLog.Content,
	})
}

// UpdateJournalAPI は指定日付のデイリー日記を更新します。
func (h *Handler) UpdateJournalAPI(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	targetDate, err := dateutil.ParseDate(dateStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効な日付形式です",
		})
		return
	}

	dailyLog, err := h.DB.GetDailyLogByDate(targetDate)
	if err != nil {
		h.Logger.Error("日記データ取得エラー (%s): %v", dateStr, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}
	if dailyLog == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "データが見つかりませんでした",
		})
		return
	}

	content := r.FormValue("content")
	isCompleted := content != ""
	if err := h.DB.UpdateDailyLog(targetDate, content, isCompleted); err != nil {
		h.Logger.Error("日記データの更新エラー (%s): %v", dateStr, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}

	h.Logger.Info("日記データを更新しました: %s", dateStr)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "日記を保存しました",
	})
}

// UpdateWeeklyAPI は週次ログを更新します。
func (h *Handler) UpdateWeeklyAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効なIDです",
		})
		return
	}

	weeklyLog, err := h.DB.GetWeeklyLogByID(id)
	if err != nil {
		h.Logger.Error("週次ログデータの更新中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}
	if weeklyLog == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "週次ログが見つかりませんでした",
		})
		return
	}

	content := r.FormValue("content")
	isCompleted := content != ""
	if err := h.DB.UpdateWeeklyLog(id, content, isCompleted); err != nil {
		h.Logger.Error("週次ログデータの更新中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}

	h.Logger.Info("週次ログデータを更新しました: %d", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "要約を保存しました",
	})
}

// UpdateMonthlyAPI は月次ログを更新します。
func (h *Handler) UpdateMonthlyAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効なIDです",
		})
		return
	}

	monthlyLog, err := h.DB.GetMonthlyLogByID(id)
	if err != nil {
		h.Logger.Error("月次ログデータの更新中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}
	if monthlyLog == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "月次ログが見つかりませんでした",
		})
		return
	}

	content := r.FormValue("content")
	isCompleted := content != ""
	if err := h.DB.UpdateMonthlyLog(id, content, isCompleted); err != nil {
		h.Logger.Error("月次ログデータの更新中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}

	h.Logger.Info("月次ログデータを更新しました: %d", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "要約を保存しました",
	})
}

// UpdateYearlyAPI は年次ログを更新します。
func (h *Handler) UpdateYearlyAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効なIDです",
		})
		return
	}

	yearlyLog, err := h.DB.GetYearlyLogByID(id)
	if err != nil {
		h.Logger.Error("年次ログデータの更新中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}
	if yearlyLog == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"status":  "error",
			"message": "年次ログが見つかりませんでした",
		})
		return
	}

	content := r.FormValue("content")
	isCompleted := content != ""
	if err := h.DB.UpdateYearlyLog(id, content, isCompleted); err != nil {
		h.Logger.Error("年次ログデータの更新中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "データの保存中にエラーが発生しました",
		})
		return
	}

	h.Logger.Info("年次ログデータを更新しました: %d", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "要約を保存しました",
	})
}

func escapeHTMLWithNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return html.EscapeString(s)
}

func renderExportCard(headerBg, headerTextColor, title, body string) string {
	return fmt.Sprintf(`                <div class="bg-white border border-gray-300 rounded-lg overflow-hidden">
                    <div class="%s px-4 py-2">
                        <div class="text-sm font-medium %s">%s</div>
                    </div>
                    <div class="p-6">
                        <div class="text-gray-900 whitespace-pre-wrap">%s</div>
                    </div>
                </div>
`, headerBg, headerTextColor, html.EscapeString(title), escapeHTMLWithNewlines(body))
}

// ExportAPI は完了済みの日記データを export.html にエクスポートします。
func (h *Handler) ExportAPI(w http.ResponseWriter, r *http.Request) {
	dailyLogs, err := h.DB.GetCompletedDailyLogs()
	if err != nil {
		h.Logger.Error("エクスポート処理中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "エクスポート処理中にエラーが発生しました",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AIダイアリー</title>
    <link rel="stylesheet" href="static/css/tailwind.css">
</head>
<body class="bg-white">
    <!-- ヘッダー -->
    <header class="bg-blue-600 border-b border-blue-700 sticky top-0 z-50">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div class="flex justify-between items-center h-16">
                <div class="flex items-center">
                    <h1 class="text-xl font-semibold text-white">AIダイアリー</h1>
                </div>
            </div>
        </div>
    </header>

    <!-- メインコンテンツ -->
    <main class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div class="space-y-4">
            <div class="space-y-4">
`)

	if len(dailyLogs) > 0 {
		firstDiaryYear := dailyLogs[0].Date.Year()
		preYearlyLogs, err := h.DB.GetYearlyLogsBeforeYear(firstDiaryYear)
		if err != nil {
			h.Logger.Error("エクスポート処理中にエラーが発生しました: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "エクスポート処理中にエラーが発生しました",
			})
			return
		}
		for _, yl := range preYearlyLogs {
			if yl.Content != "" {
				sb.WriteString(renderExportCard("bg-purple-300", "text-purple-900", fmt.Sprintf("【年次】%d年", yl.Year), yl.Content))
			}
		}

		for _, log := range dailyLogs {
			curDate := log.Date

			// 1月1日の場合、年次ログを挿入
			if curDate.Month() == time.January && curDate.Day() == 1 {
				yl, err := h.DB.GetYearlyLogByFirstDay(curDate)
				if err == nil && yl != nil && yl.Content != "" {
					sb.WriteString(renderExportCard("bg-purple-300", "text-purple-900", fmt.Sprintf("【年次】%d年", yl.Year), yl.Content))
				}
			}

			// 月曜日の場合、前週の週次ログを挿入
			if curDate.Weekday() == time.Monday {
				prevMonday := curDate.AddDate(0, 0, -7)
				wl, err := h.DB.GetWeeklyLogByStartDate(prevMonday)
				if err == nil && wl != nil && wl.Content != "" {
					weekTitle := dateutil.FormatWeeklyDateJapanese(wl.StartDate, wl.WeekNumber)
					sb.WriteString(renderExportCard("bg-blue-300", "text-blue-900", "【週次】"+weekTitle, wl.Content))

					// その月の最終週次ログなら月次ログを挿入
					hasLater, err := h.DB.HasLaterWeeklyLogInMonth(wl.Year, wl.Month, wl.StartDate)
					if err == nil && !hasLater {
						monthFirst := time.Date(wl.Year, time.Month(wl.Month), 1, 0, 0, 0, 0, dateutil.JST)
						ml, err := h.DB.GetMonthlyLogByFirstDay(monthFirst)
						if err == nil && ml != nil && ml.Content != "" {
							sb.WriteString(renderExportCard("bg-green-300", "text-green-900", fmt.Sprintf("【月次】%d年%d月", ml.Year, ml.Month), ml.Content))
						}
					}
				}
			}

			// デイリーログの挿入
			title := fmt.Sprintf("%s（%s）", dateutil.FormatDateJapanese(curDate), dateutil.GetWeekdayJapanese(curDate))
			body := log.Content
			if body == "" {
				body = "（内容なし）"
			}
			sb.WriteString(renderExportCard("bg-gray-200", "text-gray-700", title, body))
		}
	} else {
		sb.WriteString(`                <div class="text-center py-8 text-gray-500">
                    エクスポートするデータがありません
                </div>
`)
	}

	sb.WriteString(`            </div>
        </div>
    </main>
</body>
</html>`)

	exportPath, err := filepath.Abs("export.html")
	if err != nil {
		exportPath = "export.html"
	}
	if err := os.WriteFile(exportPath, []byte(sb.String()), 0644); err != nil {
		h.Logger.Error("エクスポートファイルの書き込み中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "エクスポート処理中にエラーが発生しました",
		})
		return
	}

	h.Logger.Info("日記データをエクスポートしました: %s", exportPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("日記データをexport.htmlにエクスポートしました（%d件）", len(dailyLogs)),
		"count":   len(dailyLogs),
		"path":    exportPath,
	})
}

// SearchAPI は日記・要約を検索して JSON で返します。
func (h *Handler) SearchAPI(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	logType := q.Get("type")
	if logType == "" {
		logType = "daily"
	}
	startDateStr := q.Get("start_date")
	endDateStr := q.Get("end_date")

	if startDateStr == "" || endDateStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "開始日と終了日を指定してください",
		})
		return
	}

	startDate, err := dateutil.ParseDate(startDateStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効な日付形式です",
		})
		return
	}
	endDate, err := dateutil.ParseDate(endDateStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効な日付形式です",
		})
		return
	}

	switch logType {
	case "daily", "weekly", "monthly", "yearly":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "無効な種別です",
		})
		return
	}

	results, totalCount, err := h.DB.SearchLogs(logType, startDate, endDate, 10)
	if err != nil {
		h.Logger.Error("検索処理中にエラーが発生しました: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "検索処理中にエラーが発生しました",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "success",
		"results":      results,
		"total_count":  totalCount,
		"is_truncated": totalCount > 10,
	})
}
