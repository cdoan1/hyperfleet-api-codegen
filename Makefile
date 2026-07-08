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
all: lint test ## Run lint and test (default target)

##@ Development

.PHONY: fmt
fmt: ## Run gofmt on all Go files
	$(GOFMT) -w -s .

.PHONY: vet
vet: ## Run go vet
	$(GOVET) ./...

.PHONY: test
test: ## Run unit tests
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

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
	$(PASSTHROUGH_GEN) \
		--import-path=$(HYPERSHIFT_IMPORT_PATH) \
		--types=$(HYPERSHIFT_TYPES) \
		--output-dir=$(API_DIR) \
		--package=v1alpha1

.PHONY: generate-passthrough-local
generate-passthrough-local: $(PASSTHROUGH_GEN) ## Generate passthrough types from local HyperShift clone (requires HYPERSHIFT_DIR)
	@if [ -z "$(HYPERSHIFT_DIR)" ]; then \
		echo "Error: HYPERSHIFT_DIR is not set. Export it or set it in the command:"; \
		echo "  export HYPERSHIFT_DIR=/path/to/hypershift"; \
		echo "  make generate-passthrough-local"; \
		echo "Or:"; \
		echo "  make generate-passthrough-local HYPERSHIFT_DIR=/path/to/hypershift"; \
		exit 1; \
	fi
	@echo "Generating passthrough types from $(HYPERSHIFT_TYPES_DIR)..."
	$(PASSTHROUGH_GEN) \
		--source-dir=$(HYPERSHIFT_TYPES_DIR) \
		--types=$(HYPERSHIFT_TYPES) \
		--output-dir=$(API_DIR) \
		--package=v1alpha1

.PHONY: generate-openapi
generate-openapi: $(OPENAPI_GEN) ## Generate OpenAPI schema from Go types
	@echo "Generating OpenAPI schema..."
	$(OPENAPI_GEN) --input-dirs=$(API_DIR) --output-file=openapi/openapi.yaml

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate CRD manifests
	@echo "Generating CRD manifests..."
	$(CONTROLLER_GEN) crd paths="./$(API_DIR)/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: generate-registry generate-passthrough manifests generate-openapi ## Run all code generators

.PHONY: demo-passthrough
demo-passthrough: $(PASSTHROUGH_GEN) ## Demo: Generate passthrough types to /tmp (no HYPERSHIFT_DIR needed)
	@echo "Running passthrough-gen demo with examples..."
	@mkdir -p /tmp/demo-output
	$(PASSTHROUGH_GEN) \
		--source-dir=./examples \
		--types=ClusterSpec,HostedClusterPassthrough \
		--output-dir=/tmp/demo-output \
		--package=demo
	@echo "Demo output generated at /tmp/demo-output"
	@ls -lh /tmp/demo-output

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

##@ CI

.PHONY: ci-verify
ci-verify: fmt vet ## CI verification that all passthrough fields have required markers
	@echo "Verifying all passthrough fields have required markers..."
	@# TODO: implement marker verification script
	@echo "Marker verification not yet implemented"

.PHONY: ci
ci: deps verify ci-verify ## Run all CI checks
