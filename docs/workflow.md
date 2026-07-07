# Processing HyperShift Types: Complete Workflow

## Overview

HyperShift types go through a **three-stage pipeline** to become OpenAPI schemas with runtime validation metadata.

```
HyperShift CRDs → HyperFleet CRDs → OpenAPI + Field Registry
   (upstream)      (our wrapper)      (Platform API)
```

## Stage 1: Generate Passthrough Types

**Tool**: `passthrough-gen`

**Input**: HyperShift types from cloned repo
```bash
/Users/cdoan/workspace/src/github.com/openshift/hypershift/api/hypershift/v1beta1/
├── hostedcluster_types.go
└── nodepool_types.go
```

**Output**: HyperFleet passthrough types with markers
```bash
./api/v1alpha1/zz_generated.passthrough.go
```

**Command**:
```bash
./bin/passthrough-gen \
  -source-dir=/Users/cdoan/workspace/src/github.com/openshift/hypershift \
  -types=HostedClusterSpec,NodePoolSpec \
  -output-dir=./api/v1alpha1 \
  -package=v1alpha1
```

**What it does**:
- Reads HyperShift Go types using go/parser
- Mirrors all fields into HyperFleet structs
- Adds safe default markers to ALL fields:
  - `+k8s:openapi-gen=false` (hidden from OpenAPI)
  - `+hyperfleet:write-mode=service-set` (platform-controlled)
- Generates `zz_generated.passthrough.go`

**Example output**:
```go
// HostedClusterSpecPassthrough mirrors HostedClusterSpec from upstream HyperShift
type HostedClusterSpecPassthrough struct {
    // release specifies the desired OCP release payload
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    Release Release `json:"release"`
    
    // platform specifies the underlying infrastructure
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    Platform PlatformSpec `json:"platform"`
    
    // ... 32 more fields, all with safe defaults
}
```

## Stage 2: Generate Field Registry

**Tool**: `marker-scanner`

**Input**: Generated HyperFleet types (after manual marker review/updates)
```bash
./api/v1alpha1/
├── cluster_types.go              # HyperFleet envelope types
├── nodepool_types.go             # HyperFleet envelope types
└── zz_generated.passthrough.go   # From Stage 1
```

**Output**: Field metadata registry
```bash
./pkg/registry/field_metadata.go
```

**Command**:
```bash
./bin/marker-scanner \
  -input-dirs=./api/v1alpha1 \
  -output-file=./pkg/registry/field_metadata.go \
  -verbose
```

**What it does**:
- Scans all Go types in `api/v1alpha1/`
- Extracts markers: `+k8s:openapi-gen`, `+hyperfleet:write-mode`, `+openshift:enable:FeatureGate`
- Generates registry map for runtime validation

**Example output**:
```go
var FieldRegistry = map[string]FieldMeta{
    "spec.name":                  {WriteMode: Immutable, Hidden: false},
    "spec.region":                {WriteMode: ServiceSet, Hidden: false},
    "spec.accountId":             {WriteMode: ServiceSet, Hidden: true},
    "spec.hostedCluster.release": {WriteMode: Immutable, Hidden: false},
    "spec.hostedCluster.etcd":    {WriteMode: Immutable, Hidden: false, Gate: "HyperFleetEtcdConfig"},
    // ... all fields from all types
}
```

## Stage 3: Generate OpenAPI Schema

**Tool**: `openapi-gen`

**Input**: HyperFleet types (same as Stage 2)
```bash
./api/v1alpha1/
├── cluster_types.go
├── nodepool_types.go
└── zz_generated.passthrough.go
```

**Output**: OpenAPI schema
```bash
./openapi/openapi.json
```

**Command** (future production):
```bash
./bin/openapi-gen \
  -input-dirs=./api/v1alpha1 \
  -output-file=./openapi/openapi.json \
  -title="HyperFleet API" \
  -version=v1alpha1
```

**What it does** (when fully implemented):
- Scans Go types using kube-openapi
- Respects `+k8s:openapi-gen=false` → excludes hidden fields
- Generates OpenAPI 3.0 schema with all visible fields
- Adds custom extensions for write-mode and feature gates

