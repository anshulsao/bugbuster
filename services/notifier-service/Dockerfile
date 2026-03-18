FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod.txt go.mod
COPY go.sum.txt go.sum
RUN go mod download

COPY main.go.src main.go

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /notifier-service .

FROM alpine:3.19

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

RUN apk --no-cache add ca-certificates

COPY --from=builder /notifier-service /usr/local/bin/notifier-service

USER appuser

ENTRYPOINT ["notifier-service"]
