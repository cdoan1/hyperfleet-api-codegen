# Boundary 1 Conversion Functions Design

**Status**: Proposed  
**Date**: 2026-07-13  
**Related**: ROSAENG-61385

## Overview

This document describes the design and implementation of conversion functions between HyperFleet wrapper CRDs and upstream HyperShift CRDs (Boundary 1).

## Context

HyperFleet uses **wrapper CRDs** for Cluster and NodePool that:
1. **Envelope fields** - HyperFleet-only fields (deleteProtection, expirationTimestamp, properties, etc.)
2. **Passthrough struct** - Generated mirror of all HyperShift fields

The Platform API controller needs to convert between these wrapper types and native HyperShift types when creating/updating HyperShift resources.

## Boundaries Clarification

### Boundary 1: HyperFleet CRD ↔ HyperShift CRD (THIS DOCUMENT)
- **Write path**: Platform API creates HyperShift HostedCluster from HyperFleet Cluster
- **Read path**: Platform API reads HyperShift status back into HyperFleet status
- **Use case**: Controller reconciliation loop

### Boundary 2: HyperFleet CRD ↔ REST API (ALREADY COMPLETE)
- **Write path**: HTTP request → HyperFleet CRD (with service-set enrichment)
- **Read path**: HyperFleet CRD → HTTP response (filter visible fields)
- **Use case**: Platform API HTTP handlers

## Requirements

### Functions to Implement

1. **ToHyperShiftHostedCluster**
   - Input: `*v1alpha1.Cluster` (HyperFleet wrapper)
   - Output: `*hypershiftv1beta1.HostedCluster` (upstream HyperShift)
   - Purpose: Create/update HyperShift resource from HyperFleet CRD

2. **FromHyperShiftHostedCluster**
   - Input: `*hypershiftv1beta1.HostedCluster` (upstream HyperShift)
   - Output: `v1alpha1.ClusterStatus` (HyperFleet status)
   - Purpose: Read HyperShift status back into HyperFleet

3. **ToHyperShiftNodePool**
   - Input: `*v1alpha1.NodePool` (HyperFleet wrapper)
   - Output: `*hypershiftv1beta1.NodePool` (upstream HyperShift)
   - Purpose: Create/update HyperShift NodePool from HyperFleet

4. **FromHyperShiftNodePool**
   - Input: `*hypershiftv1beta1.NodePool` (upstream HyperShift)
   - Output: `v1alpha1.NodePoolStatus` (HyperFleet status)
   - Purpose: Read HyperShift NodePool status back into HyperFleet

## Design

### Key Principles

1. **Envelope fields stay on HyperFleet side**
   - DeleteProtection, ExpirationTimestamp, Properties, etc.
   - Never sent to HyperShift
   - Controller uses them for platform logic

2. **Passthrough fields map 1:1**
   - HyperFleet passthrough struct → HyperShift native struct
   - No type mismatches (both reference same upstream types via import)
   - Direct field-by-field copy

3. **Metadata handled separately**
   - Name, Namespace from HyperFleet
   - Labels/Annotations merged (platform labels + user labels)
   - OwnerReferences set by controller, not conversion functions

4. **Status is read-only from HyperShift**
   - HyperShift owns the truth for status
   - HyperFleet mirrors it for customer visibility
   - No conversion needed for write path (spec only)

### Data Flow

#### Write Path (Create/Update HyperShift Resource)

```
HyperFleet Cluster CRD
    ↓
ToHyperShiftHostedCluster()
    ↓
HyperShift HostedCluster
    ↓
Applied to K8s
```

#### Read Path (Sync Status Back)

```
HyperShift HostedCluster (from K8s)
    ↓
FromHyperShiftHostedCluster()
    ↓
HyperFleet Cluster Status
    ↓
Update HyperFleet CRD
```

## Implementation

### File Structure

```
pkg/conversion/hypershift/
├── cluster.go          # ToHyperShiftHostedCluster, FromHyperShiftHostedCluster
├── nodepool.go         # ToHyperShiftNodePool, FromHyperShiftNodePool
└── cluster_test.go     # Unit tests
```

