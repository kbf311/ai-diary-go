package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"ai-diary-go/internal/config"
	"ai-diary-go/internal/db"
	"ai-diary-go/internal/handler"
	"ai-diary-go/internal/logger"
)

func main() {
	initFlag := flag.Bool("init", false, "DBの初期化を行う")
	flag.Parse()

	log, err := logger.New("logs", "app.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ロガーの初期化に失敗しました: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	cfg, err := config.Load()
	if err != nil {
		log.Error("設定の読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg, log)
	if err != nil {
		log.Error("データベース接続に失敗しました: %v", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.InitTables(); err != nil {
		log.Error("データベース初期化に失敗しました: %v", err)
		os.Exit(1)
	}

	if *initFlag {
		if err := database.TruncateAllTables(); err != nil {
			log.Error("テーブルのデータ削除に失敗しました: %v", err)
			os.Exit(1)
		}
	}

	if err := database.SeedInitialData(cfg.PreGenerateYears); err != nil {
		log.Error("初期データ投入に失敗しました: %v", err)
		os.Exit(1)
	}

	h, err := handler.New(database, log, "templates")
	if err != nil {
		log.Error("テンプレートの読み込みに失敗しました: %v", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := ":" + cfg.Port
	log.Info("Goアプリケーションを起動しています... http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error("HTTPサーバーの起動に失敗しました: %v", err)
		os.Exit(1)
	}
}
