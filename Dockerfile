FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN templ generate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/gitlens-pro ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata git

COPY --from=builder /app/bin/gitlens-pro /usr/local/bin/gitlens-pro
COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /app/config /app/config

WORKDIR /app

RUN mkdir -p /app/storage/repos /app/storage/forks

EXPOSE 8080

ENTRYPOINT ["gitlens-pro"]
