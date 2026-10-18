# ---------- Stage 1: build ----------
FROM golang:1.26-alpine AS builder
WORKDIR /app

# download dependencies first (cached unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/server ./cmd/server

# ---------- Stage 2: run ----------
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser

WORKDIR /app
COPY --from=builder /app/server .

USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/server"]