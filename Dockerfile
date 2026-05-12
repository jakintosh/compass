FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/compass ./cmd/compass

FROM alpine:3.22

WORKDIR /app

COPY --from=build /out/compass /app/compass
COPY internal/web/templates /app/internal/web/templates
COPY internal/web/static /app/internal/web/static

EXPOSE 8080
ENTRYPOINT ["/app/compass"]
CMD ["serve"]
