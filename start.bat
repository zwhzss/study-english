@echo off
REM ============================================================
REM  AI English Classroom - Production/Test Mode (single port 8090)
REM  1. Auto-build backend (go build)
REM  2. Check/install frontend dependencies
REM  3. Auto-build frontend (ng build, served by Gin)
REM  4. Start service at http://localhost:8090
REM  NOTE: keep this file pure ASCII to avoid codepage issues.
REM ============================================================
cd /d %~dp0

REM ---- [1/4] Build backend ----
echo [1/4] Building backend ...
cd server
go build -o study-english-server.exe .
if errorlevel 1 (
    echo.
    echo [ERROR] Backend build failed. Fix Go code and retry.
    pause
    exit /b 1
)
cd ..

REM ---- [2/4] Check frontend dependencies ----
echo [2/4] Checking frontend dependencies ...
if not exist frontend\node_modules (
    echo First run: installing frontend dependencies, this may take minutes...
    cd frontend
    call npm install
    if errorlevel 1 (
        echo.
        echo [ERROR] npm install failed. Check network and retry.
        pause
        exit /b 1
    )
    cd ..
) else (
    echo Frontend dependencies ready.
)

REM ---- [3/4] Build frontend ----
echo [3/4] Building frontend ...
cd frontend
call npx ng build
if errorlevel 1 (
    echo.
    echo [ERROR] Frontend build failed. Fix code and retry.
    pause
    exit /b 1
)
cd ..

REM ---- [4/4] Start service ----
echo [4/4] Starting service at http://localhost:8090 ...
start "AI-English" cmd /k "cd /d %~dp0server && study-english-server.exe"

echo.
echo Service started: http://localhost:8090
echo Close the popup window to stop. Rerun this script after code changes (auto rebuild).
pause
