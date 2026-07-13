# Passthrough vs Mirror Types: Configuration Field Comparison

## Overview

This document compares two approaches for handling HyperShift's `ClusterConfiguration` field:

1. **Passthrough Approach** (v0.1.1) - Use HyperShift's type directly
2. **Mirror Types Approach** (main branch since v0.1.1) - Hand-written HyperFleet-owned mirrors

## Current State: v0.1.1 (Passthrough Approach)

### Configuration Field
```go
// api/v1alpha1/hostedclusterspec.passthrough.go
type HostedClusterSpecPassthrough struct {
    // ... other fields ...
    
    // Configuration uses upstream HyperShift type directly
    Configuration *hypershiftv1beta1.ClusterConfiguration `json:"configuration,omitempty"`
}
```

### Characteristics
- **Field count**: 58 fields tracked in registry
- **Files**: No `api/v1alpha1/configuration.go`
- **Type**: `*hypershiftv1beta1.ClusterConfiguration` (imported from HyperShift)
- **Control level**: Opaque type, all-or-nothing visibility/mutability

### Pros
✅ **Automatic HyperShift tracking** - Type changes are detected on HyperShift bump
✅ **Simple codebase** - No hand-written mirror types to maintain
✅ **Less CI infrastructure** - No verify-configuration tool needed
✅ **Fewer files** - Only passthrough generation

### Cons
❌ **No granular control** - Cannot expose subset of kubelet fields
❌ **No per-field markers** - Cannot hide dangerous fields while showing safe ones
❌ **No field-level write-mode** - Cannot make some fields mutable, others immutable
❌ **No field-level gates** - Cannot feature-gate advanced settings

## Main Branch Approach (Mirror Types)

### Configuration Field
```go
// api/v1alpha1/hostedclusterspec.passthrough.go
type HostedClusterSpecPassthrough struct {
    // ... other fields ...
    
    // Configuration uses HyperFleet-owned mirror type
    // +hyperfleet:write-mode=service-set
    Configuration *ClusterConfiguration `json:"configuration,omitempty"`
}

// api/v1alpha1/configuration.go (NEW FILE - 318 lines)
type ClusterConfiguration struct {
    Kubelet       *KubeletConfig      `json:"kubelet,omitempty"`
    MachineConfig *MachineConfigSpec  `json:"machineConfig,omitempty"`
    // ... other areas ...
}

type KubeletConfig struct {
    // ✅ VISIBLE + MUTABLE - customers can set
    // +hyperfleet:write-mode=mutable
    PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`
    
    // ✅ VISIBLE + IMMUTABLE - set once on creation
    // +hyperfleet:write-mode=immutable
    SystemReserved map[string]string `json:"systemReserved,omitempty"`
    
    // ❌ HIDDEN + SERVICE-SET - platform manages for stability
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    EvictionHard map[string]string `json:"evictionHard,omitempty"`
    
    // ✅ VISIBLE + MUTABLE + GATED - TechPreview only
    // +openshift:enable:FeatureGate=HyperFleetKubeletAdvanced
    // +hyperfleet:write-mode=mutable
    SerializeImagePulls *bool `json:"serializeImagePulls,omitempty"`
    
    // ... 19 more kubelet fields ...
}

