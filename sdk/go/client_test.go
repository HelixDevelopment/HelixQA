package helixsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- helpers ---

func testClient(server *httptest.Server) *Client {
	return NewClient(server.URL)
}

func testClientWithKey(server *httptest.Server, key string) *Client {
	return NewClient(server.URL, WithAPIKey(key))
}

func jsonHandler(status int, body interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}
}

func badJSONHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(`{invalid json`))
	}
}

func methodHandler(t *testing.T, wantMethod, wantPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != wantMethod {
			t.Errorf("got method %s, want %s", r.Method, wantMethod)
		}
		if r.URL.Path != wantPath {
			t.Errorf("got path %s, want %s", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}
}

func bodyCaptureHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

var testUUID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
var testUUID2 = uuid.MustParse("22222222-2222-2222-2222-222222222222")

// =====================================================================
// NewClient & options
// =====================================================================

func TestNewClient(t *testing.T) {
	c := NewClient("http://example.com")
	if c.baseURL != "http://example.com" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if c.apiKey != "" {
		t.Errorf("apiKey = %q, want empty", c.apiKey)
	}
}

func TestWithAPIKey(t *testing.T) {
	c := NewClient("http://example.com", WithAPIKey("secret"))
	if c.apiKey != "secret" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "secret")
	}
}

func TestWithHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 5 * time.Second}
	c := NewClient("http://example.com", WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Error("custom HTTP client not set")
	}
}

func TestMultipleOptions(t *testing.T) {
	custom := &http.Client{Timeout: 1 * time.Second}
	c := NewClient("http://example.com", WithAPIKey("k"), WithHTTPClient(custom))
	if c.apiKey != "k" {
		t.Errorf("apiKey = %q", c.apiKey)
	}
	if c.httpClient != custom {
		t.Error("custom HTTP client not set")
	}
}

// =====================================================================
// Error type string methods
// =====================================================================

