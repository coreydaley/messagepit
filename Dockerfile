FROM golang:alpine AS builder

ARG VERSION=dev

COPY . /app

WORKDIR /app

RUN apk upgrade && apk add git npm && \
npm install && npm run package && \
CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/coreydaley/messagepit/config.Version=${VERSION}" -o /messagepit

FROM alpine:latest

LABEL org.opencontainers.image.title="MessagePit" \
  org.opencontainers.image.description="An email and SMS testing tool with API for developers" \
  org.opencontainers.image.source="https://github.com/coreydaley/messagepit" \
  org.opencontainers.image.licenses="MIT"

COPY --from=builder /messagepit /messagepit

RUN apk upgrade --no-cache && apk add --no-cache tzdata

EXPOSE 1025/tcp 1110/tcp 1775/tcp 8025/tcp

HEALTHCHECK --interval=15s --start-period=10s --start-interval=1s CMD ["/messagepit", "readyz"]

ENTRYPOINT ["/messagepit"]
