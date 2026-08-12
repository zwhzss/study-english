#!/usr/bin/env bash
# 本地 AI 绘图适配服务启动脚本（Linux/macOS）
# 使用前按实际路径修改下方变量，然后: chmod +x start.sh && ./start.sh
set -e
cd "$(dirname "$0")"

# ---- 按需修改：你的文件路径 ----
export LLAMA_CLI="${LLAMA_CLI:-/usr/local/bin/llama-cli}"
export FLUX_MODEL="${FLUX_MODEL:-$(pwd)/models/flux1-schnell-Q4_K_S.gguf}"
export FLUX_CLIP="${FLUX_CLIP:-$(pwd)/models/clip-l.gguf}"
export FLUX_T5="${FLUX_T5:-$(pwd)/models/t5xxl-encoder.gguf}"
export FLUX_VAE="${FLUX_VAE:-$(pwd)/models/flux-vae.gguf}"
# ---------------------------------

export FLUX_STEPS="${FLUX_STEPS:-4}"
export LOCAL_IMAGE_PORT="${LOCAL_IMAGE_PORT:-17000}"

echo "启动本地绘图服务 (http://localhost:${LOCAL_IMAGE_PORT}) ..."
echo "模型: ${FLUX_MODEL}"
exec go run .
