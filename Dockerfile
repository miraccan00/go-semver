# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Cache dependencies separately from source
COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/semver ./cmd/semver

# ── Stage 2: runtime ───────────────────────────────────────────────────────────
FROM alpine:3.21

# git is required at runtime (exec.Command calls)
RUN apk add --no-cache git ca-certificates

# Non-root user
RUN addgroup -S semver && adduser -S semver -G semver

WORKDIR /workspace

COPY --from=builder /out/semver /usr/local/bin/semver

USER semver

ENTRYPOINT ["semver"]
