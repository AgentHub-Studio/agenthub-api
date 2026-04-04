#!/usr/bin/env bash
# Build script for agenthub-go-commons (ADR-006: builds via Docker)
set -euo pipefail

GO_IMAGE="golang:1.24-alpine"
CACHE_VOL="$HOME/go/pkg/mod"

CMD="${1:-help}"
shift || true

case "$CMD" in
  compile)
    echo "==> Compiling agenthub-go-commons..."
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" \
      go build ./...
    echo "==> Compile OK"
    ;;

  test)
    echo "==> Running tests..."
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -e CGO_ENABLED=1 \
      -w /app \
      "${GO_IMAGE}" \
      sh -c "apk add --no-cache gcc musl-dev && go test -v -race -coverprofile=coverage.out ./... $*"
    echo "==> Tests OK"
    ;;

  lint)
    echo "==> Running golangci-lint..."
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      golangci/golangci-lint:latest \
      golangci-lint run ./...
    echo "==> Lint OK"
    ;;

  tidy)
    echo "==> Running go mod tidy..."
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app \
      "${GO_IMAGE}" \
      go mod tidy
    echo "==> Tidy OK"
    ;;

  *)
    echo "Usage: ./build.sh <command>"
    echo ""
    echo "Commands:"
    echo "  compile   Build all packages"
    echo "  test      Run all tests"
    echo "  lint      Run golangci-lint"
    echo "  tidy      Run go mod tidy"
    ;;
esac