**Example output**:
```json
{
  "definitions": {
    "Cluster": {
      "properties": {
        "name": { "type": "string" },           // visible
        "region": { "type": "string" },         // visible
        "hostedCluster": { "$ref": "#/..." }    // visible
        // accountId EXCLUDED (hidden)
      }
    }
  }
}
```

## Complete Sequence

### Initial Setup (One Time)
```bash
# 1. Clone HyperShift
git clone https://github.com/openshift/hypershift /path/to/hypershift

# 2. Generate initial passthrough types
./bin/passthrough-gen \
  -source-dir=/path/to/hypershift \
  -types=HostedClusterSpec,NodePoolSpec \
  -output-dir=./api/v1alpha1 \
  -package=v1alpha1

# 3. MANUAL: Review generated zz_generated.passthrough.go
# Update markers on fields you want to expose:
#   - Remove +k8s:openapi-gen=false to make visible
#   - Change +hyperfleet:write-mode to mutable/immutable as needed
#   - Add +openshift:enable:FeatureGate for gated features

# 4. Generate registry and OpenAPI
make generate  # runs marker-scanner + openapi-gen
```

### HyperShift Version Bump (Ongoing)
```bash
# 1. Update HyperShift dependency
go get github.com/openshift/hypershift@v0.2.0
go mod tidy

# 2. Regenerate passthrough types
./bin/passthrough-gen \
  -source-dir=/path/to/hypershift \
  -types=HostedClusterSpec,NodePoolSpec \
  -output-dir=./api/v1alpha1 \
  -package=v1alpha1

# 3. Review diff
git diff api/v1alpha1/zz_generated.passthrough.go
# Look for:
#   - New fields (will have safe defaults)
#   - Removed fields (deleted from output)
#   - Changed types (may need manual fixes)

# 4. Update markers on new fields as needed
# (e.g., make some visible, change write modes)

# 5. Regenerate registry and OpenAPI
make generate
```

## Key Points

### 1. Two Types of Types
- **HyperShift types**: Upstream source (read-only)
- **HyperFleet types**: Our wrapper with markers (editable)

### 2. Tools Don't Touch HyperShift
- We never modify HyperShift types
- We mirror them into HyperFleet with safe defaults
- Developers review and update markers in HyperFleet types

### 3. Safe Defaults Protect Production
- New HyperShift fields start hidden + service-set
- Must be explicitly reviewed before exposing to customers
- Prevents accidental API surface expansion

### 4. Single Source of Truth
- HyperFleet Go types (with markers) are authoritative
- Field registry is generated (don't hand-edit)
- OpenAPI schema is generated (don't hand-edit)

### 5. Marker Preservation (Future)
- Currently: passthrough-gen always applies safe defaults
- Future: passthrough-gen reads field registry to preserve reviewed markers
- This prevents losing manual marker updates on regeneration

## Demo with Existing Test Data

```bash
# We already have HyperShift cloned at:
HYPERSHIFT_DIR=/Users/cdoan/workspace/src/github.com/openshift/hypershift

# Generate passthrough types (already tested, generates 212 lines)
./bin/passthrough-gen \
  -source-dir=$HYPERSHIFT_DIR \
  -types=HostedClusterSpec,NodePoolSpec \
  -output-dir=/tmp/demo-output \
  -package=v1alpha1

# Scan markers from generated output
./bin/marker-scanner \
  -input-dirs=/tmp/demo-output \
  -output-file=/tmp/demo-registry.go \
  -verbose

# Generate OpenAPI (POC - minimal schema)
./bin/openapi-gen \
  -output-file=/tmp/demo-openapi.json \
  -title="HyperFleet Demo" \
  -version=v1alpha1
```

## Makefile Integration

The full sequence is automated via Makefile:

```makefile
.PHONY: generate
generate: generate-passthrough generate-registry generate-openapi

.PHONY: generate-passthrough
generate-passthrough:
	passthrough-gen -source-dir=$(HYPERSHIFT_DIR) ...

.PHONY: generate-registry
generate-registry:
	marker-scanner -input-dirs=./api/v1alpha1 ...

.PHONY: generate-openapi
generate-openapi:
	openapi-gen -input-dirs=./api/v1alpha1 ...
```

Single command to regenerate everything:
```bash
make generate
```
