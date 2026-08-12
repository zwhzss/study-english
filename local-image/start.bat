@echo off
REM ============================================================
REM  [DEPRECATED] Local image adapter service (llama.cpp FLUX path)
REM  llama.cpp mainline does NOT support FLUX image generation,
REM  so this service is no longer used. Use cloud instead.
REM  NOTE: keep this file pure ASCII to avoid codepage issues.
REM ============================================================
cd /d %~dp0
echo This adapter is deprecated (llama.cpp has no FLUX support).
echo Use cloud image API via the web config page instead.
pause
