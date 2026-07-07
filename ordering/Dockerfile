FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY proto/ proto/
COPY ordering/ ordering/
WORKDIR /src/ordering
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -o /out/ordering ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/ordering /usr/local/bin/ordering
USER appuser
EXPOSE 50051 8080
ENTRYPOINT ["/usr/local/bin/ordering"]
