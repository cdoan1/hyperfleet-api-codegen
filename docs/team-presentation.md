# HyperFleet API Management POC
## Controlling What We Expose and What Customers Can Set

**Presenter**: Chris Doan  
**Date**: 2026-07-10  
**Epic**: [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)  
**Repo**: [github.com/cdoan1/hyperfleet-api-codegen](https://github.com/cdoan1/hyperfleet-api-codegen)

---

## The Problem We Solved

### Before: No Control Over Passthrough Fields

HyperShift exposes **hundreds of fields** in `HostedCluster` and `NodePool`:
- Kubelet config (PID limits, reserved resources, eviction thresholds)
- Machine config (kernel parameters, systemd units, file writes)
- Network config (CNI settings, proxy configuration)
- Security config (FIPS mode, audit policies, secrets encryption)

**Questions we couldn't answer**:
- ❌ Which fields should customers see?
- ❌ Which fields can customers modify?
- ❌ Which fields are platform-managed?
- ❌ Which fields require beta/alpha feature gates?
- ❌ How do we validate customer input before submitting to K8s?

### The Vision: "Realization of the Intent to Passthrough"

We want to **selectively expose** upstream HyperShift fields while maintaining **granular control** over:
1. **Visibility** - Does the customer see this field in the API?
2. **Mutability** - Can the customer set or change this field?
3. **Entitlement** - Does the customer's tier allow access to this field?

---

## What We Built: Three Control Markers

A single source of truth in **Go types with markers** that declaratively control behavior:

### 1. Visibility Marker: `+k8s:openapi-gen=false`

**Controls**: Whether a field appears in the customer-facing API

```go
type ClusterSpec struct {
    // ✅ VISIBLE - Customers see this in API docs
    DisplayName string `json:"displayName,omitempty"`
    
    // ❌ HIDDEN - Platform-internal, never shown to customers
    // +k8s:openapi-gen=false
    AccountID string `json:"accountId"`
    
    // ❌ HIDDEN - AWS ARN of who created cluster
    // +k8s:openapi-gen=false
    CreatorARN string `json:"creatorARN,omitempty"`
}
```

**Result**:
- OpenAPI spec only includes visible fields
- Customers never see `accountId` or `creatorARN` in documentation
- Platform fills these in automatically based on authenticated context

---

### 2. Write-Mode Marker: `+hyperfleet:write-mode=X`

**Controls**: Whether customers can set or change a field

```go
type ClusterSpec struct {
    // IMMUTABLE - Customer sets on create, cannot change later
    // +hyperfleet:write-mode=immutable
    Name string `json:"name"`
    
    // MUTABLE - Customer can change anytime
    // +hyperfleet:write-mode=mutable
    DeleteProtection *DeleteProtection `json:"deleteProtection,omitempty"`
    
    // SERVICE-SET - Platform sets this, customer cannot touch it
    // +hyperfleet:write-mode=service-set
    Region string `json:"region"`
}
```

**Write-Mode Table**:

| Mode | Create (POST) | Update (PUT/PATCH) | Example |
|------|---------------|-------------------|---------|
| **mutable** | ✅ Customer sets | ✅ Customer changes | `deleteProtection` |
| **immutable** | ✅ Customer sets | ❌ Rejected if changed | `name` |
| **service-set** | ❌ Rejected if present | ❌ Rejected if present | `region`, `accountId` |

**Validation happens at runtime** using generated field metadata registry.

---

### 3. Feature Gate Marker: `+openshift:enable:FeatureGate=X`

**Controls**: Per-customer field entitlements based on tier

```go
type HostedClusterPassthrough struct {
    // ✅ GA - All customers get this
    Release Release `json:"release"`
    
    // 🔒 TECH PREVIEW - Only customers on TechPreview tier
    // +openshift:enable:FeatureGate=HyperFleetAutoScaling
    // +hyperfleet:write-mode=mutable
    AutoScaling *AutoScalingConfig `json:"autoScaling,omitempty"`
    
    // 🔒 DEV PREVIEW - Only customers on DevPreview tier
    // +openshift:enable:FeatureGate=HyperFleetKarpenter
    // +hyperfleet:write-mode=immutable
    Karpenter *KarpenterConfig `json:"karpenter,omitempty"`
}
```

**Feature Hierarchy** (inclusive):
```
DevPreview ⊃ TechPreview ⊃ Default (GA)
```

**Result**:
- Default customers: 32 fields
- TechPreview customers: 35 fields (+3 gated features)
- DevPreview customers: 35 fields (+3 experimental features)

---

## Real Example: Kubelet Config Control

### Upstream HyperShift Exposes Everything

```yaml
# HyperShift HostedCluster exposes raw kubelet config
spec:
  configuration:
    kubelet:
      maxPods: 250
      podPidsLimit: 4096              # PID limit per pod
      systemReserved:
        cpu: "500m"
        memory: "1Gi"
      evictionHard:
        memory.available: "100Mi"
      imageGCHighThresholdPercent: 85
      serializeImagePulls: false
      # ... 50+ more fields
```

### HyperFleet: Selective Exposure with Control

```go
type KubeletConfig struct {
    // ✅ VISIBLE + MUTABLE - Customer can set
    // +hyperfleet:write-mode=mutable
    MaxPods *int32 `json:"maxPods,omitempty"`
    
    // ✅ VISIBLE + MUTABLE - Customer can set PID limits
    // +hyperfleet:write-mode=mutable
    PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`
    
    // ✅ VISIBLE + IMMUTABLE - Customer sets once, cannot change
    // +hyperfleet:write-mode=immutable
    SystemReserved map[string]string `json:"systemReserved,omitempty"`
    
    // ❌ HIDDEN + SERVICE-SET - Platform controls this for safety
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    EvictionHard map[string]string `json:"evictionHard,omitempty"`
    
    // 🔒 TECH PREVIEW - Only beta customers get this
    // +openshift:enable:FeatureGate=HyperFleetKubeletAdvanced
    // +hyperfleet:write-mode=mutable
    SerializeImagePulls *bool `json:"serializeImagePulls,omitempty"`
}
```

**Customer Experience**:

```bash
# Default customer tries to set evictionHard (hidden field)
POST /clusters
{
  "spec": {
    "kubelet": {
      "evictionHard": {"memory.available": "50Mi"}  # ❌ Rejected
    }
  }
}

