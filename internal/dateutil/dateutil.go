package dateutil

import (
	"fmt"
	"time"
)

// JST は日本標準時 (UTC+9) です。
var JST = time.FixedZone("JST", 9*60*60)

// NowJST は現在の日本時間を返します。
func NowJST() time.Time {
	return time.Now().In(JST)
}

// TodayJST は日本時間の今日の日付（時刻は00:00:00 JST）を返します。
func TodayJST() time.Time {
	now := NowJST()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, JST)
}

// ParseDate は "YYYY-MM-DD" 形式の文字列を JST の日付としてパースします。
func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, JST)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// FormatISO は日付を "YYYY-MM-DD" 形式にフォーマットします。
func FormatISO(t time.Time) string {
	return t.Format("2006-01-02")
}

// GetMondayOfWeek は指定日付が含まれる週の月曜日を返します。
func GetMondayOfWeek(targetDate time.Time) time.Time {
	// Go の Weekday: Sunday=0, Monday=1, ..., Saturday=6
	wd := int(targetDate.Weekday())
	daysSinceMonday := (wd + 6) % 7
	return targetDate.AddDate(0, 0, -daysSinceMonday)
}

// GetWeekNumberInMonth は指定日付がその月の第何週かを計算します（月曜日始まり）。
// その月に含まれる月曜日を起点に第1週とします。
func GetWeekNumberInMonth(targetDate time.Time) int {
	firstDayOfMonth := time.Date(targetDate.Year(), targetDate.Month(), 1, 0, 0, 0, 0, JST)
	firstMonday := GetMondayOfWeek(firstDayOfMonth)
	if firstMonday.Month() != targetDate.Month() {
		firstMonday = firstMonday.AddDate(0, 0, 7)
	}
	targetMonday := GetMondayOfWeek(targetDate)
	daysDiff := int(targetMonday.Sub(firstMonday).Hours() / 24)
	return (daysDiff / 7) + 1
}

// FormatDateJapanese は日付を日本語形式（例: 2024年1月15日）でフォーマットします。
func FormatDateJapanese(t time.Time) string {
	return fmt.Sprintf("%d年%d月%d日", t.Year(), int(t.Month()), t.Day())
}

// GetWeekdayJapanese は日付から日本語の曜日（月〜日）を返します。
func GetWeekdayJapanese(t time.Time) string {
	// Python weekday(): 月=0 ... 日=6
	weekdays := []string{"日", "月", "火", "水", "木", "金", "土"}
	return weekdays[int(t.Weekday())]
}

// FormatWeeklyDateJapanese は週の日付を日本語形式（例: 2024年1月第1週）でフォーマットします。
func FormatWeeklyDateJapanese(startDate time.Time, weekNumber int) string {
	return fmt.Sprintf("%d年%d月第%d週", startDate.Year(), int(startDate.Month()), weekNumber)
}
