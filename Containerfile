# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e

FROM golang:1.26.7-bookworm@sha256:e8c859f5632dcfde7b32d2012b4351728f6437930887c2f6a91ea242459e5514 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -tags=netgo,osusergo \
    -ldflags='-s -w -buildid=' \
    -o /out/thinkpixelmp ./cmd/thinkpixelmp

FROM scratch

LABEL org.opencontainers.image.title="ThinkPixelMP" \
      org.opencontainers.image.description="ThinkPixel marketplace and software supply-chain control plane" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo/UTC /usr/share/zoneinfo/UTC
COPY --from=build --chown=65532:65532 /out/thinkpixelmp /thinkpixelmp

ENV TZ=UTC \
    TPMP_HTTP_ADDRESS=0.0.0.0:8080

EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/thinkpixelmp"]
