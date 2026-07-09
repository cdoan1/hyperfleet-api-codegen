# Feature-Gated Write-Mode Control

**Status**: Design Proposal  
**Jira**: [ROSAENG-61570](https://redhat.atlassian.net/browse/ROSAENG-61570)  
**Author**: Chris Doan  
**Date**: 2026-07-09  
**Reviewers**: Lucas (reach out for detailed requirements)

## Problem Statement

Currently, HyperFleet's field control system supports three independent dimensions:

1. **Visibility** (`+k8s:openapi-gen=false`) - whether fields appear in OpenAPI/API surface
2. **Write Mode** (`+hyperfleet:write-mode=X`) - customer mutability (mutable/immutable/service-set)
3. **Feature Gating** (`+openshift:enable:FeatureGate=X`) - per-customer entitlements based on feature set

**The limitation**: Feature gates only control visibility. A field's write-mode is **fixed across all feature sets**. This prevents gradual rollout of customer control over sensitive fields.

### Use Case Example

We want to introduce customer control over `etcd` configuration, but it's risky:

- **Default customers** (production): `etcd` should be `service-set` (read-only, platform-managed)
- **TechPreview customers** (early adopters): `etcd` should be `mutable` (customer-controlled for testing)
- **DevPreview customers** (internal testing): `etcd` should be `mutable`

Today, we must choose ONE write-mode for all feature sets, preventing this gradual rollout pattern.

## Proposed Solution

### Marker Syntax Extension

Add optional **feature-set-specific write-mode overrides** to the `+hyperfleet:write-mode` marker:

```go
type ClusterSpec struct {
    // Base write-mode applies to Default feature set (no gate)
    // +hyperfleet:write-mode=mutable
    DisplayName string `json:"displayName"`

    // Per-feature-set write-mode overrides
    // +hyperfleet:write-mode=service-set                        // Default: platform-controlled
    // +hyperfleet:write-mode[TechPreviewNoUpgrade]=mutable      // TechPreview: customer-controlled
    // +hyperfleet:write-mode[DevPreviewNoUpgrade]=mutable       // DevPreview: customer-controlled
    // +openshift:enable:FeatureGate=HyperFleetEtcdConfig
    Etcd *EtcdSpec `json:"etcd,omitempty"`
}
```

**Syntax**:
- Base mode: `+hyperfleet:write-mode=X`
- Override: `+hyperfleet:write-mode[FeatureSetName]=X`
- Valid feature set names: `Default`, `TechPreviewNoUpgrade`, `DevPreviewNoUpgrade`
- Valid write modes: `mutable`, `immutable`, `service-set`

### Data Model Changes

**Current FieldMeta** (`pkg/markers/types.go`):
```go
type FieldMeta struct {
    FieldPath   string
    WriteMode   WriteMode    // Single mode for all
    FeatureGate string
    Hidden      bool
}
```

**Proposed FieldMeta**:
```go
type FieldMeta struct {
    FieldPath        string
    WriteMode        WriteMode                      // Base mode (Default feature set)
    FeatureGate      string                         // Gate required for visibility
    Hidden           bool
    GatedWriteModes  map[string]WriteMode `json:"gatedWriteModes,omitempty"`  // Feature-set-specific overrides
}
```

**JSON Registry Example**:
```json
{
  "fieldPath": "spec.etcd",
  "writeMode": "service-set",
  "featureGate": "HyperFleetEtcdConfig",
  "gatedWriteModes": {
    "TechPreviewNoUpgrade": "mutable",
    "DevPreviewNoUpgrade": "mutable"
  }
}
```

## Implementation Plan

### Phase 1: Marker Parsing

**File**: `pkg/markers/scanner.go`

Update `extractMarkers()` to recognize bracketed syntax:

```go
var writeModeGatedPattern = regexp.MustCompile(`\+hyperfleet:write-mode\[([^\]]+)\]=(mutable|immutable|service-set)`)

func (s *MarkerScanner) extractMarkers(field *ast.Field, fieldPath string) *FieldMeta {
    // ... existing code ...
    
    // Extract gated write-modes
    gatedModes := make(map[string]WriteMode)
    for _, match := range writeModeGatedPattern.FindAllStringSubmatch(comments, -1) {
        featureSet := match[1]  // TechPreviewNoUpgrade
        mode := WriteMode(match[2])  // mutable
        gatedModes[featureSet] = mode
    }
    
    if len(gatedModes) > 0 {
        meta.GatedWriteModes = gatedModes
    }
}
```

### Phase 2: Registry Generation

**Files**: `pkg/markers/generator.go`, `pkg/markers/json.go`

Update templates to include `GatedWriteModes`:

```go
type templateField struct {
    FieldPath       string
    WriteMode       string
    FeatureGate     string
    Hidden          bool
    GatedWriteModes map[string]string  // NEW
}
```

### Phase 3: Runtime Validation

**File**: `pkg/validation/validator.go`

Update `validateWriteMode()` to check feature-set-specific mode:

