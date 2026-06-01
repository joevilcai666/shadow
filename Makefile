.PHONY: build dev test lint clean web-build web-dev release install version

# Build variables
BINARY_NAME := shadow
VERSION ?= $(shell grep 'Version' version.go | cut -d'"' -f2)
LDFLAGS := -ldflags "-X github.com/joevilcai666/shadow/shadow.Version=$(VERSION)"
GO := go
GOFLAGS :=

## build: Build the Shadow binary (with embedded web assets)
build: web-build
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/shadow/

## dev: Start daemon backend + web dev server concurrently
dev:
	@echo "Starting Shadow dev environment..."
	@$(MAKE) -j2 dev-backend dev-web

dev-backend:
	$(GO) run ./cmd/shadow/ serve --dev

dev-web:
	cd web && npm run dev

## test: Run all Go tests
test:
	$(GO) test -v -race ./...

## lint: Run Go linter
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf web/dist

## web-setup: Install web dependencies
web-setup:
	cd web && npm install

## web-dev: Start web dev server only
web-dev:
	cd web && npm run dev

## web-build: Build web assets for embedding
web-build:
	@if [ ! -d web/node_modules ]; then cd web && npm install; fi
	cd web && npm run build

## install: Install shadow to /usr/local/bin
install: build
	cp $(BINARY_NAME) /usr/local/bin/shadow
	@echo "Installed shadow $(VERSION) to /usr/local/bin/shadow"

## release: Create a git tag and push to trigger GoReleaser
release:
	@if [ -z "$(v)" ]; then echo "Usage: make release v=X.Y.Z"; exit 1; fi
	git tag -a v$(v) -m "Release v$(v)"
	git push origin v$(v)
	@echo "Pushed tag v$(v) — GoReleaser will build and publish."

## version: Print current version
version:
	@echo "Shadow $(VERSION)"
