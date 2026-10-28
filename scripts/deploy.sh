#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/scripts/.env"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose.yml"
IMAGE_NAME="netx:latest"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "错误：缺少 scripts/.env，请先执行 cp scripts/.env.example scripts/.env" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

: "${SERVER_USER:=root}"
: "${DEPLOY_PATH:=/opt/netx}"

missing=()
for key in SERVER_HOST SERVER_PASSWORD ADMIN_USERNAME ADMIN_PASSWORD JWT_SECRET; do
  if [[ -z "${!key:-}" ]]; then
    missing+=("$key")
  fi
done
if ((${#missing[@]} > 0)); then
  echo "错误：scripts/.env 缺少 ${missing[*]}" >&2
  exit 1
fi
for key in ADMIN_USERNAME ADMIN_PASSWORD JWT_SECRET; do
  if [[ "${!key}" == *"'"* ]]; then
    echo "错误：$key 不能包含单引号" >&2
    exit 1
  fi
done

if ! command -v docker >/dev/null 2>&1; then
  echo "错误：未找到 Docker，请先安装并启动 Docker。" >&2
  exit 1
fi

if ! command -v sshpass >/dev/null 2>&1; then
  echo "错误：未找到 sshpass。macOS 可执行：brew install hudochenkov/sshpass/sshpass" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "错误：Docker daemon 未运行，请先启动 Docker。" >&2
  exit 1
fi

export SSHPASS="$SERVER_PASSWORD"
SSH_OPTS=(
  -o StrictHostKeyChecking=accept-new
  -o PreferredAuthentications=password
  -o PubkeyAuthentication=no
  -o NumberOfPasswordPrompts=1
  -o ConnectTimeout=15
)

ssh_cmd() {
  sshpass -e ssh "${SSH_OPTS[@]}" "$SERVER_USER@$SERVER_HOST" "$@"
}

echo "检查服务器连接..."
ssh_cmd "true"

echo "构建 linux/amd64 镜像..."
docker build --platform linux/amd64 -f "$ROOT_DIR/server/Dockerfile" -t "$IMAGE_NAME" "$ROOT_DIR"

tmp_env="$(mktemp)"
trap 'rm -f "$tmp_env"' EXIT
{
  printf "ADMIN_USERNAME='%s'\n" "$ADMIN_USERNAME"
  printf "ADMIN_PASSWORD='%s'\n" "$ADMIN_PASSWORD"
  printf "JWT_SECRET='%s'\n" "$JWT_SECRET"
} > "$tmp_env"

echo "上传部署文件..."
ssh_cmd "mkdir -p '$DEPLOY_PATH/deploy'"
ssh_cmd "cat > '$DEPLOY_PATH/deploy/docker-compose.yml'" < "$COMPOSE_FILE"
ssh_cmd "cat > '$DEPLOY_PATH/deploy/.env' && chmod 600 '$DEPLOY_PATH/deploy/.env'" < "$tmp_env"

echo "传输镜像..."
docker save "$IMAGE_NAME" | ssh_cmd docker load

echo "启动服务..."
ssh_cmd "cd '$DEPLOY_PATH' && docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d --remove-orphans && docker compose --env-file deploy/.env -f deploy/docker-compose.yml ps"
