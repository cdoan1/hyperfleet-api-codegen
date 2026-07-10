# HyperFleet-Owned Configuration Types Pattern

**Created**: 2026-07-10  
**Status**: Implemented for Kubelet and Machine Config  
**Epic**: [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)

---

## Problem Statement

HyperShift's `ClusterConfiguration` is an imported type that we cannot add markers to. Marking the entire field as `service-set` prevents granular control:

```go
// Cannot add markers to nested fields!
// +hyperfleet:write-mode=service-set
Configuration *hypershiftv1beta1.ClusterConfiguration `json:"configuration,omitempty"`
```

This means we cannot:
- Expose `kubelet.podPidsLimit` as customer-mutable
- Hide `kubelet.evictionHard` as platform-managed
- Feature-gate `kubelet.serializeImagePulls` for TechPreview
- Whitelist safe kernel parameters while blocking systemd/file writes

---

## Solution: HyperFleet-Owned Mirror Types

Create **HyperFleet-owned types** that mirror HyperShift's structure, enabling per-field markers.

**Location**: `api/v1alpha1/configuration.go`

---

## Implementation

### Files Changed

| File | Change | Purpose |
|------|--------|---------|
| `api/v1alpha1/configuration.go` | NEW | HyperFleet-owned config types with granular markers |
| `api/v1alpha1/hostedclusterspec.passthrough.go` | Updated | Use `ClusterConfiguration` instead of `hypershiftv1beta1.ClusterConfiguration` |
| `pkg/featuregate/registry.go` | Added gates | `HyperFleetKubeletAdvanced`, `HyperFleetMachineConfig` |
| `pkg/registry/field_metadata.go` | Regenerated | 58 → 160 fields tracked |
| `openapi/openapi.json` | Regenerated | New visible fields, hidden fields excluded |
| `pkg/featuregate/featuregate_test.go` | Updated | Gate counts (Default=1, TechPreview=5, DevPreview=6) |

### Code Structure

```go
// api/v1alpha1/configuration.go

type ClusterConfiguration struct {
    Kubelet       *KubeletConfig      `json:"kubelet,omitempty"`
    MachineConfig *MachineConfigSpec  `json:"machineConfig,omitempty"`
    // ... other config areas (placeholders)
}

type KubeletConfig struct {
    // 25+ fields with granular markers
    // +hyperfleet:write-mode=mutable|immutable|service-set
    // +k8s:openapi-gen=false (for hidden fields)
    // +openshift:enable:FeatureGate=X (for gated fields)
}

type MachineConfigSpec struct {
    // Whitelist approach for safe kernel params
    // Block dangerous operations (systemd, files)
}
```

---

## Results

### Field Count Growth

| Phase | Fields Tracked | Change |
|-------|---------------|--------|
| Before | 58 | Baseline |
| + Kubelet | 146 | +88 fields |
| + Machine Config | 160 | +14 fields |

### Kubelet Configuration

**Visible Fields** (in OpenAPI schema):
```yaml
kubelet:
  maxPods: 250                          # Mutable
  podPidsLimit: 8192                    # Mutable
  systemReserved:                       # Immutable
    cpu: "500m"
    memory: "1Gi"
  imageGCHighThresholdPercent: 85       # Mutable
  containerLogMaxSize: "10Mi"           # Mutable
  serializeImagePulls: false            # Mutable + TechPreview gated
```

**Hidden Fields** (platform-managed):
- `evictionHard`, `evictionSoft`, `evictionSoftGracePeriod`
- `cpuManagerPolicy`, `cpuManagerPolicyOptions`
- `topologyManagerPolicy`, `topologyManagerScope`
- `allowedUnsafeSysctls`
- `memoryThrottlingFactor`

