# Security Testing Guide

## Overview

Security testing validates the application against common vulnerabilities and attack vectors. Tests cover authentication, authorization, input validation, and infrastructure security.

## Testing Areas

### 1. Authentication Testing

```go
func TestAuth_UnauthorizedAccess(t *testing.T) {
    tests := []struct {
        name     string
        endpoint string
        method   string
        token    string
        expected int
    }{
        {"no token", "/api/v1/subscriptions", "GET", "", 401},
        {"invalid token", "/api/v1/subscriptions", "GET", "invalid-token", 401},
        {"expired token", "/api/v1/subscriptions", "GET", generateExpiredToken(), 401},
        {"malformed token", "/api/v1/subscriptions", "GET", "Bearer ", 401},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(tt.method, tt.endpoint, nil)
            if tt.token != "" {
                req.Header.Set("Authorization", "Bearer "+tt.token)
            }

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expected, w.Code)
        })
    }
}
```

### 2. Authorization Testing

```go
func TestAuth_RoleBasedAccess(t *testing.T) {
    tests := []struct {
        name     string
        role     string
        endpoint string
        expected int
    }={
        {"root admin can manage merchants", "root_admin", "/api/v1/merchants", 200},
        {"account admin cannot manage users", "account_admin", "/api/v1/users", 403},
        {"standard user cannot access admin", "standard_user", "/api/v1/admin/config", 403},
        {"account admin can view own merchants", "account_admin", "/api/v1/merchants", 200},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            token := generateTokenWithRole(tt.role)
            req := httptest.NewRequest("GET", tt.endpoint, nil)
            req.Header.Set("Authorization", "Bearer "+token)

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            assert.Equal(t, tt.expected, w.Code)
        })
    }
}
```

### 3. Input Validation Testing

```go
func TestInputValidation_SQLInjection(t *testing.T) {
    payloads := []string{
        "' OR '1'='1",
        "'; DROP TABLE users; --",
        "1' UNION SELECT * FROM users --",
        "admin'--",
    }

    for _, payload := range payloads {
        t.Run("SQL injection: "+payload, func(t *testing.T) {
            body := fmt.Sprintf(`{"email": "%s"}`, payload)
            req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
            req.Header.Set("Content-Type", "application/json")

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            // Should not return 500 (server error) or expose DB errors
            assert.NotEqual(t, http.StatusInternalServerError, w.Code)
            assert.NotContains(t, w.Body.String(), "syntax error")
            assert.NotContains(t, w.Body.String(), "sql")
        })
    }
}

func TestInputValidation_XSS(t *testing.T) {
    payloads := []string{
        "<script>alert('xss')</script>",
        "<img src=x onerror=alert(1)>",
        "javascript:alert(1)",
        "<svg onload=alert(1)>",
    }

    for _, payload := range payloads {
        t.Run("XSS: "+payload, func(t *testing.T) {
            body := fmt.Sprintf(`{"name": "%s"}`, payload)
            req := httptest.NewRequest("POST", "/api/v1/merchants", strings.NewReader(body))
            req.Header.Set("Content-Type", "application/json")
            req.Header.Set("Authorization", "Bearer "+getTestToken(t))

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            // Response should escape HTML entities
            if w.Code == http.StatusOK || w.Code == http.StatusCreated {
                assert.NotContains(t, w.Body.String(), "<script>")
                assert.Contains(t, w.Body.String(), "&lt;script&gt;")
            }
        })
    }
}

func TestInputValidation_BoundaryValues(t *testing.T) {
    tests := []struct {
        name  string
        input string
    }}{
        {"empty string", ""},
        {"very long string", strings.Repeat("a", 10000)},
        {"unicode", "测试用户🎉"},
        {"null bytes", "test\x00value"},
        {"newlines", "line1\nline2\rline3"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            body := fmt.Sprintf(`{"name": "%s"}`, tt.input)
            req := httptest.NewRequest("POST", "/api/v1/merchants", strings.NewReader(body))
            req.Header.Set("Content-Type", "application/json")
            req.Header.Set("Authorization", "Bearer "+getTestToken(t))

            w := httptest.NewRecorder()
            router.ServeHTTP(w, req)

            // Should handle gracefully, not crash
            assert.NotEqual(t, http.StatusInternalServerError, w.Code)
        })
    }
}
```

