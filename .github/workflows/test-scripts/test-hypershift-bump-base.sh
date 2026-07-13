#!/bin/bash
# Base script for testing HyperShift version bumps
# Usage:
#   test-hypershift-bump-base.sh v0.1.72  # Test bump to specific version
#   test-hypershift-bump-base.sh latest   # Test bump to latest version

set -e

# Parameter: version string (e.g., "v0.1.72") or "latest"
VERSION_ARG="${1:-latest}"

# Determine if this is a specific version or latest
if [ "$VERSION_ARG" = "latest" ]; then
    BUMP_TYPE="latest"
    echo "=== Testing HyperShift Latest Version Bump Workflow ==="
else
    BUMP_TYPE="specific"
    VERSION="$VERSION_ARG"
    echo "=== Testing HyperShift Version Bump to $VERSION ==="
fi
echo ""

# Get current version for reference
CURRENT_VERSION=$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $2}')
echo "Current HyperShift version: $CURRENT_VERSION"

# Step 1: Bump HyperShift version
echo ""
if [ "$BUMP_TYPE" = "specific" ]; then
    echo "Step 1: Bumping to $VERSION..."
    make bump-hypershift-to VERSION="$VERSION"
else
    echo "Step 1: Bumping to latest HyperShift version..."
    make bump-hypershift
fi

# Verify go.mod was updated
NEW_VERSION=$(grep "github.com/openshift/hypershift/api" go.mod | awk '{print $2}')
echo ""
echo "New HyperShift version: $NEW_VERSION"

# Validate version update
if [ "$NEW_VERSION" = "$CURRENT_VERSION" ]; then
    if [ "$BUMP_TYPE" = "latest" ]; then
        echo "ℹ Already at latest version - no update needed"
        # This is OK for latest bump, not a failure
    else
        echo "✗ go.mod was not updated"
        exit 1
    fi
else
    # Verify it's a valid pseudo-version
    if echo "$NEW_VERSION" | grep -qE "v0\.0\.0-[0-9]{14}-[0-9a-f]+"; then
        if [ "$BUMP_TYPE" = "specific" ]; then
            echo "✓ go.mod updated successfully to valid pseudo-version"
        else
            echo "✓ go.mod updated successfully to latest version (valid pseudo-version)"
        fi
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

# Steps 3-4: Latest-only steps (mirror type activation and conversion generation)
STEP_OFFSET=0
if [ "$BUMP_TYPE" = "latest" ]; then
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

    STEP_OFFSET=2
fi

# Step 3/5: Regenerate registry
STEP_NUM=$((3 + STEP_OFFSET))
echo ""
echo "Step $STEP_NUM: Regenerating field registry..."
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

# Step 4/6: Regenerate OpenAPI
STEP_NUM=$((4 + STEP_OFFSET))
echo ""
echo "Step $STEP_NUM: Regenerating OpenAPI schema..."
make generate-openapi

# Verify OpenAPI file exists
if [ ! -f "openapi/openapi.json" ]; then
    echo "✗ openapi.json not found"
    exit 1
fi
echo "✓ OpenAPI schema generated"

# Step 5/7: Check for changes
STEP_NUM=$((5 + STEP_OFFSET))
echo ""
echo "Step $STEP_NUM: Checking for changes..."
echo ""

CHANGED_FILES=$(git status --porcelain | wc -l)
echo "Number of changed files: $CHANGED_FILES"
echo ""

if [ "$CHANGED_FILES" -eq 0 ]; then
    if [ "$BUMP_TYPE" = "latest" ]; then
        echo "✓ No changes - already at latest version with same API surface"
    else
        echo "✓ No changes - $VERSION has same API surface as current version"
    fi
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
            pkg/conversion/mirror_types.go)
                # Latest-only expected file
                if [ "$BUMP_TYPE" = "latest" ]; then
                    echo "✓ Expected: $FILE (Configuration mapping activated)"
                else
                    echo "✗ Unexpected: $FILE"
                    UNEXPECTED_CHANGES=$((UNEXPECTED_CHANGES + 1))
                fi
                ;;
            pkg/conversion/v1alpha1/cluster.go|pkg/conversion/v1alpha1/nodepool.go)
                # Latest-only expected file
                if [ "$BUMP_TYPE" = "latest" ]; then
                    echo "✓ Expected: $FILE (generated conversion functions)"
                else
                    echo "✗ Unexpected: $FILE"
                    UNEXPECTED_CHANGES=$((UNEXPECTED_CHANGES + 1))
                fi
                ;;
            pkg/conversion/v1alpha1/rest/*.go)
                # Latest-only expected file
                if [ "$BUMP_TYPE" = "latest" ]; then
                    echo "✓ Expected: $FILE (generated REST types)"
                else
                    echo "✗ Unexpected: $FILE"
                    UNEXPECTED_CHANGES=$((UNEXPECTED_CHANGES + 1))
                fi
                ;;
            pkg/conversion/types.go)
                # Latest-only expected file
                if [ "$BUMP_TYPE" = "latest" ]; then
                    echo "✓ Expected: $FILE (generated ServiceSetFields)"
                else
                    echo "✗ Unexpected: $FILE"
                    UNEXPECTED_CHANGES=$((UNEXPECTED_CHANGES + 1))
                fi
                ;;
            .github/workflows/test-scripts/test-hypershift-bump-latest.sh)
                # Latest-only expected file
                if [ "$BUMP_TYPE" = "latest" ]; then
                    echo "✓ Expected: $FILE (CI test script updates)"
                else
                    echo "✗ Unexpected: $FILE"
                    UNEXPECTED_CHANGES=$((UNEXPECTED_CHANGES + 1))
                fi
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

# Step 6/8: Verify tests still pass
STEP_NUM=$((6 + STEP_OFFSET))
echo ""
echo "Step $STEP_NUM: Running tests..."
make test

# Summary
echo ""
if [ "$BUMP_TYPE" = "latest" ]; then
    echo "=== HyperShift Latest Version Bump Test PASSED ==="
else
    echo "=== HyperShift Version Bump Test PASSED ==="
fi
echo ""
echo "Summary:"
echo "  - Previous version: $CURRENT_VERSION"
echo "  - New version:      $NEW_VERSION"
if [ "$NEW_VERSION" = "$CURRENT_VERSION" ]; then
    echo "  - Status: Already at latest (no update needed)"
else
    if [ "$BUMP_TYPE" = "specific" ]; then
        echo "  - Bumped to $VERSION"
    else
        echo "  - Status: Updated to latest"
        echo "  - Activated Configuration mirror type mapping"
        echo "  - Regenerated conversion code (auto-conversion for type changes)"
    fi
fi
echo "  - Regenerated passthrough types"
echo "  - Regenerated field registry"
echo "  - Regenerated OpenAPI schema"
echo "  - All tests passed"
echo "  - Only expected files changed"
