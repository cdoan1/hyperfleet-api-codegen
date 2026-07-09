# Makefile for HyperFleet API Codegen

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Directories
API_DIR=api/v1alpha1
PKG_DIR=pkg
CMD_DIR=cmd

# HyperShift configuration
# The import path is resolved via go.mod (recommended approach)
HYPERSHIFT_IMPORT_PATH ?= github.com/openshift/hypershift/api/hypershift/v1beta1
HYPERSHIFT_TYPES ?= HostedClusterSpec,NodePoolSpec

# Legacy: HyperShift source directory (deprecated, use HYPERSHIFT_IMPORT_PATH instead)
# Should point to the root of the HyperShift repo, types are in api/hypershift/v1beta1/
HYPERSHIFT_DIR ?= $(shell echo $$HYPERSHIFT_DIR)
HYPERSHIFT_TYPES_DIR ?= $(HYPERSHIFT_DIR)/api/hypershift/v1beta1

# Tools
CONTROLLER_GEN ?= $(shell pwd)/bin/controller-gen
MARKER_SCANNER ?= $(shell pwd)/bin/marker-scanner
PASSTHROUGH_GEN ?= $(shell pwd)/bin/passthrough-gen
OPENAPI_GEN ?= $(shell pwd)/bin/openapi-gen

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all
all: build-tools lint test ## Build tools, run lint and test (default target)

##@ Development

.PHONY: fmt
fmt: ## Run gofmt on all Go files
	$(GOFMT) -w -s .

.PHONY: vet
vet: ## Run go vet
	$(GOVET) ./...

.PHONY: test
test: ## Run unit tests
	$(GOTEST) -v -race -coverprofile=coverage.out $(shell go list ./... | grep -v '/cmd/')

.PHONY: test-coverage
test-coverage: test ## Run tests and show coverage
	$(GOCMD) tool cover -html=coverage.out

.PHONY: lint
lint: ## Run golangci-lint (requires golangci-lint installed)
	golangci-lint run ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	$(GOMOD) tidy

.PHONY: verify
verify: fmt vet test ## Run all verification steps

##@ Code Generation

.PHONY: generate-registry
generate-registry: $(MARKER_SCANNER) ## Generate field metadata registry from Go markers
	@echo "Generating field metadata registry..."
	$(MARKER_SCANNER) --input-dirs=$(API_DIR) --output-file=$(PKG_DIR)/registry/field_metadata.go

.PHONY: generate-passthrough
generate-passthrough: $(PASSTHROUGH_GEN) ## Generate passthrough types from HyperShift (via go.mod)
	@echo "Generating passthrough types from $(HYPERSHIFT_IMPORT_PATH)..."
	@# Use existing registry if it exists to preserve markers
	@if [ -f "$(PKG_DIR)/registry/field_metadata.json" ]; then \
		echo "Using existing field registry to preserve markers..."; \
		$(PASSTHROUGH_GEN) \
			--import-path=$(HYPERSHIFT_IMPORT_PATH) \
			--types=$(HYPERSHIFT_TYPES) \
			--output-dir=$(API_DIR) \
			--package=v1alpha1 \
			--registry=$(PKG_DIR)/registry/field_metadata.json; \
	else \
		echo "No existing registry found, using safe defaults..."; \
		$(PASSTHROUGH_GEN) \
			--import-path=$(HYPERSHIFT_IMPORT_PATH) \
			--types=$(HYPERSHIFT_TYPES) \
			--output-dir=$(API_DIR) \
			--package=v1alpha1; \
	fi
	@# Copy zz_generated file to committed filename and remove the generated one
	@if [ -f "$(API_DIR)/zz_generated.passthrough.go" ]; then \
		echo "Copying to hostedclusterspec.passthrough.go (committed source of truth)..."; \
		cp $(API_DIR)/zz_generated.passthrough.go $(API_DIR)/hostedclusterspec.passthrough.go; \
		rm $(API_DIR)/zz_generated.passthrough.go; \
		echo "Note: Edit api/v1alpha1/hostedclusterspec.passthrough.go to curate field markers"; \
	fi

