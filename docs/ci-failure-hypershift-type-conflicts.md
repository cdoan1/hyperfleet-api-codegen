# CI Failure: HyperShift Type Conflicts on Version Bump

## Issue

CI test `test-hypershift-bump-latest.sh` fails when bumping to HyperShift v0.1.78 with type mismatch errors in conversion functions.

**GitHub Actions**: https://github.com/cdoan1/hyperfleet-api-codegen/actions/runs/29274482281/job/86900026532

## Root Cause

There are **two distinct problems**:

### Problem 1: AutoNode Type Changed (HyperShift upstream change)

**Error**:
```
pkg/conversion/v1alpha1/cluster.go:78:13: cannot use crd.AutoNode 
(variable of struct type v1beta1.AutoNode) as *v1beta1.AutoNode value in struct literal

pkg/conversion/v1alpha1/cluster.go:90:13: cannot use rest.AutoNode 
(variable of type *v1beta1.AutoNode) as v1beta1.AutoNode value in struct literal
```

**What happened**:
- HyperShift v0.1.78 changed `AutoNode` from `*AutoNode` (pointer) to `AutoNode` (value type)
- Our passthrough generator picked up the new type
- Conversion functions still expect pointer type

**Current passthrough** (generated from v0.1.78):
```go
type HostedClusterSpecPassthrough struct {
    AutoNode *hypershiftv1beta1.AutoNode `json:"autoNode,omitempty"`  // Still pointer in passthrough
}
```

**But HyperShift v0.1.78 upstream**:
```go
type HostedClusterSpec struct {
    AutoNode AutoNode `json:"autoNode,omitempty"`  // Changed to value type
}
```

**Impact**: Conversion functions break because they try to assign pointer ↔ value.

### Problem 2: Configuration Type Conflict (Mirror Types Pattern)

**Error**:
```
pkg/conversion/v1alpha1/cluster.go:79:18: cannot use crd.Configuration 
(variable of type *v1beta1.ClusterConfiguration) as *v1alpha1.ClusterConfiguration value in struct literal

pkg/conversion/v1alpha1/cluster.go:91:18: cannot use rest.Configuration 
(variable of type *v1alpha1.ClusterConfiguration) as *v1beta1.ClusterConfiguration value in struct literal
```

**What happened**:
- We use HyperFleet-owned mirror type `*v1alpha1.ClusterConfiguration`
- HyperShift has their own `*v1beta1.ClusterConfiguration`
- Conversion functions try to directly assign between incompatible types

**Our passthrough** (uses our mirror type):
```go
type HostedClusterSpecPassthrough struct {
    Configuration *ClusterConfiguration `json:"configuration,omitempty"`  // v1alpha1.ClusterConfiguration
}
```

**Conversion function** (broken):
```go
func ProjectCluster(crd *v1alpha1.Cluster) *rest.Cluster {
    return &rest.Cluster{
        Spec: rest.ClusterSpec{
            HostedCluster: rest.HostedClusterSpecPassthrough{
                // ❌ Type mismatch - crd.Configuration is *v1beta1.ClusterConfiguration
                //    but we need *v1alpha1.ClusterConfiguration
                Configuration: crd.Configuration,
            },
        },
    }
}
```

**Impact**: Conversion functions cannot compile because types don't match.

## Why This Happens

### AutoNode Issue
This is a **normal upstream API change**. HyperShift changed a field type, and we need to:
1. Regenerate passthrough types (picks up new value type)
2. Update conversion functions to handle value ↔ pointer conversion
3. This is expected maintenance when tracking upstream

### Configuration Issue
This is the **fundamental problem with mirror types pattern**:
- We created `v1alpha1.ClusterConfiguration` to get granular field control
- HyperShift has `v1beta1.ClusterConfiguration` (opaque upstream type)
- These are **different types** that cannot be directly assigned
- Conversion functions need explicit type conversion logic

## Detailed Analysis

### Where Conversion Breaks

**File**: `pkg/conversion/v1alpha1/cluster.go`

**ProjectCluster (CRD → REST)**:
```go
// Line 78-79: Broken conversion
func ProjectCluster(crd *v1alpha1.Cluster) *rest.Cluster {
    return &rest.Cluster{
        Spec: rest.ClusterSpec{
            HostedCluster: rest.HostedClusterSpecPassthrough{
                AutoNode: crd.AutoNode,          // ❌ Pointer vs value mismatch
                Configuration: crd.Configuration, // ❌ Type mismatch (v1beta1 vs v1alpha1)
            },
        },
    }
}
```

**UnprojectCluster (REST → CRD)**:
```go
// Line 90-91: Broken conversion
func UnprojectCluster(rest *rest.Cluster, svc *ServiceSetFields) *v1alpha1.Cluster {
    return &v1alpha1.Cluster{
        Spec: v1alpha1.ClusterSpec{
            HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
                AutoNode: rest.AutoNode,          // ❌ Value vs pointer mismatch
                Configuration: rest.Configuration, // ❌ Type mismatch (v1alpha1 vs v1beta1)
            },
        },
    }
}
```

## Solutions

### Fix for AutoNode (Simple - Pointer/Value Conversion)

Add conversion logic to handle pointer ↔ value:

