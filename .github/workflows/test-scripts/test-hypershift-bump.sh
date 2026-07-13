#!/bin/bash
# Test HyperShift version bump to a specific version (v0.1.72)
# This validates the workflow for upgrading to a known version

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Call base script with target version
"$SCRIPT_DIR/test-hypershift-bump-base.sh" "v0.1.72"
