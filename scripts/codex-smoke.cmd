@echo off
setlocal
cd /d "%~dp0\.."
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\codex-smoke.ps1"
echo.
echo Done. Press any key to close.
pause >nul

