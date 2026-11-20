package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"ai-diary-go/internal/config"
	"ai-diary-go/internal/dateutil"
	"ai-diary-go/internal/logger"
	"ai-diary-go/internal/model"
)

// DB はデータベース接続とクエリ操作をラップします。
type DB struct {
	Conn       *sql.DB
	IsPostgres bool
	Logger     *logger.Logger
}

// Open は設定に基づいて SQLite または PostgreSQL に接続します。
func Open(cfg *config.Config, log *logger.Logger) (*DB, error) {
	var conn *sql.DB
	var err error
	isPostgres := cfg.DatabaseType != "sqlite"

	if !isPostgres {
		conn, err = sql.Open("sqlite", cfg.SQLitePath)
		if err != nil {
			return nil, fmt.Errorf("SQLite接続のオープンに失敗しました: %w", err)
		}
		// SQLite でのロック回避とタイムアウト設定
		_, _ = conn.Exec("PRAGMA busy_timeout = 5000;")
	} else {
		conn, err = sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("PostgreSQL接続のオープンに失敗しました: %w", err)
		}
	}

	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("データベースへのPingに失敗しました: %w", err)
	}

	return &DB{
		Conn:       conn,
		IsPostgres: isPostgres,
		Logger:     log,
	}, nil
}

// Close はデータベース接続を閉じます。
func (d *DB) Close() error {
	if d.Conn != nil {
		return d.Conn.Close()
	}
	return nil
}

// Rebind は "?" プレースホルダを PostgreSQL 用の "$1, $2, ..." に変換します。
func (d *DB) Rebind(query string) string {
	if !d.IsPostgres {
		return query
	}
	var sb strings.Builder
	idx := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			fmt.Fprintf(&sb, "$%d", idx)
			idx++
		} else {
			sb.WriteByte(query[i])
		}
	}
	return sb.String()
}

// InitTables は存在しないテーブルとインデックスを作成します。
func (d *DB) InitTables() error {
	pkDef := "INTEGER PRIMARY KEY AUTOINCREMENT"
	boolDefaultFalse := "BOOLEAN DEFAULT 0"
	nowDefault := "DATETIME DEFAULT CURRENT_TIMESTAMP"
	if d.IsPostgres {
		pkDef = "SERIAL PRIMARY KEY"
		boolDefaultFalse = "BOOLEAN DEFAULT FALSE"
		nowDefault = "TIMESTAMP DEFAULT CURRENT_TIMESTAMP"
	}

	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS daily_logs (
			id %s,
			date DATE NOT NULL UNIQUE,
			content TEXT,
			is_completed %s,
			created_at %s,
			updated_at %s
		)`, pkDef, boolDefaultFalse, nowDefault, nowDefault),
		`CREATE INDEX IF NOT EXISTS ix_daily_logs_date ON daily_logs (date)`,
		`CREATE INDEX IF NOT EXISTS ix_daily_logs_is_completed ON daily_logs (is_completed)`,

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS weekly_logs (
			id %s,
			start_date DATE NOT NULL UNIQUE,
			content TEXT,
			is_completed %s,
			year INTEGER,
			month INTEGER,
			week_number INTEGER,
			created_at %s,
			updated_at %s
		)`, pkDef, boolDefaultFalse, nowDefault, nowDefault),
		`CREATE INDEX IF NOT EXISTS ix_weekly_logs_start_date ON weekly_logs (start_date)`,
		`CREATE INDEX IF NOT EXISTS ix_weekly_logs_is_completed ON weekly_logs (is_completed)`,
		`CREATE INDEX IF NOT EXISTS ix_weekly_logs_year ON weekly_logs (year)`,
		`CREATE INDEX IF NOT EXISTS ix_weekly_logs_month ON weekly_logs (month)`,
		`CREATE INDEX IF NOT EXISTS ix_weekly_logs_week_number ON weekly_logs (week_number)`,

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS monthly_logs (
			id %s,
			first_day DATE NOT NULL UNIQUE,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			content TEXT,
			is_completed %s,
			created_at %s,
			updated_at %s
		)`, pkDef, boolDefaultFalse, nowDefault, nowDefault),
		`CREATE INDEX IF NOT EXISTS ix_monthly_logs_first_day ON monthly_logs (first_day)`,
		`CREATE INDEX IF NOT EXISTS ix_monthly_logs_year ON monthly_logs (year)`,
		`CREATE INDEX IF NOT EXISTS ix_monthly_logs_month ON monthly_logs (month)`,
		`CREATE INDEX IF NOT EXISTS ix_monthly_logs_is_completed ON monthly_logs (is_completed)`,

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS yearly_logs (
			id %s,
			first_day DATE NOT NULL UNIQUE,
			year INTEGER NOT NULL UNIQUE,
			content TEXT,
			is_completed %s,
			created_at %s,
			updated_at %s
		)`, pkDef, boolDefaultFalse, nowDefault, nowDefault),
		`CREATE INDEX IF NOT EXISTS ix_yearly_logs_first_day ON yearly_logs (first_day)`,
		`CREATE INDEX IF NOT EXISTS ix_yearly_logs_year ON yearly_logs (year)`,
		`CREATE INDEX IF NOT EXISTS ix_yearly_logs_is_completed ON yearly_logs (is_completed)`,

		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS prompts (
			id %s,
			log_type VARCHAR(20) NOT NULL UNIQUE,
			template TEXT,
			created_at %s,
			updated_at %s
		)`, pkDef, nowDefault, nowDefault),

		`CREATE TABLE IF NOT EXISTS system_config (
			key VARCHAR(50) PRIMARY KEY,
			value VARCHAR(255)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := d.Conn.Exec(stmt); err != nil {
			return fmt.Errorf("テーブル初期化クエリの実行に失敗しました: %w", err)
		}
	}

	d.Logger.Info("データベースの初期化が完了しました。")
	return nil
}

