# Feature Gates

**Audience**: Engineers and architects using the HyperFleet API codegen system  
**Related**: [api-management.md](./api-management.md), [feature-gated-write-mode-design.md](./feature-gated-write-mode-design.md)

## What Are Feature Gates?

Feature gates are a **customer entitlement system** that controls which API fields are available to different customer tiers. They enable gradual feature rollout from internal testing → early adopters → general availability.

### Three Feature Sets

| Feature Set | Stability | Typical Customers | Purpose |
|------------|-----------|-------------------|---------|
| **Default** | GA (Generally Available) | All production customers | Stable, fully-supported features only |
| **TechPreviewNoUpgrade** | Tech Preview | Early adopters, non-production | GA features + beta features (no upgrade path guarantee) |
| **DevPreviewNoUpgrade** | Dev Preview | Internal testing, development | All features including experimental (no support) |

**Hierarchy**: `DevPreview ⊃ TechPreview ⊃ Default`

DevPreview customers see **all** fields. TechPreview customers see GA + TechPreview fields. Default customers see only GA fields.

## Real-World Use Cases

### Use Case 1: Beta Feature Rollout

You're adding a new `autoscaling` configuration field. You want to test it internally before GA release.

**Phase 1 - Internal Testing** (DevPreview only):
```go
// +openshift:enable:FeatureGate=HyperFleetAutoScaling
// +hyperfleet:write-mode=mutable
AutoScaling *AutoScalingSpec `json:"autoScaling,omitempty"`
```

Gate in registry (`pkg/featuregate/registry.go`):
```go
"HyperFleetAutoScaling": {
    Stage: DevPreview,  // Only DevPreview customers see this
    Description: "Enables cluster autoscaling configuration",
}
```

**Phase 2 - Early Adopters** (promote to TechPreview):
```go
"HyperFleetAutoScaling": {
    Stage: TechPreview,  // Now TechPreview + DevPreview see it
    Description: "Enables cluster autoscaling configuration",
}
```

**Phase 3 - General Availability** (remove gate entirely):
```go
// Remove +openshift:enable:FeatureGate marker entirely
// +hyperfleet:write-mode=mutable
AutoScaling *AutoScalingSpec `json:"autoScaling,omitempty"`
```

### Use Case 2: Risky Configuration

You're exposing `etcd` configuration, but it's complex and risky. Start with TechPreview-only access.

```go
// Only visible to TechPreview/DevPreview customers
// +openshift:enable:FeatureGate=HyperFleetEtcdConfig
// +hyperfleet:write-mode=immutable  // Can only set on create
Etcd *EtcdSpec `json:"etcd,omitempty"`
```

**Why TechPreview instead of GA?**
- TechPreview customers opt-in to experimental features
- No upgrade guarantee = we can change the API if needed
- Limits blast radius of configuration mistakes

### Use Case 3: Experimental Internal-Only Feature

Custom DNS for development environments - never intended for production.

```go
// Internal-only, never promoted to GA
// +openshift:enable:FeatureGate=HyperFleetCustomDNS
// +hyperfleet:write-mode=mutable
CustomDNS *DNSSpec `json:"customDNS,omitempty"`
```

Gate stays at DevPreview:
```go
"HyperFleetCustomDNS": {
    Stage: DevPreview,  // Stays here permanently
    Description: "Enables custom DNS configuration for development/testing",
}
```

## How Feature Gates Work

### 1. Mark the Field

Add `+openshift:enable:FeatureGate=GateName` marker to the field:

```go
// api/v1alpha1/cluster_types.go
type ClusterSpec struct {
    // Available to all customers (no gate)
    // +hyperfleet:write-mode=mutable
    DisplayName string `json:"displayName"`
    
    // Available only to TechPreview/DevPreview customers
    // +openshift:enable:FeatureGate=HyperFleetAutoScaling
    // +hyperfleet:write-mode=mutable
    Tags map[string]string `json:"tags,omitempty"`
}
```

### 2. Register the Gate

Add to `pkg/featuregate/registry.go`:

```go
var HyperFleetFeatureGates = map[string]FeatureGateInfo{
    "HyperFleetAutoScaling": {
        Stage:       TechPreview,
        Description: "Enables cluster autoscaling configuration",
    },
}
```

### 3. Generate Artifacts

Run code generation:

```bash
make generate-registry  # Updates pkg/registry/field_metadata.json
make generate-openapi   # Updates openapi/openapi.json (respects gates)
make generate-crd-variants  # Creates per-feature-set CRD YAML
```

**Generated outputs**:

