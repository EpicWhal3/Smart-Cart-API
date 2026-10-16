FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/cart-service ./cmd/cart-service

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/cart-service /app/cart-service
COPY --from=builder /src/migrations /app/migrations

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/cart-service"]
