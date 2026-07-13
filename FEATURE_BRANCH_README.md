# Feature Branch: revert-to-passthrough

## Purpose

This branch demonstrates the **Passthrough Approach** for handling HyperShift's `ClusterConfiguration` type - using the upstream type directly without hand-written mirrors.

**Based on**: tag v0.1.1 (before configuration types pattern was added)

## What This Branch Shows

### Clean Passthrough State
```go
// api/v1alpha1/hostedclusterspec.passthrough.go
type HostedClusterSpecPassthrough struct {
    // Uses HyperShift's type directly (passthrough)
    Configuration *hypershiftv1beta1.ClusterConfiguration `json:"configuration,omitempty"`
}
```

- **Field count**: 58 fields tracked in registry
- **No hand-written mirror types** - configuration.go does not exist
- **No CI verification tool** - verify-configuration does not exist
- **Automatic HyperShift tracking** - passthrough generator detects upstream changes

## Comparison with Main Branch

| Aspect | This Branch (Passthrough) | Main Branch (Mirror Types) |
|--------|---------------------------|----------------------------|
| Configuration type | `*hypershiftv1beta1.ClusterConfiguration` | `*v1alpha1.ClusterConfiguration` |
| Field count | 58 | 163 |
| configuration.go | Does not exist | 318 lines |
| verify-configuration tool | Does not exist | AST-based CI verification |
| Granular control | No (all-or-nothing) | Yes (per-field markers) |
| HyperShift tracking | Automatic | Manual |

## Trade-Offs

### What You Get
✅ Automatic HyperShift version tracking
✅ Simpler codebase (fewer files)
✅ Less CI infrastructure
✅ Faster HyperShift bump workflow

### What You Lose
❌ Cannot expose safe kubelet subset (podPidsLimit) while hiding dangerous settings (evictionHard)
❌ Cannot make some fields customer-mutable vs platform-managed
❌ Cannot feature-gate individual settings (TechPreview for advanced kubelet)
❌ All-or-nothing visibility on entire Configuration object

## Testing This Branch

### Verify Clean State
```bash
# Should show 58 fields (not 163)
jq 'length' pkg/registry/field_metadata.json

# Should NOT exist
ls api/v1alpha1/configuration.go
ls cmd/verify-configuration/

# Should show hypershiftv1beta1.ClusterConfiguration
grep "Configuration \*" api/v1alpha1/hostedclusterspec.passthrough.go
```

### Build and Test
```bash
make all
# All tests should pass with 58 fields
```

### HyperShift Bump Test
```bash
# This should work without Configuration type conflicts
.github/workflows/test-scripts/test-hypershift-bump.sh
```

## Documentation

See `docs/passthrough-vs-mirror-comparison.md` for:
- Detailed side-by-side comparison
- Customer use case examples
- Migration paths between approaches
- Pros/cons analysis

## When to Use This Approach

**Use Passthrough (this branch) if**:
- You want automatic HyperShift tracking
- You don't need granular per-field control
- You're okay with all-or-nothing Configuration visibility
- You prefer simpler codebase over flexibility

**Use Mirror Types (main branch) if**:
- You need to expose safe kubelet subset to customers
- You need per-field write-mode (mutable vs immutable vs service-set)
- You need field-level feature gates
- You need per-field security review/audit trail

## Example: Why Main Branch Chose Mirror Types

**Customer request**: "I need to tune podPidsLimit for high-density workloads"

**With Passthrough (this branch)**:
```
❌ IMPOSSIBLE - Configuration is opaque HyperShift type

Current state at v0.1.1:
// +k8s:openapi-gen=false           ← HIDDEN
// +hyperfleet:write-mode=service-set    ← Platform-only
Configuration *hypershiftv1beta1.ClusterConfiguration

Options:
1. Keep Configuration hidden → Customer gets ZERO kubelet control
2. Expose Configuration visible → Customer gets ALL-OR-NOTHING control
   - Exposes podPidsLimit ✅
   - Also exposes evictionHard ❌ (dangerous)
   - Also exposes allowedUnsafeSysctls ❌ (dangerous)

→ Cannot give customers SAFE SUBSET
→ Customer cannot tune kubelet safely
```

**With Mirror Types (main branch)**:
```
✅ POSSIBLE - HyperFleet owns the type definition

HyperFleet creates mirror type in configuration.go:
type KubeletConfig struct {
    // ✅ Expose this - safe
    // +hyperfleet:write-mode=mutable
    PodPidsLimit *int64
    
    // ❌ Hide this - dangerous
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    EvictionHard map[string]string
}

→ Customer gets SAFE SUBSET (podPidsLimit only)
→ Platform blocks dangerous fields (evictionHard)
→ Customer gets safe, audited control
```

## Branch Status

This is a **demonstration branch** showing the v0.1.1 passthrough approach as a valid alternative to the mirror types pattern implemented on main.

**Not intended to merge** - exists to document the trade-offs and provide a working example of the simpler passthrough approach.

## Related Documentation

- `docs/passthrough-vs-mirror-comparison.md` - Side-by-side comparison
- `docs/api-management.md` - Original design document
- Main branch `docs/configuration-types-pattern.md` - Mirror types pattern (not on this branch)
