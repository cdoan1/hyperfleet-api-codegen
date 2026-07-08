# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a POC repository for marker-based code generation tools that manage three layers of types with a single source of truth:

- **HyperShift CRDs** (upstream): Full HostedCluster and NodePool from HyperShift
- **HyperFleet CRDs**: Wrapper types (Cluster/NodePool) and native resources with Go markers
- **Platform API**: REST API with OpenAPI spec generated from HyperFleet CRDs

The core innovation is using Go markers (`+k8s:openapi-gen`, `+hyperfleet:write-mode`, `+openshift:enable:FeatureGate`) to declaratively control visibility, mutability, and feature gating, with codegen producing all downstream artifacts (CRDs, OpenAPI specs, field metadata registry).

## Architecture

Two critical boundaries:

**Boundary 1: HyperShift CRD → HyperFleet CRD**
- Passthrough generator reads upstream HyperShift types and generates mirrored Go structs
- Preserves existing marker annotations via field registry on regeneration
- New upstream fields get safe defaults: `+k8s:openapi-gen=false` (hidden) and `+hyperfleet:write-mode=service-set` (platform-controlled)

**Boundary 2: HyperFleet CRD → Platform API**
- kube-openapi generates OpenAPI schema respecting visibility markers
- Marker scanner extracts write-mode and feature-gate metadata into field registry
- Runtime validation enforces markers generically with no field-specific code

See `docs/api-management.md` for complete design specification.

## Three Control Markers

1. **Visibility** (`+k8s:openapi-gen=false`) - Controls whether a field appears in OpenAPI schema (built-in kube-openapi feature)
2. **Write Mode** (`+hyperfleet:write-mode=X`) - Controls customer mutability:
   - `mutable`: Customer can set and change
   - `immutable`: Customer sets on create, cannot change
   - `service-set`: Platform fills it in, customer cannot touch
3. **Feature Gate** (`+openshift:enable:FeatureGate=X`) - Controls per-customer field entitlements via feature sets (Default, TechPreviewNoUpgrade, DevPreviewNoUpgrade)

## Implementation Status

🚧 **Proof of Concept** - Active development tracked in [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)

**Completed:**
1. ✅ Go module setup
2. ✅ Marker scanner and field registry generator (ROSAENG-61389) - 58 fields tracked with write-mode and feature gates
3. ✅ Passthrough generator (ROSAENG-61384) - go.mod-based with proper type qualification
4. ✅ OpenAPI integration (ROSAENG-61387) - full generator with $ref support for type expansion
5. ✅ Feature gate tooling - registry, filtering, and per-feature-set field counts
6. ✅ Swagger UI - interactive API documentation
7. ✅ Runtime validation - generic enforcement using field metadata registry
8. ✅ CRD variant generator - produces feature-set-specific CRD YAML

**What Works:**
- Three control markers: visibility, write-mode, feature gates
- Field metadata registry with 58 fields (3 feature-gated)
- Feature gate registry with 4 example gates (1 GA, 2 TechPreview, 1 DevPreview)
- Per-feature-set filtering: Default (32 fields), TechPreview (35 fields), DevPreview (35 fields)
- OpenAPI schema generation with proper $ref expansion
- Production workflow validated: field curation, marker-based visibility
- Runtime validation enforces write-mode and feature gate rules with no field-specific code
- CRD variant generation filters YAML by feature set

**Remaining:**
- Type conversion functions (CRD ↔ REST)

## Key Concepts

### Marker Preservation
When HyperShift upstream bumps, the passthrough generator reruns. The field registry acts as "memory" of which fields have been reviewed and marked appropriately, preventing loss of developer annotations.

### Envelope vs Passthrough Fields
Wrapper CRDs (Cluster, NodePool) separate:
- **Envelope fields**: HyperFleet-only (deleteProtection, expirationTimestamp, properties, accountId, etc.)
- **Passthrough struct**: Generated mirror of all upstream HyperShift fields

### Feature Gate Hierarchy
DevPreview ⊃ TechPreview ⊃ Default. Promoting a gate is a one-line change in the feature gate registry followed by regeneration.

### Type Conversions
- **HyperFleet ↔ HyperShift**: ToHyperShiftHostedCluster, FromHyperShiftHostedCluster, ToHyperShiftNodePool, FromHyperShiftNodePool
- **CRD ↔ REST**: ProjectCluster (CRD → REST), UnprojectCluster (REST → CRD with service-set enrichment) - auto-generated, no hand-written code

## Code Organization

Expected structure (not yet implemented):
- `api/v1alpha1/` - HyperFleet CRD types with markers
- `pkg/passthrough/` - Generator that reads HyperShift types and produces passthrough structs
- `pkg/markers/` - Scanner that extracts markers into field registry
- `pkg/openapi/` - kube-openapi integration
- `pkg/featuregate/` - Feature gate registry and CRD variant generation
- `pkg/conversion/` - Auto-generated conversion functions
- `pkg/validation/` - Runtime validator using field registry

## References

- **Design Document**: `docs/api-management.md` - driving specification
- **Context Document**: `CONTEXT.md` - session notes, Jira story breakdown, design decisions
- **Upstream Projects**:
  - [HyperShift](https://github.com/openshift/hypershift) - Source of HostedCluster and NodePool types
  - [openshift/api](https://github.com/openshift/api) - Feature gate patterns and codegen tooling reference
  - [kube-openapi](https://github.com/kubernetes/kube-openapi) - OpenAPI schema generation with marker support

## Development Workflow

**Before every commit, run:**
```bash
make all
```
This ensures linting and tests pass. CI will reject commits that don't pass `make all`.

**HyperShift version bump:**

The passthrough generator resolves HyperShift types via go.mod (no local clone needed):

1. Update HyperShift version: `go get github.com/openshift/hypershift/api@v0.1.71`
2. Run `make generate-passthrough` to regenerate passthrough types
3. Review diff for new/removed fields
4. Add appropriate markers to new fields (default is hidden + service-set)
5. Run `make manifests openapi` to regenerate CRDs and OpenAPI spec
6. CI verifies all passthrough fields have required markers

Current baseline: HyperShift v0.1.70

**Feature gate promotion:**
1. Update gate stage in `pkg/featuregate/registry.go` (e.g., TechPreview → GA)
2. Run `make featuregate-info` to verify change
3. Optionally remove gate markers from fields (now always enabled)
4. Run `make generate-registry` to update field metadata

**Adding new markers to existing fields:**
1. Edit Go type definitions in `api/v1alpha1/`
2. Run `make manifests openapi generate-registry` to regenerate all artifacts
3. CI verifies marker correctness

## Critical Patterns

**Safe Defaults**: New upstream fields default to hidden (`+k8s:openapi-gen=false`) and platform-controlled (`+hyperfleet:write-mode=service-set`) until explicitly reviewed.

**Single Source of Truth**: Go types with markers drive everything. Never hand-edit generated CRD YAML, OpenAPI schemas, or field registry.

**Generic Validation**: The Platform API validates all fields using the generated field registry with no field-specific code. This scales to hundreds of fields without maintenance burden.

**Field Registry as Memory**: The registry is both an output (for Platform API runtime validation) and an input (for preserving markers during passthrough regeneration).
