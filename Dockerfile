# syntax=docker/dockerfile:1

# ─── API Go (imagem de produção usada pelo deploy da VPS) ────────────────────
#
# O workflow `.github/workflows/deploy.yml` constrói este arquivo com o contexto
# na raiz do repositório e publica a imagem como
# `<DOCKERHUB_USERNAME>/acienciadalua-backend:latest`. O serviço `backend_prod`
# da VPS continua consumindo a mesma imagem/tag e a mesma porta interna (3000).
#
# O Dockerfile equivalente para uso local a partir de `api/src` é
# `api/src/Dockerfile`. Mantenha os dois em sincronia ao alterar a imagem.

# ─── build ───────────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

# Usa o go.mod do módulo que está em api/src.
COPY api/src/go.mod api/src/go.sum ./
RUN go mod download

COPY api/src/ ./

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

# ENTRYPOINT (em vez de CMD) garante que o binário Go seja sempre o processo
# principal do container, mesmo que um `command` antigo ainda exista no compose.
ENTRYPOINT ["/app/api"]
