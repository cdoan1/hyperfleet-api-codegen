# HyperFleet API Codegen

Code generation tools for managing the HyperFleet API: HyperShift CRD → HyperFleet CRD → Platform API OpenAPI.

## Overview

This repository contains code generation tooling that manages three layers of types with a single source of truth:

- **HyperShift CRDs**: Full HostedCluster and NodePool from upstream HyperShift
- **HyperFleet CRDs**: Wrappers (Cluster/NodePool) and native resources  
- **Platform API**: REST API with OpenAPI spec generated from HyperFleet CRDs

Go markers on types control visibility, write modes, and feature gating. Codegen reads these markers and produces all downstream artifacts (CRDs, OpenAPI spec, field metadata registry).

## Architecture

```
HyperShift CRDs (upstream)
    ↓ [passthrough generator]
HyperFleet CRDs (Go structs with markers)
    ↓ [kube-openapi + marker scanner]
Platform API (OpenAPI spec + field metadata registry)
```

### Three Control Markers

1. **Visibility** (`+k8s:openapi-gen=false`) - Controls whether a field appears in OpenAPI
2. **Write Mode** (`+hyperfleet:write-mode=X`) - Controls customer mutability (mutable/immutable/service-set)
3. **Feature Gate** (`+openshift:enable:FeatureGate=X`) - Controls per-customer entitlements

## Documentation

- **[Design Document](docs/api-management.md)** - Complete architecture and design specification
- **Developer Guide** _(coming soon)_ - Practical workflows and examples

## Components

### 1. Passthrough Generator
Reads HyperShift types and generates HyperFleet passthrough structs with safe defaults. Preserves existing marker annotations on regeneration.

### 2. Marker Scanner  
Parses Go source files and generates a field metadata registry mapping each field path to its write mode and feature gate.

### 3. OpenAPI Generator
Integrates kube-openapi to generate OpenAPI schemas respecting visibility markers.

### 4. Feature Gate Tooling
Manages feature gate registry and generates CRD variants per feature set (Default, TechPreview, DevPreview).

### 5. Type Converters
Auto-generates bidirectional conversion functions between CRD and REST types.

### 6. Runtime Validator
Generic validation using field metadata registry to enforce write-mode and feature gate rules.

## Project Status

🚧 **Proof of Concept** - Active development

See [ROSAENG-61383](https://redhat.atlassian.net/browse/ROSAENG-61383) for implementation tracking.

## Related Projects

- [HyperShift](https://github.com/openshift/hypershift) - Upstream HostedCluster and NodePool CRDs
- [openshift/api](https://github.com/openshift/api) - Feature gate patterns and tooling reference
- [kube-openapi](https://github.com/kubernetes/kube-openapi) - OpenAPI schema generation

## License

Apache License 2.0
