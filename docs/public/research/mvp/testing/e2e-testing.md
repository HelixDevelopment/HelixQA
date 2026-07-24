# End-to-End Testing Guide

## Overview

E2E tests validate complete user workflows across the full application stack: browser UI, API, database, and external providers. They simulate real user behavior.

## Framework

- **Cypress** or **Playwright** for web portal testing
- **API contract testing** with Pact or Dredd
- **SDK smoke tests** for client libraries

## Web Portal E2E Tests

### Cypress Setup

```javascript
// cypress.config.js
module.exports = defineConfig({
  e2e: {
    baseUrl: 'http://localhost:8080',
    viewportWidth: 1280,
    viewportHeight: 720,
    video: true,
    screenshotsFolder: 'cypress/screenshots',
    videosFolder: 'cypress/videos',
    supportFile: 'cypress/support/e2e.js',
    specPattern: 'cypress/e2e/**/*.cy.{js,ts}',
  },
});
```

### Login Flow Test

```javascript
// cypress/e2e/auth/login.cy.js
describe('Login', () => {
  it('should login with valid credentials', () => {
    cy.visit('/login');
    cy.get('[data-testid="email"]').type('admin@helix.dev');
    cy.get('[data-testid="password"]').type('TestPassword123!');
    cy.get('[data-testid="login-button"]').click();

    cy.url().should('include', '/dashboard');
    cy.get('[data-testid="user-menu"]').should('contain', 'admin@helix.dev');
  });

  it('should show error for invalid credentials', () => {
    cy.visit('/login');
    cy.get('[data-testid="email"]').type('wrong@example.com');
    cy.get('[data-testid="password"]').type('WrongPassword');
    cy.get('[data-testid="login-button"]').click();

    cy.get('[data-testid="error-message"]').should('be.visible');
    cy.url().should('include', '/login');
  });
});
```

### Merchant Management Flow

```javascript
// cypress/e2e/merchants/management.cy.js
describe('Merchant Management', () => {
  beforeEach(() => {
    cy.login('admin@helix.dev', 'TestPassword123!');
  });

  it('should create a new merchant', () => {
    cy.visit('/merchants');
    cy.get('[data-testid="create-merchant"]').click();

    cy.get('[data-testid="merchant-name"]').type('Acme Corp');
    cy.get('[data-testid="merchant-email"]').type('billing@acme.com');
    cy.get('[data-testid="provider-select"]').select('paddle');
    cy.get('[data-testid="submit"]').click();

    cy.get('[data-testid="success-toast"]').should('be.visible');
    cy.get('[data-testid="merchant-list"]').should('contain', 'Acme Corp');
  });

  it('should view merchant details', () => {
    cy.visit('/merchants');
    cy.get('[data-testid="merchant-row"]').first().click();

    cy.get('[data-testid="merchant-details"]').should('be.visible');
    cy.get('[data-testid="subscription-status"]').should('exist');
    cy.get('[data-testid="transaction-history"]').should('exist');
  });
});
```

### Transaction Monitoring Flow

```javascript
// cypress/e2e/transactions/monitoring.cy.js
describe('Transaction Monitoring', () => {
  beforeEach(() => {
    cy.login('admin@helix.dev', 'TestPassword123!');
  });

  it('should display transaction list with filters', () => {
    cy.visit('/transactions');

    cy.get('[data-testid="transaction-table"]').should('be.visible');
    cy.get('[data-testid="filter-status"]').select('completed');
    cy.get('[data-testid="apply-filters"]').click();

    cy.get('[data-testid="transaction-row"]').each(($row) => {
      cy.wrap($row).find('[data-testid="status"]').should('contain', 'completed');
    });
  });

  it('should export transactions to CSV', () => {
    cy.visit('/transactions');
    cy.get('[data-testid="export-csv"]').click();

    cy.readFile('cypress/downloads/transactions.csv').then((content) => {
      expect(content).to.contain('ID,Amount,Status,Date');
    });
  });
});
```

## API Contract Tests

### OpenAPI Contract Validation

```yaml
# api/openapi/contract-test.yaml
openapi: 3.0.3
info:
  title: Helix Seller API Contract
  version: 1.0.0
paths:
  /api/v1/subscriptions:
    get:
      summary: List subscriptions
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: '#/components/schemas/Subscription'
                  pagination:
                    $ref: '#/components/schemas/Pagination'
components:
  schemas:
    Subscription:
      type: object
      required:
        - id
        - status
        - provider
      properties:
        id:
          type: string
          format: uuid
        status:
          type: string
          enum: [active, past_due, cancelled, expired]
        provider:
          type: string
          enum: [paddle, lemon_squeezy]
```

## SDK Smoke Tests

```go
// sdk/smoke_test.go
func TestSDK_CreateCheckout(t *testing.T) {
    client := helix.NewClient(os.Getenv("HELIX_API_KEY"))

    checkout, err := client.Checkouts.Create(context.Background(), &helix.CheckoutRequest{
        ProductID: "prod_test",
        Email:     "smoke@test.com",
    })

    require.NoError(t, err)
    assert.NotEmpty(t, checkout.ID)
    assert.NotEmpty(t, checkout.URL)
}

func TestSDK_ListSubscriptions(t *testing.T) {
    client := helix.NewClient(os.Getenv("HELIX_API_KEY"))

    subs, err := client.Subscriptions.List(context.Background(), nil)

    require.NoError(t, err)
    assert.NotNil(t, subs)
}
```

## Running E2E Tests

```bash
# Cypress
npx cypress run                           # Headless
npx cypress open                          # Interactive

# Playwright
npx playwright test                       # All tests
npx playwright test --grep "Login"        # Specific test
npx playwright show-report                # View report

# API contract tests
dredd init                                # Initialize
dredd run api/openapi/spec.yaml http://localhost:8080

# SDK smoke tests
go test -tags=smoke ./sdk/...
```

## Best Practices

1. **Test real user flows** — Sign up, login, complete tasks, logout
2. **Use data-testid attributes** — Stable selectors for testing
3. **Mock external providers** — Use WireMock or MSW for payment providers
4. **Handle async operations** — Use retries and waits for eventual consistency
5. **Screenshots on failure** — Capture evidence for debugging
6. **Isolate test data** — Use unique identifiers per test run
7. **Run in CI** — Execute E2E tests in staging environment before release
