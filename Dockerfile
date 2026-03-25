FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod .
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o rss-proxy .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/rss-proxy /rss-proxy

ENV CONFIG=/app/config.json
EXPOSE 8080
ENTRYPOINT ["/rss-proxy"]
