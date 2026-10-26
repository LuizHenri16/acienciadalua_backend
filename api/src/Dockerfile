# syntax=docker/dockerfile:1

# ─── build ───────────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/api \
    ./cmd/api

# ─── production ──────────────────────────────────────────────────────────────
FROM alpine:3.20 AS production

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 app \
    && adduser -D -u 1000 -G app app

WORKDIR /app

COPY --from=build /out/api /app/api

# Migrations are embedded in the binary; uploads/files live in mounted volumes.
RUN mkdir -p /app/uploads /app/files \
    && chown -R app:app /app

USER app

ENV PORT=3000

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/health" || exit 1

ENTRYPOINT ["/app/api"]