### ToHyperShiftHostedCluster Implementation

```go
package hypershift

import (
    v1alpha1 "github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1"
    hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ToHyperShiftHostedCluster converts a HyperFleet Cluster to a HyperShift HostedCluster.
//
// The conversion:
// 1. Maps passthrough fields directly (no type conversion needed)
// 2. Sets metadata (name, namespace, labels, annotations)
// 3. EXCLUDES envelope fields (deleteProtection, expirationTimestamp, properties)
//    - These are HyperFleet-only and not sent to HyperShift
//
// Note: OwnerReferences and finalizers should be set by the controller, not this function.
func ToHyperShiftHostedCluster(cluster *v1alpha1.Cluster) *hypershiftv1beta1.HostedCluster {
    if cluster == nil {
        return nil
    }

    hc := &hypershiftv1beta1.HostedCluster{
        ObjectMeta: metav1.ObjectMeta{
            Name:        cluster.Name,
            Namespace:   cluster.Namespace,
            Labels:      cluster.Labels,      // Merge/filter done by controller
            Annotations: cluster.Annotations, // Merge/filter done by controller
        },
    }

    // Map passthrough spec fields
    hc.Spec = toHyperShiftHostedClusterSpec(cluster.Spec.HostedCluster)

    return hc
}

// toHyperShiftHostedClusterSpec converts the passthrough spec.
//
// This is straightforward field mapping since HyperFleet's passthrough struct
// mirrors HyperShift's types exactly (generated from upstream).
func toHyperShiftHostedClusterSpec(spec v1alpha1.HostedClusterSpecPassthrough) hypershiftv1beta1.HostedClusterSpec {
    return hypershiftv1beta1.HostedClusterSpec{
        Release:               spec.Release,
        ControlPlaneRelease:   spec.ControlPlaneRelease,
        ClusterID:             spec.ClusterID,
        InfraID:               spec.InfraID,
        UpdateService:         spec.UpdateService,
        Channel:               spec.Channel,
        PullSecret:            spec.PullSecret,
        SigningKey:            spec.SigningKey,
        SSHKey:                spec.SSHKey,
        IssuerURL:             spec.IssuerURL,
        Configuration:         spec.Configuration,
        ImageContentSources:   spec.ImageContentSources,
        AdditionalTrustBundle: spec.AdditionalTrustBundle,
        SecretEncryption:      spec.SecretEncryption,
        FIPS:                  spec.FIPS,
        PausedUntil:           spec.PausedUntil,
        OLMCatalogPlacement:   spec.OLMCatalogPlacement,
        ControllerAvailabilityPolicy: spec.ControllerAvailabilityPolicy,
        InfrastructureAvailabilityPolicy: spec.InfrastructureAvailabilityPolicy,
        DNS:                   spec.DNS,
        Networking:            spec.Networking,
        AutoNode:              spec.AutoNode,
        Etcd:                  spec.Etcd,
        Services:              spec.Services,
        OperatorConfiguration: spec.OperatorConfiguration,
        AuditWebhook:          spec.AuditWebhook,
        Platform:              spec.Platform,
        NodeSelector:          spec.NodeSelector,
        Tolerations:           spec.Tolerations,
    }
}
```

### FromHyperShiftHostedCluster Implementation

