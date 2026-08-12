#!/usr/bin/env bash
# 生产/测试模式一键启动（自动编译前后端，单端口 8090）—— Windows 用 start.bat
set -e
cd "$(dirname "$0")"

echo "[1/4] 编译后端 ..."
(cd server && go build -o study-english-server.exe .)

echo "[2/4] 检查前端依赖 ..."
if [ ! -d frontend/node_modules ]; then
  echo "首次运行，安装前端依赖 ..."
  (cd frontend && npm install)
fi

echo "[3/4] 编译前端 ..."
(cd frontend && npx ng build)

echo "[4/4] 启动服务 http://localhost:8090 ..."
(cd server && exec ./study-english-server.exe)
