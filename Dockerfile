FROM docker.io/golang:1.26.4-alpine AS builder

ARG VERSION=dev

WORKDIR /build

# Download Go module dependencies (caches them if go.mod and go.sum are not changed)
COPY go.mod go.sum ./
RUN go mod download

# Build the Go app
COPY . .
RUN go build -ldflags="-X 'github.com/mgarnier-home/utils/buildinfo.Version=${VERSION}'" -o /build/autosaver .

FROM alpine AS runtime

WORKDIR /app

COPY --from=builder /build/autosaver /app/autosaver

EXPOSE 8080

ENTRYPOINT ["/app/autosaver"]

HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD wget -q --spider --timeout=5 --tries=1 http://localhost:8080 || exit 1
