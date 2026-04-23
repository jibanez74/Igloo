FROM debian:bookworm-slim AS ffmpeg-artifacts

ARG JELLYFIN_FFMPEG_VERSION=7.1.3-5
ARG JELLYFIN_FFMPEG_DEB_SHA256=a94683ba2bda79454792aacec26d1ff17a1d42afc46586a298124aefbf5584cb

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    xz-utils \
    binutils \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp/jellyfin-ffmpeg

RUN curl -fsSL \
    "https://github.com/jellyfin/jellyfin-ffmpeg/releases/download/v${JELLYFIN_FFMPEG_VERSION}/jellyfin-ffmpeg7_${JELLYFIN_FFMPEG_VERSION}-bookworm_amd64.deb" \
    -o jellyfin-ffmpeg.deb \
  && echo "${JELLYFIN_FFMPEG_DEB_SHA256}  jellyfin-ffmpeg.deb" | sha256sum -c - \
  && ar x jellyfin-ffmpeg.deb data.tar.xz \
  && mkdir -p /artifacts \
  && tar -xJf data.tar.xz -C /artifacts \
  && install -m 0755 /artifacts/usr/lib/jellyfin-ffmpeg/ffmpeg /artifacts/ffmpeg_linux_amd64 \
  && install -m 0755 /artifacts/usr/lib/jellyfin-ffmpeg/ffprobe /artifacts/ffprobe_linux_amd64

FROM oven/bun:1 AS web-builder

WORKDIR /app/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./
RUN bun run build

FROM golang:1.26.2-bookworm AS server-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download

COPY server/ ./server/
COPY --from=web-builder /app/web/dist ./server/cmd/api/webdist
COPY --from=ffmpeg-artifacts /artifacts/ffmpeg_linux_amd64 ./server/cmd/internal/ffmpeg/ffmpeg_linux_amd64
COPY --from=ffmpeg-artifacts /artifacts/ffprobe_linux_amd64 ./server/cmd/internal/ffprobe/ffprobe_linux_amd64

RUN cd server && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o /out/igloo-server ./cmd/api

FROM debian:bookworm-slim AS runtime

COPY --from=ffmpeg-artifacts /tmp/jellyfin-ffmpeg/jellyfin-ffmpeg.deb /tmp/jellyfin-ffmpeg.deb

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    /tmp/jellyfin-ffmpeg.deb \
  && rm -rf /var/lib/apt/lists/* /tmp/jellyfin-ffmpeg.deb \
  && groupadd --gid 1000 igloo \
  && useradd  --uid 1000 --gid igloo --no-create-home igloo

RUN mkdir -p /transcode /config \
  && chown igloo:igloo /transcode /config \
  && chmod 750 /transcode /config

ENV PORT=8080
ENV TMPDIR=/transcode
ENV LOG_TO_STDOUT=true

WORKDIR /app

COPY --from=server-builder /out/igloo-server /usr/local/bin/igloo-server

USER igloo

EXPOSE 8080

CMD ["/usr/local/bin/igloo-server"]
