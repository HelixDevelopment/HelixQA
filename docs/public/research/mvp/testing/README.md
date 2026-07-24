# Helix Seller — Testing Strategy Overview

## Philosophy

Testing is mandatory and follows Test-Driven Development (TDD). Every feature begins with a failing test (RED), is implemented to pass (GREEN), then improved through refactoring while maintaining passing tests (REFACTOR).

## Test Types

| Type | Scope | Framework | When |
|------|-------|-----------|------|
| Unit | Individual functions/methods | Go stdlib + testify | Every commit |
| Integration | Component interactions | Go stdlib + testify + testcontainers | Every PR |
| End-to-End | Full user workflows | Cypress / Playwright | Pre-release |
| Contract | API compatibility | Pact / Dredd | API changes |
| Load | Performance under stress | k6 | Pre-release / weekly |
| Security | Vulnerability detection | gosec / OWASP ZAP | Every PR / monthly |

## Coverage Requirements

- **Minimum coverage:** 80%
- **New code coverage:** 90%
- **Critical paths:** 95% (authentication, payment processing, webhooks)
- **Coverage gate:** SonarQube blocks merge if coverage drops below threshold

## Test Execution

```bash
# Run all tests with race detection
go test -race ./...

# Run tests with coverage report
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run specific test suite
go test -race -run TestSubscription ./internal/service/...

# Run integration tests (requires testcontainers)
go test -race -tags=integration ./...
```

## CI/CD Integration

Tests run automatically in the CI pipeline:

1. **Unit tests** — Every push
2. **Integration tests** — Every PR
3. **E2E tests** — Pre-release branches
4. **Load tests** — Weekly + pre-release
5. **Security scans** — Every PR (static) + monthly (dynamic)

## Test Data Management

- Test data is isolated per test run
- No shared state between test suites
- Factories generate consistent test data
- Sensitive data never used in tests (use fixtures)

## Documentation Structure

- [Unit Testing Guide](./unit-testing.md)
- [Integration Testing Guide](./integration-testing.md)
- [E2E Testing Guide](./e2e-testing.md)
- [Load Testing Guide](./load-testing.md)
- [Security Testing Guide](./security-testing.md)
