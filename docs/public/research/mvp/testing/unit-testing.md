# Unit Testing Guide

## Overview

Unit tests validate individual functions, methods, and components in isolation. They form the foundation of the test pyramid and must run fast (< 10ms per test).

## Framework

- **Go standard library** `testing` package
- **testify** for assertions and mocking
- **go-sqlmock** for database mocking
- **miniredis** for Redis mocking

## Patterns

### Table-Driven Tests

```go
func TestCalculateSubscriptionPrice(t *testing.T) {
    tests := []struct {
        name     string
        base     float64
        discount float64
        expected float64
    }{
        {"no discount", 100.0, 0, 100.0},
        {"10% discount", 100.0, 0.10, 90.0},
        {"full discount", 100.0, 1.0, 0.0},
        {"invalid discount", 100.0, -0.5, 100.0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := CalculateSubscriptionPrice(tt.base, tt.discount)
            assert.Equal(t, tt.expected, got)
        })
    }
}
```

### Mocking Database Access

```go
func TestGetUserByID(t *testing.T) {
    db, mock, err := sqlmock.New()
    require.NoError(t, err)
    defer db.Close()

    rows := sqlmock.NewRows([]string{"id", "email", "created_at"}).
        AddRow("uuid-1", "user@example.com", time.Now())

    mock.ExpectQuery("SELECT (.+) FROM users WHERE id = ?").
        WithArgs("uuid-1").
        WillReturnRows(rows)

    repo := NewUserRepository(db)
    user, err := repo.GetByID(context.Background(), "uuid-1")

    require.NoError(t, err)
    assert.Equal(t, "user@example.com", user.Email)
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

### Mocking Redis

```go
func TestCacheUser(t *testing.T) {
    srv := miniredis.RunT(t)
    client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

    err := client.Set(context.Background(), "user:123", "data", time.Hour).Err()
    require.NoError(t, err)

    got, err := client.Get(context.Background(), "user:123").Result()
    require.NoError(t, err)
    assert.Equal(t, "data", got)
}
```

### HTTP Handler Tests

```go
func TestCreateOrderHandler(t *testing.T) {
    router := setupTestRouter()

    body := `{"product_id": "prod-1", "quantity": 2}`
    req := httptest.NewRequest("POST", "/api/v1/orders", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer test-token")

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusCreated, w.Code)

    var resp OrderResponse
    err := json.Unmarshal(w.Body.Bytes(), &resp)
    require.NoError(t, err)
    assert.NotEmpty(t, resp.ID)
}
```

## Test Data Factories

Create reusable factory functions for test data:

```go
func NewTestUser(t *testing.T) *User {
    t.Helper()
    return &User{
        ID:        uuid.New().String(),
        Email:     "test@example.com",
        CreatedAt: time.Now(),
    }
}

func NewTestSubscription(t *testing.T, userID string) *Subscription {
    t.Helper()
    return &Subscription{
        ID:                    uuid.New().String(),
        UserID:                userID,
        Provider:              "paddle",
        ExternalSubscriptionID: "sub_123",
        Status:                StatusActive,
        PlanID:                "plan_123",
        CurrentPeriodEnd:      time.Now().Add(30 * 24 * time.Hour),
    }
}
```

## Running Unit Tests

```bash
# Run all unit tests
go test -race ./...

# Run tests in a specific package
go test -race ./internal/service/...

# Run a specific test
go test -race -run TestGetUserByID ./internal/repository/...

# Run with verbose output
go test -race -v ./internal/...

# Run with coverage
go test -race -coverprofile=coverage.out ./internal/...
go tool cover -func=coverage.out
```

## Best Practices

1. **Isolation** — Each test is independent; no shared state
2. **Speed** — Unit tests complete in milliseconds
3. **Naming** — Use descriptive names: `TestFunctionName_Scenario_ExpectedResult`
4. **One assertion concept per test** — Keep tests focused
5. **Use `t.Helper()`** — In factory and setup functions
6. **No external dependencies** — Mock databases, HTTP calls, file systems
7. **Test edge cases** — Nil inputs, empty strings, boundary values
8. **Table-driven tests** — For functions with multiple input combinations
