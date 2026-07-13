# HyperFleet API Codegen - Developer Guide

**Status**: Complete  
**Date**: 2026-07-13  
**Related**: ROSAENG-61394

This guide provides practical "how-to" documentation for working with the HyperFleet API codegen system. For architecture and design details, see [docs/api-management.md](./api-management.md).

---

## Table of Contents

1. [Overview](#overview)
2. [Getting Started](#getting-started)
3. [Common Workflows](#common-workflows)
4. [HyperShift Version Upgrade](#hypershift-version-upgrade)
5. [Adding/Modifying Fields](#addingmodifying-fields)
6. [Feature Gate Management](#feature-gate-management)
7. [Troubleshooting](#troubleshooting)
8. [Quick Reference](#quick-reference)

---

## Overview

### Three-Layer Architecture

The HyperFleet API codegen system manages three layers of types from a single source of truth:

```
┌─────────────────────────────────────────┐
│  HyperShift CRDs (upstream)             │
│  github.com/openshift/hypershift/api    │
└──────────────┬──────────────────────────┘
               │ Passthrough Generator
               │ (preserves markers)
               ↓
┌─────────────────────────────────────────┐
│  HyperFleet CRDs (Go structs + markers) │
│  api/v1alpha1/*.go                      │
└──────────────┬──────────────────────────┘
               │ kube-openapi + marker-scanner
               │ (generates artifacts)
               ↓
┌─────────────────────────────────────────┐
│  Platform API                           │
│  - OpenAPI spec (openapi/openapi.json)  │
│  - Field registry (pkg/registry/*.go)   │
│  - REST types (pkg/conversion/rest/)    │
└─────────────────────────────────────────┘
```

**Key Principle: Single Source of Truth**

Go markers on HyperFleet CRD types control everything:
- **Visibility** (`+k8s:openapi-gen=false`) - Whether field appears in API
- **Write Mode** (`+hyperfleet:write-mode=X`) - Customer mutability
- **Feature Gates** (`+openshift:enable:FeatureGate=X`) - Per-customer entitlements
- **Gated Write Modes** (`+hyperfleet:validation:FeatureGateAwareWriteMode:...`) - Conditional mutability

All downstream artifacts (CRDs, OpenAPI specs, field metadata) are auto-generated from these markers.

### Marker-Based Control

Instead of hand-writing YAML, configuration, or validation code for each field, we use **Go markers** to declaratively control behavior:

```go
// Hidden from customers, platform manages it
// +k8s:openapi-gen=false
// +hyperfleet:write-mode=service-set
InfraID string `json:"infraID,omitempty"`

// Visible, customer sets on create, cannot change
// +hyperfleet:write-mode=immutable
ClusterID string `json:"clusterID"`

// Visible, customer can change anytime
// +hyperfleet:write-mode=mutable
Replicas *int32 `json:"replicas,omitempty"`

// TechPreview feature, mutable when gate enabled
// +hyperfleet:write-mode=service-set
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="HyperFleetEtcdConfig",writeMode="mutable"
// +openshift:enable:FeatureGate=HyperFleetEtcdConfig
Etcd *EtcdSpec `json:"etcd,omitempty"`
```

**Why markers?**
- ✅ Single source of truth (no YAML to keep in sync)
- ✅ Co-located with type definitions (easy to review)
- ✅ Compiler-verified (invalid markers = compile error)
- ✅ Automatic propagation to all artifacts
- ✅ Preserved across regeneration (marker persistence)

---

## Getting Started

### Prerequisites

- **Go 1.23+**
- **git**
- **make**
- **jq** (for CI scripts)
- **goimports** (optional, for formatting: `go install golang.org/x/tools/cmd/goimports@latest`)

### Build All Tools

```bash
make build-tools
```

This builds:
- `bin/marker-scanner` - Extract markers from Go types → generate field registry
- `bin/passthrough-gen` - Generate passthrough types from HyperShift with marker preservation
- `bin/openapi-gen` - Generate OpenAPI schema from Go types
- `bin/featuregate-info` - Display feature gate registry and field counts
- `bin/conversion-gen` - Generate type conversion functions (CRD ↔ REST)
- `bin/verify-configuration` - Verify configuration type fields have required markers

### Run Tests

```bash
make test
```

All tests must pass before committing to main.

### Run Linter (Optional)

```bash
make lint
```

Note: Currently disabled in CI due to Go version compatibility. Will re-enable when golangci-lint supports Go 1.23+.

### Complete Workflow

```bash
make all  # build-tools + test + lint
```

**CRITICAL: Always run `make all` before committing to main!**

---

## Common Workflows

### 1. View Current Field Registry

See which fields are tracked and their markers:

```bash
./bin/marker-scanner --input-dirs=./api/v1alpha1 --verbose
```

Output shows:
```
Found 163 fields with markers across 6 types

Field Path                                      Type           Write Mode    Feature Gate              Hidden
────────────────────────────────────────────────────────────────────────────────────────────────────────────
spec.clusterID                                  Cluster        immutable                               false
spec.infraID                                    Cluster        service-set                             true
spec.etcd                                       Cluster        service-set   HyperFleetEtcdConfig      false
...
```

Or check the generated JSON registry:

```bash
cat pkg/registry/field_metadata.json | jq '."spec.clusterID"'
```

```json
{
  "type": "Cluster",
  "writeMode": "immutable",
  "featureGate": "",
  "hidden": false,
  "gatedWriteModes": null
}
```

### 2. View Feature Gate Information

```bash
./bin/featuregate-info
```

Output:
```
Feature Gate Registry
═════════════════════

Gate: HyperFleetEtcdConfig
  Stage: GA
  Description: Allows customers to configure etcd settings

Gate: HyperFleetAutoScaling
  Stage: TechPreview
  Description: Enables cluster autoscaling configuration

...

Fields Per Feature Set
══════════════════════

Default (GA only):
  - Total fields: 160 (98.2%)
  - Gated fields: 0
  - Hidden fields: 104 (63.8%)

TechPreviewNoUpgrade (GA + TechPreview):
  - Total fields: 163 (100.0%)
  - Gated fields: 3
  - Hidden fields: 104 (63.8%)
```

### 3. Generate All Artifacts

After adding/modifying markers on types:

```bash
make generate-registry  # Field metadata registry
make generate-openapi   # OpenAPI schema
make generate-conversion # REST type conversions
make manifests          # CRDs (if needed)
```

Or run all at once:

```bash
make generate-all
```

Then verify CI checks:

```bash
make verify
```

---

## HyperShift Version Upgrade

When bumping to a new HyperShift version, the passthrough generator preserves existing marker annotations while adding safe defaults for new fields.

### Step-by-Step Workflow

**1. Update HyperShift dependency**

```bash
# Example: upgrade from v0.1.70 to v0.1.78
go get github.com/openshift/hypershift/api@v0.1.78
go mod tidy
```

**2. Regenerate passthrough types**

```bash
make generate-passthrough
```

This runs:
```bash
./bin/passthrough-gen \
  --source-package=github.com/openshift/hypershift/api/hypershift/v1beta1 \
  --source-type=HostedClusterSpec \
  --target-package=github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1 \
  --target-type=HostedClusterSpecPassthrough \
  --output-file=api/v1alpha1/zz_generated.passthrough.go

# Copy to committed source
cp api/v1alpha1/zz_generated.passthrough.go api/v1alpha1/hostedclusterspec.passthrough.go
rm api/v1alpha1/zz_generated.passthrough.go

# Format
goimports -w api/v1alpha1/hostedclusterspec.passthrough.go
```

**3. Review the diff**

```bash
git diff api/v1alpha1/hostedclusterspec.passthrough.go
```

Look for:
- ✅ **New fields** - Review if they should be exposed to customers
- ✅ **Removed fields** - Verify no customer-facing features break
- ✅ **Type changes** - Check for breaking changes
- ✅ **Marker preservation** - Existing fields keep their markers

**Example diff:**
```diff
+ // New field from HyperShift v0.1.78
+ // Default: hidden + service-set (safe default)
+ // +k8s:openapi-gen=false
+ // +hyperfleet:write-mode=service-set
+ KubeAPIServerDNSName string `json:"kubeAPIServerDNSName,omitempty"`
```

**4. Update markers on new fields**

If a new field should be customer-visible:

```go
// Change from:
// +k8s:openapi-gen=false
// +hyperfleet:write-mode=service-set

// To:
// +hyperfleet:write-mode=mutable  // Customer can set/change
```

**5. Regenerate all artifacts**

```bash
make generate-registry  # Update field metadata
make generate-openapi   # Update OpenAPI spec
make generate-conversion # Update REST conversions
```

**6. Run tests and verify**

```bash
make test    # May reveal API evolution issues
make verify
```

**⚠️ IMPORTANT: HyperShift API Evolution**

HyperShift types evolve between versions. Tests may fail after version bumps due to:
- Field type changes (e.g., `int` → `*int32`, `string` → `*string`)
- Nullability changes (value type → pointer type)
- Validation rule changes
- Struct reshaping

**Test Guidelines:**
- ❌ **Don't test specific pointer/value types** on HyperShift passthrough fields
- ❌ **Don't make brittle assertions** about upstream struct layout
- ✅ **Do test conversion logic** (our code, not HyperShift's types)
- ✅ **Do test field visibility** and write-mode enforcement
- ✅ **Do review test failures** carefully - may indicate legitimate upstream changes

**Example API evolution (v0.1.70 → v0.1.72):**
```go
// v0.1.70
type NodePoolAutoScaling struct {
    Min int `json:"min"`
    Max int `json:"max"`
}

// v0.1.72
type NodePoolAutoScaling struct {
    Min *int32 `json:"min,omitempty"`  // Now pointer + int32
    Max int32  `json:"max"`             // Now int32
}
```

If tests assert `Min == 2`, they'll fail on the pointer dereference after upgrade.

**When tests fail after HyperShift bump:**
1. Read the error - is it a type mismatch on a passthrough field?
2. Check if the field type changed upstream (`go doc github.com/openshift/hypershift/api/...`)
3. Remove brittle assertions on upstream types
4. Keep tests focused on our conversion logic, not HyperShift's structure

**7. Commit the changes**

```bash
git add api/v1alpha1/hostedclusterspec.passthrough.go
git add pkg/registry/field_metadata.go pkg/registry/field_metadata.json
git add openapi/openapi.json
git add pkg/conversion/v1alpha1/*.go
git add go.mod go.sum

git commit -m "Bump HyperShift to v0.1.78

- Added new field KubeAPIServerDNSName (service-set, hidden)
- Preserved existing markers on all fields
- Regenerated field registry and OpenAPI spec

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

### Automated Testing

CI automatically tests HyperShift version bumps:

```bash
# Test bump to latest HyperShift (simulated in CI)
.github/workflows/test-scripts/test-hypershift-bump-latest.sh
```

This verifies:
- ✅ Passthrough generation succeeds
- ✅ Marker preservation works
- ✅ Field registry regenerates correctly
- ✅ All tests pass
- ✅ CI verification passes

### Marker Preservation Details

**How it works:**

1. Generator reads existing field metadata from `pkg/registry/field_metadata.json`
2. For each field in HyperShift types:
   - If field already in registry → use saved markers
   - If field is new → apply safe defaults (hidden + service-set)
3. Result: Only genuinely new fields need manual review

**Safe defaults for new fields:**
```go
// +k8s:openapi-gen=false      // Hidden (not in OpenAPI)
// +hyperfleet:write-mode=service-set  // Platform-controlled
```

This ensures new upstream fields are **hidden until explicitly reviewed and marked for customer visibility**.

---

## Adding/Modifying Fields

### Adding HyperFleet-Native Fields

HyperFleet-native fields are fields that don't exist in HyperShift (envelope fields).

**Example: Add a new envelope field**

1. Edit `api/v1alpha1/cluster_types.go`:

```go
type ClusterSpec struct {
    // Envelope fields (HyperFleet-only)
    
    // DisplayName is the user-friendly name shown in UI
    // +hyperfleet:write-mode=mutable
    DisplayName string `json:"displayName,omitempty"`
    
    // NEW: Add billing account ID
    // +hyperfleet:write-mode=immutable
    BillingAccountID string `json:"billingAccountID,omitempty"`
    
    // Passthrough fields (HyperShift mirror)
    HostedCluster HostedClusterSpecPassthrough `json:",inline"`
}
```

2. Regenerate artifacts:

```bash
make generate-registry
make generate-openapi
make generate-conversion
```

3. Verify the field appears in registry:

```bash
./bin/marker-scanner --input-dirs=./api/v1alpha1 --verbose | grep billingAccountID
```

Expected:
```
spec.billingAccountID    Cluster    immutable    (no gate)    false
```

4. Test and commit:

```bash
make test
make verify
git add api/v1alpha1/cluster_types.go
git add pkg/registry/field_metadata.*
git add openapi/openapi.json
git add pkg/conversion/v1alpha1/*.go
git commit -m "Add billingAccountID field to Cluster

- Immutable (set once on create)
- Customer-visible in OpenAPI
- Validated by runtime validator

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

### Exposing HyperShift Passthrough Fields

HyperShift passthrough fields are already in the passthrough struct but hidden by default.

**Example: Expose a hidden HyperShift field**

1. Find the field in `api/v1alpha1/hostedclusterspec.passthrough.go`:

```go
// Currently hidden
// +k8s:openapi-gen=false
// +hyperfleet:write-mode=service-set
PausedUntil *string `json:"pausedUntil,omitempty"`
```

2. Change markers to make it customer-visible:

```go
// Allow customers to pause cluster upgrades
// +hyperfleet:write-mode=mutable
PausedUntil *string `json:"pausedUntil,omitempty"`
```

3. Regenerate:

```bash
make generate-registry
make generate-openapi
```

4. Verify in OpenAPI:

```bash
cat openapi/openapi.json | jq '.components.schemas.Cluster.properties.spec.properties.pausedUntil'
```

Should show the field definition (not null).

5. Test and commit.

### Choosing Appropriate Markers

**Visibility:**
- Remove `+k8s:openapi-gen=false` → field appears in OpenAPI
- Add `+k8s:openapi-gen=false` → field hidden from customers

**Write Mode:**

| Write Mode | When to Use | Example |
|------------|-------------|---------|
| `mutable` | Customer can set and change anytime | `replicas`, `labels` |
| `immutable` | Customer sets once on create, cannot change | `clusterID`, `region` |
| `service-set` | Platform fills it in, customer cannot touch | `infraID`, `accountID` |

**Feature Gates:**

Add when field should only be available to specific customer tiers:

```go
// TechPreview feature
// +hyperfleet:write-mode=mutable
// +openshift:enable:FeatureGate=HyperFleetAutoScaling
AutoScaling *AutoScalingSpec `json:"autoScaling,omitempty"`
```

**Gated Write Modes:**

Use when write-mode should vary based on feature gates:

```go
// Base write-mode: immutable (for standard customers)
// Premium customers can change it: mutable
// +hyperfleet:write-mode=immutable
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="",writeMode="immutable"
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="PremiumFeature",writeMode="mutable"
ReleaseChannel string `json:"releaseChannel"`
```

---

## Feature Gate Management

### Adding a New Feature Gate

**1. Define the gate in registry**

Edit `pkg/featuregate/registry.go`:

```go
var HyperFleetFeatureGates = map[string]FeatureGateInfo{
    // Existing gates...
    
    "HyperFleetCustomNetworking": {
        Stage:       TechPreview,
        Description: "Enables custom VPC networking configuration",
    },
}
```

**2. Add marker to fields controlled by this gate**

Edit `api/v1alpha1/hostedclusterspec.passthrough.go`:

```go
// TechPreview: Custom networking
// +hyperfleet:write-mode=mutable
// +openshift:enable:FeatureGate=HyperFleetCustomNetworking
Networking *NetworkingSpec `json:"networking,omitempty"`
```

**3. Regenerate**

```bash
make generate-registry
make generate-openapi
```

**4. Verify gate info**

```bash
./bin/featuregate-info
```

Expected output includes:
```
Gate: HyperFleetCustomNetworking
  Stage: TechPreview
  Description: Enables custom VPC networking configuration
```

**5. Test with different feature sets**

```bash
# Default feature set (GA only) - should NOT include networking
cat pkg/registry/field_metadata.json | jq '.[] | select(.featureGate == "HyperFleetCustomNetworking")'

# TechPreviewNoUpgrade - should include networking
# (runtime validator enforces this based on customer's feature set)
```

### Promoting a Feature Gate (TechPreview → GA)

**1. Update stage in registry**

Edit `pkg/featuregate/registry.go`:

```go
"HyperFleetEtcdConfig": {
    Stage:       GA,  // Changed from TechPreview
    Description: "Allows customers to configure etcd settings",
},
```

**2. (Optional) Remove gate marker from fields**

Since the gate is now GA, all customers have access. You can optionally remove the marker:

```go
// Before (TechPreview):
// +hyperfleet:write-mode=mutable
// +openshift:enable:FeatureGate=HyperFleetEtcdConfig
Etcd *EtcdSpec `json:"etcd,omitempty"`

// After (GA - gate marker optional):
// +hyperfleet:write-mode=mutable
Etcd *EtcdSpec `json:"etcd,omitempty"`
```

**3. Regenerate**

```bash
make generate-registry
make featuregate-info  # Verify promotion
```

**4. Verify field counts**

```bash
./bin/featuregate-info
```

Expected:
```
Default (GA only):
  - Total fields: 163 (was 160)  ← Etcd fields now included
```

**5. Update documentation and commit**

```bash
git add pkg/featuregate/registry.go
git add api/v1alpha1/hostedclusterspec.passthrough.go
git add pkg/registry/field_metadata.*
git commit -m "Promote HyperFleetEtcdConfig to GA

- All customers now have access to etcd configuration
- Updated field registry
- Default feature set now includes 163 fields (was 160)

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

### Feature Set Hierarchy

```
DevPreviewNoUpgrade ⊃ TechPreviewNoUpgrade ⊃ Default
```

- **Default** - Only GA features (most restrictive)
- **TechPreviewNoUpgrade** - GA + TechPreview features
- **DevPreviewNoUpgrade** - GA + TechPreview + DevPreview (least restrictive)

**Important:** Customers on TechPreview/DevPreview feature sets **cannot upgrade** their clusters (data loss risk with unstable features).

---

## Troubleshooting

### CI Marker Validation Failures

**Error:**
```
Error: pkg/registry/field_metadata.go is out of date
Run: make generate-registry
```

**Cause:** Field metadata registry doesn't match current Go types.

**Fix:**
```bash
make generate-registry
git add pkg/registry/field_metadata.go pkg/registry/field_metadata.json
git commit -m "Update field registry"
```

---

**Error:**
```
Error: Field spec.networking is visible but has no write-mode marker
```

**Cause:** Field is in OpenAPI (visible) but missing `+hyperfleet:write-mode` marker.

**Fix:**
```go
// Add write-mode marker
// +hyperfleet:write-mode=mutable
Networking *NetworkingSpec `json:"networking,omitempty"`
```

Then regenerate:
```bash
make generate-registry
```

---

### OpenAPI Generation Issues

**Error:**
```
Error: openapi/openapi.json is out of date
Run: make generate-openapi
```

**Cause:** OpenAPI schema doesn't match current Go types.

**Fix:**
```bash
make generate-openapi
git add openapi/openapi.json
git commit -m "Update OpenAPI schema"
```

---

**Error:**
```
panic: unknown type: ClusterConfiguration
```

**Cause:** Type not imported or qualified correctly.

**Fix:** Check imports in `api/v1alpha1/` files. Mirror types should be qualified:

```go
// Correct:
Configuration *hypershiftv1beta1.ClusterConfiguration `json:"configuration,omitempty"`

// Wrong:
Configuration *ClusterConfiguration `json:"configuration,omitempty"`
```

---

### Runtime Validation Errors

**Error:**
```
validation failed:
  field spec.infraID: field is platform-managed (service-set) and cannot be set by customers
```

**Cause:** Customer tried to set a `service-set` field.

**Meaning:** Working as intended! The validator is blocking unauthorized field access.

**Customer action:** Remove the field from the request.

---

**Error:**
```
validation failed:
  field spec.etcd: feature gate HyperFleetEtcdConfig not enabled for this account
```

**Cause:** Field is gated by a feature gate that customer doesn't have.

**Meaning:** Working as intended! Customer needs the TechPreview feature set.

**Platform action:** Enable TechPreview feature set for customer OR grant `HyperFleetEtcdConfig` entitlement.

---

### Compilation Errors

**Error:**
```
api/v1alpha1/hostedclusterspec.passthrough.go:42:2: undefined: ServiceAccountSigningKey
```

**Cause:** Field references a type that isn't imported.

**Fix:** Add the import:

```go
import (
    hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
    corev1 "k8s.io/api/core/v1"  // Add this
)
```

Or qualify the type:

```go
ServiceAccountSigningKey corev1.LocalObjectReference `json:"serviceAccountSigningKey,omitempty"`
```

---

**Error:**
```
make: *** [build-tools] Error 1
cmd/marker-scanner/main.go:15:2: undefined: registry
```

**Cause:** Generator tool missing imports or dependencies.

**Fix:**
```bash
go mod tidy
make build-tools
```

---

### Test Failures

**Error:**
```
--- FAIL: TestValidator_ValidateImmutableFields (0.00s)
    validator_test.go:123: Expected error but got nil
```

**Cause:** Test expectations don't match current field metadata.

**Fix:** Check if field markers changed:

```bash
./bin/marker-scanner --input-dirs=./api/v1alpha1 --verbose | grep <field-name>
```

Update test expectations to match current markers.

---

### HyperShift Bump Issues

**Error:**
```
Error: passthrough generation failed: type HostedClusterSpec not found
```

**Cause:** HyperShift API moved or renamed the type.

**Fix:** Check HyperShift release notes and update source type:

```bash
# Check what types are available
go doc github.com/openshift/hypershift/api/hypershift/v1beta1

# Update Makefile if type was renamed
```

---

**Error:**
```
diff: api/v1alpha1/hostedclusterspec.passthrough.go: No such file or directory
```

**Cause:** Passthrough file wasn't generated.

**Fix:**
```bash
# Manually run passthrough generator
make generate-passthrough

# Or debug:
./bin/passthrough-gen \
  --source-package=github.com/openshift/hypershift/api/hypershift/v1beta1 \
  --source-type=HostedClusterSpec \
  --target-package=github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1 \
  --target-type=HostedClusterSpecPassthrough \
  --output-file=api/v1alpha1/zz_generated.passthrough.go \
  --verbose
```

---

## Quick Reference

### Make Targets

| Target | Purpose |
|--------|---------|
| `make build-tools` | Build all codegen tools |
| `make test` | Run all tests |
| `make lint` | Run linters (disabled in CI) |
| `make all` | build-tools + test + lint |
| `make generate-passthrough` | Generate passthrough types from HyperShift |
| `make generate-registry` | Generate field metadata registry |
| `make generate-openapi` | Generate OpenAPI schema |
| `make generate-conversion` | Generate REST type conversions |
| `make generate-all` | Run all generators |
| `make verify` | Run CI verification checks |
| `make manifests` | Generate CRDs |
| `make featuregate-info` | Display feature gate registry |
| `make clean` | Remove generated files |

### Marker Syntax

**Visibility:**
```go
// Hidden from customers
// +k8s:openapi-gen=false

// Visible (default - can omit marker)
// No marker needed
```

**Write Mode:**
```go
// Customer can set and change
// +hyperfleet:write-mode=mutable

// Customer sets on create, cannot change
// +hyperfleet:write-mode=immutable

// Platform fills it in, customer cannot touch
// +hyperfleet:write-mode=service-set
```

**Feature Gate:**
```go
// TechPreview feature
// +openshift:enable:FeatureGate=HyperFleetAutoScaling

// DevPreview feature
// +openshift:enable:FeatureGate=HyperFleetCustomDNS
```

**Gated Write Modes:**
```go
// Default write-mode (for customers without gate)
// +hyperfleet:write-mode=immutable
// Specific gate override
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="PremiumFeature",writeMode="mutable"
// Default override (empty gate)
// +hyperfleat:validation:FeatureGateAwareWriteMode:featureGate="",writeMode="immutable"
```

**Priority:** Specific gate → default ("") → base WriteMode

### Generated Files

| File | Purpose | Auto-Generated |
|------|---------|----------------|
| `api/v1alpha1/hostedclusterspec.passthrough.go` | Passthrough types | ✅ Yes |
| `api/v1alpha1/zz_generated.deepcopy.go` | DeepCopy methods | ✅ Yes (controller-gen) |
| `pkg/registry/field_metadata.go` | Field registry (Go) | ✅ Yes |
| `pkg/registry/field_metadata.json` | Field registry (JSON) | ✅ Yes |
| `openapi/openapi.json` | OpenAPI schema | ✅ Yes |
| `pkg/conversion/v1alpha1/*.go` | REST type conversions | ✅ Yes |
| `pkg/conversion/rest/*.go` | REST type definitions | ✅ Yes |
| `pkg/conversion/types.go` | Service-set field list | ✅ Yes |
| `config/crd/bases/*.yaml` | CRD YAML | ✅ Yes (controller-gen) |

**Never hand-edit generated files!** Always regenerate using `make` targets.

### File Naming Conventions

- `zz_generated.*` - Temporary generated files (gitignored, removed after copy)
- `*.passthrough.go` - Committed passthrough types (source of truth)
- `*_test.go` - Test files
- `*.md` - Documentation

### Directory Structure

```
.
├── api/v1alpha1/          # HyperFleet CRD types (source of truth)
│   ├── cluster_types.go   # Cluster CRD with envelope fields
│   ├── nodepool_types.go  # NodePool CRD with envelope fields
│   ├── hostedclusterspec.passthrough.go  # Generated passthrough
│   └── configuration.go   # Hand-written mirror types
├── cmd/                   # Codegen tools
│   ├── marker-scanner/    # Extract markers → field registry
│   ├── passthrough-gen/   # Generate passthrough types
│   ├── openapi-gen/       # Generate OpenAPI schema
│   ├── conversion-gen/    # Generate type conversions
│   ├── featuregate-info/  # Display feature gate info
│   └── verify-configuration/ # CI verification
├── pkg/
│   ├── registry/          # Generated field metadata
│   ├── featuregate/       # Feature gate registry
│   ├── conversion/        # Type converters
│   │   ├── v1alpha1/      # CRD → REST conversions
│   │   ├── rest/          # REST type definitions
│   │   └── hypershift/    # HyperFleet ↔ HyperShift (Boundary 1)
│   └── validation/        # Runtime validator
├── openapi/               # Generated OpenAPI schema
├── docs/                  # Documentation
└── config/crd/bases/      # Generated CRDs
```

### Common Commands

**View field registry:**
```bash
./bin/marker-scanner --input-dirs=./api/v1alpha1 --verbose
```

**View feature gates:**
```bash
./bin/featuregate-info
```

**Check OpenAPI schema:**
```bash
cat openapi/openapi.json | jq '.components.schemas.Cluster'
```

**Verify specific field:**
```bash
cat pkg/registry/field_metadata.json | jq '."spec.clusterID"'
```

**Run single test:**
```bash
go test ./pkg/validation -run TestValidator_ValidateImmutableFields -v
```

**Check git diff before commit:**
```bash
git diff --stat
git diff api/v1alpha1/hostedclusterspec.passthrough.go | head -50
```

---

## Best Practices

### Before Committing

**Always run:**
```bash
make all  # Ensure everything compiles and tests pass
make verify  # CI checks
```

**Never commit:**
- ❌ Code that doesn't compile
- ❌ Failing tests
- ❌ Out-of-date generated files
- ❌ `zz_generated.*` files (should be gitignored)
- ❌ Uncommitted marker changes without regeneration

**Always commit:**
- ✅ Source Go files with marker changes
- ✅ Updated field registry (`pkg/registry/*`)
- ✅ Updated OpenAPI schema (`openapi/openapi.json`)
- ✅ Updated conversion code (`pkg/conversion/v1alpha1/*`)
- ✅ Descriptive commit message with `Co-Authored-By: Claude`

### Marker Guidelines

**Visibility:**
- Default to hidden (`+k8s:openapi-gen=false`) for new passthrough fields
- Only expose fields after security/product review
- Hidden fields can still be used internally (CRD → HyperShift conversions)

**Write Mode:**
- Default to `service-set` for new passthrough fields
- Use `immutable` for identifiers (clusterID, region)
- Use `mutable` for customer-controlled settings (replicas, labels)
- Use gated write-modes for tier-based access control

**Feature Gates:**
- Use gates for experimental/preview features
- Document gate purpose in description
- Test with different feature sets
- Promote gates (TechPreview → GA) when stable

### Documentation

**Always update:**
- ✅ Design docs when architecture changes
- ✅ README when adding new tools
- ✅ This guide when workflows change
- ✅ Commit messages with context

**Document:**
- Why fields have specific markers
- Why gates exist
- Security/product decisions
- Breaking changes

---

## Getting Help

**Documentation:**
- [Design Document](./api-management.md) - Architecture and design
- [Workflow Guide](./workflow.md) - Three-stage pipeline
- [Feature Gates](./feature-gates.md) - Feature gate design
- [Boundary 1 Conversions](./boundary1-conversions.md) - HyperFleet ↔ HyperShift

**Code:**
- `examples/README.md` - Teaching examples
- `pkg/*/README.md` - Package-specific docs
- Test files (`*_test.go`) - Working examples

**Issues:**
- GitHub Issues: https://github.com/cdoan1/hyperfleet-api-codegen/issues
- Jira Epic: ROSAENG-61383

---

## Summary

**Remember:**
1. ✅ Go markers are the single source of truth
2. ✅ Always run `make all` before committing
3. ✅ Passthrough generator preserves markers on regeneration
4. ✅ New fields default to hidden + service-set (safe)
5. ✅ CI verifies everything stays in sync
6. ✅ Never hand-edit generated files

**Happy coding! 🚀**
