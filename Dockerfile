FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate

FROM alpine:3.22 AS api

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/api /app/api
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM alpine:3.22 AS migrate

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/migrate /app/migrate
COPY migrations /app/migrations
USER app
ENTRYPOINT ["/app/migrate"]