// parseDBDate は SQLite/PostgreSQL から取得した日付値を JST の time.Time に変換します。
func parseDBDate(raw any) (time.Time, error) {
	switch v := raw.(type) {
	case time.Time:
		return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, dateutil.JST), nil
	case string:
		if len(v) >= 10 {
			return dateutil.ParseDate(v[:10])
		}
		return time.Time{}, fmt.Errorf("不正な日付文字列: %s", v)
	case []byte:
		s := string(v)
		if len(s) >= 10 {
			return dateutil.ParseDate(s[:10])
		}
		return time.Time{}, fmt.Errorf("不正な日付バイト列: %s", s)
	default:
		return time.Time{}, fmt.Errorf("未対応の日付型: %T", raw)
	}
}

func (d *DB) notCompletedCondition() string {
	if d.IsPostgres {
		return "(is_completed IS NULL OR is_completed = FALSE)"
	}
	return "(is_completed IS NULL OR is_completed = 0)"
}

func (d *DB) completedCondition() string {
	if d.IsPostgres {
		return "is_completed = TRUE"
	}
	return "is_completed = 1"
}

// GetStartDate はシステム開始日を取得します（なければ今日を設定して保存します）。
func (d *DB) GetStartDate() (time.Time, error) {
	var val string
	err := d.Conn.QueryRow(d.Rebind("SELECT value FROM system_config WHERE key = ?"), "start_date").Scan(&val)
	if err == nil {
		return dateutil.ParseDate(val)
	}
	if err != sql.ErrNoRows {
		return time.Time{}, err
	}

	today := dateutil.TodayJST()
	_, err = d.Conn.Exec(
		d.Rebind("INSERT INTO system_config (key, value) VALUES (?, ?)"),
		"start_date", dateutil.FormatISO(today),
	)
	if err != nil {
		return time.Time{}, err
	}
	return today, nil
}

