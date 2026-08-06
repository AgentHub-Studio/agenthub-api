FROM golang:1.25.12-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build
COPY --from=gocommons . /build/agenthub-go-commons
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /build/bin/api \
    ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /build/bin/migrate-workload-identities \
    ./cmd/migrate-workload-identities

FROM gcr.io/distroless/static-debian12:nonroot
ARG OCI_CREATED="unknown"
ARG OCI_REVISION="unknown"
ARG OCI_SOURCE="https://github.com/AgentHub-Studio/agenthub-api"
ARG OCI_VERSION="local"
LABEL org.opencontainers.image.title="agenthub-api" \
    org.opencontainers.image.description="AgentHub API service" \
    org.opencontainers.image.source="${OCI_SOURCE}" \
    org.opencontainers.image.revision="${OCI_REVISION}" \
    org.opencontainers.image.created="${OCI_CREATED}" \
    org.opencontainers.image.version="${OCI_VERSION}" \
    org.opencontainers.image.vendor="AgentHub Studio" \
    org.opencontainers.image.licenses="Proprietary"
COPY --from=builder /build/bin/api /api
COPY --from=builder /build/bin/migrate-workload-identities /migrate-workload-identities
COPY --from=builder /build/migrations /migrations
USER 65532:65532
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/api", "-health"]
ENTRYPOINT ["/api"]