# .PHONY: generate-passthrough-local
# generate-passthrough-local: $(PASSTHROUGH_GEN) ## Generate passthrough types from local HyperShift clone (requires HYPERSHIFT_DIR)
# 	@if [ -z "$(HYPERSHIFT_DIR)" ]; then \
# 		echo "Error: HYPERSHIFT_DIR is not set. Export it or set it in the command:"; \
# 		echo "  export HYPERSHIFT_DIR=/path/to/hypershift"; \
# 		echo "  make generate-passthrough-local"; \
# 		echo "Or:"; \
# 		echo "  make generate-passthrough-local HYPERSHIFT_DIR=/path/to/hypershift"; \
# 		exit 1; \
# 	fi
# 	@echo "Generating passthrough types from $(HYPERSHIFT_TYPES_DIR)..."
# 	$(PASSTHROUGH_GEN) \
# 		--source-dir=$(HYPERSHIFT_TYPES_DIR) \
# 		--types=$(HYPERSHIFT_TYPES) \
# 		--output-dir=$(API_DIR) \
# 		--package=v1alpha1

.PHONY: generate-openapi
generate-openapi: $(OPENAPI_GEN) ## Generate OpenAPI schema from Go types
	@echo "Generating OpenAPI schema from $(API_DIR)..."
	@mkdir -p openapi
	$(OPENAPI_GEN) \
		--input-dirs=$(API_DIR) \
		--output-file=openapi/openapi.json \
		--title="HyperFleet API" \
		--version=v1alpha1

.PHONY: featuregate-info
featuregate-info: ## Show feature gate registry and field counts per feature set
	@$(GOBUILD) -o bin/featuregate-info ./cmd/featuregate-info >/dev/null 2>&1
	@./bin/featuregate-info

.PHONY: generate-crd-variants
generate-crd-variants: ## Generate CRD variants for all feature sets
	@echo "Generating CRD variants..."
	@mkdir -p config/crd/variants
	@$(GOBUILD) -o bin/crd-variants ./cmd/crd-variants >/dev/null 2>&1
	@./bin/crd-variants \
		--input=config/crd/bases/_clusters.yaml \
		--base-name=cluster \
		--output-dir=config/crd/variants

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate CRD manifests
	@echo "Generating CRD manifests..."
	$(CONTROLLER_GEN) crd paths="./$(API_DIR)/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: generate-registry generate-passthrough manifests generate-openapi ## Run all code generators

.PHONY: test-hypershift-integration
test-hypershift-integration: $(PASSTHROUGH_GEN) ## Test: Generate from HyperShift via go.mod to test-output/
	@echo "Testing passthrough generation from HyperShift v0.1.70 (via go.mod)..."
	@mkdir -p test-output
	$(PASSTHROUGH_GEN) \
		--import-path=$(HYPERSHIFT_IMPORT_PATH) \
		--types=$(HYPERSHIFT_TYPES) \
		--output-dir=./test-output \
		--package=v1alpha1
	@echo ""
	@echo "Successfully generated from go.mod dependency!"
	@echo "Output: test-output/zz_generated.passthrough.go"
	@wc -l test-output/zz_generated.passthrough.go
	@echo ""
	@echo "Sample (first 20 lines):"
	@head -20 test-output/zz_generated.passthrough.go

##@ Build

.PHONY: build-tools
build-tools: ## Build all codegen tools
	@echo "Building marker scanner..."
	@mkdir -p bin
	$(GOBUILD) -o $(MARKER_SCANNER) ./cmd/marker-scanner
	@echo "Building passthrough generator..."
	$(GOBUILD) -o $(PASSTHROUGH_GEN) ./cmd/passthrough-gen
	@echo "Building OpenAPI generator..."
	$(GOBUILD) -o $(OPENAPI_GEN) ./cmd/openapi-gen
	@echo "Building feature gate info tool..."
	$(GOBUILD) -o bin/featuregate-info ./cmd/featuregate-info

.PHONY: build
build: build-tools ## Build all binaries

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	$(GOGET) -v ./...

.PHONY: deps-update
deps-update: ## Update dependencies
	$(GOGET) -u ./...
	$(GOMOD) tidy