// GetDailyAlerts は未完了のデイリーログを取得します。
func (d *DB) GetDailyAlerts(startDate, today time.Time, limit int) ([]model.DailyLog, error) {
	query := fmt.Sprintf(
		`SELECT id, date, content, is_completed FROM daily_logs
		 WHERE date >= ? AND date <= ? AND %s
		 ORDER BY date ASC LIMIT ?`,
		d.notCompletedCondition(),
	)
	rows, err := d.Conn.Query(d.Rebind(query), dateutil.FormatISO(startDate), dateutil.FormatISO(today), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.DailyLog
	for rows.Next() {
		var l model.DailyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.Date = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetWeeklyAlerts は今週より前の未完了ウィークリーログを取得します。
func (d *DB) GetWeeklyAlerts(startDateMonday, currentWeekMonday time.Time, limit int) ([]model.WeeklyLog, error) {
	query := fmt.Sprintf(
		`SELECT id, start_date, content, is_completed, year, month, week_number FROM weekly_logs
		 WHERE start_date >= ? AND start_date < ? AND %s
		 ORDER BY start_date ASC LIMIT ?`,
		d.notCompletedCondition(),
	)
	rows, err := d.Conn.Query(d.Rebind(query), dateutil.FormatISO(startDateMonday), dateutil.FormatISO(currentWeekMonday), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.WeeklyLog
	for rows.Next() {
		var l model.WeeklyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		var y, m, wn sql.NullInt64
		if err := rows.Scan(&l.ID, &rawDate, &content, &completed, &y, &m, &wn); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.StartDate = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		l.Year = int(y.Int64)
		l.Month = int(m.Int64)
		l.WeekNumber = int(wn.Int64)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetMonthlyAlerts は今月より前の未完了マンスリーログを取得します。
func (d *DB) GetMonthlyAlerts(startMonthFirst, currentMonthFirst time.Time, limit int) ([]model.MonthlyLog, error) {
	query := fmt.Sprintf(
		`SELECT id, first_day, year, month, content, is_completed FROM monthly_logs
		 WHERE first_day >= ? AND first_day < ? AND %s
		 ORDER BY first_day ASC LIMIT ?`,
		d.notCompletedCondition(),
	)
	rows, err := d.Conn.Query(d.Rebind(query), dateutil.FormatISO(startMonthFirst), dateutil.FormatISO(currentMonthFirst), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.MonthlyLog
	for rows.Next() {
		var l model.MonthlyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &l.Year, &l.Month, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.FirstDay = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetYearlyAlerts は今年より前の未完了イヤリーログを取得します。
func (d *DB) GetYearlyAlerts(startYearFirst, currentYearFirst time.Time, limit int) ([]model.YearlyLog, error) {
	query := fmt.Sprintf(
		`SELECT id, first_day, year, content, is_completed FROM yearly_logs
		 WHERE first_day >= ? AND first_day < ? AND %s
		 ORDER BY first_day ASC LIMIT ?`,
		d.notCompletedCondition(),
	)
	rows, err := d.Conn.Query(d.Rebind(query), dateutil.FormatISO(startYearFirst), dateutil.FormatISO(currentYearFirst), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.YearlyLog
	for rows.Next() {
		var l model.YearlyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &l.Year, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.FirstDay = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetPromptByType は log_type に一致するプロンプトを取得します。
func (d *DB) GetPromptByType(logType string) (*model.Prompt, error) {
	var p model.Prompt
	var tmpl sql.NullString
	err := d.Conn.QueryRow(
		d.Rebind("SELECT id, log_type, template FROM prompts WHERE log_type = ?"),
		logType,
	).Scan(&p.ID, &p.LogType, &tmpl)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Template = tmpl.String
	return &p, nil
}

// UpsertPrompt はプロンプトを更新または新規作成します。
func (d *DB) UpsertPrompt(logType, template string) error {
	existing, err := d.GetPromptByType(logType)
	if err != nil {
		return err
	}
	now := time.Now()
	if existing != nil {
		_, err = d.Conn.Exec(
			d.Rebind("UPDATE prompts SET template = ?, updated_at = ? WHERE log_type = ?"),
			template, now, logType,
		)
		return err
	}
	_, err = d.Conn.Exec(
		d.Rebind("INSERT INTO prompts (log_type, template, created_at, updated_at) VALUES (?, ?, ?, ?)"),
		logType, template, now, now,
	)
	return err
}

// GetDailyLogByDate は指定日付のデイリーログを取得します。
func (d *DB) GetDailyLogByDate(targetDate time.Time) (*model.DailyLog, error) {
	var l model.DailyLog
	var rawDate any
	var content sql.NullString
	var completed sql.NullBool
	err := d.Conn.QueryRow(
		d.Rebind("SELECT id, date, content, is_completed FROM daily_logs WHERE date = ?"),
		dateutil.FormatISO(targetDate),
	).Scan(&l.ID, &rawDate, &content, &completed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dt, err := parseDBDate(rawDate)
	if err != nil {
		return nil, err
	}
	l.Date = dt
	l.Content = content.String
	l.IsCompleted = completed.Bool
	return &l, nil
}

// UpdateDailyLog はデイリーログの内容と完了フラグを更新します。
func (d *DB) UpdateDailyLog(targetDate time.Time, content string, isCompleted bool) error {
	_, err := d.Conn.Exec(
		d.Rebind("UPDATE daily_logs SET content = ?, is_completed = ?, updated_at = ? WHERE date = ?"),
		content, isCompleted, time.Now(), dateutil.FormatISO(targetDate),
	)
	return err
}

// GetDailyLogsBetween は startDate から endDate までのデイリーログを昇順で取得します。
func (d *DB) GetDailyLogsBetween(startDate, endDate time.Time) ([]model.DailyLog, error) {
	rows, err := d.Conn.Query(
		d.Rebind("SELECT id, date, content, is_completed FROM daily_logs WHERE date >= ? AND date <= ? ORDER BY date ASC"),
		dateutil.FormatISO(startDate), dateutil.FormatISO(endDate),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.DailyLog
	for rows.Next() {
		var l model.DailyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.Date = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetWeeklyLogByID は ID でウィークリーログを取得します。
func (d *DB) GetWeeklyLogByID(id int64) (*model.WeeklyLog, error) {
	return d.scanSingleWeeklyLog("SELECT id, start_date, content, is_completed, year, month, week_number FROM weekly_logs WHERE id = ?", id)
}

// GetWeeklyLogByStartDate は開始日でウィークリーログを取得します。
func (d *DB) GetWeeklyLogByStartDate(startDate time.Time) (*model.WeeklyLog, error) {
	return d.scanSingleWeeklyLog("SELECT id, start_date, content, is_completed, year, month, week_number FROM weekly_logs WHERE start_date = ?", dateutil.FormatISO(startDate))
}

func (d *DB) scanSingleWeeklyLog(query string, arg any) (*model.WeeklyLog, error) {
	var l model.WeeklyLog
	var rawDate any
	var content sql.NullString
	var completed sql.NullBool
	var y, m, wn sql.NullInt64
	err := d.Conn.QueryRow(d.Rebind(query), arg).Scan(&l.ID, &rawDate, &content, &completed, &y, &m, &wn)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dt, err := parseDBDate(rawDate)
	if err != nil {
		return nil, err
	}
	l.StartDate = dt
	l.Content = content.String
	l.IsCompleted = completed.Bool
	l.Year = int(y.Int64)
	l.Month = int(m.Int64)
	l.WeekNumber = int(wn.Int64)
	return &l, nil
}

// UpdateWeeklyLog はウィークリーログを更新します。
func (d *DB) UpdateWeeklyLog(id int64, content string, isCompleted bool) error {
	_, err := d.Conn.Exec(
		d.Rebind("UPDATE weekly_logs SET content = ?, is_completed = ?, updated_at = ? WHERE id = ?"),
		content, isCompleted, time.Now(), id,
	)
	return err
}

// HasLaterWeeklyLogInMonth は同じ年・月で startDate より後の週次ログが存在するか確認します。
func (d *DB) HasLaterWeeklyLogInMonth(year, month int, startDate time.Time) (bool, error) {
	var dummy int64
	err := d.Conn.QueryRow(
		d.Rebind("SELECT id FROM weekly_logs WHERE year = ? AND month = ? AND start_date > ? LIMIT 1"),
		year, month, dateutil.FormatISO(startDate),
	).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetWeeklyLogsByYearMonth は指定した年月・月番号昇順のウィークリーログを取得します。
func (d *DB) GetWeeklyLogsByYearMonth(year, month int) ([]model.WeeklyLog, error) {
	rows, err := d.Conn.Query(
		d.Rebind("SELECT id, start_date, content, is_completed, year, month, week_number FROM weekly_logs WHERE year = ? AND month = ? ORDER BY week_number ASC"),
		year, month,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.WeeklyLog
	for rows.Next() {
		var l model.WeeklyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		var y, m, wn sql.NullInt64
		if err := rows.Scan(&l.ID, &rawDate, &content, &completed, &y, &m, &wn); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.StartDate = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		l.Year = int(y.Int64)
		l.Month = int(m.Int64)
		l.WeekNumber = int(wn.Int64)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetMonthlyLogByID は ID でマンスリーログを取得します。
func (d *DB) GetMonthlyLogByID(id int64) (*model.MonthlyLog, error) {
	return d.scanSingleMonthlyLog("SELECT id, first_day, year, month, content, is_completed FROM monthly_logs WHERE id = ?", id)
}

// GetMonthlyLogByFirstDay は月初日でマンスリーログを取得します。
func (d *DB) GetMonthlyLogByFirstDay(firstDay time.Time) (*model.MonthlyLog, error) {
	return d.scanSingleMonthlyLog("SELECT id, first_day, year, month, content, is_completed FROM monthly_logs WHERE first_day = ?", dateutil.FormatISO(firstDay))
}

func (d *DB) scanSingleMonthlyLog(query string, arg any) (*model.MonthlyLog, error) {
	var l model.MonthlyLog
	var rawDate any
	var content sql.NullString
	var completed sql.NullBool
	err := d.Conn.QueryRow(d.Rebind(query), arg).Scan(&l.ID, &rawDate, &l.Year, &l.Month, &content, &completed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dt, err := parseDBDate(rawDate)
	if err != nil {
		return nil, err
	}
	l.FirstDay = dt
	l.Content = content.String
	l.IsCompleted = completed.Bool
	return &l, nil
}

// UpdateMonthlyLog はマンスリーログを更新します。
func (d *DB) UpdateMonthlyLog(id int64, content string, isCompleted bool) error {
	_, err := d.Conn.Exec(
		d.Rebind("UPDATE monthly_logs SET content = ?, is_completed = ?, updated_at = ? WHERE id = ?"),
		content, isCompleted, time.Now(), id,
	)
	return err
}

// GetMonthlyLogsByYear は指定年のマンスリーログを月昇順で取得します。
func (d *DB) GetMonthlyLogsByYear(year int) ([]model.MonthlyLog, error) {
	rows, err := d.Conn.Query(
		d.Rebind("SELECT id, first_day, year, month, content, is_completed FROM monthly_logs WHERE year = ? ORDER BY month ASC"),
		year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.MonthlyLog
	for rows.Next() {
		var l model.MonthlyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &l.Year, &l.Month, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.FirstDay = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// GetYearlyLogByID は ID でイヤリーログを取得します。
func (d *DB) GetYearlyLogByID(id int64) (*model.YearlyLog, error) {
	return d.scanSingleYearlyLog("SELECT id, first_day, year, content, is_completed FROM yearly_logs WHERE id = ?", id)
}

// GetYearlyLogByFirstDay は元日でイヤリーログを取得します。
func (d *DB) GetYearlyLogByFirstDay(firstDay time.Time) (*model.YearlyLog, error) {
	return d.scanSingleYearlyLog("SELECT id, first_day, year, content, is_completed FROM yearly_logs WHERE first_day = ?", dateutil.FormatISO(firstDay))
}

func (d *DB) scanSingleYearlyLog(query string, arg any) (*model.YearlyLog, error) {
	var l model.YearlyLog
	var rawDate any
	var content sql.NullString
	var completed sql.NullBool
	err := d.Conn.QueryRow(d.Rebind(query), arg).Scan(&l.ID, &rawDate, &l.Year, &content, &completed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dt, err := parseDBDate(rawDate)
	if err != nil {
		return nil, err
	}
	l.FirstDay = dt
	l.Content = content.String
	l.IsCompleted = completed.Bool
	return &l, nil
}

// GetYearlyLogsBeforeYear は指定年より前のイヤリーログを年昇順で取得します。
func (d *DB) GetYearlyLogsBeforeYear(year int) ([]model.YearlyLog, error) {
	rows, err := d.Conn.Query(
		d.Rebind("SELECT id, first_day, year, content, is_completed FROM yearly_logs WHERE year < ? ORDER BY year ASC"),
		year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.YearlyLog
	for rows.Next() {
		var l model.YearlyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &l.Year, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.FirstDay = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// UpdateYearlyLog はイヤリーログを更新します。
func (d *DB) UpdateYearlyLog(id int64, content string, isCompleted bool) error {
	_, err := d.Conn.Exec(
		d.Rebind("UPDATE yearly_logs SET content = ?, is_completed = ?, updated_at = ? WHERE id = ?"),
		content, isCompleted, time.Now(), id,
	)
	return err
}

// GetCompletedDailyLogs は完了済みのデイリーログを日付昇順で取得します。
func (d *DB) GetCompletedDailyLogs() ([]model.DailyLog, error) {
	query := fmt.Sprintf(
		"SELECT id, date, content, is_completed FROM daily_logs WHERE %s ORDER BY date ASC",
		d.completedCondition(),
	)
	rows, err := d.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.DailyLog
	for rows.Next() {
		var l model.DailyLog
		var rawDate any
		var content sql.NullString
		var completed sql.NullBool
		if err := rows.Scan(&l.ID, &rawDate, &content, &completed); err != nil {
			return nil, err
		}
		dt, err := parseDBDate(rawDate)
		if err != nil {
			return nil, err
		}
		l.Date = dt
		l.Content = content.String
		l.IsCompleted = completed.Bool
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// SearchResult は検索APIの1件分の結果を表現します。
type SearchResult struct {
	ID      int64  `json:"id"`
	Date    string `json:"date"`
	DateJP  string `json:"date_jp"`
	Content string `json:"content"`
}

// SearchLogs は種別と期間でログを検索し、最大 limit 件の結果と総数を返します。
func (d *DB) SearchLogs(logType string, startDate, endDate time.Time, limit int) ([]SearchResult, int, error) {
	startStr := dateutil.FormatISO(startDate)
	endStr := dateutil.FormatISO(endDate)

	var countQuery, listQuery string
	switch logType {
	case "daily":
		countQuery = "SELECT COUNT(*) FROM daily_logs WHERE date >= ? AND date <= ?"
		listQuery = "SELECT id, date, content FROM daily_logs WHERE date >= ? AND date <= ? ORDER BY date ASC LIMIT ?"
	case "weekly":
		countQuery = "SELECT COUNT(*) FROM weekly_logs WHERE start_date >= ? AND start_date <= ?"
		listQuery = "SELECT id, start_date, week_number, content FROM weekly_logs WHERE start_date >= ? AND start_date <= ? ORDER BY start_date ASC LIMIT ?"
	case "monthly":
		countQuery = "SELECT COUNT(*) FROM monthly_logs WHERE first_day >= ? AND first_day <= ?"
		listQuery = "SELECT id, first_day, year, month, content FROM monthly_logs WHERE first_day >= ? AND first_day <= ? ORDER BY first_day ASC LIMIT ?"
	case "yearly":
		countQuery = "SELECT COUNT(*) FROM yearly_logs WHERE first_day >= ? AND first_day <= ?"
		listQuery = "SELECT id, first_day, year, content FROM yearly_logs WHERE first_day >= ? AND first_day <= ? ORDER BY first_day ASC LIMIT ?"
	default:
		return nil, 0, fmt.Errorf("無効な種別です")
	}

	var totalCount int
	if err := d.Conn.QueryRow(d.Rebind(countQuery), startStr, endStr).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	rows, err := d.Conn.Query(d.Rebind(listQuery), startStr, endStr, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	results := make([]SearchResult, 0)
	for rows.Next() {
		switch logType {
		case "daily":
			var id int64
			var rawDate any
			var content sql.NullString
			if err := rows.Scan(&id, &rawDate, &content); err != nil {
				return nil, 0, err
			}
			dt, err := parseDBDate(rawDate)
			if err != nil {
				return nil, 0, err
			}
			results = append(results, SearchResult{
				ID:      id,
				Date:    dateutil.FormatISO(dt),
				DateJP:  fmt.Sprintf("%s（%s）", dateutil.FormatDateJapanese(dt), dateutil.GetWeekdayJapanese(dt)),
				Content: content.String,
			})
		case "weekly":
			var id int64
			var rawDate any
			var wn sql.NullInt64
			var content sql.NullString
			if err := rows.Scan(&id, &rawDate, &wn, &content); err != nil {
				return nil, 0, err
			}
			dt, err := parseDBDate(rawDate)
			if err != nil {
				return nil, 0, err
			}
			results = append(results, SearchResult{
				ID:      id,
				Date:    dateutil.FormatISO(dt),
				DateJP:  dateutil.FormatWeeklyDateJapanese(dt, int(wn.Int64)),
				Content: content.String,
			})
		case "monthly":
			var id int64
			var rawDate any
			var y, m int
			var content sql.NullString
			if err := rows.Scan(&id, &rawDate, &y, &m, &content); err != nil {
				return nil, 0, err
			}
			dt, err := parseDBDate(rawDate)
			if err != nil {
				return nil, 0, err
			}
			results = append(results, SearchResult{
				ID:      id,
				Date:    dateutil.FormatISO(dt),
				DateJP:  fmt.Sprintf("%d年%d月", y, m),
				Content: content.String,
			})
		case "yearly":
			var id int64
			var rawDate any
			var y int
			var content sql.NullString
			if err := rows.Scan(&id, &rawDate, &y, &content); err != nil {
				return nil, 0, err
			}
			dt, err := parseDBDate(rawDate)
			if err != nil {
				return nil, 0, err
			}
			results = append(results, SearchResult{
				ID:      id,
				Date:    dateutil.FormatISO(dt),
				DateJP:  fmt.Sprintf("%d年", y),
				Content: content.String,
			})
		}
	}
	return results, totalCount, rows.Err()
}
