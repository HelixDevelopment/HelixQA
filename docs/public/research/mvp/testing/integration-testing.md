# Integration Testing Guide

## Overview

Integration tests validate interactions between components: database queries, API endpoint chains, and provider adapter integrations. They use real dependencies via test containers.

## Framework

- **Go standard library** `testing` package
- **testify** for assertions
- **testcontainers-go** for PostgreSQL and Redis containers
- **build tag** `//go:build integration` to separate from unit tests

## Test Containers

### PostgreSQL Container

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()

    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "postgres:16-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_DB":       "helix_test",
            "POSTGRES_USER":     "test",
            "POSTGRES_PASSWORD": "test",
        },
        WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)

    t.Cleanup(func() { container.Terminate(ctx) })

    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "5432")

    db, err := sql.Open("postgres", fmt.Sprintf("postgres://test:test@%s:%s/helix_test?sslmode=disable", host, port.Port()))
    require.NoError(t, err)

    runMigrations(t, db)
    return db
}
```

### Redis Container

```go
func setupTestRedis(t *testing.T) *redis.Client {
    t.Helper()

    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "redis:7-alpine",
        ExposedPorts: []string{"6379/tcp"},
        WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(10 * time.Second),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)

    t.Cleanup(func() { container.Terminate(ctx) })

    host, _ := container.Host(ctx)
    port, _ := container.MappedPort(ctx, "6379")

    client := redis.NewClient(&redis.Options{
        Addr: fmt.Sprintf("%s:%s", host, port.Port()),
    })

    return client
}
```

## Database Integration Tests

### Repository Tests

```go
//go:build integration

func TestUserRepository_Create(t *testing.T) {
    db := setupTestDB(t)
    repo := NewUserRepository(db)

    user := &User{
        ID:    uuid.New().String(),
        Email: "integration@test.com",
    }

    err := repo.Create(context.Background(), user)
    require.NoError(t, err)

    fetched, err := repo.GetByID(context.Background(), user.ID)
    require.NoError(t, err)
    assert.Equal(t, user.Email, fetched.Email)
}

func TestSubscriptionRepository_UpdateStatus(t *testing.T) {
    db := setupTestDB(t)
    repo := NewSubscriptionRepository(db)

    sub := createTestSubscription(t, db)

    err := repo.UpdateStatus(context.Background(), sub.ID, StatusCancelled)
    require.NoError(t, err)

    updated, err := repo.GetByID(context.Background(), sub.ID)
    require.NoError(t, err)
    assert.Equal(t, StatusCancelled, updated.Status)
}
```

## API Integration Tests

### Full Request Lifecycle

```go
//go:build integration

func TestOrderAPI_CreateAndRetrieve(t *testing.T) {
    db := setupTestDB(t)
    redis := setupTestRedis(t)
    router := setupTestRouter(db, redis)

    // Create order
    body := `{"product_id": "prod-1", "quantity": 2}`
    req := httptest.NewRequest("POST", "/api/v1/orders", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+getTestToken(t))

    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusCreated, w.Code)

    var createResp OrderResponse
    json.Unmarshal(w.Body.Bytes(), &createResp)

    // Retrieve order
    req = httptest.NewRequest("GET", "/api/v1/orders/"+createResp.ID, nil)
    req.Header.Set("Authorization", "Bearer "+getTestToken(t))

    w = httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)
}
```

## Provider Adapter Integration Tests

### Paddle Webhook Simulation

```go
//go:build integration

func TestPaddleWebhook_Processing(t *testing.T) {
    db := setupTestDB(t)
    redis := setupTestRedis(t)

    handler := NewWebhookHandler(db, redis)

    payload := loadTestPayload(t, "paddle_subscription_created.json")
    sig := computeHMAC(payload, testWebhookSecret)

    req := httptest.NewRequest("POST", "/webhooks/paddle", bytes.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Paddle-Signature", sig)

    w := httptest.NewRecorder()
    handler.HandlePaddle(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    // Verify subscription was created in database
    var count int
    db.QueryRow("SELECT COUNT(*) FROM subscriptions WHERE external_id = ?", "sub_123").Scan(&count)
    assert.Equal(t, 1, count)
}
```

## Running Integration Tests

```bash
# Run all integration tests
go test -race -tags=integration ./...

# Run specific integration test suite
go test -race -tags=integration -run TestUserRepository ./internal/repository/...

# Run with verbose output
go test -race -tags=integration -v ./internal/...

# Run with coverage
go test -race -tags=integration -coverprofile=integration_coverage.out ./...
```

## Best Practices

1. **Use build tags** — Separate integration tests from unit tests
2. **Clean up containers** — Always use `t.Cleanup()` for teardown
3. **Isolate test data** — Each test uses unique identifiers
4. **Test failure scenarios** — Invalid inputs, connection drops, timeouts
5. **Verify side effects** — Check database state, cache state, external calls
6. **Use realistic data** — Mirror production data patterns
7. **Set appropriate timeouts** — Container startup may take 30+ seconds
