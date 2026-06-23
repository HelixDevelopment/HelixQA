FROM docker.io/library/golang:1.25-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /helixqa ./cmd/helixqa

FROM docker.io/library/debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg android-tools-adb ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /helixqa /usr/local/bin/helixqa
# DS-0002: run the autonomous QA runner as an unprivileged user. The binary
# operates on the bind-mounted /project tree (owned/writable by the host caller)
# and serves no privileged port; it needs no root at runtime.
RUN groupadd --system --gid 10001 helixqa \
    && useradd --system --uid 10001 --gid 10001 --create-home --home-dir /home/helixqa helixqa
USER 10001:10001
ENTRYPOINT ["helixqa"]
CMD ["autonomous", "--project", "/project", "--platforms", "all"]