```go
// ProjectCluster (CRD → REST): value to pointer
func ProjectCluster(crd *v1alpha1.Cluster) *rest.Cluster {
    var autoNode *hypershiftv1beta1.AutoNode
    if crd.AutoNode != (hypershiftv1beta1.AutoNode{}) {  // Check if not zero value
        autoNode = &crd.AutoNode
    }
    
    return &rest.Cluster{
        Spec: rest.ClusterSpec{
            HostedCluster: rest.HostedClusterSpecPassthrough{
                AutoNode: autoNode,  // ✅ Converted to pointer
            },
        },
    }
}

// UnprojectCluster (REST → CRD): pointer to value
func UnprojectCluster(rest *rest.Cluster, svc *ServiceSetFields) *v1alpha1.Cluster {
    var autoNode hypershiftv1beta1.AutoNode
    if rest.AutoNode != nil {
        autoNode = *rest.AutoNode
    }
    
    return &v1alpha1.Cluster{
        Spec: v1alpha1.ClusterSpec{
            HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
                AutoNode: autoNode,  // ✅ Converted to value
            },
        },
    }
}
```

### Fix for Configuration (Complex - Type Conversion)

Add conversion functions between `v1alpha1.ClusterConfiguration` and `v1beta1.ClusterConfiguration`:

**Option A: Deep conversion (field-by-field copy)**
```go
// Convert HyperFleet mirror type → HyperShift upstream type
func convertClusterConfigurationToHyperShift(cfg *v1alpha1.ClusterConfiguration) *v1beta1.ClusterConfiguration {
    if cfg == nil {
        return nil
    }
    
    hypershift := &v1beta1.ClusterConfiguration{}
    
    // Convert kubelet config
    if cfg.Kubelet != nil {
        hypershift.Kubelet = &v1beta1.KubeletConfig{
            PodPidsLimit:   cfg.Kubelet.PodPidsLimit,
            SystemReserved: cfg.Kubelet.SystemReserved,
            EvictionHard:   cfg.Kubelet.EvictionHard,
            // ... map all 23 kubelet fields
        }
    }
    
    // Convert machine config
    if cfg.MachineConfig != nil {
        hypershift.MachineConfig = &v1beta1.MachineConfig{
            KernelArguments: cfg.MachineConfig.KernelArguments,
            SystemdUnits:    convertSystemdUnits(cfg.MachineConfig.SystemdUnits),
            // ... map all 7 machine config fields
        }
    }
    
    return hypershift
}

// Convert HyperShift upstream type → HyperFleet mirror type
func convertClusterConfigurationFromHyperShift(cfg *v1beta1.ClusterConfiguration) *v1alpha1.ClusterConfiguration {
    if cfg == nil {
        return nil
    }
    
    hyperfleet := &v1alpha1.ClusterConfiguration{}
    
    // Reverse conversion (map all fields back)
    if cfg.Kubelet != nil {
        hyperfleet.Kubelet = &v1alpha1.KubeletConfig{
            PodPidsLimit:   cfg.Kubelet.PodPidsLimit,
            SystemReserved: cfg.Kubelet.SystemReserved,
            // ... map all fields
        }
    }
    
    return hyperfleet
}

// Use in conversion functions:
func ProjectCluster(crd *v1alpha1.Cluster) *rest.Cluster {
    return &rest.Cluster{
        Spec: rest.ClusterSpec{
            HostedCluster: rest.HostedClusterSpecPassthrough{
                Configuration: convertClusterConfigurationToHyperShift(crd.Configuration),
            },
        },
    }
}
```

**Option B: JSON round-trip (easier but slower)**
```go
func convertConfiguration(src interface{}, dst interface{}) error {
    data, err := json.Marshal(src)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, dst)
}
```

**Option C: Skip Configuration field in conversion (simplest)**
```go
// Don't convert Configuration at all - handle it separately in Platform API
func ProjectCluster(crd *v1alpha1.Cluster) *rest.Cluster {
    return &rest.Cluster{
        Spec: rest.ClusterSpec{
            HostedCluster: rest.HostedClusterSpecPassthrough{
                // Configuration: nil,  // Skip - handled separately
            },
        },
    }
}
```

## Recommended Approach

### Short-term Fix (Unblock CI)

1. **AutoNode**: Add pointer/value conversion (10 lines of code)
2. **Configuration**: Skip in conversion functions - set to `nil` (1 line change)
   - Platform API handles Configuration separately anyway
   - Customer-facing API uses `v1alpha1.ClusterConfiguration`
   - When submitting to K8s, Platform API constructs `v1beta1.ClusterConfiguration` from HyperFleet's mirror

### Long-term Solution

**Option 1: Keep Mirror Types + Add Deep Conversion**
- Pros: Maintains granular field control
- Cons: Must maintain conversion logic for all 130+ config fields

**Option 2: Revert to Passthrough (v0.1.1 approach)**
- Pros: No conversion needed, automatic HyperShift tracking
- Cons: Loses granular field control (all-or-nothing)
- See: `docs/passthrough-vs-mirror-comparison.md`

**Option 3: Hybrid - Flatten Customer-Facing Fields**
- Keep passthrough Configuration (hidden, service-set)
- Add flat top-level fields for customer control:
  ```go
  type ClusterSpec struct {
      // Hidden passthrough (platform manages full object)
      // +k8s:openapi-gen=false
      Configuration *v1beta1.ClusterConfiguration
      
      // Visible flat fields for customers
      // +hyperfleet:write-mode=mutable
      KubeletPodPidsLimit *int64
  }
  ```
- Platform API maps flat → nested when submitting to K8s

## Related Documentation

- **Type conflict analysis**: `docs/passthrough-vs-mirror-comparison.md`
- **CI failure logs**: https://github.com/cdoan1/hyperfleet-api-codegen/actions/runs/29274482281
- **Mirror types pattern**: `docs/configuration-types-pattern.md` (main branch)

## Action Items

1. Document this failure (this doc)
2. Decide on approach:
   - Quick fix: Skip Configuration in conversion
   - OR Full fix: Implement deep conversion
   - OR Architectural change: Revert to passthrough
3. Update conversion functions
4. Add tests for type conversions
5. Update HyperShift bump workflow to detect type changes