.PHONY: get-hypershift-version
get-hypershift-version: ## Show current HyperShift version in go.mod
	@PSEUDO_VERSION=$$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $$2}'); \
	COMMIT=$$(echo $$PSEUDO_VERSION | rev | cut -d'-' -f1 | rev); \
	echo "Current HyperShift in go.mod:"; \
	echo "  Pseudo-version: $$PSEUDO_VERSION"; \
	echo "  Commit: $$COMMIT"; \
	TAG=$$(curl -s https://api.github.com/repos/openshift/hypershift/tags | jq -r ".[] | select(.commit.sha | startswith(\"$$COMMIT\")) | .name" | head -1); \
	if [ -z "$$TAG" ]; then \
		echo "  Tag: (no tag found - using commit)"; \
	else \
		echo "  Tag: $$TAG"; \
	fi

.PHONY: bump-hypershift
bump-hypershift: ## Bump HyperShift to next patch version (e.g., v0.1.71 → v0.1.72)
	@echo "Fetching latest HyperShift tags..."
	@LATEST_TAG=$$(curl -s https://api.github.com/repos/openshift/hypershift/tags | jq -r '.[0].name'); \
	LATEST_COMMIT=$$(curl -s https://api.github.com/repos/openshift/hypershift/tags | jq -r '.[0].commit.sha'); \
	echo "Latest HyperShift version: $$LATEST_TAG (commit $$LATEST_COMMIT)"; \
	echo "Updating go.mod..."; \
	$(GOGET) github.com/openshift/hypershift/api@$$LATEST_COMMIT && \
	$(GOMOD) tidy && \
	echo "Updated to $$LATEST_TAG" && \
	echo "Next steps:" && \
	echo "  1. Update Makefile comment with new version/commit" && \
	echo "  2. Run: make generate-passthrough" && \
	echo "  3. Review: git diff api/v1alpha1/hostedclusterspec.passthrough.go" && \
	echo "  4. Curate new fields and regenerate: make generate-registry generate-openapi"

.PHONY: bump-hypershift-to
bump-hypershift-to: ## Bump HyperShift to specific version (usage: make bump-hypershift-to VERSION=v0.1.72)
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION not specified"; \
		echo "Usage: make bump-hypershift-to VERSION=v0.1.72"; \
		exit 1; \
	fi
	@echo "Fetching commit for HyperShift $(VERSION)..."
	@COMMIT=$$(curl -s https://api.github.com/repos/openshift/hypershift/tags | jq -r ".[] | select(.name == \"$(VERSION)\") | .commit.sha"); \
	if [ -z "$$COMMIT" ]; then \
		echo "Error: Version $(VERSION) not found"; \
		echo "Check available tags at: https://github.com/openshift/hypershift/tags"; \
		exit 1; \
	fi; \
	echo "Found commit: $$COMMIT"; \
	echo "Updating go.mod..."; \
	$(GOGET) github.com/openshift/hypershift/api@$$COMMIT && \
	$(GOMOD) tidy && \
	echo "Updated to $(VERSION) (commit $$COMMIT)" && \
	echo "Next steps:" && \
	echo "  1. Update Makefile comment with: $(VERSION) (commit $$COMMIT)" && \
	echo "  2. Run: make generate-passthrough" && \
	echo "  3. Review: git diff api/v1alpha1/hostedclusterspec.passthrough.go" && \
	echo "  4. Curate new fields and regenerate: make generate-registry generate-openapi"

$(CONTROLLER_GEN): ## Install controller-gen
	@echo "Installing controller-gen..."
	@mkdir -p bin
	GOBIN=$(shell pwd)/bin $(GOGET) sigs.k8s.io/controller-tools/cmd/controller-gen@latest

$(MARKER_SCANNER): ## Build marker-scanner
	@echo "Building marker scanner..."
	@mkdir -p bin
	$(GOBUILD) -o $(MARKER_SCANNER) ./cmd/marker-scanner

$(PASSTHROUGH_GEN): ## Build passthrough-gen
	@echo "Building passthrough generator..."
	@mkdir -p bin
	$(GOBUILD) -o $(PASSTHROUGH_GEN) ./cmd/passthrough-gen

$(OPENAPI_GEN): ## Build openapi-gen
	@echo "Building OpenAPI generator..."
	@mkdir -p bin
	$(GOBUILD) -o $(OPENAPI_GEN) ./cmd/openapi-gen

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out

.PHONY: clean-generated
clean-generated: ## Clean generated code
	rm -rf config/crd/bases/*
	rm -rf openapi/openapi.yaml
	find $(API_DIR) -name 'zz_generated.*.go' -delete

##@ Documentation

.PHONY: swagger-ui-serve
swagger-ui-serve: ## Serve Swagger UI locally (requires Python 3)
	@echo "Starting local server for Swagger UI..."
	@echo "Swagger UI will be available at: http://localhost:8080/swagger-ui/"
	@echo "OpenAPI spec at: http://localhost:8080/openapi/openapi.json"
	@echo ""
	@echo "Press Ctrl+C to stop the server"
	@echo ""
	@command -v python3 >/dev/null 2>&1 || { echo "Error: python3 is required but not installed"; exit 1; }
	@cd .. && python3 -m http.server 8080 --directory $(shell pwd)

.PHONY: swagger-ui-open
swagger-ui-open: ## Open Swagger UI in browser (may need serve-swagger-ui running)
	@echo "Opening Swagger UI in browser..."
	@command -v open >/dev/null 2>&1 && open http://localhost:8080/swagger-ui/ || \
	command -v xdg-open >/dev/null 2>&1 && xdg-open http://localhost:8080/swagger-ui/ || \
	echo "Please open http://localhost:8080/swagger-ui/ in your browser"

##@ CI

.PHONY: ci-verify
ci-verify: $(MARKER_SCANNER) $(OPENAPI_GEN) ## CI verification that all passthrough fields have required markers
	@echo "Verifying field registry is up to date..."
	@$(MARKER_SCANNER) --input-dirs=$(API_DIR) --output-file=/tmp/field_metadata_check.go
	@diff pkg/registry/field_metadata.go /tmp/field_metadata_check.go || ( \
		echo "Error: pkg/registry/field_metadata.go is out of date"; \
		echo "Run: make generate-registry"; \
		exit 1 \
	)
	@echo "✓ Field registry is up to date"
	@echo ""
	@echo "Verifying OpenAPI schema is up to date..."
	@$(OPENAPI_GEN) --input-dirs=$(API_DIR) --output-file=/tmp/openapi_check.json --title="HyperFleet API" --version=v1alpha1 >/dev/null 2>&1
	@diff openapi/openapi.json /tmp/openapi_check.json || ( \
		echo "Error: openapi/openapi.json is out of date"; \
		echo "Run: make generate-openapi"; \
		exit 1 \
	)
	@echo "✓ OpenAPI schema is up to date"
	@echo ""
	@echo "Verifying all passthrough fields have required markers..."
	@# Check that every field in passthrough files has markers in preceding lines
	@# This is a simple check - just ensure we have roughly same number of fields and markers
	@FIELD_COUNT=$$(grep -c 'json:"[^"]*"' $(API_DIR)/*passthrough.go 2>/dev/null || echo 0); \
	MARKER_COUNT=$$(grep -c '+hyperfleet:write-mode' $(API_DIR)/*passthrough.go 2>/dev/null || echo 0); \
	if [ $$FIELD_COUNT -eq 0 ]; then \
		echo "Warning: No passthrough files found"; \
	elif [ $$MARKER_COUNT -lt $$FIELD_COUNT ]; then \
		echo "Error: Found $$FIELD_COUNT fields but only $$MARKER_COUNT write-mode markers"; \
		echo "All passthrough fields must have +hyperfleet:write-mode markers"; \
		exit 1; \
	else \
		echo "✓ All $$FIELD_COUNT passthrough fields have required markers"; \
	fi

.PHONY: test-hypershift-bump
test-hypershift-bump: ## Test HyperShift version bump workflow (bumps to v0.1.72)
	@echo "Running HyperShift version bump test..."
	@.github/workflows/test-scripts/test-hypershift-bump.sh

.PHONY: test-hypershift-bump-latest
test-hypershift-bump-latest: ## Test HyperShift version bump workflow (bumps to latest)
	@echo "Running HyperShift latest version bump test..."
	@.github/workflows/test-scripts/test-hypershift-bump-latest.sh

.PHONY: ci
ci: deps test lint ci-verify ## Run all CI checks
