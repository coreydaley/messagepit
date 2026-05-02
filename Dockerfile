FROM golang:alpine AS builder

ARG VERSION=dev

WORKDIR /app

RUN apk add --no-cache git npm

# Cache Go module downloads separately from source — only re-runs when go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Cache npm installs separately — only re-runs when package files change
COPY package.json package-lock.json ./
RUN npm install

# Copy remaining source and build
COPY . .
RUN npm run package && \
    CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/coreydaley/messagepit/config.Version=${VERSION}" -o /messagepit

FROM alpine:latest

LABEL org.opencontainers.image.title="MessagePit" \
  org.opencontainers.image.description="An email and SMS testing tool with API for developers" \
  org.opencontainers.image.source="https://github.com/coreydaley/messagepit" \
  org.opencontainers.image.licenses="MIT"

COPY --from=builder /messagepit /messagepit

RUN apk upgrade --no-cache && apk add --no-cache tzdata

EXPOSE 1025/tcp 1110/tcp 8025/tcp 8100/tcp 8200/tcp 8300/tcp

HEALTHCHECK --interval=15s --start-period=10s --start-interval=1s CMD ["/messagepit", "readyz"]

ENTRYPOINT ["/messagepit"]
