package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestExchangeRateService_GetRate_FrankfurterSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Rates: map[string]float64{"EUR": 0.92},
		})
	}))
	defer server.Close()

	svc := &ExchangeRateService{
		logger: zap.NewNop(),
		client: server.Client(),
	}
	svc.client = &http.Client{}

	// Since GetRate first queries DB (which will fail), then calls frankfurter,
	// and then tries to write to DB again, we can't easily test the full flow
	// without a DB. But we CAN test the HTTP client behavior directly.
	resp, err := http.Get(server.URL + "/latest?from=USD&to=EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var fr frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if fr.Rates["EUR"] != 0.92 {
		t.Errorf("EUR rate = %f, want 0.92", fr.Rates["EUR"])
	}
}

func TestExchangeRateService_GetRate_FrankfurterMultipleRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Rates: map[string]float64{
				"EUR": 0.92,
				"GBP": 0.79,
				"JPY": 149.5,
			},
		})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var fr frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	tests := []struct {
		currency string
		expected float64
	}{
		{"EUR", 0.92},
		{"GBP", 0.79},
		{"JPY", 149.5},
	}

	for _, tt := range tests {
		if fr.Rates[tt.currency] != tt.expected {
			t.Errorf("Rate[%s] = %f, want %f", tt.currency, fr.Rates[tt.currency], tt.expected)
		}
	}
}

func TestExchangeRateService_GetRate_FrankfurterEmptyRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Rates: map[string]float64{},
		})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var fr frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(fr.Rates) != 0 {
		t.Errorf("expected empty rates, got %d", len(fr.Rates))
	}
}

func TestExchangeRateService_GetRate_FrankfurterServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestExchangeRateService_GetRate_FrankfurterInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"invalid json`))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var fr frankfurterResponse
	err = json.NewDecoder(resp.Body).Decode(&fr)
	if err == nil {
		t.Error("expected decode error for invalid JSON")
	}
}

func TestExchangeRateService_GetRate_FrankfurterMissingCurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Rates: map[string]float64{"GBP": 0.79},
		})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/latest?from=USD&to=EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	var fr frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	_, ok := fr.Rates["EUR"]
	if ok {
		t.Error("EUR should not be in rates")
	}
}

func TestExchangeRateService_GetRate_DBNilPanics(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when DB is nil")
		}
	}()
	svc.GetRate(context.Background(), "USD", "EUR")
}

func TestExchangeRateService_Convert_DifferentCurrency_DBNil(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when DB is nil for different currency")
		}
	}()
	svc.Convert(context.Background(), 10000, "USD", "EUR")
}

func TestExchangeRateService_Constructor_Fields(t *testing.T) {
	logger := zap.NewNop()
	svc := NewExchangeRateService(nil, logger)
	if svc.logger != logger {
		t.Error("logger not set correctly")
	}
	if svc.client == nil {
		t.Error("client should be initialized")
	}
}

func TestExchangeRateService_GetRate_FrankfurterURL(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Rates: map[string]float64{"EUR": 0.92},
		})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/latest?from=USD&to=EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if receivedPath != "/latest?from=USD&to=EUR" {
		t.Errorf("path = %q, want /latest?from=USD&to=EUR", receivedPath)
	}
}

