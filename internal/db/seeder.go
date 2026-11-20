package db

import (
	"fmt"
	"time"

	"ai-diary-go/internal/constants"
	"ai-diary-go/internal/dateutil"
)

// TruncateAllTables はすべてのテーブルのデータを削除します。
func (d *DB) TruncateAllTables() error {
	tx, err := d.Conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	tables := []string{
		"daily_logs",
		"weekly_logs",
		"monthly_logs",
		"yearly_logs",
		"prompts",
		"system_config",
	}

	for _, tbl := range tables {
		if _, err = tx.Exec("DELETE FROM " + tbl); err != nil {
			d.Logger.Error("テーブルのデータ削除中にエラーが発生しました (%s): %v", tbl, err)
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		d.Logger.Error("テーブルのデータ削除のコミット中にエラーが発生しました: %v", err)
		return err
	}

	d.Logger.Info("全てのテーブルのデータを削除しました。")
	return nil
}

// SeedInitialData は初期データ（日・週・月・年の枠、システム設定、プロンプト）を投入します。
func (d *DB) SeedInitialData(preGenerateYears int) error {
	startDate, err := d.GetStartDate()
	if err != nil {
		return fmt.Errorf("システム開始日の取得に失敗しました: %w", err)
	}

	today := dateutil.TodayJST()
	endDate := time.Date(today.Year()+preGenerateYears, 12, 31, 0, 0, 0, 0, dateutil.JST)

	d.Logger.Info("初期データ投入を開始します...")
	d.Logger.Info("開始日: %s, 終了日: %s", dateutil.FormatISO(startDate), dateutil.FormatISO(endDate))

	if err := d.generateDailyLogs(startDate, endDate); err != nil {
		return err
	}
	if err := d.generateWeeklyLogs(startDate, endDate); err != nil {
		return err
	}
	if err := d.generateMonthlyLogs(startDate, endDate); err != nil {
		return err
	}
	if err := d.generateYearlyLogs(startDate, endDate); err != nil {
		return err
	}
	if err := d.seedPrompts(); err != nil {
		return err
	}

	d.Logger.Info("初期データ投入が完了しました。")
	return nil
}

func (d *DB) generateDailyLogs(startDate, endDate time.Time) error {
	existing := make(map[string]bool)
	rows, err := d.Conn.Query(
		d.Rebind("SELECT date FROM daily_logs WHERE date >= ? AND date <= ?"),
		dateutil.FormatISO(startDate), dateutil.FormatISO(endDate),
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rawDate any
		if err := rows.Scan(&rawDate); err != nil {
			_ = rows.Close()
			return err
		}
		dt, err := parseDBDate(rawDate)
		if err == nil {
			existing[dateutil.FormatISO(dt)] = true
		}
	}
	_ = rows.Close()

	tx, err := d.Conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(d.Rebind("INSERT INTO daily_logs (date, is_completed, created_at, updated_at) VALUES (?, ?, ?, ?)"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	count := 0
	for cur := startDate; !cur.After(endDate); cur = cur.AddDate(0, 0, 1) {
		iso := dateutil.FormatISO(cur)
		if !existing[iso] {
			if _, err := stmt.Exec(iso, false, now, now); err != nil {
				_ = tx.Rollback()
				return err
			}
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if count > 0 {
		d.Logger.Info("デイリーログ: %d件を生成しました。", count)
	}
	return nil
}

func (d *DB) generateWeeklyLogs(startDate, endDate time.Time) error {
	currentMonday := dateutil.GetMondayOfWeek(startDate)

	existing := make(map[string]bool)
	rows, err := d.Conn.Query(
		d.Rebind("SELECT start_date FROM weekly_logs WHERE start_date >= ? AND start_date <= ?"),
		dateutil.FormatISO(currentMonday), dateutil.FormatISO(endDate),
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rawDate any
		if err := rows.Scan(&rawDate); err != nil {
			_ = rows.Close()
			return err
		}
		dt, err := parseDBDate(rawDate)
		if err == nil {
			existing[dateutil.FormatISO(dt)] = true
		}
	}
	_ = rows.Close()

	tx, err := d.Conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(d.Rebind("INSERT INTO weekly_logs (start_date, year, month, week_number, is_completed, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	count := 0
	for cur := currentMonday; !cur.After(endDate); cur = cur.AddDate(0, 0, 7) {
		iso := dateutil.FormatISO(cur)
		if !existing[iso] {
			weekNum := dateutil.GetWeekNumberInMonth(cur)
			if _, err := stmt.Exec(iso, cur.Year(), int(cur.Month()), weekNum, false, now, now); err != nil {
				_ = tx.Rollback()
				return err
			}
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if count > 0 {
		d.Logger.Info("ウィークリーログ: %d件を生成しました。", count)
	}
	return nil
}

func (d *DB) generateMonthlyLogs(startDate, endDate time.Time) error {
	currentFirstDay := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, dateutil.JST)

	existing := make(map[string]bool)
	rows, err := d.Conn.Query(
		d.Rebind("SELECT first_day FROM monthly_logs WHERE first_day >= ? AND first_day <= ?"),
		dateutil.FormatISO(currentFirstDay), dateutil.FormatISO(endDate),
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rawDate any
		if err := rows.Scan(&rawDate); err != nil {
			_ = rows.Close()
			return err
		}
		dt, err := parseDBDate(rawDate)
		if err == nil {
			existing[dateutil.FormatISO(dt)] = true
		}
	}
	_ = rows.Close()

	tx, err := d.Conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(d.Rebind("INSERT INTO monthly_logs (first_day, year, month, is_completed, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	count := 0
	for cur := currentFirstDay; !cur.After(endDate); cur = cur.AddDate(0, 1, 0) {
		iso := dateutil.FormatISO(cur)
		if !existing[iso] {
			if _, err := stmt.Exec(iso, cur.Year(), int(cur.Month()), false, now, now); err != nil {
				_ = tx.Rollback()
				return err
			}
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if count > 0 {
		d.Logger.Info("マンスリーログ: %d件を生成しました。", count)
	}
	return nil
}

func (d *DB) generateYearlyLogs(startDate, endDate time.Time) error {
	currentFirstDay := time.Date(startDate.Year(), 1, 1, 0, 0, 0, 0, dateutil.JST)

	existing := make(map[string]bool)
	rows, err := d.Conn.Query(
		d.Rebind("SELECT first_day FROM yearly_logs WHERE first_day >= ? AND first_day <= ?"),
		dateutil.FormatISO(currentFirstDay), dateutil.FormatISO(endDate),
	)
	if err != nil {
		return err
	}
	for rows.Next() {
		var rawDate any
		if err := rows.Scan(&rawDate); err != nil {
			_ = rows.Close()
			return err
		}
		dt, err := parseDBDate(rawDate)
		if err == nil {
			existing[dateutil.FormatISO(dt)] = true
		}
	}
	_ = rows.Close()

	tx, err := d.Conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(d.Rebind("INSERT INTO yearly_logs (first_day, year, is_completed, created_at, updated_at) VALUES (?, ?, ?, ?, ?)"))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	count := 0
	for cur := currentFirstDay; cur.Year() <= endDate.Year(); cur = cur.AddDate(1, 0, 0) {
		iso := dateutil.FormatISO(cur)
		if !existing[iso] {
			if _, err := stmt.Exec(iso, cur.Year(), false, now, now); err != nil {
				_ = tx.Rollback()
				return err
			}
			count++
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	if count > 0 {
		d.Logger.Info("イヤリーログ: %d件を生成しました。", count)
	}
	return nil
}

func (d *DB) seedPrompts() error {
	existing := make(map[string]bool)
	rows, err := d.Conn.Query("SELECT log_type FROM prompts")
	if err != nil {
		return err
	}
	for rows.Next() {
		var lt string
		if err := rows.Scan(&lt); err != nil {
			_ = rows.Close()
			return err
		}
		existing[lt] = true
	}
	_ = rows.Close()

	now := time.Now()
	count := 0
	for _, logType := range []string{"daily", "weekly", "monthly", "yearly"} {
		if !existing[logType] {
			tmpl := constants.SeedPrompts[logType]
			_, err := d.Conn.Exec(
				d.Rebind("INSERT INTO prompts (log_type, template, created_at, updated_at) VALUES (?, ?, ?, ?)"),
				logType, tmpl, now, now,
			)
			if err != nil {
				return err
			}
			count++
		}
	}

	if count > 0 {
		d.Logger.Info("プロンプト: %d件を生成しました。", count)
	} else {
		d.Logger.Info("プロンプトは既に全て設定されています。")
	}
	return nil
}
