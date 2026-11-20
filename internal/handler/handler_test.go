package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-diary-go/internal/config"
	"ai-diary-go/internal/dateutil"
	"ai-diary-go/internal/db"
	"ai-diary-go/internal/logger"
)

func setupTestServer(t *testing.T) (*http.ServeMux, *db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// templates と static はプロジェクトルートにあるため相対パスで参照できるよう調整
	templateDir := filepath.Join(origDir, "..", "..", "templates")
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	log, err := logger.New(filepath.Join(tmpDir, "logs"), "test.log")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		DatabaseType:     "sqlite",
		SQLitePath:       filepath.Join(tmpDir, "test.db"),
		PreGenerateYears: 1,
		Port:             "5000",
	}

	database, err := db.Open(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.InitTables(); err != nil {
		t.Fatal(err)
	}
	if err := database.SeedInitialData(cfg.PreGenerateYears); err != nil {
		t.Fatal(err)
	}

	h, err := New(database, log, templateDir)
	if err != nil {
		_ = database.Close()
		_ = log.Close()
		_ = os.Chdir(origDir)
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	cleanup := func() {
		_ = database.Close()
		_ = log.Close()
		_ = os.Chdir(origDir)
	}
	return mux, database, cleanup
}

func TestPagesAndAPIs(t *testing.T) {
	mux, _, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. ページ表示チェック
	pages := []string{"/", "/dashboard", "/journal", "/milestones", "/config", "/export", "/summary?type=weekly&id=1"}
	for _, p := range pages {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s returned status %d; want 200", p, rec.Code)
		}
	}

	// 2. プロンプト取得と更新
	req := httptest.NewRequest("GET", "/api/prompt/daily", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/prompt/daily status = %d; want 200", rec.Code)
	}

	form := url.Values{}
	form.Set("daily_prompt", "updated daily")
	form.Set("weekly_prompt", "updated weekly")
	form.Set("monthly_prompt", "updated monthly")
	form.Set("yearly_prompt", "updated yearly")
	req = httptest.NewRequest("POST", "/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /config status = %d; want 200", rec.Code)
	}

	// 3. 日記の更新と取得
	todayStr := dateutil.FormatISO(dateutil.TodayJST())
	jForm := url.Values{}
	jForm.Set("content", "今日はGo言語への移植をテストした。")
	req = httptest.NewRequest("POST", "/api/journal/"+todayStr, strings.NewReader(jForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/journal/%s status = %d; want 200", todayStr, rec.Code)
	}

	req = httptest.NewRequest("GET", "/api/journal/"+todayStr, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/journal/%s status = %d; want 200", todayStr, rec.Code)
	}
	var jResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &jResp); err != nil {
		t.Fatal(err)
	}
	if jResp["content"] != "今日はGo言語への移植をテストした。" {
		t.Errorf("GET journal content = %v; want updated content", jResp["content"])
	}

	// 4. 検索 API
	req = httptest.NewRequest("GET", "/api/search?type=daily&start_date="+todayStr+"&end_date="+todayStr, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/search status = %d; want 200", rec.Code)
	}
	var sResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &sResp); err != nil {
		t.Fatal(err)
	}
	if sResp["total_count"] != float64(1) {
		t.Errorf("search total_count = %v; want 1", sResp["total_count"])
	}

	// 5. エクスポート API
	req = httptest.NewRequest("GET", "/api/export", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/export status = %d; want 200", rec.Code)
	}
	exportBytes, err := os.ReadFile("export.html")
	if err != nil {
		t.Fatalf("failed to read export.html: %v", err)
	}
	if !strings.Contains(string(exportBytes), "今日はGo言語への移植をテストした。") {
		t.Errorf("export.html does not contain saved journal content")
	}
}
