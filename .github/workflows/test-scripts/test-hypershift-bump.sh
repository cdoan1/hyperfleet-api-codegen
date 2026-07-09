#!/bin/bash
set -e

echo "=== Testing HyperShift Version Bump Workflow ==="
echo ""

# Get current version for reference
CURRENT_VERSION=$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $2}')
echo "Current HyperShift version: $CURRENT_VERSION"

# Bump to v0.1.72 specifically
echo ""
echo "Step 1: Bumping to v0.1.72..."
make bump-hypershift-to VERSION=v0.1.72

# Verify go.mod was updated
NEW_VERSION=$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $2}')
echo ""
echo "New HyperShift version: $NEW_VERSION"

# Verify it's different from current version
if [ "$NEW_VERSION" = "$CURRENT_VERSION" ]; then
    echo "✗ go.mod was not updated"
    exit 1
fi

# Verify it's a valid pseudo-version for v0.1.72
if echo "$NEW_VERSION" | grep -qE "v0\.0\.0-[0-9]{14}-[0-9a-f]+"; then
    echo "✓ go.mod updated successfully to valid pseudo-version"
else
    echo "✗ go.mod update failed - invalid pseudo-version format"
    exit 1
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

# Step 3: Regenerate registry
echo ""
echo "Step 3: Regenerating field registry..."
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

# Step 4: Regenerate OpenAPI
echo ""
echo "Step 4: Regenerating OpenAPI schema..."
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
# 1. go.mod - HyperShift version bump
# 2. go.sum - dependency checksums
# 3. api/v1alpha1/hostedclusterspec.passthrough.go - potentially new/changed fields
# 4. pkg/registry/field_metadata.go - if new fields added
# 5. pkg/registry/field_metadata.json - if new fields added
# 6. openapi/openapi.json - if new visible fields added

CHANGED_FILES=$(git status --porcelain | wc -l)
echo "Number of changed files: $CHANGED_FILES"
echo ""

if [ "$CHANGED_FILES" -eq 0 ]; then
    echo "✓ No changes - v0.1.72 has same API surface as current version"
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
            pkg/registry/field_metadata.go|pkg/registry/field_metadata.json)
                echo "✓ Expected: $FILE (registry update from new fields)"
                ;;
            openapi/openapi.json)
                echo "✓ Expected: $FILE (OpenAPI update from new visible fields)"
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

# Step 6: Verify tests still pass
echo ""
echo "Step 6: Running tests..."
make test

echo ""
echo "=== HyperShift Version Bump Test PASSED ==="
echo ""
echo "Summary:"
echo "  - Bumped from $CURRENT_VERSION"
echo "  - Bumped to   $NEW_VERSION"
echo "  - Regenerated passthrough types"
echo "  - Regenerated field registry"
echo "  - Regenerated OpenAPI schema"
echo "  - All tests passed"
echo "  - Only expected files changed"