**Field Registry Example**:
```json
{
  "fieldPath": "spec.hostedCluster.configuration.kubelet.podPidsLimit",
  "writeMode": "mutable"
},
{
  "fieldPath": "spec.hostedCluster.configuration.kubelet.systemReserved",
  "writeMode": "immutable"
},
{
  "fieldPath": "spec.hostedCluster.configuration.kubelet.evictionHard",
  "writeMode": "service-set",
  "hidden": true
},
{
  "fieldPath": "spec.hostedCluster.configuration.kubelet.serializeImagePulls",
  "writeMode": "mutable",
  "featureGate": "HyperFleetKubeletAdvanced"
}
```

### Machine Config (Security-Focused)

**Visible Fields** (whitelist approach):
```yaml
machineConfig:
  allowedKernelArguments:               # Immutable + TechPreview gated
    - "vm.max_map_count=262144"
  fips: true                            # Immutable
```

**Hidden Fields** (platform-only for security):
- `kernelArguments` - platform sets from whitelist
- `systemdUnits` - dangerous, platform-only
- `files` - dangerous, platform-only
- `kernelType` - platform-managed
- `extensions` - platform-managed

**Security Model**:
1. Customer requests kernel params via `allowedKernelArguments` (whitelist)
2. Platform validates against approved list
3. Platform sets actual `kernelArguments` (hidden field)
4. Customer **never** sees or sets `systemdUnits` or `files`

**Field Registry Example**:
```json
{
  "fieldPath": "spec.hostedCluster.configuration.machineConfig.allowedKernelArguments",
  "writeMode": "immutable",
  "featureGate": "HyperFleetMachineConfig"
},
{
  "fieldPath": "spec.hostedCluster.configuration.machineConfig.systemdUnits",
  "writeMode": "service-set",
  "hidden": true
},
{
  "fieldPath": "spec.hostedCluster.configuration.machineConfig.files",
  "writeMode": "service-set",
  "hidden": true
}
```

---

## Validation Examples

### Kubelet: PID Limits (Mutable)

```bash
# Customer sets PID limit
POST /clusters
{
  "spec": {
    "hostedCluster": {
      "configuration": {
        "kubelet": {
          "podPidsLimit": 8192
        }
      }
    }
  }
}

Response: 201 Created ✅
```

### Kubelet: Eviction Thresholds (Hidden + Service-Set)

```bash
# Customer tries to set eviction thresholds
POST /clusters
{
  "spec": {
    "hostedCluster": {
      "configuration": {
        "kubelet": {
          "evictionHard": {"memory.available": "100Mi"}
        }
      }
    }
  }
}

Response: 400 Bad Request ❌
{
  "error": "field spec.hostedCluster.configuration.kubelet.evictionHard is platform-managed (service-set) and cannot be set by customers"
}
```

### Machine Config: Kernel Parameters (TechPreview Gated)

```bash
# TechPreview customer requests kernel params
POST /clusters
{
  "spec": {
    "hostedCluster": {
      "configuration": {
        "machineConfig": {
          "allowedKernelArguments": ["vm.max_map_count=262144"]
        }
      }
    }
  }
}

Response: 201 Created ✅ (TechPreview customer)
```

```bash
# Default customer tries (no gate)
POST /clusters
{
  "spec": {
    "hostedCluster": {
      "configuration": {
        "machineConfig": {
          "allowedKernelArguments": ["vm.max_map_count=262144"]
        }
      }
    }
  }
}

Response: 400 Bad Request ❌
{
  "error": "field spec.hostedCluster.configuration.machineConfig.allowedKernelArguments requires feature gate HyperFleetMachineConfig which is not enabled in Default feature set"
}
```

### Machine Config: Systemd Units (Blocked)

```bash
# Customer tries to set systemd units (dangerous)
POST /clusters
{
  "spec": {
    "hostedCluster": {
      "configuration": {
        "machineConfig": {
          "systemdUnits": [
            {
              "name": "custom.service",
              "contents": "..."
            }
          ]
        }
      }
    }
  }
}

Response: 400 Bad Request ❌
{
  "error": "field spec.hostedCluster.configuration.machineConfig.systemdUnits is platform-managed (service-set) and cannot be set by customers"
}
```

