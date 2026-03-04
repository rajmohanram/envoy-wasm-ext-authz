# CI/CD Documentation

This document describes the CI/CD setup for the envoy-wasm-ext-authz monorepo.

## Overview

The repository uses GitHub Actions for continuous integration and deployment, with separate workflows for:

- **CI**: Automated testing, linting, and building on every push/PR
- **Release**: Creating releases and publishing artifacts when tags are pushed
- **Dependabot**: Automated dependency updates

## Repository Structure

```
.github/
├── workflows/
│   ├── ci.yml              # Main CI workflow
│   └── release.yml         # Release workflow
├── dependabot.yml          # Dependency update configuration
├── CODEOWNERS              # Code ownership definitions
└── pull_request_template.md
```

## CI Workflow (ci.yml)

### Trigger Conditions

- Push to: `main`, `feat/*`, `fix/*` branches
- Pull requests to: `main`
- Manual dispatch via GitHub UI

### Path-Based Filtering

The workflow uses path filters to run only relevant jobs:

- **authz-service changes**: Triggers Go-related jobs
- **wasm-plugin changes**: Triggers Rust-related jobs
- **workflow changes**: Triggers all jobs

### Smart WASM Job Execution

**Important:** WASM jobs only run when `wasm-plugin/Cargo.toml` exists.

This allows you to:

- ✅ Work on authz-service first without CI failures
- ✅ Push code before wasm-plugin is ready
- ✅ WASM jobs automatically activate when you add Cargo.toml

The `check-wasm` job verifies Cargo.toml exists before running any Rust jobs.

**Status Messages:**

- With Cargo.toml: "✅ All CI jobs passed successfully (Go + WASM)"
- Without Cargo.toml: "✅ All Go CI jobs passed successfully (WASM skipped - no Cargo.toml found)"

### Job Matrix

#### Go Service (authz-service)

| Job            | Description     | Checks                                     |
| -------------- | --------------- | ------------------------------------------ |
| `authz-format` | Code formatting | `gofmt`                                    |
| `authz-lint`   | Linting         | `golangci-lint`                            |
| `authz-vet`    | Static analysis | `go vet`                                   |
| `authz-test`   | Unit tests      | `go test` with race detection              |
| `authz-build`  | Binary builds   | Multi-platform: linux/darwin × amd64/arm64 |
| `authz-docker` | Container build | Multi-arch Docker image                    |

#### Rust WASM Plugin (wasm-plugin)

| Job           | Description     | Checks                 |
| ------------- | --------------- | ---------------------- |
| `wasm-format` | Code formatting | `cargo fmt`            |
| `wasm-clippy` | Linting         | `cargo clippy`         |
| `wasm-test`   | Unit tests      | `cargo test`           |
| `wasm-build`  | WASM build      | `wasm32-wasip1` target |

### Build Matrix

**Go Binaries:**

- linux/amd64
- linux/arm64

**Docker Platforms:**

- linux/amd64
- linux/arm64

### Artifacts

All builds produce artifacts retained for 7 days:

- Go binaries: `authz-service-{os}-{arch}`
- WASM module: `wasm-plugin`

## Release Workflow (release.yml)

### Trigger Conditions

- Tags matching `v*.*.*` (e.g., v1.0.0)
- Manual dispatch with custom tag input

### Build Process

1. **Build Go Service**
   - Cross-compile for all platforms
   - Create tarballs for distribution
   - Version embedded via ldflags

2. **Build Docker Images**
   - Multi-arch build (amd64/arm64)
   - Push to GitHub Container Registry
   - Tags: semver, major.minor, major, sha

3. **Build WASM Plugin**
   - Compile to wasm32-wasip1 target
   - Release-optimized build

4. **Create GitHub Release**
   - Attach all build artifacts
   - Generate release notes automatically
   - Mark pre-releases (alpha/beta/rc)

### Container Registry

Images published to: `ghcr.io/rajmohanram/envoy-wasm-ext-authz/authz-service`

Tag strategy:

```
v1.2.3 -> 1.2.3, 1.2, 1, sha-abc1234
```

## Dependabot Configuration

Automated updates for:

- **Go modules** (authz-service): Weekly on Monday
- **Cargo dependencies** (wasm-plugin): Weekly on Monday
- **GitHub Actions**: Weekly on Monday
- **Docker base images**: Weekly on Monday

PRs limited to 5 per ecosystem to avoid noise.

## golangci-lint Configuration

Located at: `authz-service/.golangci.yml`

**Enabled Linters:**

- errcheck (unchecked errors)
- gosimple (code simplification)
- govet (go vet checks)
- staticcheck (static analysis)
- gosec (security issues)
- revive (golint replacement)
- gocritic (opinionated checks)
- misspell (spelling)
- bodyclose (HTTP body leaks)
- And more...

## Local Development

### Running CI Checks Locally

**Go Service:**

```bash
cd authz-service

# Format code
make format

# Run linter
make lint

# Run vet
make vet

# Run tests
make test

# Build binaries
make build

# All checks
make check
```

**WASM Plugin (future):**

```bash
cd wasm-plugin

# Format code
cargo fmt

# Run linter
cargo clippy --all-targets --all-features

# Run tests
cargo test

# Build WASM
cargo build --target wasm32-wasip1 --release
```

## Pull Request Process

1. Create feature branch: `feat/my-feature` or `fix/my-fix`
2. Make changes and commit
3. Push and create PR (template auto-fills)
4. CI runs automatically:
   - Format checks
   - Linting
   - Tests
   - Builds
5. Review and merge (requires passing CI)

## Code Ownership

Defined in `.github/CODEOWNERS`:

- All changes: @rajmohanram
- Specific ownership for authz-service and wasm-plugin

## Troubleshooting

### CI Failures

**Format check fails:**

```bash
cd authz-service
make format
git add .
git commit --amend --no-edit
```

**Lint errors:**

```bash
cd authz-service
make lint
# Fix reported issues
```

**Tests fail:**

```bash
cd authz-service
make test
# Debug failing tests
```

### Local Testing

To verify CI will pass before pushing:

```bash
cd authz-service
make check  # Runs format-check, vet, lint, test
make build  # Verifies build
```

## Best Practices

1. **Always run `make check` before pushing**
2. **Keep PRs focused and small**
3. **Update tests with code changes**
4. **Use semantic commit messages**
5. **Wait for CI to pass before merging**
6. **Review Dependabot PRs promptly**

## GitHub Actions Secrets

Required for release workflow:

- `GITHUB_TOKEN` (auto-provided)

Optional for external services:

- `CODECOV_TOKEN` (for coverage reporting)

## Future Improvements

- [ ] Add integration tests
- [ ] Add E2E tests with Envoy
- [ ] Add performance benchmarks
- [ ] Add security scanning (Snyk, Trivy)
- [ ] Add automated changelog generation
- [ ] Add deployment workflows (staging/production)