func TestAPIError_Error(t *testing.T) {
	e := &APIError{StatusCode: 404, Body: "not found"}
	got := e.Error()
	want := "API error 404: not found"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAPIError_ErrorEmptyBody(t *testing.T) {
	e := &APIError{StatusCode: 500, Body: ""}
	got := e.Error()
	want := "API error 500: "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNotFoundError_Error(t *testing.T) {
	e := &NotFoundError{Message: "merchant not found"}
	if e.Error() != "merchant not found" {
		t.Errorf("got %q", e.Error())
	}
}

func TestUnauthorizedError_Error(t *testing.T) {
	e := &UnauthorizedError{Message: "unauthorized access"}
	if e.Error() != "unauthorized access" {
		t.Errorf("got %q", e.Error())
	}
}

// =====================================================================
// do() method
// =====================================================================

func TestDo_NilBody(t *testing.T) {
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"ok"`))
	}))
	defer server.Close()

	c := testClient(server)
	data, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "GET" {
		t.Errorf("method = %q", gotMethod)
	}
	if string(data) != `"ok"` {
		t.Errorf("data = %q", string(data))
	}
}

func TestDo_WithBody(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "POST", "/test", map[string]string{"key": "val"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(receivedBody, `"key"`) {
		t.Errorf("body = %q", receivedBody)
	}
}

func TestDo_ContentJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := testClient(server)
	_, _ = c.do(context.Background(), "GET", "/test", nil)
}

func TestDo_AuthorizationHeaderWithKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := testClientWithKey(server, "my-api-key")
	_, _ = c.do(context.Background(), "GET", "/test", nil)

	if gotAuth != "Bearer my-api-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer my-api-key")
	}
}

func TestDo_NoAuthHeaderWithoutKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := testClient(server)
	_, _ = c.do(context.Background(), "GET", "/test", nil)

	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty", gotAuth)
	}
}

func TestDo_StatusCode4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "POST", "/test", nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
}

func TestDo_StatusCode5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "POST", "/test", nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.Body != "internal error" {
		t.Errorf("Body = %q", apiErr.Body)
	}
}

func TestDo_NetworkError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1")
	_, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, err) {
		// Just check it wraps a real error
	}
	if !strings.Contains(err.Error(), "execute request") {
		t.Errorf("error = %q, want 'execute request' prefix", err.Error())
	}
}

func TestDo_InvalidJSONBody(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, "{}"))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "POST", "/test", math.Inf(1))
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal body") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := testClient(server)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.do(ctx, "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected context error")
	}
}

// =====================================================================
// Auth: Login
// =====================================================================

func TestLogin_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, AuthTokens{
		AccessToken:  "at",
		RefreshToken: "rt",
	}))
	defer server.Close()

	c := testClient(server)
	tokens, err := c.Login(context.Background(), &LoginRequest{
		Email: "a@b.com", Password: "pass",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "at" || tokens.RefreshToken != "rt" {
		t.Errorf("tokens = %+v", tokens)
	}
}

func TestLogin_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(401, map[string]string{"error": "unauthorized"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.Login(context.Background(), &LoginRequest{
		Email: "a@b.com", Password: "bad",
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.Login(context.Background(), &LoginRequest{
		Email: "a@b.com", Password: "pass",
	})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Auth: Register
// =====================================================================

func TestRegister_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, AuthTokens{
		AccessToken:  "reg-at",
		RefreshToken: "reg-rt",
	}))
	defer server.Close()

	c := testClient(server)
	tokens, err := c.Register(context.Background(), &RegisterRequest{
		Email: "a@b.com", Password: "pass", Name: "Test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "reg-at" {
		t.Errorf("AccessToken = %q", tokens.AccessToken)
	}
}

func TestRegister_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(409, map[string]string{"error": "exists"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.Register(context.Background(), &RegisterRequest{
		Email: "a@b.com", Password: "pass", Name: "Test",
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegister_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.Register(context.Background(), &RegisterRequest{
		Email: "a@b.com", Password: "pass", Name: "Test",
	})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Auth: RefreshToken
// =====================================================================

func TestRefreshToken_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, AuthTokens{
		AccessToken:  "new-at",
		RefreshToken: "new-rt",
	}))
	defer server.Close()

	c := testClient(server)
	tokens, err := c.RefreshToken(context.Background(), "old-rt")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "new-at" {
		t.Errorf("AccessToken = %q", tokens.AccessToken)
	}
}

func TestRefreshToken_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(401, map[string]string{"error": "expired"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.RefreshToken(context.Background(), "bad")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRefreshToken_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.RefreshToken(context.Background(), "rt")
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Merchant: CreateMerchant
// =====================================================================

func TestCreateMerchant_Success(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(jsonHandler(200, Merchant{
		ID: testUUID, LegalName: "Acme", TradeName: "Acme Inc",
		Email: "a@b.com", Phone: "+123", Country: "US",
		Currency: "USD", Status: "active", CreatedAt: now,
	}))
	defer server.Close()

	c := testClient(server)
	m, err := c.CreateMerchant(context.Background(), &CreateMerchantRequest{
		LegalName: "Acme", TradeName: "Acme Inc",
		Email: "a@b.com", Phone: "+123", Country: "US", Currency: "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != testUUID {
		t.Errorf("ID = %v", m.ID)
	}
	if m.LegalName != "Acme" {
		t.Errorf("LegalName = %q", m.LegalName)
	}
}

func TestCreateMerchant_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(400, map[string]string{"error": "invalid"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateMerchant(context.Background(), &CreateMerchantRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateMerchant_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateMerchant(context.Background(), &CreateMerchantRequest{})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Merchant: GetMerchant
// =====================================================================

func TestGetMerchant_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, Merchant{
		ID: testUUID, LegalName: "Acme", Status: "active",
	}))
	defer server.Close()

	c := testClient(server)
	m, err := c.GetMerchant(context.Background(), testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != testUUID {
		t.Errorf("ID = %v", m.ID)
	}
}

func TestGetMerchant_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(404, map[string]string{"error": "not found"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetMerchant(context.Background(), testUUID)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetMerchant_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetMerchant(context.Background(), testUUID)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Merchant: ListMerchants
// =====================================================================

func TestListMerchants_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, map[string]interface{}{
		"merchants": []Merchant{
			{ID: testUUID, LegalName: "A"},
			{ID: testUUID2, LegalName: "B"},
		},
	}))
	defer server.Close()

	c := testClient(server)
	list, err := c.ListMerchants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].LegalName != "A" {
		t.Errorf("LegalName = %q", list[0].LegalName)
	}
}

func TestListMerchants_Empty(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, map[string]interface{}{
		"merchants": []Merchant{},
	}))
	defer server.Close()

	c := testClient(server)
	list, err := c.ListMerchants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestListMerchants_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(500, map[string]string{"error": "fail"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListMerchants(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListMerchants_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListMerchants(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Customer: CreateCustomer
// =====================================================================

func TestCreateCustomer_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, Customer{
		ID: testUUID2, MerchantID: testUUID, Name: "John", Email: "j@x.com",
	}))
	defer server.Close()

	c := testClient(server)
	cust, err := c.CreateCustomer(context.Background(), testUUID, &CreateCustomerRequest{
		Name: "John", Email: "j@x.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cust.Name != "John" {
		t.Errorf("Name = %q", cust.Name)
	}
}

func TestCreateCustomer_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(403, map[string]string{"error": "forbidden"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateCustomer(context.Background(), testUUID, &CreateCustomerRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateCustomer_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateCustomer(context.Background(), testUUID, &CreateCustomerRequest{})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Customer: GetCustomer
// =====================================================================

func TestGetCustomer_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, Customer{
		ID: testUUID2, MerchantID: testUUID, Name: "Jane",
	}))
	defer server.Close()

	c := testClient(server)
	cust, err := c.GetCustomer(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
	if cust.ID != testUUID2 {
		t.Errorf("ID = %v", cust.ID)
	}
}

func TestGetCustomer_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(404, map[string]string{"error": "not found"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetCustomer(context.Background(), testUUID, testUUID2)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetCustomer_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetCustomer(context.Background(), testUUID, testUUID2)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Customer: ListCustomers
// =====================================================================

func TestListCustomers_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, map[string]interface{}{
		"customers": []Customer{
			{ID: testUUID2, Name: "A"},
		},
	}))
	defer server.Close()

	c := testClient(server)
	list, err := c.ListCustomers(context.Background(), testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0].Name != "A" {
		t.Errorf("Name = %q", list[0].Name)
	}
}

func TestListCustomers_Empty(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, map[string]interface{}{
		"customers": []Customer{},
	}))
	defer server.Close()

	c := testClient(server)
	list, err := c.ListCustomers(context.Background(), testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestListCustomers_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(500, map[string]string{"error": "fail"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListCustomers(context.Background(), testUUID)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListCustomers_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListCustomers(context.Background(), testUUID)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Subscription: CreateSubscription
// =====================================================================

func TestCreateSubscription_Success(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(jsonHandler(200, Subscription{
		ID: testUUID2, MerchantID: testUUID, CustomerID: testUUID,
		Amount: 1000, Currency: "USD", Status: "active",
		Interval: "monthly", CreatedAt: now,
	}))
	defer server.Close()

	c := testClient(server)
	sub, err := c.CreateSubscription(context.Background(), testUUID, &CreateSubscriptionRequest{
		CustomerID: testUUID.String(), Amount: 1000, Currency: "USD",
		Interval: "monthly", IntervalCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sub.Amount != 1000 {
		t.Errorf("Amount = %d", sub.Amount)
	}
}

func TestCreateSubscription_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(400, map[string]string{"error": "bad sub"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateSubscription(context.Background(), testUUID, &CreateSubscriptionRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateSubscription_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateSubscription(context.Background(), testUUID, &CreateSubscriptionRequest{})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Subscription: GetSubscription
// =====================================================================

func TestGetSubscription_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, Subscription{
		ID: testUUID2, MerchantID: testUUID, Status: "active",
	}))
	defer server.Close()

	c := testClient(server)
	sub, err := c.GetSubscription(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
	if sub.ID != testUUID2 {
		t.Errorf("ID = %v", sub.ID)
	}
}

func TestGetSubscription_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(404, map[string]string{"error": "gone"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetSubscription(context.Background(), testUUID, testUUID2)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetSubscription_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetSubscription(context.Background(), testUUID, testUUID2)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Subscription: CancelSubscription
// =====================================================================

func TestCancelSubscription_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := testClient(server)
	err := c.CancelSubscription(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCancelSubscription_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(404, map[string]string{"error": "not found"}))
	defer server.Close()

	c := testClient(server)
	err := c.CancelSubscription(context.Background(), testUUID, testUUID2)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCancelSubscription_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "DELETE",
		fmt.Sprintf("/api/v1/merchants/%s/subscriptions/%s", testUUID, testUUID2)))
	defer server.Close()

	c := testClient(server)
	err := c.CancelSubscription(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
}

// =====================================================================
// Transaction: ProcessPayment
// =====================================================================

func TestProcessPayment_Success(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	server := httptest.NewServer(jsonHandler(200, Transaction{
		ID: testUUID2, MerchantID: testUUID, CustomerID: testUUID,
		Amount: 5000, Currency: "USD", Status: "completed",
		Provider: "stripe", CreatedAt: now,
	}))
	defer server.Close()

	c := testClient(server)
	tx, err := c.ProcessPayment(context.Background(), testUUID, &ProcessPaymentRequest{
		CustomerID: testUUID.String(), PaymentMethodID: "pm_1",
		Amount: 5000, Currency: "USD", IdempotencyKey: "idk1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Amount != 5000 {
		t.Errorf("Amount = %d", tx.Amount)
	}
}

func TestProcessPayment_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(402, map[string]string{"error": "payment failed"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.ProcessPayment(context.Background(), testUUID, &ProcessPaymentRequest{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 402 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProcessPayment_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.ProcessPayment(context.Background(), testUUID, &ProcessPaymentRequest{})
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Transaction: GetTransaction
// =====================================================================

func TestGetTransaction_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, Transaction{
		ID: testUUID2, MerchantID: testUUID, Status: "completed",
	}))
	defer server.Close()

	c := testClient(server)
	tx, err := c.GetTransaction(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
	if tx.ID != testUUID2 {
		t.Errorf("ID = %v", tx.ID)
	}
}

func TestGetTransaction_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(404, map[string]string{"error": "not found"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetTransaction(context.Background(), testUUID, testUUID2)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetTransaction_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetTransaction(context.Background(), testUUID, testUUID2)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Transaction: ListTransactions
// =====================================================================

func TestListTransactions_Success(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, map[string]interface{}{
		"transactions": []Transaction{
			{ID: testUUID2, Amount: 100},
			{ID: testUUID, Amount: 200},
		},
	}))
	defer server.Close()

	c := testClient(server)
	list, err := c.ListTransactions(context.Background(), testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Amount != 100 {
		t.Errorf("Amount = %d", list[0].Amount)
	}
}

func TestListTransactions_Empty(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, map[string]interface{}{
		"transactions": []Transaction{},
	}))
	defer server.Close()

	c := testClient(server)
	list, err := c.ListTransactions(context.Background(), testUUID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}

func TestListTransactions_Error(t *testing.T) {
	server := httptest.NewServer(jsonHandler(500, map[string]string{"error": "fail"}))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListTransactions(context.Background(), testUUID)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 500 {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListTransactions_BadJSON(t *testing.T) {
	server := httptest.NewServer(badJSONHandler(200))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListTransactions(context.Background(), testUUID)
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// =====================================================================
// Request path & method verification (spot checks)
// =====================================================================

func TestGetMerchant_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "GET",
		fmt.Sprintf("/api/v1/merchants/%s", testUUID)))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetMerchant(context.Background(), testUUID)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateMerchant_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "POST", "/api/v1/merchants"))
	defer server.Close()

	c := testClient(server)
	_, err := c.CreateMerchant(context.Background(), &CreateMerchantRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListMerchants_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "GET", "/api/v1/merchants"))
	defer server.Close()

	c := testClient(server)
	_, err := c.ListMerchants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetCustomer_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "GET",
		fmt.Sprintf("/api/v1/merchants/%s/customers/%s", testUUID, testUUID2)))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetCustomer(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetSubscription_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "GET",
		fmt.Sprintf("/api/v1/merchants/%s/subscriptions/%s", testUUID, testUUID2)))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetSubscription(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetTransaction_MethodPath(t *testing.T) {
	server := httptest.NewServer(methodHandler(t, "GET",
		fmt.Sprintf("/api/v1/merchants/%s/transactions/%s", testUUID, testUUID2)))
	defer server.Close()

	c := testClient(server)
	_, err := c.GetTransaction(context.Background(), testUUID, testUUID2)
	if err != nil {
		t.Fatal(err)
	}
}

// =====================================================================
// Auth header tests on actual API methods
// =====================================================================

func TestLogin_SendsAuthHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AuthTokens{AccessToken: "at", RefreshToken: "rt"})
	}))
	defer server.Close()

	c := testClientWithKey(server, "test-key")
	_, _ = c.Login(context.Background(), &LoginRequest{Email: "a@b.com", Password: "p"})

	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
}

func TestLogin_NoAuthHeaderWithoutKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AuthTokens{AccessToken: "at", RefreshToken: "rt"})
	}))
	defer server.Close()

	c := testClient(server)
	_, _ = c.Login(context.Background(), &LoginRequest{Email: "a@b.com", Password: "p"})

	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty", gotAuth)
	}
}

// =====================================================================
// do() edge cases
// =====================================================================

func TestDo_BodyMarshalFailure(t *testing.T) {
	server := httptest.NewServer(jsonHandler(200, "{}"))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "POST", "/test", math.Inf(1))
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "marshal body") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestDo_ReadBodyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDo_StatusOKWithEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := testClient(server)
	data, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("data = %q, want empty", string(data))
	}
}

func TestDo_StatusBoundary399(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(399)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Errorf("unexpected error for 399: %v", err)
	}
}

func TestDo_StatusBoundary400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`bad`))
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.do(context.Background(), "GET", "/test", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError for 400, got %T", err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
}

func TestDo_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// drain
	}))
	defer server.Close()

	c := testClient(server)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.do(ctx, "GET", "/test", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// =====================================================================
// Body capture test (verify request body round-trip)
// =====================================================================

func TestDo_RequestBodyRoundTrip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(bodyCaptureHandler))
	defer server.Close()

	c := testClient(server)
	input := map[string]string{"foo": "bar", "baz": "qux"}
	data, err := c.do(context.Background(), "POST", "/echo", input)
	if err != nil {
		t.Fatal(err)
	}

	var output map[string]string
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output["foo"] != "bar" || output["baz"] != "qux" {
		t.Errorf("output = %v", output)
	}
}

// =====================================================================
// RefreshToken sends correct body
// =====================================================================

func TestRefreshToken_SendsRefreshToken(t *testing.T) {
	var receivedBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AuthTokens{AccessToken: "new"})
	}))
	defer server.Close()

	c := testClient(server)
	_, err := c.RefreshToken(context.Background(), "my-refresh-token")
	if err != nil {
		t.Fatal(err)
	}
	if receivedBody["refresh_token"] != "my-refresh-token" {
		t.Errorf("refresh_token = %q", receivedBody["refresh_token"])
	}
}

// =====================================================================
// Concurrent request safety
// =====================================================================

func TestDo_ConcurrentRequests(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := testClient(server)
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			_, _ = c.do(context.Background(), "GET", "/test", nil)
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	if int(count.Load()) != 50 {
		t.Errorf("count = %d, want 50", count.Load())
	}
}

// =====================================================================
// Large payload
// =====================================================================

func TestDo_LargeResponse(t *testing.T) {
	large := strings.Repeat("x", 1<<20) // 1MB
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"` + large + `"`))
	}))
	defer server.Close()

	c := testClient(server)
	data, err := c.do(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("empty response")
	}
}