---

## Pattern for Adding New Configuration Types

### Step-by-Step Guide

#### 1. Define HyperFleet-Owned Type

Edit `api/v1alpha1/configuration.go`:

```go
// Example: Network configuration
type NetworkConfiguration struct {
    // Customer-visible, mutable
    // +hyperfleet:write-mode=mutable
    ClusterNetworkCIDR string `json:"clusterNetworkCIDR,omitempty"`
    
    // Customer-visible, immutable
    // +hyperfleet:write-mode=immutable
    ServiceNetworkCIDR string `json:"serviceNetworkCIDR,omitempty"`
    
    // Platform-managed, hidden
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    NetworkType string `json:"networkType,omitempty"`
    
    // Feature-gated (TechPreview)
    // +openshift:enable:FeatureGate=HyperFleetNetworkAdvanced
    // +hyperfleet:write-mode=mutable
    CustomDNS *DNSConfig `json:"customDNS,omitempty"`
}
```

#### 2. Add to ClusterConfiguration

```go
type ClusterConfiguration struct {
    // ... existing fields
    
    // Add new config area
    Network *NetworkConfiguration `json:"network,omitempty"`
}
```

#### 3. Add Feature Gates (if needed)

Edit `pkg/featuregate/registry.go`:

```go
var HyperFleetFeatureGates = map[string]FeatureGateInfo{
    // ... existing gates
    
    "HyperFleetNetworkAdvanced": {
        Stage:       TechPreview,
        Description: "Enables advanced network configuration",
    },
}
```

#### 4. Regenerate Artifacts

```bash
# Verify types compile
go build ./api/v1alpha1

# Regenerate field registry
make generate-registry

# Regenerate OpenAPI spec
make generate-openapi

# Run tests
make test
```

#### 5. Update Tests (if gate count changed)

Edit `pkg/featuregate/featuregate_test.go`:

```go
// Update expected gate counts
{"TechPreview has X gates", TechPreviewNoUpgrade, X},
{"DevPreview has Y gates", DevPreviewNoUpgrade, Y},
```

#### 6. Verify Results

```bash
# Check field count increased
jq '. | length' pkg/registry/field_metadata.json

# Check OpenAPI includes visible fields
jq '.definitions.NetworkConfiguration.properties | keys' openapi/openapi.json

# Check hidden fields excluded (should return null)
jq '.definitions.NetworkConfiguration.properties.networkType' openapi/openapi.json

# Verify in Swagger UI
make swagger-ui-serve
# Open http://localhost:8080
```

---

## Configuration Areas Ready for Expansion

Currently **placeholder types** in `configuration.go`:

| Type | Purpose | Priority |
|------|---------|----------|
| `APIServerNetworkConfiguration` | API server network settings | Medium |
| `ClusterAuthentication` | Cluster authentication config | Medium |
| `FeatureGateConfiguration` | Feature gate settings | Low |
| `ImageConfiguration` | Internal registry config | Medium |
| `IngressConfiguration` | Ingress config | High |
| `NetworkConfiguration` | Cluster networking | High |
| `OAuthConfiguration` | OAuth settings | Medium |
| `SchedulerConfiguration` | Scheduler config | Low |
| `ProxyConfiguration` | Cluster-wide proxy | Medium |

Each follows the same 6-step pattern above.

---

## Code Template

```go
// Template for adding a new configuration type
type XConfiguration struct {
    // Marker patterns:
    
    // 1. Customer-visible, mutable field
    // +hyperfleet:write-mode=mutable
    CustomerField string `json:"customerField,omitempty"`
    
    // 2. Customer-visible, immutable field (set once on create)
    // +hyperfleet:write-mode=immutable
    ImmutableField string `json:"immutableField,omitempty"`
    
    // 3. Platform-managed, hidden field
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    PlatformField string `json:"platformField,omitempty"`
    
    // 4. Feature-gated field (TechPreview/DevPreview)
    // +openshift:enable:FeatureGate=HyperFleetXAdvanced
    // +hyperfleet:write-mode=mutable
    AdvancedField string `json:"advancedField,omitempty"`
    
    // 5. Feature-gated + immutable
    // +openshift:enable:FeatureGate=HyperFleetXBeta
    // +hyperfleet:write-mode=immutable
    BetaField string `json:"betaField,omitempty"`
}
```

