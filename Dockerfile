FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod *.go sites.txt ./
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/mihomo-health . \
    && mkdir -p /out/data && chown 65532:65532 /out/data

FROM scratch
LABEL org.opencontainers.image.title="z-mihomo-health-check" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY LICENSE /LICENSE
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/mihomo-health /mihomo-health
COPY --from=build --chown=65532:65532 /out/data /data
USER 65532:65532
ENV DATA_DIR=/data LISTEN_ADDR=0.0.0.0:8080 TZ=Asia/Shanghai
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/mihomo-health", "healthcheck"]
ENTRYPOINT ["/mihomo-health"]
