# CRD Client-Side Validation Design

**JIRA**: [ROSAENG-61569](https://redhat.atlassian.net/browse/ROSAENG-61569)  
**Status**: Design Proposal  
**Date**: 2026-07-09

## Overview

This document proposes implementing client-side validation for HyperFleet CRDs using the objectvalidation pattern from [openshift/cluster-capi-operator](https://github.com/openshift/cluster-capi-operator/tree/main/pkg/controllers/crdcompatibility/objectvalidation). This enables validating Custom Resources against CRD schemas without relying solely on server-side admission webhooks.

## Motivation

### Current State

The HyperFleet API currently has:
- ✅ Runtime validation in `pkg/validation/` that enforces write-mode and feature-gate rules using the field registry
- ✅ CRD variant generation that produces feature-set-specific CRD YAML files (`config/crd/variants/`)
- ✅ OpenAPI schema generation with marker-driven visibility control
- ❌ No client-side schema validation before resource submission

### Problems to Solve

1. **Missing Schema Validation**: The runtime validator (`pkg/validation/validator.go`) validates write-mode and feature-gate rules but doesn't validate against the CRD's OpenAPI schema (type constraints, required fields, enum values, etc.)

2. **Late Error Detection**: Schema validation errors are only discovered when the API server rejects the resource, requiring a full round-trip

3. **Feature Set Mismatch**: A customer on the `Default` feature set could submit resources containing fields only available in `TechPreview`, but this isn't caught until server-side validation

4. **Forward/Backward Compatibility**: No mechanism to validate resources against both current and future/past CRD schemas for upgrade/downgrade compatibility testing

## Goals

1. **Schema Validation**: Validate CRs against their full OpenAPI v3 schema (types, required fields, enums, min/max, patterns, etc.)
2. **Feature Set Enforcement**: Use the correct CRD variant based on customer feature set (Default/TechPreview/DevPreview)
3. **Client-Side Validation**: Catch validation errors before API submission
4. **Compatibility Testing**: Support validating against multiple schema versions for forward/backward compatibility
5. **Unified Validation**: Combine schema validation with existing write-mode/feature-gate validation

## Reference Implementation

The cluster-capi-operator's objectvalidation package provides the foundation:

### Key Components

```go
// From cluster-capi-operator/pkg/controllers/crdcompatibility/objectvalidation/

type Webhook struct {
    // Validates CRs against schemas embedded in CompatibilityRequirement CRs
    decoder       *admission.Decoder
    cache         map[string]*versionedStrategy
}

func (w *Webhook) Handle(ctx context.Context, req admission.Request) admission.Response {
    // Routes by operation: Create, Update, Delete
    // Decodes objects and calls Validate*()
}

func (w *Webhook) ValidateCreate/Update/Delete(obj unstructured.Unstructured) error {
    // Uses Kubernetes' native CRD validation machinery
    strategy := w.getOrCreateStrategy(requirementName, version)
    return strategy.Validate(ctx, obj)
}
```

### How It Works

1. **Schema Loading**: Extracts CRD YAML from a `CompatibilityRequirement` CR
2. **Strategy Creation**: Builds a `customresource.Strategy` using the CRD's OpenAPI schema
3. **Validation**: Leverages `apiextensionsvalidation.NewSchemaValidator` (same code the API server uses)
4. **Caching**: Caches validation strategies by CRD name/version

## Proposed Design

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Client Application                      │
└───────────────────────┬─────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│              pkg/validation/crdvalidator.go              │
│  • Loads CRD YAML from config/crd/variants/             │
│  • Selects variant based on FeatureSet                  │
│  • Uses k8s.io/apiextensions-apiserver validation       │
└───────────────────────┬─────────────────────────────────┘
                        │
        ┌───────────────┴───────────────┐
        ▼                               ▼
┌──────────────────┐          ┌──────────────────┐
│ Schema Validator │          │ Write-Mode       │
│ (OpenAPI v3)     │          │ Validator        │
│ • Types          │          │ (Field Registry) │
│ • Required       │          │ • Mutable        │
│ • Enums          │          │ • Immutable      │
│ • Patterns       │          │ • Service-Set    │
└──────────────────┘          └──────────────────┘
```

### File Structure

```
pkg/validation/
├── validator.go              # Existing write-mode/feature-gate validator
├── validator_test.go
├── crdvalidator.go           # NEW: CRD schema validator
├── crdvalidator_test.go      # NEW: Tests
├── unified.go                # NEW: Combines both validators
└── testdata/
    └── crds/                 # Embedded CRD YAML for testing
        ├── cluster_default.yaml
        ├── cluster_techpreview.yaml
        └── cluster_devpreview.yaml
```

### Implementation Details

#### 1. CRD Schema Validator

```go
// pkg/validation/crdvalidator.go

package validation

import (
    "embed"
    "fmt"
    
    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "sigs.k8s.io/yaml"
    
    "github.com/cdoan1/hyperfleet-api-codegen/pkg/featuregate"
)

//go:embed testdata/crds/*.yaml
var crdFS embed.FS

// CRDValidator validates objects against CRD OpenAPI schemas
type CRDValidator struct {
    // schemas maps feature set to loaded CRD schema
    schemas map[featuregate.FeatureSet]*apiextensionsv1.JSONSchemaProps
    
    // validators maps feature set to schema validator
    validators map[featuregate.FeatureSet]*validation.SchemaValidator
}

// NewCRDValidator creates a validator that loads CRDs from embedded YAML
func NewCRDValidator() (*CRDValidator, error) {
    v := &CRDValidator{
        schemas:    make(map[featuregate.FeatureSet]*apiextensionsv1.JSONSchemaProps),
        validators: make(map[featuregate.FeatureSet]*validation.SchemaValidator),
    }
    
    // Load CRD variants for each feature set
    for _, fs := range []featuregate.FeatureSet{
        featuregate.Default,
        featuregate.TechPreview,
        featuregate.DevPreview,
    } {
        if err := v.loadSchema(fs); err != nil {
            return nil, fmt.Errorf("failed to load schema for %s: %w", fs, err)
        }
    }
    
    return v, nil
}

// loadSchema loads and parses the CRD YAML for a feature set
func (v *CRDValidator) loadSchema(fs featuregate.FeatureSet) error {
    // Read embedded CRD YAML
    filename := fmt.Sprintf("testdata/crds/cluster_%s.yaml", fs)
    data, err := crdFS.ReadFile(filename)
    if err != nil {
        return fmt.Errorf("failed to read %s: %w", filename, err)
    }
    
    // Parse CRD
    var crd apiextensionsv1.CustomResourceDefinition
    if err := yaml.Unmarshal(data, &crd); err != nil {
        return fmt.Errorf("failed to unmarshal CRD: %w", err)
    }
    
    // Extract OpenAPI schema from the first version
    if len(crd.Spec.Versions) == 0 {
        return fmt.Errorf("CRD has no versions")
    }
    schema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
    
    // Create schema validator (same code API server uses)
    validator, _, err := validation.NewSchemaValidator(schema)
    if err != nil {
        return fmt.Errorf("failed to create schema validator: %w", err)
    }
    
    v.schemas[fs] = schema
    v.validators[fs] = validator
    
    return nil
}

// ValidateSchema validates an object against the CRD schema for a feature set
func (v *CRDValidator) ValidateSchema(obj *unstructured.Unstructured, fs featuregate.FeatureSet) error {
    validator, exists := v.validators[fs]
    if !exists {
        return fmt.Errorf("no schema loaded for feature set %s", fs)
    }
    
    // Validate against OpenAPI schema
    if errs := validation.ValidateCustomResource(nil, obj.Object, validator); len(errs) > 0 {
        return errs.ToAggregate()
    }
    
    return nil
}
```

#### 2. Unified Validator

```go
// pkg/validation/unified.go

package validation

import (
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// UnifiedValidator combines CRD schema validation with write-mode/feature-gate validation
type UnifiedValidator struct {
    crdValidator   *CRDValidator
    fieldValidator *Validator
}

// NewUnifiedValidator creates a validator that performs both schema and field-level validation
func NewUnifiedValidator() (*UnifiedValidator, error) {
    crdValidator, err := NewCRDValidator()
    if err != nil {
        return nil, err
    }
    
    return &UnifiedValidator{
        crdValidator:   crdValidator,
        fieldValidator: NewValidator(),
    }, nil
}

// ValidateCreate validates a create operation
func (v *UnifiedValidator) ValidateCreate(obj *unstructured.Unstructured, req *Request) error {
    // 1. Validate against CRD schema (types, required fields, enums, etc.)
    if err := v.crdValidator.ValidateSchema(obj, req.FeatureSet); err != nil {
        return fmt.Errorf("schema validation failed: %w", err)
    }
    
    // 2. Validate write-mode and feature-gate rules
    if err := v.fieldValidator.Validate(req); err != nil {
        return fmt.Errorf("field validation failed: %w", err)
    }
    
    return nil
}

// ValidateUpdate validates an update operation
func (v *UnifiedValidator) ValidateUpdate(oldObj, newObj *unstructured.Unstructured, req *Request) error {
    // 1. Validate new object against CRD schema
    if err := v.crdValidator.ValidateSchema(newObj, req.FeatureSet); err != nil {
        return fmt.Errorf("schema validation failed: %w", err)
    }
    
    // 2. Validate write-mode rules (including immutability)
    req.Operation = OperationUpdate
    if err := v.fieldValidator.Validate(req); err != nil {
        return fmt.Errorf("field validation failed: %w", err)
    }
    
    return nil
}
```

### CRD Embedding Strategy

**Option A: Embed via go:embed** (Recommended)
- Store CRD YAML files in `pkg/validation/testdata/crds/`
- Use `//go:embed` to compile them into the binary
- Pros: Self-contained, no external file dependencies
- Cons: Binary size increase (~100KB per CRD)

**Option B: Runtime Loading**
- Load from `config/crd/variants/` at runtime
- Pros: No binary bloat
- Cons: Requires CRD files to be deployed alongside binary

**Decision**: Use Option A for simplicity and self-containment.

### Testing Strategy

```go
// pkg/validation/crdvalidator_test.go

func TestCRDValidator_ValidateSchema(t *testing.T) {
    tests := []struct {
        name        string
        featureSet  featuregate.FeatureSet
        obj         *unstructured.Unstructured
        wantErr     bool
        errContains string
    }{
        {
            name:       "valid cluster with Default features",
            featureSet: featuregate.Default,
            obj:        newClusterWithFields(map[string]interface{}{
                "spec.displayName": "test-cluster",
                "spec.hostedCluster.additionalTrustBundle.name": "ca-bundle",
            }),
            wantErr: false,
        },
        {
            name:       "invalid type - displayName is integer",
            featureSet: featuregate.Default,
            obj:        newClusterWithFields(map[string]interface{}{
                "spec.displayName": 12345, // Should be string
            }),
            wantErr:     true,
            errContains: "Invalid type",
        },
        {
            name:       "DevPreview field in Default feature set",
            featureSet: featuregate.Default,
            obj:        newClusterWithFields(map[string]interface{}{
                "spec.hostedCluster.autoNode.provisionerConfig.karpenter.enabled": true,
            }),
            wantErr:     true,
            errContains: "not found", // Field doesn't exist in Default CRD
        },
        {
            name:       "DevPreview field in DevPreview feature set",
            featureSet: featuregate.DevPreview,
            obj:        newClusterWithFields(map[string]interface{}{
                "spec.hostedCluster.autoNode.provisionerConfig.karpenter.enabled": true,
            }),
            wantErr: false,
        },
    }
    
    validator, err := NewCRDValidator()
    require.NoError(t, err)
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator.ValidateSchema(tt.obj, tt.featureSet)
            if tt.wantErr {
                require.Error(t, err)
                if tt.errContains != "" {
                    require.Contains(t, err.Error(), tt.errContains)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## Integration Points

### 1. CLI Tool Integration

```go
// Example: CLI command to validate a cluster YAML before submission

func validateClusterCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "validate cluster.yaml",
        Short: "Validate a cluster YAML file",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Load cluster YAML
            obj, err := loadYAML(args[0])
            if err != nil {
                return err
            }
            
            // Create unified validator
            validator, err := validation.NewUnifiedValidator()
            if err != nil {
                return err
            }
            
            // Validate
            req := &validation.Request{
                Operation:  validation.OperationCreate,
                Fields:     extractFields(obj),
                FeatureSet: featuregate.Default, // From cluster metadata
            }
            
            if err := validator.ValidateCreate(obj, req); err != nil {
                return fmt.Errorf("validation failed: %w", err)
            }
            
            fmt.Println("✓ Cluster is valid")
            return nil
        },
    }
}
```

### 2. API Server Integration

For server-side validation (future work), this could be wrapped in an admission webhook:

```go
// Example: Admission webhook handler