- **Field Registry** (`pkg/registry/field_metadata.json`):
  ```json
  {
    "fieldPath": "spec.tags",
    "writeMode": "mutable",
    "featureGate": "HyperFleetAutoScaling"
  }
  ```

- **OpenAPI Schema** (`openapi/openapi.json`):
  - Includes `tags` if customer has TechPreview/DevPreview
  - Omits `tags` if customer has Default

- **CRD Variants** (`config/crd/variants/`):
  - `cluster_default.yaml` - no `tags` field
  - `cluster_techpreview.yaml` - includes `tags` field
  - `cluster_devpreview.yaml` - includes `tags` field

### 4. Runtime Validation

The Platform API validates field access based on customer's feature set:

```go
// pkg/validation/validator.go
validator := validation.NewValidator()

req := &validation.Request{
    Operation:  validation.OperationCreate,
    Fields:     map[string]any{"spec.tags": map[string]string{"env": "prod"}},
    FeatureSet: featuregate.Default,  // Customer's tier
}

err := validator.Validate(req)
// Error: "field spec.tags requires feature gate HyperFleetAutoScaling 
//         which is not enabled in Default feature set"
```

## Current Implementation Details

### Adding a New Feature Gate

**Step 1**: Add to registry

```go
// pkg/featuregate/registry.go
"HyperFleetNewFeature": {
    Stage:       TechPreview,  // or GA or DevPreview
    Description: "Enables the new feature",
}
```

**Step 2**: Mark fields

```go
// api/v1alpha1/cluster_types.go
// +openshift:enable:FeatureGate=HyperFleetNewFeature
// +hyperfleet:write-mode=mutable
NewField *NewSpec `json:"newField,omitempty"`
```

**Step 3**: Regenerate

```bash
make generate
```

### Checking Feature Gate Status

```bash
make featuregate-info
```

Output:
```
=== HyperFleet Feature Gate Registry ===

Registered Feature Gates:
  HyperFleetEtcdConfig            Stage: GA            Allows customers to configure etcd settings
  HyperFleetAutoScaling           Stage: TechPreview   Enables cluster autoscaling configuration
  HyperFleetSecretEncryption      Stage: TechPreview   Allows customers to configure secret encryption
  HyperFleetCustomDNS             Stage: DevPreview    Enables custom DNS configuration for development/testing

=== Feature Set Field Summary ===

Default:
  Total visible fields: 11
  Enabled gates: [HyperFleetEtcdConfig]

TechPreviewNoUpgrade:
  Total visible fields: 12
  Enabled gates: [HyperFleetEtcdConfig HyperFleetAutoScaling HyperFleetSecretEncryption]

DevPreviewNoUpgrade:
  Total visible fields: 12
  Enabled gates: [HyperFleetEtcdConfig HyperFleetAutoScaling HyperFleetSecretEncryption HyperFleetCustomDNS]
```

### Testing Feature Gates

```go
// pkg/validation/example_test.go
func ExampleValidator_Validate_featureGate() {
    v := validation.NewValidator()
    
    // Customer with Default feature set tries to use gated field
    req := &validation.Request{
        Operation:  validation.OperationCreate,
        Fields:     map[string]any{"spec.tags": map[string]string{"env": "dev"}},
        FeatureSet: featuregate.Default,
    }
    
    err := v.Validate(req)
    fmt.Println(err)
    // Output:
    // validation failed:
    //   field spec.tags: requires feature gate HyperFleetAutoScaling which is not enabled in Default feature set
}
```

## Current Limitations

### Feature Gates Only Control Visibility

Today, feature gates **only** control whether a field appears in the API. They **cannot** control the field's write-mode (mutable/immutable/service-set).

**Example of what you CANNOT do today**:

```go
// DESIRED but NOT SUPPORTED:
// - Default customers: field is service-set (read-only)
// - TechPreview customers: field is mutable (writable)
//
// Today you must pick ONE write-mode for all feature sets
// +hyperfleet:write-mode=service-set  // <-- Applies to ALL feature sets
// +openshift:enable:FeatureGate=HyperFleetEtcdConfig
Etcd *EtcdSpec `json:"etcd,omitempty"`
```

### Future Enhancement: Feature-Gate-Aware Write-Mode

