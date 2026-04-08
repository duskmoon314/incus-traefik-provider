FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o incus-traefik-provider ./cmd/incus-traefik-provider/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /build/incus-traefik-provider /usr/local/bin/incus-traefik-provider

EXPOSE 9000

ENTRYPOINT ["incus-traefik-provider"]