type MachineConfigSpec struct {
    // ✅ VISIBLE + IMMUTABLE + GATED - TechPreview whitelist
    // +openshift:enable:FeatureGate=HyperFleetMachineConfig
    // +hyperfleet:write-mode=immutable
    AllowedKernelArguments []string `json:"allowedKernelArguments,omitempty"`
    
    // ❌ HIDDEN + SERVICE-SET - platform-only, dangerous
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    SystemdUnits []SystemdUnit `json:"systemdUnits,omitempty"`
    
    // ... 5 more machine config fields ...
}
```

### Characteristics
- **Field count**: 163 fields tracked (58 passthrough + 105 configuration)
- **Files**: Added `api/v1alpha1/configuration.go` (318 lines), `cmd/verify-configuration/`, docs
- **Type**: `*v1alpha1.ClusterConfiguration` (HyperFleet-owned mirror)
- **Control level**: Granular per-field control with 3 write-modes and 2 gates

### Pros
✅ **Granular field control** - Expose podPidsLimit, hide evictionHard
✅ **Per-field write-mode** - mutable vs immutable vs service-set
✅ **Per-field feature gates** - TechPreview for advanced settings
✅ **Security reviewed** - Each exposed field explicitly approved
✅ **Customer flexibility** - Safe subset exposed for tuning

### Cons
❌ **Manual HyperShift tracking** - Must update mirrors when upstream changes
❌ **More code to maintain** - 318 lines of hand-written types
❌ **CI verification needed** - AST-based tool ensures markers present
❌ **More files** - configuration.go, verify-configuration tool, docs

## Side-by-Side Comparison

| Aspect | Passthrough (v0.1.1) | Mirror Types (main) |
|--------|---------------------|---------------------|
| **Field count** | 58 | 163 |
| **Lines of code** | 0 (generated) | 318 (hand-written) |
| **Kubelet control** | None (opaque) | 23 fields, 3 write-modes |
| **Machine config** | None (opaque) | 7 fields, security whitelist |
| **HyperShift tracking** | Automatic | Manual updates needed |
| **CI verification** | Not needed | AST-based tool required |
| **Feature gates** | Object-level only | Per-field granular |
| **Customer exposure** | All or nothing | Curated subset |
| **Security model** | Block entire Configuration | Expose safe fields, block dangerous |

## Example: Customer Wants to Tune Kubelet

### Passthrough Approach (v0.1.1)
**Customer request**: "I need to increase podPidsLimit to 8192 for high-density workloads"

**Platform response**:
```
❌ Cannot expose podPidsLimit without also exposing:
  - evictionHard (dangerous - affects cluster stability)
  - allowedUnsafeSysctls (dangerous - kernel-level access)
  - cpuManagerPolicy (complex - requires deep K8s knowledge)
  
Decision: Keep Configuration hidden (service-set)
Result: Customer cannot tune kubelet settings
```

### Mirror Types Approach (main)
**Customer request**: "I need to increase podPidsLimit to 8192"

**Platform response**:
```
✅ podPidsLimit is exposed as mutable field

Customer can set in API:
{
  "spec": {
    "kubelet": {
      "podPidsLimit": 8192
    }
  }
}

Platform hides dangerous fields (evictionHard, allowedUnsafeSysctls)
Platform manages complex fields (cpuManagerPolicy)
Result: Customer gets safe, audited control
```

## Recommendation

**For POC/Experimentation**: Use Passthrough (v0.1.1 state)
- Faster iteration
- Less code to maintain
- Good for understanding the pattern

**For Production**: Use Mirror Types (main branch)
- Granular security control
- Field-level feature gates
- Better customer experience (expose safe subsets)
- Required for compliance (audit which fields customers touch)

## Migration Path

### From Passthrough → Mirror Types
1. Create `api/v1alpha1/configuration.go` with curated types
2. Change passthrough to use `*ClusterConfiguration`
3. Add CI verification tool
4. Regenerate registry (58 → 163 fields)
5. Security review each exposed field

**Commits**: See v0.1.1..main (commits 9823619, b154a28, fc9fa06, etc.)

### From Mirror Types → Passthrough
1. Delete `api/v1alpha1/configuration.go`
2. Delete `cmd/verify-configuration/`
3. Change passthrough to use `*hypershiftv1beta1.ClusterConfiguration`
4. Regenerate registry (163 → 58 fields)
5. Document trade-offs for stakeholders

## Files Involved

### Passthrough Approach
- `api/v1alpha1/hostedclusterspec.passthrough.go` - Uses `*hypershiftv1beta1.ClusterConfiguration`
- `pkg/registry/field_metadata.go` - 58 fields

### Mirror Types Approach
- `api/v1alpha1/hostedclusterspec.passthrough.go` - Uses `*ClusterConfiguration`
- `api/v1alpha1/configuration.go` - **NEW** - 318 lines of mirror types
- `cmd/verify-configuration/main.go` - **NEW** - AST-based CI tool
- `docs/configuration-types-pattern.md` - **NEW** - Pattern documentation
- `pkg/registry/field_metadata.go` - 163 fields

## Conclusion

Both approaches are valid. The choice depends on requirements:

- **Need granular control?** → Mirror Types
- **Want automatic tracking?** → Passthrough
- **Compliance requirements?** → Mirror Types (field-level audit trail)
- **Rapid prototyping?** → Passthrough (less code)

The v0.1.1 passthrough approach is simpler but sacrifices granular control.
The main branch mirror types approach is more complex but enables per-field security and customer flexibility.
