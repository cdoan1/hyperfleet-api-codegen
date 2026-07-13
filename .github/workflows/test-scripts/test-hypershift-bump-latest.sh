#!/bin/bash
set -e

echo "=== Testing HyperShift Latest Version Bump Workflow ==="
echo ""

# Get current version for reference
CURRENT_VERSION=$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $2}')
echo "Current HyperShift version: $CURRENT_VERSION"

# Bump to latest version
echo ""
echo "Step 1: Bumping to latest HyperShift version..."
make bump-hypershift

# Verify go.mod was updated
NEW_VERSION=$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $2}')
echo ""
echo "New HyperShift version: $NEW_VERSION"

# Verify it's different from current version or same (if already latest)
if [ "$NEW_VERSION" = "$CURRENT_VERSION" ]; then
    echo "ℹ Already at latest version - no update needed"
    # This is OK, not a failure
else
    # Verify it's a valid pseudo-version
    if echo "$NEW_VERSION" | grep -qE "v0\.0\.0-[0-9]{14}-[0-9a-f]+"; then
        echo "✓ go.mod updated successfully to latest version (valid pseudo-version)"
    else
        echo "✗ go.mod update failed - invalid pseudo-version format"
        exit 1
    fi
fi

# Step 2: Regenerate passthrough types
echo ""
echo "Step 2: Regenerating passthrough types..."
make generate-passthrough

# Verify hostedclusterspec.passthrough.go was created/updated
if [ ! -f "api/v1alpha1/hostedclusterspec.passthrough.go" ]; then
    echo "✗ hostedclusterspec.passthrough.go not found"
    exit 1
fi
echo "✓ hostedclusterspec.passthrough.go generated"

# Verify zz_generated.passthrough.go was cleaned up
if [ -f "api/v1alpha1/zz_generated.passthrough.go" ]; then
    echo "✗ zz_generated.passthrough.go should have been removed"
    exit 1
fi
echo "✓ zz_generated.passthrough.go cleaned up"

# Step 3: Activate mirror type mappings
echo ""
echo "Step 3: Activating mirror type mappings..."
# Uncomment Configuration mapping to enable auto-conversion
sed -i.bak '/FieldName:.*"Configuration"/,/},/ s/^[[:space:]]*# //' pkg/conversion/mirror_types.go
rm -f pkg/conversion/mirror_types.go.bak
echo "✓ Configuration mapping activated"

# Step 4: Regenerate conversion code
echo ""
echo "Step 4: Regenerating conversion code..."
make generate-conversion

# Verify conversion files exist
if [ ! -f "pkg/conversion/v1alpha1/cluster.go" ]; then
    echo "✗ pkg/conversion/v1alpha1/cluster.go not found"
    exit 1
fi
echo "✓ Conversion code generated"

# Step 5: Regenerate registry
echo ""
echo "Step 5: Regenerating field registry..."
make generate-registry

# Verify registry files exist
if [ ! -f "pkg/registry/field_metadata.go" ]; then
    echo "✗ field_metadata.go not found"
    exit 1
fi
if [ ! -f "pkg/registry/field_metadata.json" ]; then
    echo "✗ field_metadata.json not found"
    exit 1
fi
echo "✓ Field registry files generated"

# Step 6: Regenerate OpenAPI
echo ""
echo "Step 6: Regenerating OpenAPI schema..."
make generate-openapi

# Verify OpenAPI file exists
if [ ! -f "openapi/openapi.json" ]; then
    echo "✗ openapi.json not found"
    exit 1
fi
echo "✓ OpenAPI schema generated"

# Step 5: Check for changes
echo ""
echo "Step 5: Checking for changes..."
echo ""

# Expected changes:
# 1. go.mod - HyperShift version bump (if not already latest)
# 2. go.sum - dependency checksums (if not already latest)
# 3. api/v1alpha1/hostedclusterspec.passthrough.go - potentially new/changed fields
# 4. pkg/conversion/mirror_types.go - Configuration mapping activated
# 5. pkg/conversion/v1alpha1/cluster.go - generated conversion functions
# 6. pkg/conversion/v1alpha1/rest/*.go - generated REST types
# 7. pkg/conversion/types.go - generated ServiceSetFields
# 8. pkg/registry/field_metadata.go - if new fields added
# 9. pkg/registry/field_metadata.json - if new fields added
# 10. openapi/openapi.json - if new visible fields added

CHANGED_FILES=$(git status --porcelain | wc -l)
echo "Number of changed files: $CHANGED_FILES"
echo ""

if [ "$CHANGED_FILES" -eq 0 ]; then
    echo "✓ No changes - already at latest version with same API surface"
else
    echo "Changed files:"
    git status --porcelain
    echo ""

    # Verify only expected files changed
    UNEXPECTED_CHANGES=0

    while IFS= read -r line; do
        FILE=$(echo "$line" | awk '{print $2}')

        case "$FILE" in
            go.mod|go.sum)
                echo "✓ Expected: $FILE (dependency update)"
                ;;
            api/v1alpha1/hostedclusterspec.passthrough.go)
                echo "✓ Expected: $FILE (upstream API changes)"
                ;;
            pkg/conversion/mirror_types.go)
                echo "✓ Expected: $FILE (Configuration mapping activated)"
                ;;
            pkg/conversion/v1alpha1/cluster.go|pkg/conversion/v1alpha1/nodepool.go)
                echo "✓ Expected: $FILE (generated conversion functions)"
                ;;
            pkg/conversion/v1alpha1/rest/*.go)
                echo "✓ Expected: $FILE (generated REST types)"
                ;;
            pkg/conversion/types.go)
                echo "✓ Expected: $FILE (generated ServiceSetFields)"
                ;;
            pkg/registry/field_metadata.go|pkg/registry/field_metadata.json)
                echo "✓ Expected: $FILE (registry update from new fields)"
                ;;
            openapi/openapi.json)
                echo "✓ Expected: $FILE (OpenAPI update from new visible fields)"
                ;;
            .github/workflows/test-scripts/test-hypershift-bump-latest.sh)
                echo "✓ Expected: $FILE (CI test script updates)"
                ;;
            *)
                echo "✗ Unexpected: $FILE"
                UNEXPECTED_CHANGES=$((UNEXPECTED_CHANGES + 1))
                ;;
        esac
    done < <(git status --porcelain)

    echo ""
    if [ "$UNEXPECTED_CHANGES" -gt 0 ]; then
        echo "✗ Found $UNEXPECTED_CHANGES unexpected file changes"
        exit 1
    else
        echo "✓ All file changes are expected"
    fi
fi

# Step 7: Verify tests still pass
echo ""
echo "Step 7: Running tests..."
make test

echo ""
echo "=== HyperShift Latest Version Bump Test PASSED ==="
echo ""
echo "Summary:"
echo "  - Previous version: $CURRENT_VERSION"
echo "  - New version:      $NEW_VERSION"
if [ "$NEW_VERSION" = "$CURRENT_VERSION" ]; then
    echo "  - Status: Already at latest (no update needed)"
else
    echo "  - Status: Updated to latest"
fi
echo "  - Activated Configuration mirror type mapping"
echo "  - Regenerated passthrough types"
echo "  - Regenerated conversion code (auto-conversion for type changes)"
echo "  - Regenerated field registry"
echo "  - Regenerated OpenAPI schema"
echo "  - All tests passed"
echo "  - Only expected files changed"
