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
Reads HyperShift types and generates HyperFleet passthrough structs with safe defaults. **Preserves existing marker annotations on regeneration** using the field metadata registry.

**Marker Preservation:**
When regenerating passthrough types (e.g., after bumping HyperShift version):
1. Generator loads existing field metadata from `pkg/registry/field_metadata.json`
2. Fields already in registry use their saved markers (write-mode, visibility, feature gates)
3. New upstream fields get safe defaults: hidden (`+k8s:openapi-gen=false`) + service-set (`+hyperfleet:write-mode=service-set`)
4. Result: Reviewed field markers persist across regenerations, only genuinely new fields need curation

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
- `bin/marker-scanner` - Extract markers from Go types and generate field registry (JSON + Go)
- `bin/passthrough-gen` - Generate passthrough types from upstream Go code with marker preservation
- `bin/openapi-gen` - Generate OpenAPI schema from Go types
- `bin/featuregate-info` - Display feature gate registry and field counts per feature set

### Try the Marker Scanner

Scan the actual HyperFleet API types and generate a field registry:

```bash
# Scan current API types (58 fields tracked)
./bin/marker-scanner \
  --input-dirs=./api/v1alpha1 \
  --output-file=/tmp/field_metadata.go \
  --verbose
```

Output shows:
- Table of all fields with their markers and visibility
- Summary statistics (mutable/immutable/service-set counts)
- Visibility breakdown (visible vs hidden fields)
- Generates both `.go` file (for Go consumers) and `.json` file (for tooling)

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

Currently exposed fields: 11 visible in Default feature set (see `make featuregate-info` for breakdown)

**Bumping HyperShift version:**

```bash
# Update to a newer version
go get github.com/openshift/hypershift/api@v0.1.71

# Regenerate passthrough types (preserves existing markers from registry)
make generate-passthrough

# Review diff - only NEW upstream fields will appear with safe defaults
git diff api/v1alpha1/hostedclusterspec.passthrough.go

# Curate new fields: remove +k8s:openapi-gen=false to expose them
# Update markers as needed, then regenerate registry
make generate-registry generate-openapi
```

**How marker preservation works:**
1. `make generate-registry` creates both `field_metadata.go` (for Go consumers) and `field_metadata.json` (for tools)
2. `make generate-passthrough` loads `field_metadata.json` and applies saved markers to existing fields
3. New fields from upstream HyperShift that aren't in the registry get safe defaults (hidden + service-set)
4. Your reviewed marker choices persist automatically - no manual re-annotation needed

See [docs/workflow.md](docs/workflow.md) for the complete three-stage pipeline.

### Feature Gates and Field Entitlements

View feature gate registry and field counts per feature set:

```bash
make featuregate-info
```

Output shows:
- All registered feature gates with their stages (GA, TechPreview, DevPreview)
- Field counts per feature set (Default: 11 fields, TechPreview: 12 fields, DevPreview: 12 fields)
- Which gates are enabled in each feature set

**Feature set hierarchy:**
- **Default** - GA features only (production-ready) - 11 visible fields
- **TechPreviewNoUpgrade** - GA + TechPreview features - 12 visible fields
- **DevPreviewNoUpgrade** - All features - 12 visible fields

Example gates in current registry:
- `HyperFleetEtcdConfig` (GA) - enabled in all feature sets
- `HyperFleetAutoScaling` (TechPreview) - enabled in TechPreview and DevPreview only
- `HyperFleetCustomDNS` (DevPreview) - enabled in DevPreview only

### CRD Variants by Feature Set

Generate feature-set-specific CRD variants that only include fields available in each tier:

```bash
# Generate all variants (Default, TechPreview, DevPreview)
make generate-crd-variants

# Or generate a specific variant
bin/crd-variants \
  --input=config/crd/bases/_clusters.yaml \
  --base-name=cluster \
  --feature-set=default \
  --output-dir=config/crd/variants
```

This produces:
- `cluster_default.yaml` - Only GA features (11 visible fields)
- `cluster_techpreview.yaml` - GA + TechPreview features (12 visible fields)
- `cluster_devpreview.yaml` - All features (12 visible fields)

Each variant is a complete, valid CRD that can be applied to a cluster. Use the variant matching your customer's feature set entitlement.

### Runtime Validation

Validate API requests using the field metadata registry:

```go
import (
    "github.com/cdoan1/hyperfleet-api-codegen/pkg/validation"
    "github.com/cdoan1/hyperfleet-api-codegen/pkg/featuregate"
)

// Create validator
v := validation.NewValidator()

// Validate a create request
req := &validation.Request{
    Operation: validation.OperationCreate,
    Fields: map[string]any{
        "spec.displayName": "my-cluster",
        "spec.tags":        map[string]string{"env": "prod"}, // Gated field
    },
    FeatureSet: featuregate.TechPreviewNoUpgrade,
}

if err := v.Validate(req); err != nil {
    // Returns errors for:
    // - Service-set fields (customer cannot set)
    // - Feature-gated fields (customer lacks entitlement)
    // - Immutable fields being changed on update
    log.Fatalf("Validation failed: %v", err)
}
```

The validator enforces:
- **Write mode rules**: service-set fields rejected, immutable fields can't change on update
- **Feature gate entitlements**: gated fields only accessible to customers with correct feature set
- **Generic validation**: No field-specific code, scales to hundreds of fields

See [pkg/validation/example_test.go](pkg/validation/example_test.go) for more examples.

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
- ✅ Marker scanner with field registry generator - 58 fields tracked ([ROSAENG-61389](https://redhat.atlassian.net/browse/ROSAENG-61389))
- ✅ Passthrough type generator - go.mod-based with proper imports ([ROSAENG-61384](https://redhat.atlassian.net/browse/ROSAENG-61384))
- ✅ Marker preservation - registry-based marker persistence across regenerations
- ✅ OpenAPI generator with $ref support - proper type expansion ([ROSAENG-61387](https://redhat.atlassian.net/browse/ROSAENG-61387))
- ✅ Swagger UI integration - interactive API documentation
- ✅ Production workflow validated - field curation and marker-based visibility
- ✅ Feature gate tooling - registry, filtering, and per-feature-set field counts
- ✅ Runtime validation - generic enforcement of write-mode and feature gate rules
- ✅ CRD variant generator - produces feature-set-specific CRD YAML

**What Works:**
- Generate passthrough types from HyperShift v0.1.70
- All fields start hidden (safe defaults)
- Marker preservation across regenerations via JSON registry
- Developers curate which fields to expose
- Field metadata registry tracks all 58 fields (4 feature-gated)
- Feature gate registry with 4 gates (1 GA, 2 TechPreview, 1 DevPreview)
- Per-feature-set field filtering (Default: 11 visible, TechPreview: 12 visible, DevPreview: 12 visible)
- OpenAPI schema properly expands nested types with $ref
- Swagger UI allows interactive browsing
- Runtime validation enforces write-mode (mutable/immutable/service-set) and feature gates
- CRD variants per feature set (Default/TechPreview/DevPreview)

**Future Work:**
- Auto-generated type conversion functions (CRD ↔ REST)

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
