@echo off
REM ============================================================
REM  AI English Classroom - Dev Mode (auto build + HMR hot reload)
REM  1. Auto-build backend (go build)
REM  2. Check/install frontend dependencies
REM  3. Start: backend http://localhost:8090 + frontend http://localhost:4200
REM     Frontend /api requests are proxied to backend 8090
REM  NOTE: keep this file pure ASCII to avoid codepage issues.
REM ============================================================
cd /d %~dp0

REM ---- [1/3] Build backend ----
echo [1/3] Building backend ...
cd server
go build -o study-english-server.exe .
if errorlevel 1 (
    echo.
    echo [ERROR] Backend build failed. Fix Go code and retry.
    pause
    exit /b 1
)
cd ..

REM ---- [2/3] Check frontend dependencies ----
echo [2/3] Checking frontend dependencies ...
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

REM ---- [3/3] Start services ----
echo [3/3] Starting backend(8090) + frontend HMR(4200) ...
start "AI-Backend" cmd /k "cd /d %~dp0server && study-english-server.exe"
start "AI-Frontend-HMR" cmd /k "cd /d %~dp0frontend && npx ng serve --open"

echo.
echo Dev mode started:
echo   Frontend http://localhost:4200  (HMR: save .ts/.html/.css to hot reload)
echo   Backend  http://localhost:8090  (Go changes need backend restart)
echo Close both popup windows to stop.
pause