Response: 400 Bad Request
{
  "error": "field spec.kubelet.evictionHard is platform-managed (service-set) and cannot be set by customers"
}
```

```bash
# Default customer tries to use TechPreview feature
POST /clusters
{
  "spec": {
    "kubelet": {
      "serializeImagePulls": false  # ❌ Rejected - requires gate
    }
  }
}

Response: 400 Bad Request
{
  "error": "field spec.kubelet.serializeImagePulls requires feature gate HyperFleetKubeletAdvanced which is not enabled in Default feature set"
}
```

---

## Real Example: Machine Config Control

### The Challenge: Kernel Parameters & Systemd Units

HyperShift allows arbitrary machine config (kernel params, systemd units, file writes):

```yaml
# Upstream HyperShift - unrestricted access
spec:
  configuration:
    machineConfig:
      kernelArguments:
        - "vm.max_map_count=262144"     # Elasticsearch tuning
        - "net.ipv4.ip_forward=1"        # Routing
      storage:
        files:
          - path: "/etc/custom.conf"
            contents: "..."
      systemd:
        units:
          - name: "custom.service"
            contents: "..."
```

**Problem**: Unrestricted machine config is a **security and stability risk**.

### HyperFleet: Allow Safe Subset, Block Dangerous Configs

```go
type MachineConfigSpec struct {
    // ✅ TECH PREVIEW - Allowed kernel params (whitelist)
    // +openshift:enable:FeatureGate=HyperFleetMachineConfig
    // +hyperfleet:write-mode=immutable
    AllowedKernelArguments []string `json:"allowedKernelArguments,omitempty"`
    
    // ❌ HIDDEN - Platform manages systemd units, not customers
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    SystemdUnits []SystemdUnit `json:"systemdUnits,omitempty"`
    
    // ❌ HIDDEN - File writes are dangerous, platform-only
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    Files []File `json:"files,omitempty"`
}
```

**Result**:
- Customers can request **specific kernel params** (from allowlist) via TechPreview gate
- Platform controls systemd units and file writes (hidden, service-set)
- Dangerous configs never exposed to customer API

---

## Architecture: Single Source of Truth

```
                    ┌─────────────────────────────┐
                    │   Go Types with Markers     │
                    │   (api/v1alpha1/*.go)       │
                    │                             │
                    │  +k8s:openapi-gen=false     │
                    │  +hyperfleet:write-mode=X   │
                    │  +openshift:enable:Gate=Y   │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │                             │
                    ▼                             ▼
    ┌───────────────────────────┐   ┌───────────────────────────┐
    │   Marker Scanner          │   │  OpenAPI Generator        │
    │   (pkg/markers)           │   │  (kube-openapi)           │
    │                           │   │                           │
    │  Extracts markers         │   │  Respects visibility      │
    │  Generates registry       │   │  Generates spec           │
    └───────────┬───────────────┘   └───────────┬───────────────┘
                │                               │
                ▼                               ▼
    ┌───────────────────────────┐   ┌───────────────────────────┐
    │  Field Metadata Registry  │   │    OpenAPI Spec           │
    │  (pkg/registry)           │   │    (swagger.json)         │
    │                           │   │                           │
    │  fieldPath → metadata     │   │  Only visible fields      │
    │  - writeMode              │   │  Customer-facing API      │
    │  - featureGate            │   │                           │
    └───────────┬───────────────┘   └───────────────────────────┘
                │
                ▼
    ┌───────────────────────────┐
    │   Runtime Validator       │
    │   (pkg/validation)        │
    │                           │
    │  • Write-mode enforcement │
    │  • Feature gate checks    │
    │  • Generic, no field code │
    └───────────────────────────┘
```

**Key Insight**: Change a marker in Go → regenerate → everything downstream updates automatically.

---

## What We Built (POC Status)

### ✅ Completed Components

1. **Passthrough Generator** (ROSAENG-61384)
   - Reads upstream HyperShift types from go.mod
   - Generates mirrored Go structs with safe defaults
   - New fields: `+k8s:openapi-gen=false` + `+hyperfleet:write-mode=service-set`
   - Preserves existing markers on regeneration

2. **Marker Scanner & Field Registry** (ROSAENG-61389)
   - Extracts markers from Go types
   - Generates `pkg/registry/field_registry.go`
   - **58 fields tracked** with write-mode and feature gates

3. **OpenAPI Integration** (ROSAENG-61387)
   - Generates OpenAPI spec from Go types
   - Respects visibility markers (`+k8s:openapi-gen=false`)
   - Proper `$ref` expansion for nested types

4. **Feature Gate Tooling**
   - Registry of 4 example gates (1 GA, 2 TechPreview, 1 DevPreview)
   - CRD variant generator (3 variants: Default/TechPreview/DevPreview)
   - Per-feature-set field counts: Default (32), TechPreview (35), DevPreview (35)

5. **Runtime Validation**
   - Generic validator using field registry
   - Enforces write-mode rules (mutable/immutable/service-set)
   - Enforces feature gate access
   - **No field-specific code** - scales to hundreds of fields

6. **Swagger UI**
   - Interactive API documentation
   - Shows only visible fields
   - Serves generated OpenAPI spec

### 📊 By the Numbers

- **58 fields** tracked with metadata
- **4 feature gates** defined
- **3 CRD variants** generated per feature set
- **0 lines** of field-specific validation code (all generic!)
- **100%** CI coverage on marker verification

---

## How It Works: Validation Flow

### Example: Customer Creates Cluster with Invalid Fields

```bash
POST /api/v1/clusters
{
  "spec": {
    "name": "prod-cluster",              # ✅ Immutable, allowed on create
    "region": "us-west-2",               # ❌ Service-set, customer cannot set
    "kubelet": {
      "podPidsLimit": 8192,              # ✅ Mutable, allowed
      "evictionHard": {...}              # ❌ Hidden + service-set, rejected
    },
    "autoScaling": {...}                 # ❌ Requires TechPreview gate
  }
}
```

### Validation Process (Platform API)

```
┌──────────────────────────────────────────────────┐
│  1. Lookup Account from DynamoDB                 │
│     accountId: "123456"                          │
│     featureSet: "Default"                        │
│     enabledGates: []                             │
└────────────────────┬─────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────┐
│  2. Runtime Validator (pkg/validation)           │
│                                                  │
│  Field: "region"                                 │
│    ❌ Write-mode: service-set                    │
│    → Rejected: "platform-managed field"         │
│                                                  │
│  Field: "kubelet.evictionHard"                   │
│    ❌ Hidden + service-set                       │
│    → Rejected: "platform-managed field"         │
│                                                  │
│  Field: "autoScaling"                            │
│    ❌ Feature gate: HyperFleetAutoScaling        │
│    → Rejected: "requires TechPreview gate"      │
└────────────────────┬─────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────┐
│  3. Return 400 Bad Request                       │
│                                                  │
│  {                                               │
│    "errors": [                                   │
│      "field region is service-set",              │
│      "field kubelet.evictionHard is service-set",│
│      "field autoScaling requires gate..."        │
│    ]                                             │
│  }                                               │
└──────────────────────────────────────────────────┘
```

**Key Point**: Platform API enriches with service-set fields **after validation**:

```go
// After validation passes:
crd.Spec.AccountID = lookupFromAuth(ctx)      // Service-set
crd.Spec.CreatorARN = getCallerARN(ctx)       // Service-set
crd.Spec.Region = getRegionalEndpoint(ctx)    // Service-set
```

---

## Key Benefits

### 1. Declarative Control (Markers in Go)

**Before**:
```go
// Hard-coded validation logic per field
if req.Region != "" {
    return errors.New("region cannot be set by customer")
}
if req.AutoScaling != nil && !customer.HasTechPreview() {
    return errors.New("autoScaling requires TechPreview")
}
// ... repeat for 58+ fields
```

**After**:
```go
// Marker on type
// +hyperfleet:write-mode=service-set
Region string `json:"region"`

// +openshift:enable:FeatureGate=HyperFleetAutoScaling
AutoScaling *AutoScalingConfig `json:"autoScaling,omitempty"`

// Generic validator reads markers, no field-specific code!
```

**Result**: Add 100 new fields → 0 lines of validation code.

---

### 2. Production Workflow: Field Curation

**When HyperShift adds a new field** (e.g., `kubelet.podMaxPidsLimit`):

```bash
# 1. Bump HyperShift version
go get github.com/openshift/hypershift/api@v0.1.71

# 2. Regenerate passthrough types (gets safe defaults)
make generate-passthrough

# Output: New field appears with safe defaults
# +k8s:openapi-gen=false          # Hidden from customers
# +hyperfleet:write-mode=service-set  # Platform-controlled
PodMaxPidsLimit *int64 `json:"podMaxPidsLimit,omitempty"`

# 3. Developer reviews and updates markers
# Decision: "Let customers set this, make it mutable"
# +hyperfleet:write-mode=mutable
PodMaxPidsLimit *int64 `json:"podMaxPidsLimit,omitempty"`

# 4. Regenerate everything
make manifests openapi generate-registry

# 5. Commit
git add api/v1alpha1/hostedclusterspec.passthrough.go
git add pkg/registry/field_registry.go
git add openapi/swagger.json
git commit -m "Expose podMaxPidsLimit as mutable field"
```

**Result**: Field now visible in API, customers can set it, validation enforces mutability.

---

### 3. Feature Gate Promotion (One-Line Change)

**Scenario**: `HyperFleetAutoScaling` graduates from TechPreview → GA

```go
// pkg/featuregate/registry.go

var HyperFleetFeatureGates = map[string]FeatureGateInfo{
    "HyperFleetEtcdConfig":   {Stage: GA},
    "HyperFleetAutoScaling":  {Stage: GA},  // Changed from TechPreview
    "HyperFleetKarpenter":    {Stage: TechPreview},
}
```

```bash
make featuregate-info  # Verify change
make manifests         # Regenerate CRDs
```

**Result**: `autoScaling` field now in **Default CRD variant**, all Default customers get access.

---

### 4. No Breaking Changes on Upstream Bumps

**Safe Defaults Prevent Accidents**:

```
HyperShift v0.1.70 → v0.1.71
  + kubelet.podMaxPidsLimit (NEW)
  + machineConfig.kernelType (NEW)
  - kubelet.cgroupDriver (REMOVED)
```

```bash
make generate-passthrough

# New fields get safe defaults (hidden + service-set)
# Removed fields disappear from passthrough types
# CI fails if developer forgets to add markers

# Developer must explicitly review and mark new fields before merge
```

**Result**: No accidental exposure of dangerous fields.

---

## Real-World Use Cases

### Use Case 1: PID Limits for High-Density Workloads

**Customer Need**: Run hundreds of pods per node, default PID limit too low

**Solution**:
```go
// Expose podPidsLimit as mutable
// +hyperfleet:write-mode=mutable
PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`
```

**Customer sets it**:
```yaml
spec:
  kubelet:
    podPidsLimit: 8192  # ✅ Allowed
```

**Platform validates**:
- ✅ Field is visible
- ✅ Customer has permission to set it (mutable)
- ✅ Passes through to HyperShift

---

### Use Case 2: Reserved Resources for System Pods

**Customer Need**: Reserve CPU/memory for kubelet and system daemons

**Solution**:
```go
// Expose systemReserved as immutable (set once on cluster creation)
// +hyperfleet:write-mode=immutable
SystemReserved map[string]string `json:"systemReserved,omitempty"`
```

**Customer sets on create**:
```yaml
spec:
  kubelet:
    systemReserved:
      cpu: "1000m"
      memory: "2Gi"
```

**On update attempt**:
```bash
PATCH /clusters/prod-cluster
{
  "spec": {
    "kubelet": {
      "systemReserved": {"cpu": "500m"}  # ❌ Rejected - immutable
    }
  }
}

Response: 400 Bad Request
{
  "error": "field spec.kubelet.systemReserved is immutable and cannot be changed after creation"
}
```

---

### Use Case 3: Beta Feature Access (Image Registry Mirror)

**Customer Need**: TechPreview customer wants to test image registry mirroring

**Solution**:
```go
// Feature-gated field
// +openshift:enable:FeatureGate=HyperFleetRegistryMirror
// +hyperfleet:write-mode=mutable
ImageContentSources []ImageContentSource `json:"imageContentSources,omitempty"`
```

**Default customer** (accountId: 123456, featureSet: "Default"):
```bash
POST /clusters
{
  "spec": {
    "imageContentSources": [...]  # ❌ Rejected - requires gate
  }
}

Response: 400 Bad Request
{
  "error": "field spec.imageContentSources requires feature gate HyperFleetRegistryMirror which is not enabled in Default feature set"
}
```

**TechPreview customer** (accountId: 789012, featureSet: "TechPreviewNoUpgrade"):
```bash
POST /clusters
{
  "spec": {
    "imageContentSources": [...]  # ✅ Allowed - has TechPreview gate
  }
}

Response: 201 Created
```

---

## What's Next: Conversion Functions

### Current Gap: Platform API → K8s CRD

```
Customer REST Request → Platform API → ??? → K8s CRD → HyperShift
```

**Missing piece**: Type conversion functions

### Next Sprint: Implement Conversions

**1. REST → CRD** (Enrich with service-set fields):

```go
// pkg/conversion/cluster.go

func UnprojectCluster(restCluster map[string]interface{}, ctx ServiceContext) (*v1alpha1.Cluster, error) {
    crd := &v1alpha1.Cluster{}
    
    // Copy customer-provided fields
    crd.Spec.Name = restCluster["name"]
    crd.Spec.Kubelet = restCluster["kubelet"]
    
    // Enrich with service-set fields (platform provides)
    crd.Spec.AccountID = ctx.AccountID       // From auth token
    crd.Spec.CreatorARN = ctx.CreatorARN     // From AWS STS
    crd.Spec.Region = ctx.Region             // From endpoint
    
    return crd, nil
}
```

**2. CRD → REST** (Strip hidden/service-set fields):

```go
func ProjectCluster(crd *v1alpha1.Cluster) (map[string]interface{}, error) {
    result := make(map[string]interface{})
    
    // Copy visible fields only
    result["name"] = crd.Spec.Name
    result["kubelet"] = crd.Spec.Kubelet
    
    // Strip hidden fields (accountID, creatorARN never returned)
    // Strip service-set fields (region never returned)
    
    return result, nil
}
```

**Timeline**: 1-2 weeks, unblocks Platform API implementation

---

## Demo: Live Validation

### Setup

```bash
# Clone POC repo
git clone https://github.com/cdoan1/hyperfleet-api-codegen
cd hyperfleet-api-codegen

# Run validator example
go run pkg/validation/example_test.go
```

### Example Output

```
✅ PASS: Mutable field on create
   Field: spec.displayName
   Value: "test-cluster"
   Write-mode: mutable
   
❌ FAIL: Service-set field rejected
   Field: spec.accountId
   Value: "123456"
   Write-mode: service-set
   Error: field is platform-managed (service-set) and cannot be set by customers

❌ FAIL: Feature gate not enabled
   Field: spec.autoScaling
   Feature gate: HyperFleetAutoScaling
   Customer tier: Default
   Error: requires feature gate HyperFleetAutoScaling which is not enabled in Default feature set

✅ PASS: Immutable field on create
   Field: spec.name
   Value: "prod-cluster"
   Write-mode: immutable

❌ FAIL: Immutable field changed on update
   Field: spec.name
   Old: "prod-cluster"
   New: "staging-cluster"
   Write-mode: immutable
   Error: field is immutable and cannot be changed after creation
```

---

## Key Takeaways

### 1. We Control What We Expose
- **Markers in Go types** = single source of truth
- Hidden fields never appear in customer API
- Safe defaults on new upstream fields prevent accidents

### 2. We Control What Customers Can Set
- **Write-mode markers** enforce mutability rules
- Service-set fields (region, accountId) are platform-managed
- Immutable fields prevent dangerous post-creation changes

### 3. We Control Customer Entitlements
- **Feature gates** enable per-tier access
- Default customers: GA features only
- TechPreview/DevPreview: Beta/alpha features with opt-in

### 4. Validation is Generic and Scalable
- **No field-specific code** - all validation uses field registry
- Add 100 new fields → 0 lines of validation logic
- Scales to hundreds of fields without maintenance burden

### 5. Production Workflow is Proven
- Bump HyperShift → regenerate → review → mark → commit
- Field curation is explicit and auditable
- CI prevents unmarked fields from merging

---

## Questions & Discussion

### Open for Questions

1. **Marker semantics**: Should we add more control levels?
2. **Feature gate migration**: How do customers upgrade from Default → TechPreview?
3. **Conversion strategy**: Hand-written vs auto-generated?
4. **Platform API timeline**: When can we integrate?
5. **DynamoDB schema**: How should we store feature sets + enabled gates?

### Resources

- **Epic**: [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)
- **Design Doc**: [docs/api-management.md](./api-management.md)
- **POC Repo**: [github.com/cdoan1/hyperfleet-api-codegen](https://github.com/cdoan1/hyperfleet-api-codegen)
- **Feature Gate Q&A**: [docs/lucas-questions-feature-gates.md](./lucas-questions-feature-gates.md)
- **CRD Validation Design**: [docs/crd-client-validation.md](./crd-client-validation.md)

---

## Appendix: Field Registry Example

```go
// pkg/registry/field_registry.go (generated)

var FieldRegistry = map[string]FieldMeta{
    "spec.name": {
        WriteMode:   Immutable,
        FeatureGate: "",
    },
    "spec.displayName": {
        WriteMode:   Mutable,
        FeatureGate: "",
    },
    "spec.accountId": {
        WriteMode:   ServiceSet,
        FeatureGate: "",
    },
    "spec.region": {
        WriteMode:   ServiceSet,
        FeatureGate: "",
    },
    "spec.kubelet.podPidsLimit": {
        WriteMode:   Mutable,
        FeatureGate: "",
    },
    "spec.kubelet.systemReserved": {
        WriteMode:   Immutable,
        FeatureGate: "",
    },
    "spec.autoScaling": {
        WriteMode:   Mutable,
        FeatureGate: "HyperFleetAutoScaling",
    },
    // ... 51 more fields
}
```

---

## Appendix: CRD Variant Comparison

### Default CRD (32 fields)
```yaml
spec:
  properties:
    name: {type: string}
    displayName: {type: string}
    deleteProtection: {type: object}
    kubelet:
      properties:
        maxPods: {type: integer}
        podPidsLimit: {type: integer}
        systemReserved: {type: object}
    # NO autoScaling (requires gate)
    # NO karpenter (requires gate)
```

### TechPreview CRD (35 fields)
```yaml
spec:
  properties:
    # ... same as Default
    autoScaling:  # ✅ Added (gate: HyperFleetAutoScaling)
      properties:
        enabled: {type: boolean}
        minReplicas: {type: integer}
    # Still NO karpenter (requires DevPreview)
```

### DevPreview CRD (35 fields)
```yaml
spec:
  properties:
    # ... same as TechPreview
    karpenter:  # ✅ Added (gate: HyperFleetKarpenter)
      properties:
        enabled: {type: boolean}
        provisioner: {type: string}
```

---

**Thank you!**

Questions? Reach out:
- Slack: #hyperfleet-api
- Jira: [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)
- GitHub: [cdoan1/hyperfleet-api-codegen](https://github.com/cdoan1/hyperfleet-api-codegen)
