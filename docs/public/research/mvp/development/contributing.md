# Contributing Guidelines

## Branch Naming

Use descriptive branch names with a type prefix:

| Prefix | Usage | Example |
|--------|-------|---------|
| `feat/` | New feature | `feat/order-creation` |
| `fix/` | Bug fix | `fix/payment-timeout` |
| `refactor/` | Code restructuring | `refactor/repository-layer` |
| `docs/` | Documentation | `docs/deployment-guide` |
| `test/` | Test additions/fixes | `test/order-service` |
| `chore/` | Maintenance tasks | `chore/dependency-update` |

```bash
# Create a feature branch
git checkout -b feat/my-new-feature

# Keep branch up to date
git fetch origin
git rebase origin/main
```

## Commit Message Format

Follow Conventional Commits:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `style` | Formatting, no code change |
| `refactor` | Code restructuring |
| `test` | Adding/fixing tests |
| `chore` | Build process, dependencies |
| `perf` | Performance improvement |

### Examples

```
feat(orders): add order creation endpoint

Implement POST /api/v1/orders with validation and
payment provider integration.

Closes #123
```

```
fix(stripe): handle webhook signature verification failure

Log warning and return 200 to prevent Stripe retries
while alerting via monitoring.
```

```
docs(deployment): add Hetzner infrastructure guide

Document hardware requirements, networking, and
container architecture.
```

### Rules

- Subject line: imperative mood, lowercase, no period, max 72 chars
- Body: wrap at 72 chars, explain what and why (not how)
- Reference issues and PRs in footer

## Pull Request Process

### 1. Create Your Branch

```bash
git checkout -b feat/my-feature main
```

### 2. Make Changes

- Write code following project style
- Add tests for new functionality
- Update documentation if needed
- Ensure all tests pass

### 3. Prepare Your Commit

```bash
# Stage specific files
git add internal/service/order.go
git add internal/service/order_test.go

# Or stage all changes
git add .

# Commit with conventional format
git commit -m "feat(orders): add order status tracking"
```

### 4. Push and Create PR

```bash
git push origin feat/my-feature
```

Create a Pull Request targeting `main`.

### 5. PR Description

```markdown
## Summary
Brief description of changes.

## Changes
- What was changed
- Why it was changed
- How it works

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests pass
- [ ] Manual testing performed (if applicable)

## Related Issues
Closes #123
```

### 6. Review and Merge

- Address review feedback
- Ensure CI passes
- Squash-merge to keep history clean

## Code Review Requirements

### For Authors

- Self-review before requesting review
- Keep PRs focused (one logical change)
- Write clear PR descriptions
- Respond to feedback promptly

### For Reviewers

- Review within 24 hours
- Focus on correctness, security, performance
- Check test coverage
- Verify documentation updates

### Review Checklist

- [ ] Code follows project style
- [ ] Tests are comprehensive
- [ ] No security vulnerabilities
- [ ] No performance regressions
- [ ] Documentation updated
- [ ] Error handling is proper
- [ ] Logging is appropriate
- [ ] No hardcoded values
- [ ] Database changes are backward-compatible

## Testing Requirements

### Unit Tests

- All new functions must have tests
- Test edge cases and error paths
- Use table-driven tests
- Aim for >80% coverage on new code

```bash
# Check coverage
make test-cover
go tool cover -func=coverage.out
```

### Integration Tests

- Test database interactions
- Test external service integrations
- Use test containers when possible

### Test Naming Convention

```go
func Test_FunctionName_Scenario_ExpectedResult(t *testing.T)
// Example:
func TestCreateOrder_InvalidInput_ReturnsError(t *testing.T)
func TestCreateOrder_ValidInput_CreatesOrder(t *testing.T)
func TestCreateOrder_PaymentFails_ReturnsError(t *testing.T)
```

## Documentation Requirements

### Code Documentation

- All exported functions/types need doc comments
- Complex algorithms need inline comments
- Package-level doc comments for new packages

### README Updates

- Update README.md if adding new features
- Add setup instructions for new dependencies
- Document new environment variables

### API Documentation

- Update API specs for new endpoints
- Include request/response examples
- Document error responses

## Development Workflow

### Daily Development

```bash
# Start your day
git pull origin main
make deps-up
make test

# Create feature branch
git checkout -b feat/my-feature

# Develop
# ... write code, tests ...

# Validate
make lint
make test

# Commit and push
git add .
git commit -m "feat(scope): description"
git push origin feat/my-feature

# Create PR, get review, merge
```

### Keeping Branches Clean

```bash
# Rebase on main regularly
git fetch origin
git rebase origin/main

# Resolve conflicts if any
# Continue rebase
git rebase --continue

# Force push (only for your own branches)
git push origin feat/my-feature --force-with-lease
```

## Issue Templates

### Bug Report

```markdown
## Description
Clear description of the bug.

## Steps to Reproduce
1. Step one
2. Step two
3. Step three

## Expected Behavior
What should happen.

## Actual Behavior
What actually happens.

## Environment
- Go version:
- OS:
- Database version:
```

### Feature Request

```markdown
## Description
Clear description of the feature.

## Use Case
Why is this feature needed?

## Proposed Solution
How it should work.

## Alternatives Considered
Other approaches considered.
```