```go
// FromHyperShiftHostedCluster extracts status from a HyperShift HostedCluster.
//
// The conversion:
// 1. Copies HyperShift status fields to HyperFleet status
// 2. DOES NOT touch envelope fields (controller manages those separately)
// 3. Returns just the status (not full Cluster)
//
// The controller will merge this into the HyperFleet Cluster's status.
func FromHyperShiftHostedCluster(hc *hypershiftv1beta1.HostedCluster) v1alpha1.ClusterStatus {
    if hc == nil {
        return v1alpha1.ClusterStatus{}
    }

    return v1alpha1.ClusterStatus{
        // Copy HyperShift status to HyperFleet passthrough status
        HostedCluster: fromHyperShiftHostedClusterStatus(hc.Status),
    }
}

// fromHyperShiftHostedClusterStatus converts HyperShift status to HyperFleet passthrough status.
func fromHyperShiftHostedClusterStatus(status hypershiftv1beta1.HostedClusterStatus) v1alpha1.HostedClusterStatusPassthrough {
    return v1alpha1.HostedClusterStatusPassthrough{
        Version:              status.Version,
        KubeConfig:           status.KubeConfig,
        KubeadminPassword:    status.KubeadminPassword,
        ControlPlaneEndpoint: status.ControlPlaneEndpoint,
        OAuthCallbackURLTemplate: status.OAuthCallbackURLTemplate,
        Conditions:           status.Conditions,
        IgnitionEndpoint:     status.IgnitionEndpoint,
        Platform:             status.Platform,
    }
}
```

### ToHyperShiftNodePool Implementation

```go
// ToHyperShiftNodePool converts a HyperFleet NodePool to a HyperShift NodePool.
//
// Similar to HostedCluster conversion: maps passthrough fields, excludes envelope fields.
func ToHyperShiftNodePool(np *v1alpha1.NodePool) *hypershiftv1beta1.NodePool {
    if np == nil {
        return nil
    }

    hsnp := &hypershiftv1beta1.NodePool{
        ObjectMeta: metav1.ObjectMeta{
            Name:        np.Name,
            Namespace:   np.Namespace,
            Labels:      np.Labels,
            Annotations: np.Annotations,
        },
    }

    // Map passthrough spec
    hsnp.Spec = toHyperShiftNodePoolSpec(np.Spec.NodePool)

    return hsnp
}

// toHyperShiftNodePoolSpec converts the passthrough NodePool spec.
func toHyperShiftNodePoolSpec(spec v1alpha1.NodePoolSpecPassthrough) hypershiftv1beta1.NodePoolSpec {
    return hypershiftv1beta1.NodePoolSpec{
        ClusterName:      spec.ClusterName,
        Release:          spec.Release,
        Platform:         spec.Platform,
        Replicas:         spec.Replicas,
        Management:       spec.Management,
        AutoScaling:      spec.AutoScaling,
        Config:           spec.Config,
        NodeDrainTimeout: spec.NodeDrainTimeout,
        NodeVolumeDetachTimeout: spec.NodeVolumeDetachTimeout,
        Arch:             spec.Arch,
        Taints:           spec.Taints,
        PausedUntil:      spec.PausedUntil,
        TuningConfig:     spec.TuningConfig,
    }
}
```

### FromHyperShiftNodePool Implementation

```go
// FromHyperShiftNodePool extracts status from a HyperShift NodePool.
func FromHyperShiftNodePool(np *hypershiftv1beta1.NodePool) v1alpha1.NodePoolStatus {
    if np == nil {
        return v1alpha1.NodePoolStatus{}
    }

    return v1alpha1.NodePoolStatus{
        NodePool: fromHyperShiftNodePoolStatus(np.Status),
    }
}

// fromHyperShiftNodePoolStatus converts HyperShift NodePool status.
func fromHyperShiftNodePoolStatus(status hypershiftv1beta1.NodePoolStatus) v1alpha1.NodePoolStatusPassthrough {
    return v1alpha1.NodePoolStatusPassthrough{
        Replicas:     status.Replicas,
        Version:      status.Version,
        Platform:     status.Platform,
        Conditions:   status.Conditions,
    }
}
```

## Testing Strategy

### Unit Tests

Test each conversion function with:

1. **Nil safety**
   ```go
   func TestToHyperShiftHostedCluster_Nil(t *testing.T) {
       result := ToHyperShiftHostedCluster(nil)
       assert.Nil(t, result)
   }
   ```

