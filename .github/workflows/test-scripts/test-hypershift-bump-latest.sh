#!/bin/bash
# Test HyperShift version bump to latest version
# This validates the workflow for staying current with upstream

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Call base script with "latest"
"$SCRIPT_DIR/test-hypershift-bump-base.sh" "latest"
