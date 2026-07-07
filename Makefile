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
generate-passthrough: $(PASSTHROUGH_GEN) ## Generate passthrough types from HyperShift CRDs
	@echo "Generating passthrough types..."
	$(PASSTHROUGH_GEN) --registry=$(PKG_DIR)/registry/field_metadata.go --output-dir=$(API_DIR)

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

$(MARKER_SCANNER): ## Build marker-scanner (placeholder until implemented)
	@echo "Marker scanner not yet implemented"
	@exit 1

$(PASSTHROUGH_GEN): ## Build passthrough-gen (placeholder until implemented)
	@echo "Passthrough generator not yet implemented"
	@exit 1

$(OPENAPI_GEN): ## Build openapi-gen (placeholder until implemented)
	@echo "OpenAPI generator not yet implemented"
	@exit 1

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
