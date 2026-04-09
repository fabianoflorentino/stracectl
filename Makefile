# ============================================================================
# stracectl — Makefile
# ============================================================================
# Development environment management for stracectl and the Hugo docs site
# ============================================================================

.PHONY: help \
	build run test test-short coverage fmt vet lint tidy clean clean-cache clean-images all \
	generate-bpf build-ebpf clean-bpf \
	site-dev site-build site-clean \
	docker-build docker-build-dev docker-build-site docker-push \
	up up-site up-detach down logs logs-tail ps restart prune \
	check-deps info

# ============================================================================
# Variables
# ============================================================================

BINARY        := stracectl
MODULE        := github.com/fabianoflorentino/stracectl
IMAGE         := fabianoflorentino/stracectl
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS       := -s -w -X main.version=$(VERSION)
BUILD_FLAGS   := CGO_ENABLED=0 GOARCH=amd64
GOTEST_FLAGS  := -v -race -count=1

SITE_DIR      := site
SITE_BASE_URL ?= http://localhost:1313/stracectl/

# ============================================================================
# Output colors
# ============================================================================

RED    := \033[0;31m
GREEN  := \033[0;32m
YELLOW := \033[0;33m
BLUE   := \033[0;34m
NC     := \033[0m

# ============================================================================
# Default target
# ============================================================================

.DEFAULT_GOAL := help

##@ Help

help: ## Show this help message
	@echo ""
	@echo -e "$(BLUE)╔══════════════════════════════════════════════════════════════╗$(NC)"
	@echo -e "$(BLUE)║               stracectl — Available Commands                 ║$(NC)"
	@echo -e "$(BLUE)╚══════════════════════════════════════════════════════════════╝$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""

##@ Go — Build & Run

build: ## Compile the binary to ./bin/stracectl
	@echo -e "$(BLUE)🔨 Building $(BINARY)...$(NC)"
	@mkdir -p bin
	@$(BUILD_FLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) .
	@echo -e "$(GREEN)✓ Binary created at bin/$(BINARY)$(NC)"

##@ BPF

generate-bpf: ## Generate BPF artifacts (requires clang + bpf2go)
	@echo -e "$(BLUE)🔧 Generating BPF artifacts...$(NC)"
	@command -v clang >/dev/null 2>&1 || { echo -e "$(RED)❌ clang not found (required to compile BPF programs)$(NC)"; exit 1; }
	@go generate -tags=ebpf ./internal/tracer/...
	@echo -e "$(GREEN)✓ BPF artifacts generated$(NC)"

# Build only the bpf-build stage and export vmlinux.h from the build image
.PHONY: export-vmlinux
export-vmlinux: ## Build bpf-build stage and copy vmlinux.h to internal/tracer/bpf/
	@echo -e "$(BLUE)📦 Building bpf-build stage to export vmlinux.h...$(NC)"
	@DOCKER_BUILDKIT=1 docker build --no-cache --target bpf-build -t stracectl-bpf -f Dockerfile .
	@echo -e "$(BLUE)📤 Extracting vmlinux.h from build image...$(NC)"
	-@docker create --name stracectl-bpf-export stracectl-bpf >/dev/null 2>&1 || true
	-@docker cp stracectl-bpf-export:/bpf/vmlinux.h internal/tracer/bpf/vmlinux.h >/dev/null 2>&1 || (echo "warning: vmlinux.h not found inside image" && true)
	-@docker rm -f stracectl-bpf-export >/dev/null 2>&1 || true
	@echo -e "$(GREEN)✓ vmlinux.h exported to internal/tracer/bpf/vmlinux.h$(NC)"

build-ebpf: generate-bpf ## Build the binary with eBPF support (uses -tags=ebpf)
	@echo -e "$(BLUE)🔨 Building $(BINARY) with eBPF support...$(NC)"
	@mkdir -p bin
	@$(BUILD_FLAGS) go build -tags=ebpf -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) .
	@echo -e "$(GREEN)✓ Binary created at bin/$(BINARY) (ebpf)$(NC)"

clean-bpf: ## Remove generated BPF artifacts
	@echo -e "$(YELLOW)🧹 Removing generated BPF artifacts...$(NC)"
	@rm -f internal/tracer/ebpf_bpfel.go internal/tracer/ebpf_bpfeb.go internal/tracer/ebpf_bpfel.o internal/tracer/ebpf_bpfeb.o
	@echo -e "$(GREEN)✓ BPF artifacts removed$(NC)"

run: ## Run with go run (pass ARGS="..." for extra flags)
	@echo -e "$(BLUE)▶  Running $(BINARY)...$(NC)"
	@go run . $(ARGS)

##@ Go — Tests & Quality

all: fmt vet test build ## Format, vet, test and build everything

test: ## Run all unit tests (with race detector)
	@echo -e "$(BLUE)🧪 Running tests...$(NC)"
	@go test $(GOTEST_FLAGS) ./...
	@echo -e "$(GREEN)✓ All tests passed!$(NC)"

test-short: ## Run tests without the race detector (faster)
	@echo -e "$(BLUE)🧪 Running tests (fast mode)...$(NC)"
	@go test -v -count=1 ./...
	@echo -e "$(GREEN)✓ Tests completed!$(NC)"

coverage: ## Run tests and generate an HTML coverage report
	@echo -e "$(BLUE)📊 Generating coverage report...$(NC)"
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo -e "$(GREEN)✓ Report generated: coverage.html$(NC)"

fmt: ## Format Go source files
	@echo -e "$(BLUE)✏️  Formatting code...$(NC)"
	@go fmt ./...
	@echo -e "$(GREEN)✓ Formatting done!$(NC)"

vet: ## Run go vet
	@echo -e "$(BLUE)🔍 Running go vet...$(NC)"
	@go vet ./...
	@echo -e "$(GREEN)✓ go vet passed!$(NC)"

lint: ## Run golangci-lint (must be installed)
	@echo -e "$(BLUE)🔍 Running golangci-lint...$(NC)"
	@golangci-lint run ./...
	@echo -e "$(GREEN)✓ Lint passed!$(NC)"

tidy: ## Tidy and verify Go modules
	@echo -e "$(BLUE)📦 Tidying modules...$(NC)"
	@go mod tidy
	@go mod verify
	@echo -e "$(GREEN)✓ Modules up to date!$(NC)"

clean: ## Remove build artifacts
	@echo -e "$(YELLOW)🧹 Removing artifacts...$(NC)"
	@$(MAKE) clean-images || true
	@rm -rf bin/ coverage.out coverage.html
	@echo -e "$(GREEN)✓ Clean done!$(NC)"

clean-cache: ## Clear the Go build cache (fixes version mismatch errors)
	@echo -e "$(YELLOW)🧹 Clearing Go build cache...$(NC)"
	@go clean -cache
	@echo -e "$(GREEN)✓ Go build cache cleared!$(NC)"

##@ Site (Hugo)

site-dev: ## Start the Hugo dev server with live-reload (requires Hugo extended)
	@echo -e "$(BLUE)🚀 Starting Hugo server...$(NC)"
	@echo -e "$(YELLOW)📝 Site available at: $(SITE_BASE_URL)$(NC)"
	@hugo server \
		--source $(SITE_DIR) \
		--bind 0.0.0.0 \
		--disableFastRender \
		--buildDrafts \
		--baseURL "$(SITE_BASE_URL)"

site-build: ## Build the static site into site/public/ (minified)
	@echo -e "$(BLUE)🏗️  Building static site...$(NC)"
	@hugo --source $(SITE_DIR) --minify
	@echo -e "$(GREEN)✓ Site built at $(SITE_DIR)/public$(NC)"

site-clean: ## Remove the site/public/ directory
	@echo -e "$(YELLOW)🧹 Removing $(SITE_DIR)/public/...$(NC)"
	@rm -rf $(SITE_DIR)/public
	@echo -e "$(GREEN)✓ $(SITE_DIR)/public/ removed!$(NC)"

##@ Docker — Images

docker-build: ## Build the production Docker image (distroless)
	@echo -e "$(BLUE)🔨 Building production image...$(NC)"
	@docker build --target production \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest .
	@echo -e "$(GREEN)✓ Image $(IMAGE):$(VERSION) created!$(NC)"

docker-build-dev: ## Build the development Docker image
	@echo -e "$(BLUE)🔨 Building development image...$(NC)"
	@docker build --target development -t $(IMAGE):dev .
	@echo -e "$(GREEN)✓ Image $(IMAGE):dev created!$(NC)"

docker-inspect-dev: ## Build development image and show image size and history
	@echo -e "$(BLUE)🔎 Building and inspecting development image...$(NC)"
	@docker build --target development -t $(IMAGE):dev .
	@echo -e "$(YELLOW)Image list:$(NC)"
	@docker images $(IMAGE):dev
	@echo -e "$(YELLOW)Image size (bytes):$(NC)"
	@docker image inspect $(IMAGE):dev --format '{{.Size}}' || true
	@echo -e "$(YELLOW)Image history:$(NC)"
	@docker history $(IMAGE):dev

docker-dive-dev: ## Run dive on the development image (if installed)
	@command -v dive >/dev/null 2>&1 || { echo -e "$(RED)dive not found; install dive to use this target$(NC)"; exit 1; }
	@dive $(IMAGE):dev

docker-build-site: ## Build the Hugo site Docker image
	@echo -e "$(BLUE)🔨 Building site image...$(NC)"
	@docker build --target site -t $(IMAGE):site .
	@echo -e "$(GREEN)✓ Image $(IMAGE):site created!$(NC)"

docker-push: ## Push production images to the registry
	@echo -e "$(BLUE)📤 Pushing images to the registry...$(NC)"
	@docker push $(IMAGE):$(VERSION)
	@docker push $(IMAGE):latest
	@echo -e "$(GREEN)✓ Images pushed successfully!$(NC)"

clean-images: ## Remove Docker images built by this project
	@echo -e "$(YELLOW)🧹 Removing Docker images for $(IMAGE)...$(NC)"
	@command -v docker >/dev/null 2>&1 || { echo -e "$(YELLOW)Docker not found; skipping image cleanup$(NC)"; exit 0; }
	@ids=$$(docker images --filter=reference="$(IMAGE)*" -q); \
	if [ -n "$$ids" ]; then \
		docker rmi -f $$ids && echo -e "$(GREEN)✓ Removed images for $(IMAGE)$(NC)"; \
	else \
		echo -e "$(GREEN)✓ No images found for $(IMAGE)$(NC)"; \
	fi
##@ Docker Compose — Services

up: ## Start stracectl in dev mode with live-reload
	@echo -e "$(BLUE)🚀 Starting stracectl (dev)...$(NC)"
	@docker compose up

up-site: ## Start the Hugo dev server via docker compose
	@echo -e "$(BLUE)🚀 Starting Hugo site...$(NC)"
	@docker compose up site

up-detach: ## Start all services in the background
	@echo -e "$(BLUE)🚀 Starting services in the background...$(NC)"
	@docker compose up -d
	@echo -e "$(GREEN)✓ Services running in the background!$(NC)"

down: ## Stop and remove all compose containers
	@echo -e "$(BLUE)🛑 Stopping services...$(NC)"
	@docker compose down
	@echo -e "$(GREEN)✓ Services stopped!$(NC)"

logs: ## Follow compose service logs (Ctrl-C to exit)
	@docker compose logs -f

logs-tail: ## Show the last 100 lines of logs
	@docker compose logs --tail=100

ps: ## List running compose services
	@echo -e "$(BLUE)📊 Service status:$(NC)"
	@docker compose ps

restart: ## Restart all compose services
	@echo -e "$(BLUE)🔄 Restarting services...$(NC)"
	@docker compose restart
	@echo -e "$(GREEN)✓ Services restarted!$(NC)"

prune: ## Remove dangling images and stopped containers (safe cleanup)
	@echo -e "$(YELLOW)🧹 Removing unused Docker resources...$(NC)"
	@docker image prune -f
	@docker container prune -f
	@echo -e "$(GREEN)✓ Cleanup done!$(NC)"

##@ Utilities

check-deps: ## Check that all required dependencies are installed
	@echo -e "$(BLUE)🔍 Checking dependencies...$(NC)"
	@command -v go     >/dev/null 2>&1 || { echo -e "$(RED)❌ Go is not installed$(NC)";      exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo -e "$(RED)❌ Docker is not installed$(NC)";  exit 1; }
	@command -v hugo   >/dev/null 2>&1 || { echo -e "$(RED)❌ Hugo is not installed$(NC)";    exit 1; }
	@echo -e "$(GREEN)✓ All dependencies are installed!$(NC)"

info: ## Show development environment information
	@echo -e "$(BLUE)ℹ️  Environment info:$(NC)"
	@echo -e "$(YELLOW)Go:$(NC)"
	@go version
	@echo -e "$(YELLOW)Docker:$(NC)"
	@docker --version
	@echo -e "$(YELLOW)Docker Compose:$(NC)"
	@docker compose version
	@echo -e "$(YELLOW)Hugo:$(NC)"
	@hugo version 2>/dev/null || echo "  Hugo not found"
	@echo -e "$(YELLOW)Version:$(NC) $(VERSION)"

##@ Changelog

update-changelog: ## Update changelogs for a version (Usage: make update-changelog VERSION=vX.Y.Z [NOTES="..."] [NOTES_FILE=path])
	@test -n "$(VERSION)" || (echo "Provide VERSION=vX.Y.Z"; exit 1)
	@if [ -n "$(NOTES_FILE)" ]; then \
		bash scripts/update_changelog.sh --version "$(VERSION)" --notes-file "$(NOTES_FILE)"; \
	elif [ -n "$(NOTES)" ]; then \
		bash scripts/update_changelog.sh --version "$(VERSION)" --notes "$(NOTES)"; \
	else \
		bash scripts/update_changelog.sh --version "$(VERSION)"; \
	fi && echo "Updated changelogs for $(VERSION)"

.PHONY: go-clean
go-clean: ## Clean Go build, module and test caches
	@echo -e "$(YELLOW)🧹 Cleaning Go caches...$(NC)"
	@go clean -cache -modcache -testcache
	@echo -e "$(GREEN)✓ Go caches cleaned$(NC)"
