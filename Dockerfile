# Build stage: runs natively on the build machine and cross-compiles for the
# target platform (no CGO), so multi-arch builds do not need emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /phorga ./cmd/phorga

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /phorga /usr/local/bin/phorga

RUN mkdir -p /data/state /config

ENV PHORGA_CONFIG=/config/config.yaml

ENTRYPOINT ["/usr/local/bin/phorga"]
