ARG BUILD_FROM=ghcr.io/home-assistant/base:latest
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG BUILD_ARCH

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN if [ -z "$TARGETARCH" ]; then \
		case "$BUILD_ARCH" in \
			aarch64) TARGETARCH=arm64 ;; \
			armv7) TARGETARCH=arm ;; \
			armhf) TARGETARCH=arm ;; \
			amd64) TARGETARCH=amd64 ;; \
			i386) TARGETARCH=386 ;; \
			"") TARGETARCH=amd64 ;; \
			*) echo "Unsupported BUILD_ARCH: $BUILD_ARCH" && exit 1 ;; \
		esac; \
	fi && \
	if [ "$BUILD_ARCH" = "armv7" ]; then export GOARM=7; fi && \
	if [ "$BUILD_ARCH" = "armhf" ]; then export GOARM=6; fi && \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-w -s" -o server ./cmd/server

FROM $BUILD_FROM

ARG BUILD_VERSION=dev
ARG BUILD_ARCH

WORKDIR /app

ENV PORT=8080

COPY --from=builder /app/server .
COPY run.sh /run.sh

RUN chmod a+x /run.sh

LABEL \
	io.hass.version="${BUILD_VERSION}" \
	io.hass.type="app" \
	io.hass.arch="${BUILD_ARCH}"

EXPOSE 8080

CMD ["/run.sh"]
