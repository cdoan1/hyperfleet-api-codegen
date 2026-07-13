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
❌ IMPOSSIBLE with passthrough approach

Why:
1. Configuration field is HIDDEN (+k8s:openapi-gen=false)
2. Configuration field is SERVICE-SET (platform-only)
3. HyperShift's ClusterConfiguration is opaque - cannot add markers to nested fields

Options:
a) Expose entire Configuration object (all-or-nothing)
   → Exposes dangerous fields (evictionHard, allowedUnsafeSysctls)
   → Security risk

b) Keep Configuration hidden (current state at v0.1.1)
   → Customer cannot tune ANY kubelet settings
   → ✅ Safe, ❌ Inflexible

Decision: Keep Configuration hidden
Result: Customer cannot tune kubelet settings AT ALL
```

### Mirror Types Approach (main)
**Customer request**: "I need to increase podPidsLimit to 8192"

**Platform response**:
```
✅ POSSIBLE with mirror types - granular field control

Why:
1. HyperFleet owns the type definition in configuration.go
2. Can add markers to individual nested fields
3. Can expose safe fields, hide dangerous fields

Implementation:
// api/v1alpha1/configuration.go
type KubeletConfig struct {
    // ✅ EXPOSED + MUTABLE - safe for customers
    // +hyperfleet:write-mode=mutable
    PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`
    
    // ❌ HIDDEN + SERVICE-SET - dangerous, platform-only
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    EvictionHard map[string]string `json:"evictionHard,omitempty"`
}

Customer can set in API:
{
  "spec": {
    "kubelet": {
      "podPidsLimit": 8192  // ✅ Allowed
    }
  }
}

Customer CANNOT set:
{
  "spec": {
    "kubelet": {
      "evictionHard": {...}  // ❌ Hidden, rejected by validator
    }
  }
}

Result: Customer gets safe, audited control of approved fields
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

## FAQ: Common Questions About Passthrough vs Mirror Types

### Q: If ClusterConfiguration becomes a passthrough type, can we make MachineConfig a passthrough type where we can expose individual fields?

**A: No. Making ClusterConfiguration passthrough does NOT allow you to expose individual MachineConfig fields.**

**Why not?**

If `ClusterConfiguration` is passthrough, you're using **HyperShift's type**:

```go
// You use HyperShift's ClusterConfiguration (passthrough)
type HostedClusterSpecPassthrough struct {
    Configuration *hypershiftv1beta1.ClusterConfiguration `json:"configuration,omitempty"`
}
```

Inside HyperShift's `ClusterConfiguration`, the nested types are **also HyperShift's types** (or other upstream types):

```go
// From HyperShift's codebase (you DON'T control this)
package v1beta1

type ClusterConfiguration struct {
    Kubelet *KubeletConfig        // HyperShift's or MCO's type
    MachineConfig *MachineConfig  // HyperShift's or MCO's type
}
```

**You cannot go into HyperShift's (or MCO's) codebase and add your markers:**

```go
// This is in HyperShift's repo or MCO's repo - you DON'T control it
type MachineConfig struct {
    SystemdUnits []SystemdUnit     // ❌ Cannot add +k8s:openapi-gen=false here
    KernelArguments []string        // ❌ Cannot add +hyperfleet:write-mode here
}
```

**The type ownership hierarchy**:

**Passthrough chain (CANNOT control nested fields)**:
```
HostedClusterSpecPassthrough (yours)
  └─ Configuration *hypershiftv1beta1.ClusterConfiguration (HyperShift's)
       └─ MachineConfig *MachineConfig (HyperShift's or MCO's)
            └─ SystemdUnits []SystemdUnit  ❌ Cannot add markers
            └─ KernelArguments []string     ❌ Cannot add markers
```

**Owned chain (CAN control nested fields)**:
```
HostedClusterSpecPassthrough (yours)
  └─ Configuration *ClusterConfiguration (yours - HyperFleet-owned)
       └─ MachineConfig *MachineConfigSpec (yours - HyperFleet-owned)
            └─ SystemdUnits []SystemdUnit  ✅ Can add +k8s:openapi-gen=false
            └─ AllowedKernelArguments []string ✅ Can add +hyperfleet:write-mode
```

**Key insight**: To get granular field control, you must **own the entire type chain** from top to bottom. Making one level passthrough doesn't help if the nested levels are still upstream types you don't control.

### Q: Why does HostedClusterSpec passthrough work then?

**A: Because we only need top-level field control on HostedClusterSpec.**

With `HostedClusterSpec` passthrough, we can mark **top-level fields**:

```go
type HostedClusterSpecPassthrough struct {
    // ✅ We control THIS field (top-level)
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    ClusterID string `json:"clusterID,omitempty"`
    
    // ✅ We control THIS field (top-level)
    // +hyperfleet:write-mode=mutable
    Channel string `json:"channel,omitempty"`
}
```

We don't need to expose a **subset** of fields inside `ClusterID` or `Channel` - they're primitive types.

But for `Configuration`, we DO need subset control:
- Expose `Configuration.Kubelet.PodPidsLimit` (safe)
- Hide `Configuration.Kubelet.EvictionHard` (dangerous)

Passthrough cannot do this because we don't own the nested type definitions.

### Q: Can we make JUST KubeletConfig and MachineConfigSpec passthrough types?

**A: No, they don't exist as exported types in HyperShift.**

From checking HyperShift's API package:
- HyperShift's `ClusterConfiguration` does NOT have `Kubelet` or `MachineConfig` fields
- NodePool accepts kubelet/machine config as **opaque YAML in ConfigMaps**
- These are machine-config-operator (MCO) types, not HyperShift types
- MCO types are loaded dynamically, not part of HyperShift's API schema

Even if they existed:
- We'd need to import from MCO (`github.com/openshift/machine-config-operator/...`)
- We still couldn't add markers to MCO's type definitions
- Same all-or-nothing problem

**The only solution**: HyperFleet-owned mirror types where we control every field.

## Conclusion

**The passthrough approach CANNOT expose individual kubelet/machine config fields to customers.**

At v0.1.1, the Configuration field is:
```go
// +k8s:openapi-gen=false        ← HIDDEN from customer API
// +hyperfleet:write-mode=service-set  ← Platform-managed only
Configuration *hypershiftv1beta1.ClusterConfiguration `json:"configuration,omitempty"`
```

This is **by design** - it's an opaque HyperShift type we don't control.

### When Passthrough Works
✅ Use passthrough when you want ALL-OR-NOTHING control:
- Hide entire Configuration (current v0.1.1 state) - **SAFE**
- Expose entire Configuration (all fields visible) - **DANGEROUS**

### When Mirror Types Are Required
✅ Use mirror types when you need GRANULAR control:
- Expose safe kubelet subset (podPidsLimit) ✅
- Hide dangerous fields (evictionHard, allowedUnsafeSysctls) ✅
- Per-field write-mode (mutable vs immutable vs service-set) ✅
- Field-level feature gates (TechPreview for advanced settings) ✅

### The Real Trade-Off

**Passthrough**:
- ✅ Simpler (less code)
- ✅ Automatic HyperShift tracking
- ❌ Cannot give customers ANY kubelet control without exposing EVERYTHING

**Mirror Types**:
- ❌ More complex (318 lines)
- ❌ Manual HyperShift tracking
- ✅ Can give customers SAFE SUBSET of kubelet control

**If customers need ANY kubelet/machine config control, mirror types are the ONLY option.**
