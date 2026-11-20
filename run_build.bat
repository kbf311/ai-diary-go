@echo off
cd /d %~dp0

where go >nul 2>&1
if errorlevel 1 (
    if exist "C:\Program Files\Go\bin\go.exe" (
        set "PATH=C:\Program Files\Go\bin;%PATH%"
    )
)

echo AIダイアリーをビルドしています...
go build -o ai-diary-go.exe .
if errorlevel 1 (
    echo ビルドに失敗しました。
    pause
    exit /b 1
)

echo ビルドが完了しました: ai-diary-go.exe
pause
