@echo off
REM ============================================================
REM  [DEPRECATED] AI English Classroom - local FLUX image mode
REM  This script is deprecated: llama.cpp mainline does NOT support
REM  FLUX image generation. Use cloud (SiliconFlow) instead:
REM    set IMAGE_API_KEY=sk-your-siliconflow-key
REM    study-english-server.exe
REM ============================================================
cd /d %~dp0\server
echo This script is deprecated. Configure IMAGE_API_KEY and run study-english-server.exe directly.
pause
