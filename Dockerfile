FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /helix-seller \
    cmd/server/main.go

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 1000 helix

COPY --from=builder /helix-seller /usr/local/bin/helix-seller

USER helix

EXPOSE 8080

ENTRYPOINT ["helix-seller"]
