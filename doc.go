// Package hyperfleet provides code generation tools for managing the HyperFleet API.
//
// This package contains tooling that manages three layers of types with a single source of truth:
//   - HyperShift CRDs: Full HostedCluster and NodePool from upstream HyperShift
//   - HyperFleet CRDs: Wrappers (Cluster/NodePool) and native resources
//   - Platform API: REST API with OpenAPI spec generated from HyperFleet CRDs
//
// Go markers on types control visibility, write modes, and feature gating. Codegen reads
// these markers and produces all downstream artifacts (CRDs, OpenAPI spec, field metadata registry).
package hyperfleet
