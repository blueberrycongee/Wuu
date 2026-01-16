@echo off
setlocal
cd /d "%~dp0\.."
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\wsl-bootstrap-rust.ps1"
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\autoloop.ps1" -MaxIters 200
echo.
echo Autoloop exited. Press any key to close.
pause >nul

