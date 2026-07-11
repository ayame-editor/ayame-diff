VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test race vet check build-all clean

build:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/ayame-diff ./cmd/ayame-diff

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

check: test race vet

build-all:
	VERSION=$(VERSION) ./scripts/build-all.sh

clean:
	rm -rf dist
