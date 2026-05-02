BINARY    := messagepit
MODULE    := github.com/coreydaley/messagepit
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)-$(shell date +%s)
LDFLAGS   := -s -w -X $(MODULE)/config.Version=$(VERSION)

.PHONY: all build ui run clean lint lint-fix test docker swagger help

all: ui build

## build: compile the Go binary (requires ui assets to exist)
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

## ui: build frontend assets (minified)
ui:
	npm run build

## ui-watch: rebuild frontend assets on change
ui-watch:
	npm run watch

## run: build and run the binary with sane dev defaults
run: all
	./$(BINARY) \
		--smtp 0.0.0.0:1025 \
		--mailtrap 0.0.0.0:8100 \
		--sendgrid 0.0.0.0:8101 \
		--twilio 0.0.0.0:8200 \
		--webhook 0.0.0.0:8300 \
		--listen 0.0.0.0:8025 \
		--disable-version-check

## dev: run from source without producing a binary (does not rebuild ui)
dev:
	CGO_ENABLED=0 go run . \
		--smtp 0.0.0.0:1025 \
		--mailtrap 0.0.0.0:8100 \
		--sendgrid 0.0.0.0:8101 \
		--twilio 0.0.0.0:8200 \
		--webhook 0.0.0.0:8300 \
		--listen 0.0.0.0:8025 \
		--disable-version-check

## lint: run gofmt, Go vet, eslint, and prettier checks
lint:
	gofmt -s -w . && git diff --exit-code
	go vet ./...
	npm run lint

## lint-fix: auto-fix JS/CSS lint and formatting issues
lint-fix:
	gofmt -s -w .
	npm run lint-fix

## test: run Go test suite
test:
	go test ./...

## docker: build the Docker image
docker:
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY):$(VERSION) -t $(BINARY):latest .

## install: install the binary to $GOPATH/bin (requires ui assets)
install: ui
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(shell go env GOPATH)/bin/$(BINARY) .

## swagger: regenerate the API swagger spec (requires go-swagger: go install github.com/go-swagger/go-swagger/cmd/swagger@latest)
swagger:
	swagger generate spec -w . -i server/apiv1/swagger-config.yml -o server/ui/api/v1/swagger.json --scan-models

## clean: remove the compiled binary and built ui assets
clean:
	rm -f $(BINARY)
	rm -f server/ui/dist/app.js server/ui/dist/app.css server/ui/dist/docs.js
	rm -f server/ui/dist/*.woff server/ui/dist/*.woff2

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## /  /'
