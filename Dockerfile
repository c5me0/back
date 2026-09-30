FROM golang:1.27-trixie AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .

ENV GOCACHE=/root/.cache/go-build
ENV CGO_ENABLED=0
ARG VERSION=local

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -ldflags="-s -w -X=cameo/internal/config.Version=$VERSION" -o /app/cameo .

FROM debian:trixie-slim
EXPOSE 80
EXPOSE 50000/udp

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates ffmpeg && \
    apt-get clean && rm -rf /var/lib/apt/lists/* && \
    useradd -u 1000 -m cameo && \
    mkdir -p /data/recordings && chown cameo:cameo /data/recordings

# atlasutil.Migrate runs $XDG_CACHE_HOME/atlas/atlas-<os>-<arch>-v1.1.0 and downloads it at boot when missing.
ARG TARGETARCH
ADD --chown=cameo:cameo --chmod=755 https://release.ariga.io/atlas/atlas-linux-${TARGETARCH}-v1.1.0 /home/cameo/.cache/atlas/atlas-linux-${TARGETARCH}-v1.1.0

WORKDIR /app
COPY --from=builder /app/cameo /app/
COPY internal/ent/migrate/migrations /app/assets/migrations
RUN chown -R cameo:cameo /app /home/cameo/.cache

USER cameo
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/app/cameo", "healthcheck"]
ENTRYPOINT ["/app/cameo"]
