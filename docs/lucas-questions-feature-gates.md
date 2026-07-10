# Response to Lucas's Questions on Feature Gates and Validation Flow

**Date**: 2026-07-10  
**Context**: Discussion on ROSAENG-61569 (CRD Client-Side Validation)

---

## Terminology: Feature Gate vs Feature Set

Before diving into the questions, it's important to clarify these two related but distinct concepts:

### Feature Gate (Individual Feature)
A **feature gate** is a **single named capability/feature** that can be enabled or disabled.

**Examples:**
- `HyperFleetAutoScaling` - one feature gate
- `HyperFleetEtcdConfig` - another feature gate  
- `HyperFleetSecretEncryption` - another feature gate

**In code:**
```go
// Individual gates control specific fields
// +openshift:enable:FeatureGate=HyperFleetAutoScaling
Autoscaling *AutoScalingSpec `json:"autoscaling,omitempty"`
```

### Feature Set (Collection of Gates)
A **feature set** is a **grouping of multiple feature gates** based on stability tier. This is **not arbitrary** - it follows the [OpenShift API pattern](https://github.com/openshift/api/blob/master/features/features.go).

**The three feature sets:**
- `Default` = GA gates only (stable, production-ready features)
- `TechPreviewNoUpgrade` = Default gates + TechPreview gates (beta features, may have breaking changes)
- `DevPreviewNoUpgrade` = TechPreview gates + DevPreview gates (alpha features, experimental)

**Why "NoUpgrade"?** These feature sets may contain breaking changes that prevent seamless cluster upgrades. Once enabled, the cluster may need to be recreated to downgrade.

### The Relationship

Think of it like this:

```
Feature Set (Tier/Bundle)          Feature Gates (Individual Features)
─────────────────────────────      ────────────────────────────────────
Default                      →     [HyperFleetEtcdConfig]
                                   
TechPreviewNoUpgrade         →     [HyperFleetEtcdConfig,
                                    HyperFleetAutoScaling,
                                    HyperFleetSecretEncryption]
                                   
DevPreviewNoUpgrade          →     [HyperFleetEtcdConfig,
                                    HyperFleetAutoScaling,
                                    HyperFleetSecretEncryption,
                                    HyperFleetKarpenter]
```

**Key Difference:**
- **Feature Set** = Coarse-grained entitlement tier ("This customer is on TechPreview tier")
- **Feature Gate** = Fine-grained capability flag ("This specific feature is enabled")

### Why Two Layers?

1. **Simplicity**: Most customers just pick a tier (Default/TechPreview/DevPreview)
2. **Flexibility**: Individual gates can be enabled as exceptions (e.g., a Default customer gets beta access to one TechPreview feature)
3. **Promotion**: When a gate moves from TechPreview → GA, it automatically becomes available to all Default customers

---

## Question 1: Feature Gate Storage in DynamoDB

> "Then the featureGate flag/label/tag will be linked per account in Dynamo, right?"

**Answer**: Yes, but with nuance - there are **two layers** of feature gate determination:

### Layer 1: Feature Set Assignment (Per-Account)

Each customer account is assigned a **feature set** which determines their baseline entitlements:

- **Default**: GA features only
- **TechPreviewNoUpgrade**: Default + TechPreview gates
- **DevPreviewNoUpgrade**: TechPreview + DevPreview gates

This assignment would typically be stored in DynamoDB (or your account metadata store) as:

```json
{
  "accountId": "123456789",
  
  // FEATURE SET - which tier is this account on?
  "featureSet": "Default",  // Options: "Default", "TechPreviewNoUpgrade", "DevPreviewNoUpgrade"
  
  ...
}
```

The hierarchy is **inclusive**:
```
DevPreview ⊃ TechPreview ⊃ Default
```

### Layer 2: Per-Account Gate Entitlements (Optional)

Individual **feature gates** can be enabled for specific accounts **independent of their feature set**. This allows:

- Beta testing specific features with select customers
- Gradual rollout of TechPreview features to Default customers
- Account-specific exceptions

This would also be stored in DynamoDB, for example:

```json
{
  "accountId": "123456789",
  
  // FEATURE SET - customer tier (determines baseline gates)
  "featureSet": "Default",
  
  // FEATURE GATES - individual features enabled as exceptions
  "enabledGates": [
    "HyperFleetAutoScaling",      // GATE: Exception - allow this TechPreview feature
    "HyperFleetSecretEncryption"  // GATE: Exception - allow this TechPreview feature
  ]
}
```

### How the Platform API Resolves Effective Gates

When validating a request, the Platform API:

1. **Looks up the account** from DynamoDB (or equivalent)
2. **Resolves base gates** from feature set:
   ```
   Default        → ["HyperFleetEtcdConfig"]  // GA gates only
   TechPreview    → ["HyperFleetEtcdConfig", "HyperFleetAutoScaling", ...]
   DevPreview     → [all gates]
   ```
3. **Merges per-account entitlements**: `effectiveGates = baseGates ∪ enabledGates`
4. **Passes to validator**:
   ```go
   req := &validation.Request{
       Operation:    validation.OperationCreate,
       Fields:       extractFields(cluster),
       FeatureSet:   account.FeatureSet,        // From DynamoDB
       EnabledGates: effectiveGates,            // Resolved list
   }
   ```

### Visual Comparison: Feature Set vs Feature Gate

| Concept | What It Is | Example Values | Granularity | Purpose |
|---------|-----------|----------------|-------------|---------|
| **Feature Set** | Customer tier/bundle | `Default`<br>`TechPreviewNoUpgrade`<br>`DevPreviewNoUpgrade` | Coarse | "What tier is this account on?" |
| **Feature Gate** | Individual feature flag | `HyperFleetAutoScaling`<br>`HyperFleetEtcdConfig`<br>`HyperFleetKarpenter` | Fine | "Is this specific feature enabled?" |

**In DynamoDB:**
```json
{
  "featureSet": "Default",              // ONE value (the tier)
  "enabledGates": ["gate1", "gate2"]    // ZERO or MORE values (individual features)
}
```

### Example Storage Schema (DynamoDB)

**Table: Accounts**
```
PK: accountId (String)
SK: "METADATA" (String)
---
featureSet: "Default" | "TechPreviewNoUpgrade" | "DevPreviewNoUpgrade"  // FEATURE SET
enabledGates: ["gate1", "gate2", ...]  // FEATURE GATES (optional overrides)
createdAt: timestamp
updatedAt: timestamp
```

**Alternative: Separate Gates Table**
```
PK: accountId (String)
SK: "GATE#<gateName>" (String)
---
gateName: "HyperFleetAutoScaling"  // Individual FEATURE GATE
enabled: true
grantedAt: timestamp
expiresAt: timestamp (optional)
```

### Implementation Location

The DynamoDB lookup happens in the **Platform API** at request time:

```go
// Platform API handler
func (h *ClusterHandler) CreateCluster(ctx context.Context, req *CreateClusterRequest) (*Cluster, error) {
    // 1. Lookup account from DynamoDB
    account, err := h.accountStore.GetAccount(ctx, req.AccountID)
    if err != nil {
        return nil, err
    }
    
    // 2. Resolve effective feature gates
    effectiveGates := resolveFeatureGates(account.FeatureSet, account.EnabledGates)
    
    // 3. Validate using unified validator
    validationReq := &validation.Request{
        Operation:    validation.OperationCreate,
        Fields:       extractFields(req.Cluster),
        FeatureSet:   account.FeatureSet,
        EnabledGates: effectiveGates,
    }
    
    if err := h.validator.ValidateCreate(req.Cluster, validationReq); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 4. Proceed with cluster creation...
}
```

---

## Question 2: Where Does Validation Happen?

> "I was also thinking in the flow that would perform the validation can/can't write, it should be the PlatformAPI I guess initially."

**Answer**: Correct! Validation happens at **multiple layers** with different purposes:

### Validation Flow Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Customer Request                         │
│                   (REST API: POST /clusters)                 │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   Platform API (Go)                          │
│  1. Lookup account from DynamoDB                             │
│  2. Resolve feature gates (FeatureSet + EnabledGates)        │
│  3. Call UnifiedValidator (pkg/validation)                   │
│     • CRD schema validation (types, required, enums)         │
│     • Write-mode validation (mutable/immutable/service-set)  │
│     • Feature gate validation (gate enabled?)                │
│  4. Reject if validation fails → 400 Bad Request             │
└────────────────────────┬────────────────────────────────────┘
                         │ (if valid)
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Transform to HyperFleet CRD                     │
│  • Enrich with service-set fields (accountId, creatorARN)    │
│  • Strip fields customer cannot set                          │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│         Submit to Kubernetes API (CRD admission)             │
│  • Structural schema validation (CRD OpenAPI schema)         │
│  • Admission webhooks (if any)                               │
│  • CEL validation rules (if any)                             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                 HyperFleet Operator                          │
│  • Convert HyperFleet CRD → HyperShift HostedCluster         │
│  • Submit to HyperShift                                      │
└─────────────────────────────────────────────────────────────┘
```

### Layer-by-Layer Breakdown

#### ✅ Layer 1: Platform API (PRIMARY - This is where Lucas is thinking)

**Where**: Platform API HTTP handlers  
**When**: **Before** submitting to Kubernetes  
**What**: Write-mode + feature gate + schema validation  
**Why**: 
- Fast feedback to customers (no K8s round-trip)
- Feature gate enforcement based on account entitlements
- Prevents unauthorized field access

**Code Location**: Platform API service (likely a separate repo/service)

```go
// Example: Platform API handler
func (api *ClusterAPI) CreateCluster(w http.ResponseWriter, r *http.Request) {
    var req CreateClusterRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Get account context from auth token
    accountID := extractAccountID(r.Context())
    
    // Lookup account metadata from DynamoDB
    account, _ := api.accountStore.GetAccount(r.Context(), accountID)
    
    // VALIDATION HAPPENS HERE
    validationReq := &validation.Request{
        Operation:    validation.OperationCreate,
        Fields:       extractFields(req.Spec),
        FeatureSet:   account.FeatureSet,
        EnabledGates: resolveGates(account),
    }
    
    validator, _ := validation.NewUnifiedValidator()
    if err := validator.ValidateCreate(req.Spec, validationReq); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)  // 400
        return
    }
    
    // Transform and submit to K8s...
}
```

**What Gets Validated**:
- ✅ Schema compliance (string vs int, required fields, enum values)
- ✅ Write-mode rules (service-set fields rejected, immutable fields allowed on create)
- ✅ Feature gate access (does customer have gate X enabled?)

#### Layer 2: Kubernetes API Server (DEFENSIVE)

**Where**: K8s admission  
**When**: When Platform API submits the CRD  
**What**: CRD structural schema validation  
**Why**: 
- Defense-in-depth
- Catches bugs in Platform API transformation logic
- Enforces CRD invariants

**Not Needed for Feature Gates**: K8s doesn't know about account-level feature sets - that's Platform API's job.

#### Layer 3: Admission Webhooks (OPTIONAL - Future)

**Where**: K8s ValidatingWebhookConfiguration  
**When**: After Platform API submits, before persisting to etcd  
**What**: Additional validation logic (cross-field constraints, quotas, etc.)  
**Why**: 
- Complex validation beyond OpenAPI schema
- Multi-resource validation
- Can call external services

**Example Use Cases**:
- Quota enforcement: "Account can only have 10 clusters"
- Cross-field validation: "If etcd.managed=false, etcd.endpoints is required"
- Regional restrictions: "Account not authorized for region us-east-1"

---

## Summary

### Question 1: Feature Gate Storage
**Yes, feature gates are linked per account in DynamoDB** (or equivalent), with two layers:
1. **Feature Set** (Default/TechPreview/DevPreview) - determines baseline gates
2. **Enabled Gates** (optional per-account overrides) - allows exceptions

The Platform API reads this at request time and passes to the validator.

### Question 2: Validation Location
**Yes, the Platform API is the primary validation point**, specifically:

**Primary Enforcement**: Platform API
- Runs **before** K8s submission
- Uses `pkg/validation` (this POC codebase)
- Enforces write-mode, feature gates, and schema
- Reads account metadata from DynamoDB

**Defensive Layer**: K8s CRD admission
- Structural schema validation only
- No feature gate awareness
- Catches Platform API bugs

**Optional Future**: Admission webhooks
- Complex validation scenarios
- Cross-resource constraints
- Quota enforcement

---

## Code Organization

From this POC repo's perspective:

```
hyperfleet-api-codegen/              # This repo (POC)
├── pkg/validation/                  # Validation library
│   ├── validator.go                 # Write-mode + feature gate validator
│   ├── crdvalidator.go              # CRD schema validator (ROSAENG-61569)
│   └── unified.go                   # Combines both
└── pkg/registry/                    # Generated field metadata

platform-api/                        # Separate repo (Platform API service)
├── handlers/
│   └── cluster_handler.go           # HTTP handlers
├── store/
│   └── dynamodb.go                  # Account lookup
└── go.mod
    └── require github.com/cdoan1/hyperfleet-api-codegen v0.1.0  # Uses validation lib
```

The **Platform API imports this POC repo's `pkg/validation` package** and uses it at request time after looking up account metadata from DynamoDB.

---

## Open Design Questions for Discussion

1. **DynamoDB Schema**: Should feature gates be denormalized in the Accounts table, or use a separate Gates table for flexibility?

2. **Feature Set Migration**: How do we handle upgrading customers from Default → TechPreview? One-time batch update or self-service?

3. **Gate Expiration**: Should per-account gate entitlements have TTLs (e.g., "trial for 30 days")?

4. **Audit Trail**: Do we need to log feature gate lookups/denials for compliance?

5. **Cache Strategy**: Should the Platform API cache account metadata + gates to reduce DynamoDB load? (Redis? In-memory with TTL?)

6. **Gate Discovery**: Should the OpenAPI spec include gate metadata so customers know which fields require which gates?
