#!/usr/bin/env bash
# 开发模式一键启动（自动编译后端 + 前端热部署 HMR）—— Windows 用 start-dev.bat
set -e
cd "$(dirname "$0")"

echo "[1/3] 编译后端 ..."
(cd server && go build -o study-english-server.exe .)

echo "[2/3] 检查前端依赖 ..."
if [ ! -d frontend/node_modules ]; then
  echo "首次运行，安装前端依赖 ..."
  (cd frontend && npm install)
fi

echo "[3/3] 启动后端(8090) + 前端热部署(4200) ..."
(cd server && ./study-english-server.exe) &
BACK_PID=$!
(cd frontend && npx ng serve --open) &
FRONT_PID=$!

trap "kill $BACK_PID $FRONT_PID 2>/dev/null" EXIT
echo "前端 http://localhost:4200 | 后端 http://localhost:8090"
wait
