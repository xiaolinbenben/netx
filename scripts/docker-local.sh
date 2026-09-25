#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
ENV_FILE="${NETX_ENV_FILE:-$ROOT_DIR/deploy/.env}"
PROJECT_NAME="netx-local"
IMAGE_NAMESPACE="netx-local"
IMAGE_TAG="local"

if ! command -v docker >/dev/null 2>&1; then
  echo "错误：未找到 Docker，请先安装并启动 Docker Desktop。" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "错误：当前 Docker 缺少 Compose 插件。" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "错误：Docker daemon 未运行，请先启动 Docker Desktop。" >&2
  exit 1
fi

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

: "${ADMIN_USERNAME:=admin}"
: "${ADMIN_PASSWORD:=admin123456}"
: "${JWT_SECRET:=netx-local-development-secret}"
export ADMIN_USERNAME ADMIN_PASSWORD JWT_SECRET
export DOCKERHUB_USERNAME="$IMAGE_NAMESPACE" IMAGE_TAG

echo "[1/2] 构建 Go 服务镜像（包含落地页、React 用户端和 Ant Design Pro 管理端）..."
docker build -f "$ROOT_DIR/server/Dockerfile" -t "$IMAGE_NAMESPACE/netx:server-$IMAGE_TAG" "$ROOT_DIR"

echo "[2/2] 启动本地服务..."
docker compose \
  --project-name "$PROJECT_NAME" \
  --file "$COMPOSE_FILE" \
  up -d --remove-orphans

echo
echo "NetX 已启动："
echo "  首页：   http://localhost:8000/"
echo "  用户端： http://localhost:8000/dashboard"
echo "  管理端： http://localhost:8000/admin/"
echo "  管理员： $ADMIN_USERNAME"
echo
echo "查看日志：docker compose --project-name $PROJECT_NAME --file deploy/docker-compose.yml logs -f"
