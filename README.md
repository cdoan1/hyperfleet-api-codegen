# HyperFleet API Codegen

[![Go Report Card](https://goreportcard.com/badge/github.com/cdoan1/hyperfleet-api-codegen)](https://goreportcard.com/report/github.com/cdoan1/hyperfleet-api-codegen)
[![Coverage](https://img.shields.io/badge/coverage-73%25-brightgreen)](https://github.com/cdoan1/hyperfleet-api-codegen)
[![Go Version](https://img.shields.io/github/go-mod/go-version/cdoan1/hyperfleet-api-codegen)](go.mod)

Code generation tools for managing the HyperFleet API: HyperShift CRD → HyperFleet CRD → Platform API OpenAPI.

## Overview

This repository contains code generation tooling that manages three layers of types with a single source of truth:

- **HyperShift CRDs**: Full HostedCluster and NodePool from upstream HyperShift
- **HyperFleet CRDs**: Wrappers (Cluster/NodePool) and native resources  
- **Platform API**: REST API with OpenAPI spec generated from HyperFleet CRDs

Go markers on types control visibility, write modes, and feature gating. Codegen reads these markers and produces all downstream artifacts (CRDs, OpenAPI spec, field metadata registry).

## Architecture

```
HyperShift CRDs (upstream)
    ↓ [passthrough generator]
HyperFleet CRDs (Go structs with markers)
    ↓ [kube-openapi + marker scanner]
Platform API (OpenAPI spec + field metadata registry)
```

### Three Control Markers

1. **Visibility** (`+k8s:openapi-gen=false`) - Controls whether a field appears in OpenAPI
2. **Write Mode** (`+hyperfleet:write-mode=X`) - Controls customer mutability (mutable/immutable/service-set)
3. **Feature Gate** (`+openshift:enable:FeatureGate=X`) - Controls per-customer entitlements

## Documentation

- **[Design Document](docs/api-management.md)** - Complete architecture and design specification
- **[Workflow Guide](docs/workflow.md)** - HyperShift types to OpenAPI: complete three-stage pipeline
- **[Examples](examples/README.md)** - Teaching examples and realistic CRD templates

## Components

### 1. Passthrough Generator
Reads HyperShift types and generates HyperFleet passthrough structs with safe defaults. Preserves existing marker annotations on regeneration.

### 2. Marker Scanner  
Parses Go source files and generates a field metadata registry mapping each field path to its write mode and feature gate.

### 3. OpenAPI Generator
Integrates kube-openapi to generate OpenAPI schemas respecting visibility markers.

### 4. Feature Gate Tooling
Manages feature gate registry and generates CRD variants per feature set (Default, TechPreview, DevPreview).

### 5. Type Converters
Auto-generates bidirectional conversion functions between CRD and REST types.

### 6. Runtime Validator
Generic validation using field metadata registry to enforce write-mode and feature gate rules.

## Quick Start

### Build the Tools

```bash
make build-tools
```

This builds:
- `bin/marker-scanner` - Extract markers from Go types and generate field registry
- `bin/passthrough-gen` - Generate passthrough types from upstream Go code

### Try the Marker Scanner

Scan the example types and generate a field registry:

```bash
# Simple example - 15 fields
./bin/marker-scanner \
  --input-dirs=./examples/original \
  --output-file=/tmp/field_metadata.go \
  --verbose

# Or realistic CRD example - 33 fields
./bin/marker-scanner \
  --input-dirs=./examples/hypershift \
  --output-file=/tmp/field_metadata.go \
  --verbose
```

Output shows:
- Table of all fields with their markers
- Summary statistics (mutable/immutable/service-set counts)
- Generated registry file location

See [examples/README.md](examples/README.md) for details on the two example sets.

### Try the Passthrough Generator

Quick demo using built-in examples (no HyperShift needed):

```bash
make demo-passthrough
```

This generates passthrough types from `examples/original/` to `/tmp/demo-output`.

**Generate from HyperShift via go.mod (recommended):**

The passthrough generator can resolve HyperShift types directly from the go.mod dependency:

```bash
# Using Makefile (imports github.com/openshift/hypershift/api@v0.1.70)
make generate-passthrough

# Or use the CLI directly with import path
./bin/passthrough-gen \
  --import-path=github.com/openshift/hypershift/api/hypershift/v1beta1 \
  --output-dir=./api/v1alpha1 \
  --types=HostedClusterSpec,NodePoolSpec \
  --package=v1alpha1
```

The `go.mod` currently pins HyperShift to v0.1.70 (baseline version). To bump:

```bash
# Update to a specific version
go get github.com/openshift/hypershift/api@v0.1.71

# Then regenerate
make generate-passthrough
```

**Alternative: Generate from local HyperShift clone:**

```bash
# First, clone HyperShift
export HYPERSHIFT_DIR=/path/to/hypershift
git clone https://github.com/openshift/hypershift $HYPERSHIFT_DIR

# Then generate using Makefile
make generate-passthrough-local HYPERSHIFT_DIR=$HYPERSHIFT_DIR

# Or use the CLI directly with source-dir
./bin/passthrough-gen \
  --source-dir=$HYPERSHIFT_DIR/api/hypershift/v1beta1 \
  --output-dir=./api/v1alpha1 \
  --types=HostedClusterSpec,NodePoolSpec \
  --package=v1alpha1
```

See [docs/workflow.md](docs/workflow.md) for the complete three-stage pipeline.

## Project Status

🚧 **Proof of Concept** - Active development

**Completed:**
- ✅ Marker scanner with field registry generator ([ROSAENG-61389](https://redhat.atlassian.net/browse/ROSAENG-61389))
- ✅ Passthrough type generator ([ROSAENG-61384](https://redhat.atlassian.net/browse/ROSAENG-61384))
- ✅ OpenAPI integration POC ([ROSAENG-61387](https://redhat.atlassian.net/browse/ROSAENG-61387))

**Future Work:**
- Full openapi-gen integration (current POC generates minimal schema)
- Marker preservation from field registry during passthrough regeneration
- Feature gate tooling and CRD variant generation
- Auto-generated type conversion functions

See [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383) for full implementation tracking.

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter and tests
make all
```

**Current test coverage:**
- `pkg/markers`: 63.4%
- `pkg/passthrough`: 78.3%
- `pkg/openapi`: 70.0%
- **Overall**: 73%

## Related Projects

- [HyperShift](https://github.com/openshift/hypershift) - Upstream HostedCluster and NodePool CRDs
- [openshift/api](https://github.com/openshift/api) - Feature gate patterns and tooling reference
- [kube-openapi](https://github.com/kubernetes/kube-openapi) - OpenAPI schema generation

## License

Apache License 2.0