```go
func (v *Validator) validateWriteMode(fieldPath string, meta registry.FieldMeta, req *Request) error {
    // Determine effective write-mode for this request's feature set
    effectiveMode := meta.WriteMode  // Default
    
    // Check for feature-set-specific override
    if meta.GatedWriteModes != nil {
        if override, ok := meta.GatedWriteModes[string(req.FeatureSet)]; ok {
            effectiveMode = override
        }
    }
    
    // Enforce the effective mode
    switch effectiveMode {
    case registry.ServiceSet:
        return &ValidationError{
            FieldPath: fieldPath,
            Reason:    "field is platform-managed (service-set) for this feature set",
        }
    // ... rest of validation
    }
}
```

### Phase 4: Testing

**File**: `pkg/validation/validator_test.go`

Add test cases:

```go
func TestValidator_GatedWriteMode(t *testing.T) {
    tests := []struct {
        name        string
        fieldPath   string
        featureSet  featuregate.FeatureSet
        operation   Operation
        expectError bool
    }{
        {
            name:        "service-set for Default, mutable for TechPreview",
            fieldPath:   "spec.etcd",
            featureSet:  featuregate.Default,
            operation:   OperationCreate,
            expectError: true,  // service-set blocks customer writes
        },
        {
            name:        "mutable for TechPreview",
            fieldPath:   "spec.etcd",
            featureSet:  featuregate.TechPreviewNoUpgrade,
            operation:   OperationCreate,
            expectError: false,  // mutable allows writes
        },
    }
    // ...
}
```

## Migration Strategy

### Backward Compatibility

Existing fields without gated write-modes continue to work unchanged:
- No breaking changes to current markers
- New syntax is opt-in
- Empty `GatedWriteModes` map treated as "no overrides"

### Rollout

1. Merge code changes
2. Update documentation
3. Add gated write-modes to specific fields as needed (opt-in)
4. Monitor validation metrics

No mass migration required - fields adopt gated modes incrementally.

## Examples

### Example 1: Beta Feature Rollout

```go
// New experimental field: read-only initially, writable in TechPreview
// +hyperfleet:write-mode=service-set                       // Default: platform fills it
// +hyperfleet:write-mode[TechPreviewNoUpgrade]=mutable     // TechPreview: customer controls
// +openshift:enable:FeatureGate=HyperFleetCustomDNS
CustomDNS *DNSSpec `json:"customDNS,omitempty"`
```

### Example 2: Immutable → Mutable Promotion

```go
// Field was immutable, now mutable in DevPreview for testing
// +hyperfleet:write-mode=immutable                         // Default/TechPreview: set on create
// +hyperfleet:write-mode[DevPreviewNoUpgrade]=mutable      // DevPreview: fully mutable
ReleaseChannel string `json:"releaseChannel"`
```

### Example 3: No Gating (Current Behavior)

```go
// Works today, continues to work unchanged
// +hyperfleet:write-mode=mutable
Tags map[string]string `json:"tags,omitempty"`
```

## Trade-offs and Alternatives

### Alternative 1: Separate Fields Per Feature Set

Create different field names: `etcdDefault`, `etcdTechPreview`.

**Rejected**: API surface explosion, confusing for customers.

### Alternative 2: Runtime-Only Feature Flag

Use environment variables or config to control write-mode at runtime.

**Rejected**: Not declarative, harder to audit, doesn't integrate with existing marker system.

### Alternative 3: Multiple Feature Gates

Use multiple gates: `CanViewEtcd`, `CanWriteEtcd`.

**Rejected**: Doubles the number of gates, doesn't scale, violates single-gate-per-field principle.

### Chosen Approach: Gated Write-Modes

**Pros**:
- Declarative (visible in code)
- Extends existing marker system
- Backward compatible
- Scales to many fields

**Cons**:
- Adds complexity to marker syntax
- Requires parser changes
- Validation logic more complex

## Open Questions

1. **Marker syntax**: Is `[FeatureSetName]` the right syntax? Alternatives: `@FeatureSet`, `:FeatureSet`, `{FeatureSet}`?

2. **Default inheritance**: If only `TechPreview` override specified, should `DevPreview` inherit it or fall back to base mode?
   - **Recommendation**: Fall back to base mode (explicit > implicit)

3. **Validation error messages**: How detailed should errors be for feature-set-specific rejections?
   - **Recommendation**: Include both feature set and effective mode in error

4. **CRD variant generation**: Should CRD YAML show different write-modes per variant?
   - **Recommendation**: Not initially - CRDs show schema, not validation rules

5. **Migration path**: Should we auto-migrate existing fields or require explicit opt-in?
   - **Recommendation**: Explicit opt-in only

## Next Steps

1. **Review this design** with Lucas and team
2. **Gather feedback** on marker syntax and open questions
3. **Prototype** marker parsing to validate regex approach
4. **Implement** phases 1-4 after approval
5. **Document** in `docs/feature-gates.md` with examples

## References

- **Jira**: [ROSAENG-61570](https://redhat.atlassian.net/browse/ROSAENG-61570)
- **Parent Epic**: [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)
- **Current Implementation**: `pkg/featuregate/`, `pkg/validation/`, `pkg/markers/`
- **OpenShift API Pattern**: https://github.com/openshift/api/tree/master/tools/codegen
