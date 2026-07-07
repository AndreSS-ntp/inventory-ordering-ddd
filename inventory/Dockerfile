FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY proto/ proto/
COPY inventory/ inventory/
WORKDIR /src/inventory
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -o /out/inventory ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
COPY --from=builder /out/inventory /usr/local/bin/inventory
USER appuser
EXPOSE 50052
ENTRYPOINT ["/usr/local/bin/inventory"]
