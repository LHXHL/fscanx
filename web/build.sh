#!/usr/bin/env bash
# fscanx-web 前端构建脚本（BL-POC-004 前端重做）：
#   web/（vue-element-plus-admin monorepo）→ dist-pro → cmd/fscanx-web/static（go:embed）
# 用法：在仓库根执行 bash web/build.sh，完成后 go build ./cmd/fscanx-web 产出新二进制。
# 开发模式：cd web && pnpm dev:admin（vite :4000，/api 代理到 127.0.0.1:7758 的 fscanx-web）
set -euo pipefail
cd "$(dirname "$0")"

echo "[1/3] 安装依赖（有 node_modules 则跳过）"
[ -d node_modules ] || pnpm install

echo "[2/3] 构建前端"
pnpm build:admin

echo "[3/3] 同步产物到 cmd/fscanx-web/static（go:embed 真源）"
rm -rf ../cmd/fscanx-web/static
cp -r apps/admin/dist-pro ../cmd/fscanx-web/static

echo "完成。执行 go build ./cmd/fscanx-web 生成新 fscanx-web 二进制。"
