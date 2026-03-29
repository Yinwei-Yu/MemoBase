#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${PROJECT_ROOT}"

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="${1:-${PROJECT_ROOT}/backups/${TIMESTAMP}}"

mkdir -p "${BACKUP_DIR}"

echo "[backup] dumping postgres..."
docker compose exec -T postgres pg_dump -U memo -d memo > "${BACKUP_DIR}/postgres.sql"

echo "[backup] copying backend storage..."
docker compose cp backend:/app/storage "${BACKUP_DIR}/backend-storage"

echo "[backup] copying qdrant storage..."
docker compose cp qdrant:/qdrant/storage "${BACKUP_DIR}/qdrant-storage"

cat > "${BACKUP_DIR}/manifest.txt" <<EOF
created_at=${TIMESTAMP}
project=memobase
components=postgres,backend-storage,qdrant-storage
EOF

ARCHIVE_PATH="${BACKUP_DIR}.tar.gz"
echo "[backup] creating archive ${ARCHIVE_PATH}..."
tar -czf "${ARCHIVE_PATH}" -C "$(dirname "${BACKUP_DIR}")" "$(basename "${BACKUP_DIR}")"

echo "[backup] done"
echo "backup_dir=${BACKUP_DIR}"
echo "backup_archive=${ARCHIVE_PATH}"