---

## CI Verification

**Convention**: Hand-written configuration types require markers, enforced by CI.

### Why Hand-Written (Not Generated)?

Unlike `HostedClusterSpecPassthrough` and `NodePoolSpecPassthrough` (generated from upstream HyperShift), configuration types are **HyperFleet-owned** and hand-written:

1. **HyperShift doesn't have these types** - kubelet/machine config are HyperFleet-specific additions
2. **Granular control needed** - Each field requires deliberate marker decisions
3. **Security review** - Each exposed field must be reviewed for safety

### Verification Tool

`cmd/verify-configuration` enforces that all exported fields in configuration types have `+hyperfleet:write-mode` markers.

**Runs automatically**:
- `make verify` (local)
- CI on every commit
- Pre-push git hook (optional)

**How it works**:
1. Parses `api/v1alpha1/configuration.go` AST
2. Finds structs ending in `Configuration`, `Config`, or `Spec`
3. Checks each exported field has `+hyperfleet:write-mode` marker
4. Exits with error listing missing markers

**Excluded**: Support types (`SystemdUnit`, `SystemdDropin`, `FileSpec`) that are never exposed directly.

**Example error**:
```
Configuration verification failed:
  ❌ KubeletConfig.MaxPods: missing +hyperfleet:write-mode marker
  ❌ MachineConfigSpec.FIPS: missing +hyperfleet:write-mode marker

All fields in configuration types must have +hyperfleet:write-mode markers.
Add one of: mutable, immutable, service-set
```

---

## Best Practices

### Security-First

- **Default deny**: New fields should be `hidden + service-set` by default
- **Explicit allow**: Only expose fields after security review
- **Dangerous operations**: systemd, file writes, arbitrary commands = always hidden

### Field Naming

- Use HyperShift field names when mirroring (maintain consistency)
- Add HyperFleet-specific fields with clear naming (e.g., `allowedKernelArguments`)

### Write-Mode Selection

| Use Case | Write-Mode | Example |
|----------|-----------|---------|
| Customer can change anytime | `mutable` | `podPidsLimit`, `maxPods` |
| Customer sets once on create | `immutable` | `systemReserved`, `fips` |
| Platform manages completely | `service-set` | `evictionHard`, `systemdUnits` |

### Feature Gates

- Use gates for **beta/alpha features**, not for **stable fields**
- Gate names: `HyperFleet<Area><Feature>` (e.g., `HyperFleetKubeletAdvanced`)
- Always add description in feature gate registry

### Documentation

- Update `docs/team-presentation.md` with examples
- Add validation examples to this document
- Document security rationale for hidden fields

---

## Testing Checklist

When adding a new configuration type:

- [ ] Types compile: `go build ./api/v1alpha1`
- [ ] Registry regenerated: `make generate-registry`
- [ ] OpenAPI regenerated: `make generate-openapi`
- [ ] Field count increased in registry
- [ ] Visible fields appear in OpenAPI schema
- [ ] Hidden fields excluded from OpenAPI schema
- [ ] Tests updated (gate counts if needed)
- [ ] All tests pass: `make test`
- [ ] Swagger UI shows correct fields (clear browser cache!)

---

## Commits

- **9823619** - Add granular kubelet config control with HyperFleet-owned types
- **b154a28** - Add granular machine config control with security-focused markers

---

## References

- **Design Document**: [docs/api-management.md](./api-management.md)
- **Team Presentation**: [docs/team-presentation.md](./team-presentation.md)
- **Code**: `api/v1alpha1/configuration.go`
- **Epic**: [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)
