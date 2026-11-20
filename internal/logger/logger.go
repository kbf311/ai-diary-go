package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultMaxBytes    = 1 * 1024 * 1024 // 1MB
	defaultBackupCount = 5
)

// Logger はローテーション機能付きのファイル出力と標準出力を行うロガーです。
type Logger struct {
	mu          sync.Mutex
	filePath    string
	file        *os.File
	currentSize int64
	maxBytes    int64
	backupCount int
}

// New は指定されたログファイル名で Logger を初期化します。
func New(logDir, fileName string) (*Logger, error) {
	if logDir == "" {
		logDir = "logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("ログディレクトリの作成に失敗しました: %w", err)
	}

	filePath := filepath.Join(logDir, fileName)
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("ログファイルのオープンに失敗しました: %w", err)
	}

	info, err := f.Stat()
	var size int64
	if err == nil {
		size = info.Size()
	}

	return &Logger{
		filePath:    filePath,
		file:        f,
		currentSize: size,
		maxBytes:    defaultMaxBytes,
		backupCount: defaultBackupCount,
	}, nil
}

// Close はログファイルを閉じます。
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Info は INFO レベルのログを出力します。
func (l *Logger) Info(format string, args ...any) {
	l.write("INFO", format, args...)
}

// Error は ERROR レベルのログを出力します。
func (l *Logger) Error(format string, args ...any) {
	l.write("ERROR", format, args...)
}

func (l *Logger) write(level, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05,000")
	line := fmt.Sprintf("%s\t%s\t%s\n", timestamp, level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentSize+int64(len(line)) > l.maxBytes {
		_ = l.rotate()
	}

	mw := io.MultiWriter(os.Stdout, l.file)
	n, _ := mw.Write([]byte(line))
	if l.file != nil {
		l.currentSize += int64(n)
	}
}

func (l *Logger) rotate() error {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	// バックアップファイルを後ろから順にリネーム (.4 -> .5, ..., .log -> .log.1)
	for i := l.backupCount - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", l.filePath, i)
		dst := fmt.Sprintf("%s.%d", l.filePath, i+1)
		if _, err := os.Stat(src); err == nil {
			_ = os.Rename(src, dst)
		}
	}
	if _, err := os.Stat(l.filePath); err == nil {
		_ = os.Rename(l.filePath, l.filePath+".1")
	}

	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	l.file = f
	l.currentSize = 0
	return nil
}
