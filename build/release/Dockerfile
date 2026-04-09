# Build any Go service in the monorepo. Required build-args:
#   SERVICE   — directory name: api-server | controller | auth-sidecar | contract-syncer
#   OCI_NAME  — short name for org.opencontainers.image.title (e.g. "API Server")
#
# Examples:
#   docker build --build-arg SERVICE=controller --build-arg OCI_NAME=Controller \
#     --target runtime-alpine -f build/release/Dockerfile .

FROM golang:1.26.2-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

ARG SERVICE
WORKDIR /${SERVICE}

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY pkg/ pkg/
COPY cmd/${SERVICE}/ cmd/${SERVICE}/
COPY internal/shared/ internal/shared/
COPY internal/${SERVICE}/ internal/${SERVICE}/

ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o main cmd/${SERVICE}/main.go

FROM gcr.io/distroless/static-debian12:nonroot AS runtime-distroless

ARG SERVICE
ARG OCI_NAME

WORKDIR /${SERVICE}

COPY --from=builder --chown=nonroot:nonroot /${SERVICE}/main ./main

USER nonroot:nonroot

LABEL org.opencontainers.image.title="Merionyx API Gateway — ${OCI_NAME}" \
      org.opencontainers.image.source="https://github.com/merionyx/api-gateway" \
      org.opencontainers.image.url="https://github.com/merionyx/api-gateway" \
      org.opencontainers.image.documentation="https://github.com/merionyx/api-gateway-control-plane#readme" \
      org.opencontainers.image.vendor="Merionyx" \
      org.opencontainers.image.logo="https://avatars.githubusercontent.com/u/208366109?s=512" \
      com.docker.extension.icon="https://avatars.githubusercontent.com/u/208366109?s=512" \
      dev.orbstack.icon="https://avatars.githubusercontent.com/u/208366109?s=512"

EXPOSE 8080 8443 8444

CMD ["./main"]

FROM alpine:3.23 AS runtime-alpine

ARG SERVICE
ARG OCI_NAME

RUN apk add --no-cache ca-certificates \
    && addgroup -g 1001 app \
    && adduser -u 1001 -G app -s /bin/sh -D app

WORKDIR /${SERVICE}

COPY --from=builder --chown=1001:1001 /${SERVICE}/main ./main

USER app

LABEL org.opencontainers.image.title="Merionyx API Gateway — ${OCI_NAME} (Alpine)" \
      org.opencontainers.image.source="https://github.com/merionyx/api-gateway" \
      org.opencontainers.image.url="https://github.com/merionyx/api-gateway" \
      org.opencontainers.image.documentation="https://github.com/merionyx/api-gateway-control-plane#readme" \
      org.opencontainers.image.vendor="Merionyx" \
      org.opencontainers.image.logo="https://avatars.githubusercontent.com/u/208366109?s=512" \
      com.docker.extension.icon="https://avatars.githubusercontent.com/u/208366109?s=512" \
      dev.orbstack.icon="https://avatars.githubusercontent.com/u/208366109?s=512"

EXPOSE 8080 8443 8444

CMD ["./main"]
