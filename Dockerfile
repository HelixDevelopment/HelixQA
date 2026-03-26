FROM docker.io/library/golang:1.25-bookworm AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /helixqa ./cmd/helixqa

FROM docker.io/library/debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg android-tools-adb ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /helixqa /usr/local/bin/helixqa
ENTRYPOINT ["helixqa"]
CMD ["autonomous", "--project", "/project", "--platforms", "all"]
