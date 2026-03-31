#!/usr/bin/env bash
set -euo pipefail
GO_IMAGE="golang:1.24-alpine"
CACHE_VOL="$HOME/go/pkg/mod"
CMD="${1:-help}"
shift || true
case "$CMD" in
  compile)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -w /app \
      "${GO_IMAGE}" go build ./...
    ;;
  test)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -w /app \
      "${GO_IMAGE}" go test -v -race -coverprofile=coverage.out ./... "$@"
    ;;
  package)
    docker build -t "agenthub-studio/agenthub-api:local" .
    ;;
  lint)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      golangci/golangci-lint:latest golangci-lint run ./...
    ;;
  tidy)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -w /app \
      "${GO_IMAGE}" go mod tidy
    ;;
  *)
    echo "Usage: ./build.sh <compile|test|package|lint|tidy>"
    ;;
esac
