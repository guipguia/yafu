SHELL := /bin/sh
.DEFAULT_GOAL := help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X github.com/guipguia/yafu/internal/version.Version=$(VERSION) \
  -X github.com/guipguia/yafu/internal/version.Commit=$(COMMIT) \
  -X github.com/guipguia/yafu/internal/version.Date=$(DATE)

GO        ?= go
NPM       ?= npm
DOCKER    ?= docker
IMAGE     ?= ghcr.io/guipguia/yafu
IMAGE_TAG ?= $(VERSION)

WEB_DIR   := web
BIN_DIR   := bin
EMBED_DIR := internal/web/dist

.PHONY: help
help: ## Show this help
	@awk 'BEGIN { FS = ":.*##"; printf "\nUsage: make \033[36m<target>\033[0m\n\nTargets:\n" } /^[a-zA-Z][a-zA-Z0-9_-]*:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---- dev ----

.PHONY: dev
dev: ## Run Go server (:8080) and Vite dev server (:5173) in parallel
	@$(MAKE) -j 2 dev-server dev-web

.PHONY: dev-server
dev-server: ## Run Go server with debug logs (no embedded UI)
	$(GO) run ./cmd/yafu --log-level debug

.PHONY: dev-web
dev-web: ## Run Vite dev server
	cd $(WEB_DIR) && $(NPM) run dev

# ---- install ----

.PHONY: install
install: ## Install Go modules and web dependencies
	$(GO) mod download
	cd $(WEB_DIR) && $(NPM) install

# ---- build ----

.PHONY: build
build: build-web build-go-embed ## Production build (web + Go with embedded UI)

.PHONY: build-go
build-go: ## Build Go binary without embedded UI (dev)
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/yafu ./cmd/yafu

.PHONY: build-go-embed
build-go-embed: $(EMBED_DIR) ## Build Go binary with embedded UI
	mkdir -p $(BIN_DIR)
	$(GO) build -tags embed -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/yafu ./cmd/yafu

.PHONY: build-web
build-web: ## Build web production bundle into web/dist
	cd $(WEB_DIR) && $(NPM) run build

$(EMBED_DIR): build-web
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	cp -r $(WEB_DIR)/dist/. $(EMBED_DIR)/

# ---- test / lint ----

.PHONY: test
test: test-go test-web ## Run all tests

.PHONY: test-go
test-go: ## Run Go tests
	$(GO) test ./...

.PHONY: test-web
test-web: ## Run web tests once
	cd $(WEB_DIR) && $(NPM) test -- --run

.PHONY: lint
lint: lint-go lint-web ## Run all linters

.PHONY: lint-go
lint-go: ## Lint Go code
	$(GO) vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "(golangci-lint not installed; skipping)"

.PHONY: lint-web
lint-web: ## Lint web (eslint, prettier, tsc)
	cd $(WEB_DIR) && $(NPM) run lint
	cd $(WEB_DIR) && $(NPM) run format:check
	cd $(WEB_DIR) && $(NPM) run typecheck

.PHONY: fmt
fmt: ## Format Go and web code
	$(GO) fmt ./...
	cd $(WEB_DIR) && $(NPM) run format

# ---- container image ----

.PHONY: image
image: ## Build container image
	$(DOCKER) build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg COMMIT=$(COMMIT) \
	  --build-arg DATE=$(DATE) \
	  -t $(IMAGE):$(IMAGE_TAG) \
	  -t $(IMAGE):latest \
	  .

# ---- maintenance ----

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(WEB_DIR)/dist $(EMBED_DIR)

.PHONY: tidy
tidy: ## Tidy Go modules
	$(GO) mod tidy
