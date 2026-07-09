# CI Verification Status (ROSAENG-61386)

**Jira**: [ROSAENG-61386](https://redhat.atlassian.net/browse/ROSAENG-61386)  
**Current Status**: Review  
**Evaluation Date**: 2026-07-09

## Acceptance Criteria Review

### AC 1: ✅ CI job validates all passthrough fields have required markers

**Status**: **COMPLETE**

**Implementation**:
```yaml
# .github/workflows/ci.yml - verify job, line 71-78
- name: Verify field registry is up to date
  run: |
    ./bin/marker-scanner --input-dirs=./api/v1alpha1 --output-file=/tmp/field_metadata.go
    diff pkg/registry/field_metadata.go /tmp/field_metadata.go || (
      echo "Error: pkg/registry/field_metadata.go is out of date"
      echo "Run: make generate-registry"
      exit 1
    )
```

**How it works**:
1. Runs `marker-scanner` on `api/v1alpha1/` (passthrough types)
2. marker-scanner has `--validate` flag (default: true) that checks visible fields have write-mode markers
3. Generates fresh registry to `/tmp/field_metadata.go`
4. Diffs against committed `pkg/registry/field_metadata.go`
5. Fails if they differ (missing markers → different registry → diff fails)

**Validation logic** (`pkg/markers/scanner.go`):
```go
func (r FieldRegistry) Validate() error {
    var errors []string
    for path, meta := range r {
        // All visible fields must have a write mode
        if !meta.Hidden && meta.WriteMode == "" {
            errors = append(errors, fmt.Sprintf(
                "field %s is missing +hyperfleet:write-mode marker", path))
        }
    }
    if len(errors) > 0 {
        return fmt.Errorf("validation failed:\n  %s", 
            strings.Join(errors, "\n  "))
    }
    return nil
}
```

**Evidence**: Line 89 comment confirms this:
```yaml
# Note: Marker validation is done by the marker-scanner --validate flag above
# which properly parses Go AST to verify visible fields have write-mode markers
```

---

### AC 2: ✅ Build fails if markers are missing

**Status**: **COMPLETE**

**How it fails**:
1. Developer adds new field to passthrough type without markers
2. Developer runs `make generate-registry` → marker-scanner validates and generates registry
3. Developer commits passthrough type + registry
4. **Scenario A**: Developer forgets to run `make generate-registry`
   - CI runs marker-scanner, generates registry to `/tmp`
   - Diff fails: `/tmp/field_metadata.go` ≠ `pkg/registry/field_metadata.go`
   - Build fails with: "Error: pkg/registry/field_metadata.go is out of date"
5. **Scenario B**: Developer commits invalid markers (visible field without write-mode)
   - marker-scanner validation fails during registry generation
   - Build fails with: "validation failed: field X is missing +hyperfleet:write-mode marker"

**Exit codes**:
- marker-scanner exits non-zero on validation failure
- diff exits non-zero if files differ
- Either causes CI job to fail

---

### AC 3: ⚠️ Clear error messages indicate which fields need attention

**Status**: **MOSTLY COMPLETE** (could be improved)

**Current error messages**:

**Scenario 1**: Registry out of date
```
Error: pkg/registry/field_metadata.go is out of date
Run: make generate-registry
```
✅ **Clear**: Tells developer exactly what to do

**Scenario 2**: Validation failure during scan
```
validation failed:
  field spec.badField is missing +hyperfleet:write-mode marker
  field spec.anotherField is missing +hyperfleet:write-mode marker
```
✅ **Clear**: Lists exact fields and what's missing

**Scenario 3**: OpenAPI schema out of date
```
Error: openapi/openapi.json is out of date
Run: make generate-openapi
```
✅ **Clear**: Tells developer exactly what to do

**Improvement opportunity**:
Could add more context about *why* the registry is out of date:
- "New fields were added without markers"
- "Markers were changed but registry not regenerated"

**Recommendation**: Current error messages are sufficient for POC. Consider enhancement for production.

---

### AC 4: ✅ Check runs on every PR touching passthrough types

**Status**: **COMPLETE**

**Configuration** (`.github/workflows/ci.yml` lines 3-7):
```yaml
on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]
```

**What runs**:
- `test` job: Unit tests
- `verify` job: **Marker validation + Registry up-to-date check**
- `build` job: Build all tools
- `test-hypershift-bump`: Test v0.1.72 upgrade workflow
- `test-hypershift-bump-latest`: Test latest version upgrade workflow

**Scope**: Runs on **all** PRs to main, not just those touching passthrough types.

**Why this is better**: 
- Any change that affects codegen tooling is caught
- No need to configure path filters
- Simpler CI configuration

✅ **Exceeds requirement**: Runs on every PR, not just those touching passthrough types

---

### AC 5: ⚠️ Documentation on required markers for developers

**Status**: **PARTIAL**

**What exists**:

1. ✅ **README.md** - Quick start with marker examples
   - Shows 3 control markers
   - Basic usage examples
   
2. ✅ **docs/api-management.md** - Comprehensive design doc
   - Marker 1: Visibility (`+k8s:openapi-gen=false`)
   - Marker 2: Write Mode (`+hyperfleet:write-mode=X`)
   - Marker 3: Feature Gate (`+openshift:enable:FeatureGate=X`)
   - Examples and semantics
   
3. ✅ **docs/feature-gates.md** - Feature gate user guide
   - How to add/promote gates
   - Real-world use cases
   - Best practices

4. ✅ **docs/workflow.md** - End-to-end workflow
   - HyperShift bump process
   - Field curation workflow

5. ✅ **CLAUDE.md** - Project guidance
   - Development workflow
   - Critical patterns

**What's missing**:

❌ **CI Troubleshooting Guide** - How to fix common CI failures:
- "field_metadata.go is out of date" → `make generate-registry`
- "openapi.json is out of date" → `make generate-openapi`
- "field X is missing write-mode marker" → Add marker to field
- How to choose correct write-mode (mutable/immutable/service-set)

❌ **Marker Reference Card** - Quick reference:
- When to use each marker
- Valid values for each marker
- Common combinations
- Examples

**Recommendation**: Create `docs/TROUBLESHOOTING.md` with CI error solutions

---

## Additional CI Checks (Beyond AC)

### Registry JSON Validation

**Current**: JSON registry is generated but not explicitly validated

**What we have**:
- `pkg/registry/field_metadata.json` generated alongside `.go` file
- Used by passthrough generator for marker preservation
- Not separately validated in CI (relies on diff check)

**Recommendation**: No additional check needed - diff check covers this

---

### OpenAPI Schema Validation

**Current**: Schema generation verified but not validated

**What we have** (`.github/workflows/ci.yml` lines 80-87):
```yaml
- name: Verify OpenAPI schema is up to date
  run: |
    ./bin/openapi-gen --input-dirs=./api/v1alpha1 --output-file=/tmp/openapi.json
    diff openapi/openapi.json /tmp/openapi.json || (
      echo "Error: openapi/openapi.json is out of date"
      echo "Run: make generate-openapi"
      exit 1
    )
```

**Additional validation opportunity**:
- Validate OpenAPI schema against OpenAPI 3.0 spec
- Check for common schema errors

**Implementation**:
```yaml
- name: Validate OpenAPI schema
  run: |
    npm install -g @apidevtools/swagger-cli
    swagger-cli validate openapi/openapi.json
```

**Recommendation**: Nice-to-have but not critical for POC

---

### HyperShift Version Bump Tests

**Current**: Two comprehensive tests

**What we have**:
1. `test-hypershift-bump` - Tests bump to v0.1.72 (specific version)
2. `test-hypershift-bump-latest` - Tests bump to latest version

**What they validate**:
- go.mod updated correctly
- Passthrough types regenerated
- `hostedclusterspec.passthrough.go` created (not `zz_generated.passthrough.go`)
- Field registry regenerated
- OpenAPI schema regenerated
- Only expected files changed
- Tests still pass

**Coverage**: ✅ Excellent - validates entire workflow

---

## Summary

### Acceptance Criteria Status:

| AC | Status | Notes |
|----|--------|-------|
| 1. CI job validates markers | ✅ COMPLETE | marker-scanner --validate in verify job |
| 2. Build fails if missing | ✅ COMPLETE | Diff check + validation exit codes |
| 3. Clear error messages | ⚠️ MOSTLY COMPLETE | Good messages, could add more context |
| 4. Runs on every PR | ✅ COMPLETE | Runs on all PRs to main (exceeds requirement) |
| 5. Developer documentation | ⚠️ PARTIAL | Have design docs, missing troubleshooting guide |

### Recommendation: Close with Follow-up

**Overall Assessment**: **ROSAENG-61386 is substantially complete**

- Core validation is working (AC 1-4)
- Error messages are adequate for POC (AC 3)
- Documentation exists but could be enhanced (AC 5)

**Proposed Action**:

1. **Close ROSAENG-61386** as complete
   - 4/5 AC fully met
   - 1/5 AC partially met (sufficient for POC)

2. **Create follow-up task** (optional, low priority):
   - "Create CI troubleshooting guide" (docs/TROUBLESHOOTING.md)
   - "Add OpenAPI schema validation to CI"
   - Link to ROSAENG-61394 (Developer Documentation epic)

---

## Verification Commands

**Test current validation**:
```bash
# Should pass (all fields have markers)
./bin/marker-scanner --input-dirs=./api/v1alpha1 --output-file=/tmp/test.go

# Check validation logic
grep -A 20 "func.*Validate" pkg/markers/scanner.go

# See what CI runs
cat .github/workflows/ci.yml
```

**Simulate CI failure**:
```bash
# Modify a field to remove its marker
# Run marker-scanner
./bin/marker-scanner --input-dirs=./api/v1alpha1 --output-file=/tmp/test.go

# Should fail validation
echo $?  # Non-zero exit code
```

---

## Recommendation Summary

✅ **Close ROSAENG-61386** as complete with these notes:
- Marker validation is working in CI
- Build fails appropriately when markers are missing
- Error messages are clear enough for developers
- Runs on every PR to main
- Documentation is sufficient for POC phase

📝 **Optional follow-up** (can be part of ROSAENG-61394):
- Create docs/TROUBLESHOOTING.md with CI error solutions
- Add marker reference card
- Add OpenAPI schema validation (nice-to-have)

**No additional CI tests are needed to close this story.**
