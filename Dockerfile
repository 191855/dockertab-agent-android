FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build \
    -ldflags="-s -w -X main.agentVersion=${VERSION}" \
    -o dockertab-agent-android \
    .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata docker-cli docker-cli-compose

WORKDIR /app
COPY --from=builder /build/dockertab-agent-android .

RUN mkdir -p /root/.config/dockertab

EXPOSE 8378

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8378/healthz || exit 1

ENTRYPOINT ["./dockertab-agent-android"]