### 4. Webhook Security Testing

```go
func TestWebhook_SignatureVerification(t *testing.T) {
    tests := []struct {
        name      string
        payload   []byte
        signature string
        expected  int
    }={
        {"valid signature", validPayload, validSig, 200},
        {"invalid signature", validPayload, "invalid-sig", 401},
        {"missing signature", validPayload, "", 401},
        {"tampered payload", tamperedPayload, validSig, 401},
        {"empty payload", []byte{}, validSig, 401},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest("POST", "/webhooks/paddle", bytes.NewReader(tt.payload))
            req.Header.Set("Content-Type", "application/json")
            req.Header.Set("Paddle-Signature", tt.signature)

            w := httptest.NewRecorder()
            webhookHandler.HandlePaddle(w, req)

            assert.Equal(t, tt.expected, w.Code)
        })
    }
}
```

### 5. Rate Limiting Testing

```go
func TestRateLimiting(t *testing.T) {
    // Send 100 requests rapidly
    for i := 0; i < 100; i++ {
        req := httptest.NewRequest("GET", "/api/v1/health", nil)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)

        if i >= 50 { // After rate limit threshold
            if w.Code == http.StatusTooManyRequests {
                t.Logf("Rate limited at request %d", i)
                return
            }
        }
    }
    t.Error("Rate limiting did not trigger after 100 rapid requests")
}
```

### 6. Security Headers Testing

```go
func TestSecurityHeaders(t *testing.T) {
    req := httptest.NewRequest("GET", "/api/v1/health", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
    assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
    assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
    assert.Contains(t, w.Header().Get("Content-Security-Policy"), "default-src")
    assert.NotEmpty(t, w.Header().Get("Strict-Transport-Security"))
}
```

## Static Analysis Tools

### gosec — Go Security Scanner

```bash
# Install
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run on entire project
gosec ./...

# Run with specific rules
gosec -exclude=G104 ./...

# Generate JSON report
gosec -fmt=json -out=security-report.json ./...
```

### Dependency CVE Checking

```bash
# Go vulnerability database
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# npm audit (for frontend)
npm audit
npm audit fix

# Trivy (container scanning)
trivy image helix-seller:latest
```

### Secret Scanning

```bash
# gitleaks
gitleaks detect --source . --report-format json --report-path gitleaks-report.json

# detect-secrets
detect-secrets scan --all-files --exclude-files '\.go$' > .secrets.baseline
```

## DDoS Simulation

```bash
# Slowloris simulation (connection exhaustion)
slowhttptest -H -i 10 -r 200 -s 8192 -t https://localhost:8443

# HTTP flood
hping3 -S -p 443 --flood localhost
```

## Security Testing Checklist

- [ ] Authentication: Valid/invalid/expired tokens
- [ ] Authorization: Role-based access control
- [ ] Input validation: SQL injection, XSS, boundary values
- [ ] Webhook verification: Signature validation
- [ ] Rate limiting: Request throttling
- [ ] Security headers: CSP, HSTS, X-Frame-Options
- [ ] Static analysis: gosec, govulncheck
- [ ] Dependency scanning: CVE checking
- [ ] Secret scanning: No hardcoded credentials
- [ ] Container scanning: Image vulnerabilities
- [ ] HTTPS enforcement: TLS 1.3 only
- [ ] Error handling: No sensitive data in errors

## Running Security Tests

```bash
# Run all security tests
go test -race -tags=security ./internal/security/...

# Run static analysis
gosec ./...
govulncheck ./...

# Run secret scanning
gitleaks detect --source .

# Run dependency audit
npm audit
govulncheck ./...
```

## Best Practices

1. **Test early and often** — Security tests in every PR
2. **Automate scanning** — CI pipeline includes security checks
3. **Keep dependencies updated** — Regular dependency updates
4. **Use parameterized queries** — Never concatenate user input into SQL
5. **Validate all inputs** — Server-side validation for every endpoint
6. **Implement defense in depth** — Multiple security layers
7. **Log security events** — Audit trail for suspicious activity
8. **Regular penetration testing** — Quarterly external assessments
