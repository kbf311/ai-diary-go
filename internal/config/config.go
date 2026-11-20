package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config はアプリケーションの設定を保持します。
type Config struct {
	DatabaseType     string
	SQLitePath       string
	DatabaseURL      string
	PreGenerateYears int
	Port             string
}

// Load は .env ファイルおよび環境変数から設定を読み込みます。
func Load() (*Config, error) {
	// .env ファイルがあれば読み込む（存在しない場合は無視）
	_ = godotenv.Load()

	dbType := strings.ToLower(getEnvOrDefault("DATABASE_TYPE", "sqlite"))
	cfg := &Config{
		DatabaseType: dbType,
		Port:         getEnvOrDefault("PORT", "5000"),
	}

	preGenStr := getEnvOrDefault("PRE_GENERATE_YEARS", "2")
	preGen, err := strconv.Atoi(preGenStr)
	if err != nil {
		preGen = 2
	}
	cfg.PreGenerateYears = preGen

	if dbType == "sqlite" {
		rawURL := getEnvOrDefault("SQLITE_DATABASE_URL", "sqlite:///db/ai-diary.db")
		dbPath := extractSQLitePath(rawURL)
		if dbPath != ":memory:" && dbPath != "" {
			absPath, err := filepath.Abs(dbPath)
			if err != nil {
				return nil, fmt.Errorf("SQLiteパスの解決に失敗しました: %w", err)
			}
			dirName := filepath.Dir(absPath)
			if dirName != "" {
				if err := os.MkdirAll(dirName, 0755); err != nil {
					return nil, fmt.Errorf("SQLite用ディレクトリの作成に失敗しました: %w", err)
				}
			}
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				f, err := os.Create(absPath)
				if err != nil {
					return nil, fmt.Errorf("SQLiteデータベースファイルの作成に失敗しました: %w", err)
				}
				_ = f.Close()
			}
			cfg.SQLitePath = absPath
		} else {
			cfg.SQLitePath = dbPath
		}
	} else {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			return nil, fmt.Errorf("DATABASE_URL環境変数が設定されていません。.envファイルを確認してください")
		}
		cfg.DatabaseURL = dbURL
	}

	return cfg, nil
}

func extractSQLitePath(rawURL string) string {
	if strings.HasPrefix(rawURL, "sqlite:///") {
		return rawURL[len("sqlite:///"):]
	}
	if strings.HasPrefix(rawURL, "sqlite://") {
		return rawURL[len("sqlite://"):]
	}
	return rawURL
}

func getEnvOrDefault(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}
