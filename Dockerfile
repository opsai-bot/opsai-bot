# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=1 go build \
    -ldflags "-X github.com/jonny/opsai-bot/pkg/version.Version=${VERSION} \
              -X github.com/jonny/opsai-bot/pkg/version.Commit=${COMMIT} \
              -X github.com/jonny/opsai-bot/pkg/version.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
    -o /app/bin/opsai-bot ./cmd/opsai/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata sqlite

WORKDIR /app

COPY --from=builder /app/bin/opsai-bot /app/opsai-bot
COPY configs/ /app/configs/

RUN mkdir -p /data

USER nobody:nobody

EXPOSE 8080 9090

ENTRYPOINT ["/app/opsai-bot"]
CMD ["--config", "/app/configs/config.yaml"]
