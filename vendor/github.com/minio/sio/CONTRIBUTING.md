# Contributing to sio

Thank you for your interest in contributing to `sio`! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful and professional in all interactions. We're here to build great software together.

## Getting Started

### Prerequisites

- Go 1.24 or later
- Git
- golangci-lint (for linting)
- Basic understanding of cryptography (helpful but not required)

### Development Setup

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/sio.git
   cd sio
   ```
3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/minio/sio.git
   ```
4. Install dependencies:
   ```bash
   go mod download
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

Use prefixes:

- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `perf/` - Performance improvements
- `refactor/` - Code refactoring

### 2. Make Changes

Follow the coding standards below and ensure your code:

- Is well-tested
- Includes documentation
- Passes all existing tests
- Doesn't introduce security vulnerabilities

### 3. Run Tests

```bash
# Run all tests
go test -v ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 4. Run Linters

```bash
# Run gofmt
gofmt -s -w .

# Run go vet
go vet ./...

# Run golangci-lint
golangci-lint run
```

### 5. Commit Changes

Write clear commit messages following this format:

```
Short summary (50 chars or less)

More detailed explanation if needed. Wrap at 72 characters.
Explain the problem this commit solves and why you chose
this solution.

Fixes #123
```

### 6. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub with:

- Clear description of the changes
- Reference to related issues
- Screenshots/examples if applicable

## Coding Standards

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Keep functions small and focused
- Write self-documenting code with clear names

### Error Handling

- Always check errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors, don't panic (except for truly unrecoverable situations)
- Use typed errors for API boundaries

### Testing

- Write table-driven tests where appropriate
- Test edge cases and error conditions
- Aim for >80% code coverage
- Use meaningful test names: `TestFunctionName_Scenario`

### Documentation

- Add godoc comments for all exported types, functions, and constants
- Include usage examples for complex functionality
- Update README.md if adding user-facing features
- Document security considerations

### Security

- Never commit secrets or sensitive data
- Be cautious with cryptographic code
- Consider timing attacks and side channels
- Add tests for security-critical code paths

## Pull Request Process

1. **Update documentation** - README.md, godoc comments, etc.
2. **Add tests** - New code must include tests
3. **Pass CI checks** - All tests and linters must pass
4. **Get reviewed** - At least one maintainer must approve
5. **Squash commits** - Keep history clean with meaningful commits

### PR Checklist

- [ ] Tests added/updated and passing
- [ ] Documentation updated
- [ ] golangci-lint passes
- [ ] No breaking changes (or documented in PR)
- [ ] Commit messages are clear
- [ ] Branch is up to date with master

## Testing Guidelines

### Unit Tests

Focus on:

- Individual function behavior
- Edge cases (empty inputs, max size, etc.)
- Error conditions
- Different cipher suites

### Integration Tests

Focus on:

- End-to-end encryption/decryption
- Different stream sizes
- Reader/Writer interfaces
- Version compatibility

### Fuzzing

For cryptographic code, consider adding fuzz tests:

```go
func FuzzDecrypt(f *testing.F) {
    // Add corpus and fuzz implementation
}
```

## Benchmarking

When making performance-related changes:

```bash
# Run benchmarks
go test -bench=. -benchmem

# Compare before/after
go test -bench=. -benchmem > old.txt
# make changes
go test -bench=. -benchmem > new.txt
benchstat old.txt new.txt
```

## Release Process

(For maintainers)

1. Update version numbers and CHANGELOG
2. Run full test suite including race detector
3. Tag release: `git tag v1.x.x`
4. Push tag: `git push origin v1.x.x`
5. GitHub Actions will create the release

## Common Tasks

### Adding a New Function

1. Implement the function
2. Add godoc comment
3. Add unit tests
4. Add example test
5. Update README if user-facing

### Fixing a Bug

1. Add a test that reproduces the bug
2. Fix the bug
3. Verify the test now passes
4. Consider adding additional edge case tests

### Improving Performance

1. Add benchmark before changes
2. Make improvements
3. Run benchmark again
4. Include benchmark results in PR
5. Verify no functionality regression

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: Open a GitHub Issue
- **Security**: Email security@min.io
- **Chat**: Join MinIO Slack (link in README)

## Recognition

Contributors will be:

- Listed in release notes
- Mentioned in commit history
- Added to CONTRIBUTORS file (if significant contribution)

Thank you for contributing to sio!
