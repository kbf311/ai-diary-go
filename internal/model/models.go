package model

import "time"

// DailyLog はデイリー日記テーブルのモデルです。
type DailyLog struct {
	ID          int64
	Date        time.Time
	Content     string
	IsCompleted bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WeeklyLog はウィークリー日記テーブルのモデルです。
type WeeklyLog struct {
	ID          int64
	StartDate   time.Time
	Content     string
	IsCompleted bool
	Year        int
	Month       int
	WeekNumber  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// MonthlyLog はマンスリー日記テーブルのモデルです。
type MonthlyLog struct {
	ID          int64
	FirstDay    time.Time
	Year        int
	Month       int
	Content     string
	IsCompleted bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// YearlyLog は年次ログテーブルのモデルです。
type YearlyLog struct {
	ID          int64
	FirstDay    time.Time
	Year        int
	Content     string
	IsCompleted bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Prompt はプロンプトテンプレートテーブルのモデルです。
type Prompt struct {
	ID        int64
	LogType   string
	Template  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SystemConfig はシステム設定テーブルのモデルです。
type SystemConfig struct {
	Key   string
	Value string
}
