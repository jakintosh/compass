# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/compass ./cmd/compass

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/compass /app/compass
COPY internal/app/templates /app/internal/app/templates
COPY internal/app/static /app/internal/app/static

EXPOSE 80
VOLUME ["/app/data"]
ENTRYPOINT ["/app/compass"]
CMD ["serve", "--addr", ":80", "--data-dir", "/app/data", "--config-dir", "/app/config"]
