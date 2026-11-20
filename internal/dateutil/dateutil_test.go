package dateutil

import (
	"testing"
	"time"
)

func TestGetMondayOfWeek(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026-11-16", "2026-11-16"}, // 月曜
		{"2026-11-20", "2026-11-16"}, // 金曜
		{"2026-11-22", "2026-11-16"}, // 日曜
		{"2026-11-23", "2026-11-23"}, // 翌週月曜
	}
	for _, tc := range tests {
		dt, err := ParseDate(tc.input)
		if err != nil {
			t.Fatalf("ParseDate(%s) failed: %v", tc.input, err)
		}
		got := FormatISO(GetMondayOfWeek(dt))
		if got != tc.want {
			t.Errorf("GetMondayOfWeek(%s) = %s; want %s", tc.input, got, tc.want)
		}
	}
}

func TestGetWeekNumberInMonth(t *testing.T) {
	// 2026-11-01 は日曜、最初の月曜は 2026-11-02 (第1週)
	dt1, _ := ParseDate("2026-11-02")
	if got := GetWeekNumberInMonth(dt1); got != 1 {
		t.Errorf("GetWeekNumberInMonth(2026-11-02) = %d; want 1", got)
	}
	dt2, _ := ParseDate("2026-11-16")
	if got := GetWeekNumberInMonth(dt2); got != 3 {
		t.Errorf("GetWeekNumberInMonth(2026-11-16) = %d; want 3", got)
	}
}

func TestJapaneseFormatting(t *testing.T) {
	dt := time.Date(2026, 11, 20, 0, 0, 0, 0, JST)
	if got := FormatDateJapanese(dt); got != "2026年11月20日" {
		t.Errorf("FormatDateJapanese = %s; want 2026年11月20日", got)
	}
	if got := GetWeekdayJapanese(dt); got != "金" {
		t.Errorf("GetWeekdayJapanese = %s; want 金", got)
	}
	if got := FormatWeeklyDateJapanese(dt, 3); got != "2026年11月第3週" {
		t.Errorf("FormatWeeklyDateJapanese = %s; want 2026年11月第3週", got)
	}
}
