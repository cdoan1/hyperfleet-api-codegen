# Contributing to HyperFleet API Codegen

Thank you for contributing to HyperFleet API Codegen!

## Development Workflow

### Before Committing Code

**Always run before committing:**

```bash
make all
```

This runs linting and tests with race detection. All checks must pass before pushing.

The full pre-push verification suite:

```bash
make verify
```

This runs:
- `gofmt` - Code formatting
- `go vet` - Static analysis
- Unit tests with race detection

You can also run checks individually:

```bash
make fmt          # Format code
make vet          # Run static analysis
make test         # Run unit tests
make lint         # Run golangci-lint (required before commit)
```

**Note**: `make lint` is required and will catch issues like unchecked errors that may not be caught by `go vet`.

### Code Generation

After modifying CRD types or markers, regenerate artifacts:

```bash
make generate
```

This runs all generators in order:
1. Field metadata registry from markers
2. Passthrough types from HyperShift CRDs
3. CRD manifests
4. OpenAPI schema

### Running Individual Generators

```bash
make generate-registry      # Generate field metadata registry
make generate-passthrough   # Generate passthrough types
make manifests              # Generate CRD manifests
make generate-openapi       # Generate OpenAPI schema
```

## Adding Markers to Types

When adding or modifying HyperFleet CRD types, ensure all passthrough fields have:

1. **Visibility marker** (if hidden): `+k8s:openapi-gen=false`
2. **Write mode marker** (required): `+hyperfleet:write-mode=mutable|immutable|service-set`
3. **Feature gate marker** (if gated): `+openshift:enable:FeatureGate=FeatureName`

Example:

```go
type ClusterSpec struct {
    // Customer can set and change
    // +hyperfleet:write-mode=mutable
    DeleteProtection *DeleteProtection `json:"deleteProtection,omitempty"`

    // Customer sets on create, cannot change
    // +hyperfleet:write-mode=immutable
    Name string `json:"name"`

    // Platform sets, customer cannot see
    // +k8s:openapi-gen=false
    // +hyperfleet:write-mode=service-set
    AccountID string `json:"accountId"`
}
```

CI will fail if passthrough fields are missing required markers.

## HyperShift Version Updates

When bumping the HyperShift dependency:

1. Update `go.mod`:
   ```bash
   go get github.com/openshift/hypershift@<version>
   go mod tidy
   ```

2. Regenerate passthrough types:
   ```bash
   make generate-passthrough
   ```

3. Review the diff for new/removed/changed fields

4. Add appropriate markers to new fields (defaults are hidden + service-set)

5. Regenerate all artifacts:
   ```bash
   make generate
   ```

6. Run verification:
   ```bash
   make verify
   ```

## Testing

### Unit Tests

```bash
make test                # Run all tests
make test-coverage       # Run tests and show coverage report
```

### Writing Tests

- Test files should be named `*_test.go`
- Place tests alongside the code they test
- Use table-driven tests for multiple test cases
- Run tests with race detection enabled (included in `make test`)

## Code Style

- Follow standard Go conventions
- Run `make fmt` before committing
- Avoid unnecessary complexity - don't add abstractions beyond what's needed
- Write comments only when the "why" is non-obvious
- Keep functions focused and small

## Pull Requests

1. Create a feature branch from `main`
2. Make your changes
3. Run `make verify` to ensure all checks pass
4. Run `make generate` if you modified types or markers
5. Commit with clear, descriptive messages
6. Push and create a PR
7. Ensure CI passes

## Getting Help

- **Design Document**: See `docs/api-management.md` for architecture details
- **Development Guide**: See `CLAUDE.md` for repository-specific guidance
- **Issues**: Check existing issues or create a new one
