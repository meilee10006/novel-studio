FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS builder

WORKDIR /src
ENV CGO_ENABLED=0 GOWORK=off
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

COPY go.mod go.sum ./
COPY third_party/litellm/go.mod ./third_party/litellm/go.mod
RUN go mod download
COPY . .
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" \
    -o /out/novel-core \
    ./cmd/novel-core

FROM alpine:3.24.1
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /workspace
COPY --from=builder /out/novel-core /usr/local/bin/novel-core
ENTRYPOINT ["novel-core"]
