# syntax=docker/dockerfile:1

FROM golang:1.25 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /bin/grapery-agent ./cmd/server

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /bin/grapery-agent /app/grapery-agent

ENV SERVER_PORT=9020
ENV GRAPERY_BASE_URL=http://server:8080
ENV AGENT_ARTIFACT_DIR=/app/data/agent-artifacts
# 运行环境由 compose .env 覆盖：ENVIRONMENT / GRAPERY_ENV（development | production）

EXPOSE 9020

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:9020/health || exit 1

CMD ["/app/grapery-agent"]
