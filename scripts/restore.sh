#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <backup_dir_or_tar_gz>"
  exit 1
fi

INPUT_PATH="$1"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${PROJECT_ROOT}"

WORK_DIR=""
BACKUP_DIR=""

cleanup() {
  if [[ -n "${WORK_DIR}" && -d "${WORK_DIR}" ]]; then
    rm -rf "${WORK_DIR}"
  fi
}
trap cleanup EXIT

if [[ -d "${INPUT_PATH}" ]]; then
  BACKUP_DIR="${INPUT_PATH}"
elif [[ -f "${INPUT_PATH}" && "${INPUT_PATH}" == *.tar.gz ]]; then
  WORK_DIR="$(mktemp -d)"
  tar -xzf "${INPUT_PATH}" -C "${WORK_DIR}"
  BACKUP_DIR="$(find "${WORK_DIR}" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
else
  echo "invalid input path: ${INPUT_PATH}"
  exit 1
fi

if [[ ! -f "${BACKUP_DIR}/postgres.sql" ]]; then
  echo "missing ${BACKUP_DIR}/postgres.sql"
  exit 1
fi

echo "[restore] starting dependencies..."
docker compose up -d postgres qdrant backend

echo "[restore] restoring postgres..."
cat "${BACKUP_DIR}/postgres.sql" | docker compose exec -T postgres psql -U memo -d memo

if [[ -d "${BACKUP_DIR}/backend-storage" ]]; then
  echo "[restore] restoring backend storage..."
  docker compose exec -T backend sh -lc 'rm -rf /app/storage/*'
  docker compose cp "${BACKUP_DIR}/backend-storage/." backend:/app/storage
fi

if [[ -d "${BACKUP_DIR}/qdrant-storage" ]]; then
  echo "[restore] restoring qdrant storage..."
  docker compose exec -T qdrant sh -lc 'rm -rf /qdrant/storage/*'
  docker compose cp "${BACKUP_DIR}/qdrant-storage/." qdrant:/qdrant/storage
fi

echo "[restore] restarting services..."
docker compose restart qdrant backend

echo "[restore] done"
