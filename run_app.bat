@echo off
cd /d %~dp0

if not exist "ai-diary-go.exe" (
    echo ai-diary-go.exe ‚ªŒ©‚Â‚©‚è‚Ü‚¹‚ñBæ‚É run_build.bat ‚ğÀs‚µ‚Ä‚­‚¾‚³‚¢B
    pause
    exit /b 1
)

start http://localhost:5000
ai-diary-go.exe %*
pause
