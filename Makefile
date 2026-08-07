VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test race vet smoke policy check build-all clean

build:
	mkdir -p dist
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/ayame-diff ./cmd/ayame-diff

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

smoke: build
	./scripts/smoke-test.sh ./dist/ayame-diff

policy:
	./scripts/check-adr-i18n.sh
	./scripts/check-docs-i18n.sh
	./scripts/check-contributor-policy.sh

check: policy test race vet smoke

build-all:
	VERSION=$(VERSION) ./scripts/build-all.sh

clean:
	rm -rf dist

.PHONY: graphify-setup graphify-update

## Install graphify and register its skill with Claude, Copilot and Codex.
graphify-setup:
	@sh scripts/graphify.sh setup

## Upgrade graphify, refresh the skill, and update the knowledge graph.
graphify-update:
	@sh scripts/graphify.sh update
