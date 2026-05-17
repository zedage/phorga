# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /phorganizer ./cmd/phorganizer

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /phorganizer /usr/local/bin/phorganizer

RUN mkdir -p /data/state /config

ENV PHORGANIZER_CONFIG=/config/config.yaml

ENTRYPOINT ["/usr/local/bin/phorganizer"]

