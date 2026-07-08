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

Go markers on fields control visibility, mutability, and entitlements:

| Marker | Purpose | Values | Description |
|--------|---------|--------|-------------|
| `+k8s:openapi-gen=false` | **Visibility** | `true` (visible, default)<br/>`false` (hidden) | Controls whether field appears in OpenAPI schema |
| `+hyperfleet:write-mode=X` | **Write Mode** | `mutable`<br/>`immutable`<br/>`service-set` | `mutable`: customer can set and change<br/>`immutable`: customer sets on create, cannot change<br/>`service-set`: platform fills it, customer cannot touch |
| `+openshift:enable:FeatureGate=X` | **Feature Gate** | Feature set name | Controls per-customer entitlements (e.g., `TechPreviewNoUpgrade`) |

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

Scan the actual HyperFleet API types and generate a field registry:

```bash
# Scan current API types (214 fields: 18 mutable, 196 service-set, 26 visible)
./bin/marker-scanner \
  --input-dirs=./api/v1alpha1 \
  --output-file=/tmp/field_metadata.go \
  --verbose
```

Output shows:
- Table of all fields with their markers and visibility
- Summary statistics (mutable/immutable/service-set counts)
- Visibility breakdown (visible vs hidden fields)

For teaching examples, see [examples/README.md](examples/README.md) which has simple demonstration types.

### Try the Passthrough Generator

**Test HyperShift integration** (uses go.mod dependency):

```bash
make test-hypershift-integration
```

This generates passthrough types from HyperShift v0.1.70 (resolved via go.mod) to `test-output/`. This demonstrates the complete workflow without requiring a local HyperShift clone.

**Production workflow: Generate and curate HyperShift types**

The passthrough generator creates HyperFleet wrappers from upstream HyperShift types:

```bash
# Generate passthrough types from HyperShift v0.1.70 (via go.mod)
make generate-passthrough

# This creates api/v1alpha1/hostedclusterspec.passthrough.go
# All fields start hidden (+k8s:openapi-gen=false) with safe defaults
```

**Field curation workflow:**

1. Edit `api/v1alpha1/hostedclusterspec.passthrough.go`
2. Remove `+k8s:openapi-gen=false` from fields you want to expose
3. Regenerate field registry and OpenAPI:
   ```bash
   make generate-registry generate-openapi
   ```

Currently exposed fields: `etcd`, `platform`, `controlPlaneRelease`, `kubeAPIServerDNSName`

**Bumping HyperShift version:**

```bash
# Update to a newer version
go get github.com/openshift/hypershift/api@v0.1.71

# Regenerate passthrough types
make generate-passthrough

# Review new/changed fields and update markers as needed
```

See [docs/workflow.md](docs/workflow.md) for the complete three-stage pipeline.

### Browse the API with Swagger UI

View interactive API documentation:

```bash
# Generate the OpenAPI schema
make generate-openapi

# Start Swagger UI server
make serve-swagger-ui

# Open in browser (in another terminal, or visit http://localhost:8080/swagger-ui/)
make open-swagger-ui
```

The Swagger UI provides:
- Interactive browsing of all HyperFleet types
- Schema details with field descriptions
- Model explorer to expand/collapse definitions
- Filter and search capabilities

See [swagger-ui/README.md](swagger-ui/README.md) for more details.

## Project Status

🚧 **Proof of Concept** - Active development

**Completed:**
- ✅ Marker scanner with field registry generator - 214 fields tracked ([ROSAENG-61389](https://redhat.atlassian.net/browse/ROSAENG-61389))
- ✅ Passthrough type generator - go.mod-based with proper imports ([ROSAENG-61384](https://redhat.atlassian.net/browse/ROSAENG-61384))
- ✅ OpenAPI generator with $ref support - proper type expansion ([ROSAENG-61387](https://redhat.atlassian.net/browse/ROSAENG-61387))
- ✅ Swagger UI integration - interactive API documentation
- ✅ Production workflow validated - field curation and marker-based visibility

**What Works:**
- Generate passthrough types from HyperShift v0.1.70
- All fields start hidden (safe defaults)
- Developers curate which fields to expose
- Field metadata registry tracks all 214 fields
- OpenAPI schema properly expands nested types with $ref
- Swagger UI allows interactive browsing

**Future Work:**
- Feature gate tooling and CRD variant generation
- Auto-generated type conversion functions (CRD ↔ REST)
- Runtime validation using field metadata registry

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
- `pkg/passthrough`: 82.8%
- `pkg/openapi`: 70.0%
- **Overall**: 75%

## Related Projects

- [HyperShift](https://github.com/openshift/hypershift) - Upstream HostedCluster and NodePool CRDs
- [openshift/api](https://github.com/openshift/api) - Feature gate patterns and tooling reference
- [kube-openapi](https://github.com/kubernetes/kube-openapi) - OpenAPI schema generation

## License

Apache License 2.0