func (h *ClusterWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
    validator, _ := validation.NewUnifiedValidator()
    
    obj := &unstructured.Unstructured{}
    if err := json.Unmarshal(req.Object.Raw, obj); err != nil {
        return admission.Errored(http.StatusBadRequest, err)
    }
    
    validationReq := &validation.Request{
        Operation:  validation.Operation(req.Operation),
        Fields:     extractFields(obj),
        FeatureSet: getCustomerFeatureSet(ctx),
    }
    
    if req.Operation == admissionv1.Create {
        if err := validator.ValidateCreate(obj, validationReq); err != nil {
            return admission.Denied(err.Error())
        }
    }
    
    return admission.Allowed("")
}
```

## Implementation Phases

### Phase 1: CRD Schema Validator (Week 1)
- [ ] Create `pkg/validation/crdvalidator.go`
- [ ] Implement CRD loading from embedded YAML
- [ ] Integrate `k8s.io/apiextensions-apiserver` validation
- [ ] Write unit tests for schema validation
- [ ] Document usage in README

**Deliverable**: Standalone CRD schema validator that works independently

### Phase 2: Unified Validation (Week 2)
- [ ] Create `pkg/validation/unified.go`
- [ ] Combine CRD schema validation with existing field validator
- [ ] Add integration tests covering both validation layers
- [ ] Update CLAUDE.md with validation workflow

**Deliverable**: Single entry point for all validation logic

### Phase 3: CLI Integration (Week 3)
- [ ] Create CLI tool or subcommand for validation
- [ ] Support loading YAML files
- [ ] Pretty-print validation errors
- [ ] Add examples to documentation

**Deliverable**: `hyperfleet-validate cluster.yaml` command

### Phase 4: Compatibility Testing (Week 4 - Optional)
- [ ] Support loading multiple CRD versions
- [ ] Implement forward/backward compatibility checks
- [ ] Add regression test suite

**Deliverable**: Validation against past/future CRD versions

## Dependencies

### Go Modules Required

```go
require (
    k8s.io/apiextensions-apiserver v0.32.1  // CRD validation machinery
    k8s.io/apimachinery v0.32.1             // Unstructured objects
    sigs.k8s.io/yaml v1.4.0                 // YAML parsing
)
```

### Existing Components

- ✅ `pkg/validation/validator.go` - Write-mode/feature-gate validation
- ✅ `pkg/registry/` - Field metadata registry
- ✅ `pkg/featuregate/` - Feature gate registry
- ✅ `config/crd/variants/` - Generated CRD YAML files

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Binary size increase from embedded CRDs | Low | Each CRD ~50-100KB, total ~300KB |
| Schema validation performance overhead | Medium | Cache validators, only validate on create/update |
| CRD schema drift (generated vs embedded) | High | Add CI check to verify embedded CRDs match generated ones |
| Kubernetes API compatibility | Medium | Pin to stable k8s.io/apiextensions-apiserver version |

### CI Verification

Add a CI check to ensure embedded CRDs stay in sync:

```bash
# .github/workflows/verify.yaml
- name: Verify embedded CRDs match generated
  run: |
    make manifests
    for fs in default techpreview devpreview; do
      diff -u \
        config/crd/variants/cluster_${fs}.yaml \
        pkg/validation/testdata/crds/cluster_${fs}.yaml
    done
