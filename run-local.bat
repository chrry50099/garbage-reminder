@echo off
setlocal

cd /d "%~dp0"

echo Starting Telegram garbage reminder service...
echo.

set "PWSH_EXE=C:\Program Files\PowerShell\7\pwsh.exe"

if exist "%PWSH_EXE%" (
    "%PWSH_EXE%" -ExecutionPolicy Bypass -File "%~dp0scripts\run-local.ps1"
) else (
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\run-local.ps1"
)
set "EXIT_CODE=%ERRORLEVEL%"

if not "%EXIT_CODE%"=="0" (
    echo.
    echo Service exited with code %EXIT_CODE%.
    pause
    exit /b %EXIT_CODE%
)

echo.
echo Service stopped.
pause
