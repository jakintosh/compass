FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/compass ./cmd/compass

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/compass /app/compass
COPY internal/app/templates /app/internal/app/templates
COPY internal/app/static /app/internal/app/static

EXPOSE 80
VOLUME ["/app/data"]
ENTRYPOINT ["/app/compass"]
CMD ["serve", "--addr", ":80", "--data-dir", "/app/data"]