```

## Success Metrics

1. **Schema Coverage**: 100% of CRD fields validated against OpenAPI schema
2. **Feature Set Enforcement**: No Default customer can submit TechPreview/DevPreview fields
3. **Error Detection**: Validation errors caught client-side before API submission
4. **Performance**: Validation completes in <100ms for typical cluster specs
5. **Test Coverage**: >90% coverage for validation package

## Future Enhancements

1. **Multi-Version Validation**: Support validating against multiple CRD versions simultaneously
2. **Diff-Based Validation**: For updates, only validate changed fields
3. **Dry-Run Mode**: Validate without actual API submission
4. **Validation Webhooks**: Deploy as admission webhook for server-side enforcement
5. **Policy Engine Integration**: Combine with OPA/Kyverno for custom policies

## References

- [cluster-capi-operator objectvalidation](https://github.com/openshift/cluster-capi-operator/tree/main/pkg/controllers/crdcompatibility/objectvalidation)
- [k8s.io/apiextensions-apiserver validation](https://github.com/kubernetes/apiextensions-apiserver/tree/master/pkg/apiserver/validation)
- [HyperFleet API Management Design](./api-management.md)
- [JIRA: ROSAENG-61569](https://redhat.atlassian.net/browse/ROSAENG-61569)
- [Parent Epic: ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383)

## Open Questions

1. **Q**: Should we validate against the installed CRD schema or the embedded schema?  
   **A**: Use embedded schema for client-side validation. This ensures consistent behavior regardless of cluster state.

2. **Q**: How do we handle custom fields added by customers (e.g., via CRD patches)?  
   **A**: Out of scope for Phase 1. Could be addressed with schema merging in future.

3. **Q**: Should validation be blocking or advisory (warnings)?  
   **A**: Blocking for CLI tools, configurable (warn/deny) for webhooks.

4. **Q**: How do we handle schema evolution when HyperShift bumps?  
   **A**: Regenerate CRD variants → copy to pkg/validation/testdata/ → commit both.

## Conclusion

This design provides a path to comprehensive client-side validation by:
1. Reusing Kubernetes' battle-tested CRD validation machinery
2. Integrating with existing write-mode/feature-gate validation
3. Enforcing feature set boundaries at validation time
4. Enabling forward/backward compatibility testing

The implementation is low-risk, self-contained, and aligns with existing codegen architecture.
