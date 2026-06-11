.PHONY: build dev test vet lint clean web-build web-static web-dev release install version

# Build variables
BINARY_NAME := shadow
VERSION ?= $(shell awk -F'"' '/^var Version/ {print $$2}' version.go)
LDFLAGS := -ldflags "-X github.com/joevilcai666/shadow.Version=$(VERSION)"
GO := go
GOFLAGS :=
GO_PACKAGES := $(shell $(GO) list ./... | grep -v '/web/node_modules/')

## build: Build the Shadow binary (with embedded web assets)
build: web-static
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
	$(GO) test -v -race $(GO_PACKAGES)

## vet: Run go vet
vet:
	$(GO) vet $(GO_PACKAGES)

## lint: Run Go linter
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run $(GO_PACKAGES)

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

## web-static: Build web assets and copy them to the Go embed directory
web-static: web-build
	rm -rf internal/server/static
	mkdir -p internal/server/static
	cp -R web/dist/. internal/server/static/

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
