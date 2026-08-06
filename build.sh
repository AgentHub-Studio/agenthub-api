#!/usr/bin/env bash
set -euo pipefail
GO_IMAGE="golang:1.25.12-alpine"
GOLANGCI_IMAGE="${GOLANGCI_IMAGE:-golangci/golangci-lint:v2.12.2}"
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
      -e CGO_ENABLED=1 \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -w /app \
      "${GO_IMAGE}" sh -c 'apk add --no-cache gcc musl-dev >/dev/null 2>&1 && go test -v -race -coverprofile=coverage.out ./... "$@"' _ "$@"
    ;;
  fuzz)
    docker run --rm \
      -e CGO_ENABLED=1 \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -w /app \
      "${GO_IMAGE}" sh -c 'apk add --no-cache gcc musl-dev >/dev/null 2>&1 && go test -race "$@"' _ "$@"
    ;;
  test-integration)
    integration_parallelism="${INTEGRATION_TEST_PARALLELISM:-2}"
    integration_timeout="${INTEGRATION_TEST_TIMEOUT:-30m}"
    if [[ ! "${integration_parallelism}" =~ ^[1-9][0-9]*$ ]]; then
      echo "INTEGRATION_TEST_PARALLELISM must be a positive integer." >&2
      exit 2
    fi
    integration_run_id="$$"
    docker run --rm \
      -e CGO_ENABLED=1 \
      -e "INTEGRATION_TEST_PARALLELISM=${integration_parallelism}" \
      -e "INTEGRATION_TEST_TIMEOUT=${integration_timeout}" \
      -e "AGENTHUB_INTEGRATION_RUN_ID=${integration_run_id}" \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -w /app \
      "${GO_IMAGE}" sh -ec '
        apk add --no-cache gcc musl-dev docker-cli >/dev/null 2>&1
        shared_postgres_name="agenthub-api-integration-postgres-${AGENTHUB_INTEGRATION_RUN_ID}"
        cleanup() {
          docker rm -f "$shared_postgres_name" >/dev/null 2>&1 || true
        }
        trap cleanup EXIT HUP INT TERM
        docker run -d --rm \
          --name "$shared_postgres_name" \
          --network "container:${HOSTNAME}" \
          -e POSTGRES_DB=testdb \
          -e POSTGRES_USER=testuser \
          -e POSTGRES_PASSWORD=testpass \
          pgvector/pgvector:pg16 \
          postgres -c listen_addresses='*' -c fsync=off -c synchronous_commit=off -c full_page_writes=off >/dev/null
        for attempt in $(seq 1 60); do
          if docker exec "$shared_postgres_name" pg_isready -U testuser -d testdb >/dev/null 2>&1; then
            break
          fi
          if [ "$attempt" -eq 60 ]; then
            echo "Shared integration PostgreSQL did not become ready." >&2
            exit 1
          fi
          sleep 1
        done
        export AGENTHUB_TEST_POSTGRES_DSN="postgres://testuser:testpass@127.0.0.1:5432/testdb?sslmode=disable"
        go test -p "$INTEGRATION_TEST_PARALLELISM" -timeout "$INTEGRATION_TEST_TIMEOUT" -tags=integration -v -race ./... "$@"
      ' _ "$@"
    ;;
  test-e2e)
    if [[ "${E2E_TESTS:-}" != "1" ]]; then
      echo "E2E_TESTS=1 is required to run the disposable local E2E suite." >&2
      exit 2
    fi
    docker run --rm --network host \
      -e CGO_ENABLED=1 \
      -e "E2E_TESTS=${E2E_TESTS}" \
  -e "E2E_TEST_TIMEOUT=${E2E_TEST_TIMEOUT:-60m}" \
      -e "API_URL=${API_URL:-http://127.0.0.1:28081}" \
      -e "KEYCLOAK_URL=${KEYCLOAK_URL:-http://127.0.0.1:28080}" \
      -e "KEYCLOAK_ADMIN_USER=${KEYCLOAK_ADMIN_USER:-admin}" \
      -e "KEYCLOAK_ADMIN_PASSWORD=${KEYCLOAK_ADMIN_PASSWORD:-@admin#}" \
      -e "E2E_USER_PASSWORD=${E2E_USER_PASSWORD:-E2eTestPass#1}" \
      -e "E2E_LLM_API_KEY=${E2E_LLM_API_KEY:-}" \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -w /app/e2e \
      "${GO_IMAGE}" sh -c 'apk add --no-cache gcc musl-dev >/dev/null 2>&1 && go test -tags=e2e -v -race -timeout "$E2E_TEST_TIMEOUT" ./... "$@"' _ "$@"
    ;;
  package)
    oci_created="${OCI_CREATED:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
    oci_revision="${OCI_REVISION:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}"
    oci_source="${OCI_SOURCE:-https://github.com/AgentHub-Studio/agenthub-api}"
    oci_version="${OCI_VERSION:-local}"
    docker build \
      --build-arg "OCI_CREATED=${oci_created}" \
      --build-arg "OCI_REVISION=${oci_revision}" \
      --build-arg "OCI_SOURCE=${oci_source}" \
      --build-arg "OCI_VERSION=${oci_version}" \
      --build-context gocommons="$(dirname "$(pwd)")/agenthub-go-commons" \
      -t "agenthub-studio/agenthub-api:local" .
    ;;
  lint)
    docker run --rm \
      -v "$(pwd)":/app \
      -v "${CACHE_VOL}":/go/pkg/mod \
      -v "$(dirname "$(pwd)")/agenthub-go-commons":/agenthub-go-commons \
      -w /app \
      "${GOLANGCI_IMAGE}" golangci-lint run ./...
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
    echo "Usage: ./build.sh <compile|test|fuzz|test-integration|test-e2e|package|lint|tidy>" >&2
    exit 1
    ;;
esac
