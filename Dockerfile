# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/sentinal-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/sentinal-migrate ./cmd/migrate

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates curl tzdata

COPY --from=builder /out/sentinal-api /usr/local/bin/sentinal-api
COPY --from=builder /out/sentinal-migrate /usr/local/bin/sentinal-migrate
COPY migrations ./migrations

RUN addgroup -S sentinal && adduser -S -G sentinal sentinal && chown -R sentinal:sentinal /app

USER sentinal

EXPOSE 5000

CMD ["/usr/local/bin/sentinal-api"]