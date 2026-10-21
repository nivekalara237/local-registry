# Runtime image of local-registry. goreleaser (dockers_v2) builds it: the build context holds
# the compiled binary at <os>/<arch>/local-registry, so nothing is compiled here.
FROM alpine:3.24

ARG TARGETPLATFORM

RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 10001 registry \
 && adduser -S -D -H -u 10001 -G registry registry \
 && mkdir -p /data \
 && chown registry:registry /data

COPY ${TARGETPLATFORM}/local-registry /usr/local/bin/local-registry
COPY LICENSE NOTICE /usr/share/licenses/local-registry/

USER registry:registry
ENV LOCAL_REGISTRY_ASSET_PATH=/data \
    LOCAL_REGISTRY_ADDRESS=0.0.0.0 \
    LOCAL_REGISTRY_PORT=8077
VOLUME ["/data"]
EXPOSE 8077

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -q -O /dev/null "http://127.0.0.1:${LOCAL_REGISTRY_PORT}/healthz" || exit 1

ENTRYPOINT ["/usr/local/bin/local-registry"]
CMD ["serve"]
