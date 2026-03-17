# Contributing to HelixQA

Thank you for your interest in contributing to HelixQA.

## Getting Started

1. Clone the repository
2. Ensure Go 1.24+ is installed
3. Ensure sibling modules exist:
   - `../Challenges` (digital.vasic.challenges)
   - `../Containers` (digital.vasic.containers)
4. Run `go build ./...` to verify the setup
5. Run `go test ./... -race -count=1` to verify tests pass

## Development Process

1. Create a feature branch from `main`
2. Make your changes
3. Ensure all tests pass: `go test ./... -race -count=1`
4. Ensure vet passes: `go vet ./...`
5. Ensure code is formatted: `gofmt -w .`
6. Submit a pull request

## Code Standards

- Follow standard Go conventions
- Add SPDX license headers to all `.go` files
- Write tests for all new functionality
- Use `testify` for assertions
- Keep line length under 100 characters
- Wrap errors with context: `fmt.Errorf("description: %w", err)`

## Commit Messages

Use conventional commits:

- `feat(scope): description` -- New features
- `fix(scope): description` -- Bug fixes
- `test(scope): description` -- Test additions/changes
- `docs(scope): description` -- Documentation
- `refactor(scope): description` -- Code restructuring

## Testing

- Unit tests: `go test ./... -count=1`
- Race detection: `go test ./... -race -count=1`
- Coverage: `make test-cover`

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
