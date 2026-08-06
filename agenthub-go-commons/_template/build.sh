#!/usr/bin/env bash
# Build script para serviços AgentHub Go (ADR-006: builds via Docker)
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-$(basename "$(pwd)")}"
GO_IMAGE="golang:1.25.12-alpine"
CACHE_VOL="$HOME/go/pkg/mod"

CMD="${1:-help}"
shift || true

case "$CMD" in
  compile)
    echo "==> Compilando ${SERVICE_NAME}..."
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" \
      go build ./...
    echo "==> OK"
    ;;
  test)
    echo "==> Testando ${SERVICE_NAME}..."
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -w /app \
      "${GO_IMAGE}" \
      go test -v -race -coverprofile=coverage.out ./... "$@"
    echo "==> OK"
    ;;
  package)
    echo "==> Empacotando ${SERVICE_NAME}..."
    docker build \
      --build-arg SERVICE_NAME="${SERVICE_NAME}" \
      -t "agenthub-studio/${SERVICE_NAME}:local" .
    echo "==> Imagem: agenthub-studio/${SERVICE_NAME}:local"
    ;;
  lint)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      golangci/golangci-lint:latest \
      golangci-lint run ./...
    ;;
  tidy)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" \
      go mod tidy
    ;;
  *)
    echo "Usage: ./build.sh <compile|test|package|lint|tidy>"
    ;;
esac
