FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -o /build/bin/api \
    ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /build/bin/api /api
COPY --from=builder /build/migrations /migrations
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/api", "-health"] || exit 1
ENTRYPOINT ["/api"]
