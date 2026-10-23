# Upstream community images were withdrawn; build pinned official sources.
FROM golang:1.25 AS server-build
RUN CGO_ENABLED=0 go install github.com/minio/minio@RELEASE.2025-10-15T17-29-55Z
FROM golang:1.25 AS client-build
RUN CGO_ENABLED=0 go install github.com/minio/mc@RELEASE.2025-08-13T08-35-41Z
FROM debian:bookworm-slim AS server
COPY --from=server-build /go/bin/minio /usr/local/bin/minio
EXPOSE 9000 9001
ENTRYPOINT ["minio"]
FROM debian:bookworm-slim AS client
COPY --from=client-build /go/bin/mc /usr/local/bin/mc
ENTRYPOINT ["mc"]
