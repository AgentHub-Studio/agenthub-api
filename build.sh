#!/usr/bin/env bash
set -euo pipefail
GO_IMAGE="golang:1.24-alpine"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CACHE_VOL="agenthub-api-go-cache"
CMD="${1:-help}"
shift || true
case "$CMD" in
  compile)
    docker run --rm \
      -v "${SCRIPT_DIR}":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" go build ./...
    ;;
  test)
    docker run --rm \
      -e CGO_ENABLED=1 \
      -v "${SCRIPT_DIR}":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -w /app \
      "${GO_IMAGE}" sh -c "apk add --no-cache gcc musl-dev >/dev/null 2>&1 && go test -v -race -coverprofile=coverage.out ./... $*"
    ;;
  package)
    docker build \
      --build-context gocommons="${SCRIPT_DIR}/agenthub-go-commons" \
      -t "agenthub-studio/agenthub-api:local" \
      "${SCRIPT_DIR}"
    ;;
  lint)
    docker run --rm \
      -v "${SCRIPT_DIR}":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      golangci/golangci-lint:latest golangci-lint run ./...
    ;;
  tidy)
    docker run --rm \
      -v "${SCRIPT_DIR}":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" go mod tidy
    ;;
  *)
    echo "Usage: ./build.sh <compile|test|package|lint|tidy>"
    ;;
esac
