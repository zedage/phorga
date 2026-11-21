# Build stage: runs natively on the build machine and cross-compiles for the
# target platform (no CGO), so multi-arch builds do not need emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/phorga ./cmd/phorga \
    && mkdir -p /out/data/state /out/config

# Runtime stage: distroless static image, no shell or package manager. It
# has CA certificates (ntfy over HTTPS) and time zone data (TZ). Phorga runs
# as the unprivileged user "nonroot" (65532) unless docker-compose sets
# another user, e.g. the NAS user.
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /out/phorga /usr/local/bin/phorga
COPY --from=builder --chown=65532:65532 /out/data /data
COPY --from=builder --chown=65532:65532 /out/config /config

# The config may be missing: Phorga then starts its web UI for setup and
# creates the file on the first save (CFG-009). The UI listens on all
# interfaces inside the container; the port mapping decides who reaches it.
ENV PHORGA_CONFIG=/config/config.yaml
ENV PHORGA_WEB_UI_ADDR=0.0.0.0:8080
EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/phorga"]
