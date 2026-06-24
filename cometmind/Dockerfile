# Build from repository root:
#   docker build -f cometmind/Dockerfile -t cometmind:local .

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY comet-sdk/ ./comet-sdk/
COPY cometmind/ ./cometmind/

WORKDIR /src/cometmind
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cometmind .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app
COPY --from=builder /out/cometmind /app/cometmind

ENV COMETMIND_DATA_DIR=/data
ENV COMETMIND_BIND_ADDR=0.0.0.0

VOLUME ["/data"]

EXPOSE 7700

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:7700/api/v1/health || exit 1

ENTRYPOINT ["/app/cometmind"]
CMD ["serve", "--port", "7700", "--bind", "0.0.0.0"]