2. **Field mapping**
   ```go
   func TestToHyperShiftHostedCluster_FieldMapping(t *testing.T) {
       cluster := &v1alpha1.Cluster{
           ObjectMeta: metav1.ObjectMeta{
               Name: "test-cluster",
               Namespace: "clusters",
           },
           Spec: v1alpha1.ClusterSpec{
               HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
                   ClusterID: "test-id",
                   FIPS: true,
                   // ... test all fields
               },
           },
       }
       
       hc := ToHyperShiftHostedCluster(cluster)
       
       assert.Equal(t, "test-cluster", hc.Name)
       assert.Equal(t, "clusters", hc.Namespace)
       assert.Equal(t, "test-id", hc.Spec.ClusterID)
       assert.True(t, hc.Spec.FIPS)
   }
   ```

3. **Envelope fields excluded**
   ```go
   func TestToHyperShiftHostedCluster_EnvelopeFieldsNotCopied(t *testing.T) {
       cluster := &v1alpha1.Cluster{
           Spec: v1alpha1.ClusterSpec{
               DeleteProtection: &v1alpha1.DeleteProtection{Enabled: true},
               ExpirationTimestamp: &metav1.Time{Time: time.Now()},
               Properties: map[string]string{"key": "value"},
               // ... passthrough fields
           },
       }
       
       hc := ToHyperShiftHostedCluster(cluster)
       
       // HyperShift HostedCluster should not have these fields
       // (they don't exist in the HyperShift type)
       // This test just verifies the conversion doesn't panic
       assert.NotNil(t, hc)
   }
   ```

4. **Status conversion**
   ```go
   func TestFromHyperShiftHostedCluster_StatusMapping(t *testing.T) {
       hc := &hypershiftv1beta1.HostedCluster{
           Status: hypershiftv1beta1.HostedClusterStatus{
               Version: hypershiftv1beta1.ClusterVersionStatus{
                   Desired: configv1.Release{Version: "4.14.0"},
               },
           },
       }
       
       status := FromHyperShiftHostedCluster(hc)
       
       assert.Equal(t, "4.14.0", status.HostedCluster.Version.Desired.Version)
   }
   ```

### Integration Tests (Future)

Once a controller exists:
- Create HyperFleet Cluster → verify HyperShift HostedCluster created correctly
- Update HyperShift status → verify HyperFleet status updated
- Envelope fields remain on HyperFleet side only

## Non-Goals

### What These Functions DO NOT Do

1. **Set OwnerReferences** - Controller's responsibility
2. **Set Finalizers** - Controller's responsibility  
3. **Merge labels/annotations** - Controller filters platform vs user labels
4. **Validate fields** - CRD schema validation handles this
5. **Handle errors** - Return nil for nil input, let caller handle validation

### Why Simple Is Correct

These are **pure data transformations** with no business logic:
- No external dependencies
- No error conditions (beyond nil checks)
- No state management
- Just struct field mapping

The **controller** handles all business logic:
- Validation
- Ownership
- Reconciliation
- Error handling
- Status updates

## Implementation Checklist

- [ ] Create `pkg/conversion/hypershift/` package
- [ ] Implement `ToHyperShiftHostedCluster()`
- [ ] Implement `FromHyperShiftHostedCluster()`
- [ ] Implement `ToHyperShiftNodePool()`
- [ ] Implement `FromHyperShiftNodePool()`
- [ ] Write unit tests for all functions
- [ ] Verify compilation with HyperShift types
- [ ] Document in README
- [ ] Close ROSAENG-61385

## Future Enhancements

### Potential Automation

Unlike Boundary 2 (which required complex mirror type handling), Boundary 1 conversions are simple enough that they could potentially be auto-generated:

1. **AST analysis** to discover passthrough struct fields
2. **Generate field mapping** code automatically
3. **Update on HyperShift bump** when fields change

However, **manual implementation first** is the right approach:
- Only ~200 lines of code
- High confidence from explicit code review
- Easy to maintain
- Future automation can be validated against manual version

## References

- **Parent Epic**: ROSAENG-61383
- **Jira Story**: ROSAENG-61385
- **Design Doc**: `docs/api-management.md` (Boundary 1 section)
- **Boundary 2 Design**: `docs/auto-conversion-generation.md`