**Status**: Design proposal - REVISED ([ROSAENG-61570](https://redhat.atlassian.net/browse/ROSAENG-61570))

See [feature-gated-write-mode-design.md](./feature-gated-write-mode-design.md) for the complete design.

**Key insight**: Write-mode control will work **independently** of feature gating - it's not only for fields behind a FeatureGate. This allows customer-tier-based control on GA fields.

**Proposed syntax** (not yet implemented):

```go
// Use Case 1: GA field with customer-tier write-mode control
// Standard customers: immutable (set on create only)
// Premium customers: mutable (can change anytime)
// +hyperfleet:write-mode=immutable
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="",writeMode="immutable"
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="MyPremiumFeature",writeMode="mutable"
ReleaseChannel string `json:"releaseChannel"`

// Use Case 2: Gated field with progressive write-mode rollout
// Default customers: service-set (platform-controlled)
// TechPreview+ customers: mutable (customer-controlled)
// +hyperfleet:write-mode=service-set
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="",writeMode="service-set"
// +hyperfleet:validation:FeatureGateAwareWriteMode:featureGate="HyperFleetEtcdConfig",writeMode="mutable"
// +openshift:enable:FeatureGate=HyperFleetEtcdConfig
Etcd *EtcdSpec `json:"etcd,omitempty"`
```

**Behavior** (when implemented):
- **Use Case 1**: GA field varies by customer subscription tier (no feature gate required)
  - Standard tier: `releaseChannel` is immutable
  - Premium tier: `releaseChannel` is mutable
- **Use Case 2**: Gated field with progressive rollout
  - Default customers: `etcd` is service-set (platform-controlled)
  - TechPreview+ customers: `etcd` is mutable (customer-controlled)

**Why this approach**: Follows existing OpenShift API marker patterns (`FeatureGateAwareEnum`, `FeatureGateAwareXValidation`) instead of inventing new syntax.

This enables:
- **Customer-tier-based control**: Different write-modes for different subscription tiers
- **Gradual rollout**: Progressively open up customer control over sensitive fields
- **Independent of gating**: Works on GA fields without requiring feature gates

## Best Practices

### When to Use Feature Gates

**DO use feature gates for**:
- New beta features (test with early adopters first)
- Risky configuration (limit blast radius)
- Experimental APIs (internal testing)
- Features with unclear customer demand

**DON'T use feature gates for**:
- Stable, well-understood features (just make them GA)
- Internal-only fields (use `+k8s:openapi-gen=false` instead)
- Fields that vary by account/subscription (use different mechanism)

### Naming Conventions

- Prefix: `HyperFleet`
- PascalCase: `HyperFleetAutoScaling` (not `hyperfleet-auto-scaling`)
- Descriptive: Name should indicate what it controls

### Promotion Path

1. **DevPreview** - Internal testing, no customer exposure
2. **TechPreview** - Early adopters, gather feedback
3. **GA** - Remove gate entirely, available to all

**Timing**: Stay in TechPreview for at least 1-2 releases before GA promotion.

### Documentation

When adding a gate:
1. Update registry with clear description
2. Document in API reference
3. Add examples to this doc
4. Notify customer-facing teams of new TechPreview features

## Frequently Asked Questions

### Q: Can a field have multiple feature gates?

**A**: No. Each field can have at most one `+openshift:enable:FeatureGate` marker. If you need complex gating logic, use a single gate with a descriptive name.

### Q: What happens if a customer downgrades their feature set?

**A**: The Platform API will reject requests containing fields they no longer have access to. Existing cluster state is preserved but immutable.

### Q: Can I make a GA field gated after the fact?

**A**: No. Once a field is GA (no gate), you cannot gate it without breaking backward compatibility. Design carefully before GA.

### Q: Do feature gates affect CRD validation?

**A**: Yes, indirectly. CRD variants are generated per feature set, so each variant only includes fields accessible to that tier.

### Q: How do feature gates interact with RBAC?

**A**: They are independent. Feature gates control **field availability** based on customer tier. RBAC controls **operation permissions** based on user roles. Both checks apply.

## Summary

- **Feature gates** = customer entitlement system for gradual feature rollout
- **Three tiers**: Default (GA only), TechPreview (GA + beta), DevPreview (all)
- **Current capability**: Controls field visibility only
- **Future enhancement**: Will control write-mode per feature set ([design doc](./feature-gated-write-mode-design.md))
- **Use for**: Beta features, risky config, experimental APIs
- **Promotion path**: DevPreview → TechPreview → GA (remove gate)

## See Also

- [api-management.md](./api-management.md) - Complete marker system documentation
- [feature-gated-write-mode-design.md](./feature-gated-write-mode-design.md) - Technical design for ROSAENG-61570
- [workflow.md](./workflow.md) - End-to-end code generation workflow
- `pkg/featuregate/` - Implementation code
- `pkg/validation/` - Runtime validation logic