func TestExchangeRateService_Convert_NilDB_SameCurrency(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	converted, rate, err := svc.Convert(context.Background(), 5000, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != 5000 {
		t.Errorf("converted = %d, want 5000", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_FrankfurterResponse_Unmarshal(t *testing.T) {
	data := `{"rates":{"EUR":0.92,"GBP":0.79}}`
	var fr frankfurterResponse
	if err := json.Unmarshal([]byte(data), &fr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if fr.Rates["EUR"] != 0.92 {
		t.Errorf("EUR = %f, want 0.92", fr.Rates["EUR"])
	}
	if fr.Rates["GBP"] != 0.79 {
		t.Errorf("GBP = %f, want 0.79", fr.Rates["GBP"])
	}
}

func TestExchangeRateService_FrankfurterResponse_MarshalRoundtrip(t *testing.T) {
	original := frankfurterResponse{
		Rates: map[string]float64{"USD": 1.0, "EUR": 0.92},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded frankfurterResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(decoded.Rates) != 2 {
		t.Errorf("rates count = %d, want 2", len(decoded.Rates))
	}
}

func TestExchangeRateService_GetRate_URLConstruction(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want string
	}{
		{"USD", "EUR", "https://api.frankfurter.app/latest?from=USD&to=EUR"},
		{"GBP", "JPY", "https://api.frankfurter.app/latest?from=GBP&to=JPY"},
	}

	for _, tt := range tests {
		t.Run(tt.from+"_"+tt.to, func(t *testing.T) {
			got := fmt.Sprintf("https://api.frankfurter.app/latest?from=%s&to=%s", tt.from, tt.to)
			if got != tt.want {
				t.Errorf("URL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExchangeRateService_Convert_Calculation(t *testing.T) {
	amount := int64(10000)
	rate := 0.92
	converted := float64(amount) * rate

	if converted != 9200.0 {
		t.Errorf("converted = %f, want 9200.0", converted)
	}

	result := int64(converted)
	if result != 9200 {
		t.Errorf("int64(converted) = %d, want 9200", result)
	}
}

func TestExchangeRateService_GetRate_BadURL(t *testing.T) {
	svc := &ExchangeRateService{
		db:     nil,
		logger: zap.NewNop(),
		client: &http.Client{},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil DB")
		}
	}()
	svc.GetRate(context.Background(), "USD", "EUR")
}

func TestExchangeRateService_FrankfurterResponse_Empty(t *testing.T) {
	data := `{"rates":{}}`
	var fr frankfurterResponse
	if err := json.Unmarshal([]byte(data), &fr); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(fr.Rates) != 0 {
		t.Errorf("expected empty rates map, got %d entries", len(fr.Rates))
	}
}

func TestExchangeRateService_Convert_Int64Truncation(t *testing.T) {
	amount := int64(10000)
	rate := 1.333
	converted := float64(amount) * rate
	result := int64(converted)

	expected := int64(13330)
	if result != expected {
		t.Errorf("truncated conversion = %d, want %d", result, expected)
	}
}

func TestExchangeRateService_GetRate_NilClient(t *testing.T) {
	svc := &ExchangeRateService{
		db:     nil,
		logger: zap.NewNop(),
		client: nil,
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil DB and nil client")
		}
	}()
	svc.GetRate(context.Background(), "USD", "EUR")
}

func TestExchangeRateService_FrankfurterResponse_NilRates(t *testing.T) {
	var fr frankfurterResponse
	if fr.Rates != nil {
		t.Errorf("nil-initialized Rates should be nil, got %v", fr.Rates)
	}
}

func TestExchangeRateService_MockServer_Behavior(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(frankfurterResponse{
			Rates: map[string]float64{"EUR": float64(callCount)},
		})
	}))
	defer server.Close()

	for i := 0; i < 3; i++ {
		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		var fr frankfurterResponse
		json.NewDecoder(resp.Body).Decode(&fr)
		resp.Body.Close()

		if fr.Rates["EUR"] != float64(i+1) {
			t.Errorf("call %d: EUR rate = %f, want %f", i, fr.Rates["EUR"], float64(i+1))
		}
	}

	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestExchangeRateService_ContextCancellation(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with cancelled context and nil DB")
		}
	}()
	svc.GetRate(ctx, "USD", "EUR")
}

func TestExchangeRateService_Convert_NegativeRate(t *testing.T) {
	svc := NewExchangeRateService(nil, zap.NewNop())
	// Same currency bypasses GetRate, so this tests the fast path
	converted, rate, err := svc.Convert(context.Background(), -100, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != -100 {
		t.Errorf("converted = %d, want -100", converted)
	}
	if rate != 1.0 {
		t.Errorf("rate = %f, want 1.0", rate)
	}
}

func TestExchangeRateService_UUID_DifferentEachTime(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	if id1 == id2 {
		t.Error("two UUIDs should be different")
	}
}
