// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package infer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock Triton server
// ---------------------------------------------------------------------------

func mockTritonWithOutputs(outputs []wireOutput) (*httptest.Server, *string) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		captured = string(body)
		resp := wireResponse{
			ModelName:    "test-model",
			ModelVersion: "1",
			Outputs:      outputs,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return srv, &captured
}

func fp32Output(name string, values []float32) wireOutput {
	b, _ := json.Marshal(values)
	return wireOutput{Name: name, Datatype: "FP32", Shape: []int{len(values)}, Data: b}
}

// ---------------------------------------------------------------------------
// Happy path — every datatype
// ---------------------------------------------------------------------------

func TestInfer_FP32RoundTrip(t *testing.T) {
	srv, _ := mockTritonWithOutputs([]wireOutput{fp32Output("OUT", []float32{0.5, 1.5, 2.5})})
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.Infer(context.Background(), Request{
		Model: "m",
		Inputs: []Input{
			{Name: "IN", Datatype: "FP32", Shape: []int{3}, Float32Data: []float32{1, 2, 3}},
		},
	})
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if resp.ModelName != "test-model" || resp.ModelVersion != "1" {
		t.Fatalf("model metadata = (%q, %q)", resp.ModelName, resp.ModelVersion)
	}
	out, err := resp.Find("OUT")
	if err != nil {
		t.Fatal(err)
	}
	if out.Datatype != "FP32" || len(out.Float32Data) != 3 {
		t.Fatalf("FP32 output: %+v", out)
	}
	if out.Float32Data[0] != 0.5 || out.Float32Data[2] != 2.5 {
		t.Fatalf("FP32 values = %v", out.Float32Data)
	}
}

func TestInfer_EveryDatatypeDecoded(t *testing.T) {
	f32, _ := json.Marshal([]float32{1.1})
	f64, _ := json.Marshal([]float64{2.2})
	i32, _ := json.Marshal([]int32{3})
	i64, _ := json.Marshal([]int64{4})
	u8, _ := json.Marshal([]uint8{5})
	boolB, _ := json.Marshal([]bool{true})
	bytesB, _ := json.Marshal([]string{"hello"})
	srv, _ := mockTritonWithOutputs([]wireOutput{
		{Name: "F32", Datatype: "FP32", Shape: []int{1}, Data: f32},
		{Name: "F64", Datatype: "FP64", Shape: []int{1}, Data: f64},
		{Name: "I32", Datatype: "INT32", Shape: []int{1}, Data: i32},
		{Name: "I64", Datatype: "INT64", Shape: []int{1}, Data: i64},
		{Name: "U8", Datatype: "UINT8", Shape: []int{1}, Data: u8},
		{Name: "BOOL", Datatype: "BOOL", Shape: []int{1}, Data: boolB},
		{Name: "BYTES", Datatype: "BYTES", Shape: []int{1}, Data: bytesB},
	})
	defer srv.Close()
	c := New(srv.URL)
	resp, err := c.Infer(context.Background(), Request{
		Model:  "m",
		Inputs: []Input{{Name: "IN", Datatype: "FP32", Shape: []int{1}, Float32Data: []float32{0}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		want string
	}{
		{"F32", "FP32"}, {"F64", "FP64"}, {"I32", "INT32"}, {"I64", "INT64"},
		{"U8", "UINT8"}, {"BOOL", "BOOL"}, {"BYTES", "BYTES"},
	}
	for _, ck := range checks {
		o, err := resp.Find(ck.name)
		if err != nil {
			t.Errorf("missing %q: %v", ck.name, err)
			continue
		}
		if o.Datatype != ck.want {
			t.Errorf("%q datatype = %q, want %q", ck.name, o.Datatype, ck.want)
		}
	}
	// Spot-check specific values.
	f32Out, _ := resp.Find("F32")
	if len(f32Out.Float32Data) != 1 || f32Out.Float32Data[0] < 1.09 || f32Out.Float32Data[0] > 1.11 {
		t.Errorf("F32 = %v", f32Out.Float32Data)
	}
	boolOut, _ := resp.Find("BOOL")
	if len(boolOut.BoolData) != 1 || !boolOut.BoolData[0] {
		t.Errorf("BOOL = %v", boolOut.BoolData)
	}
	bytesOut, _ := resp.Find("BYTES")
	if len(bytesOut.StringData) != 1 || bytesOut.StringData[0] != "hello" {
		t.Errorf("BYTES = %v", bytesOut.StringData)
	}
}

func TestInfer_RequestShapeMatchesKServeV2(t *testing.T) {
	srv, captured := mockTritonWithOutputs([]wireOutput{fp32Output("OUT", []float32{0})})
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Infer(context.Background(), Request{
		Model: "m",
		Inputs: []Input{
			{Name: "IMAGE", Datatype: "BYTES", Shape: []int{1}, StringData: []string{"base64blob"}},
			{Name: "SCALE", Datatype: "FP32", Shape: []int{1}, Float32Data: []float32{1.5}},
		},
	})
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	var req wireRequest
	if err := json.Unmarshal([]byte(*captured), &req); err != nil {
		t.Fatalf("captured not valid KServe JSON: %v", err)
	}
	if len(req.Inputs) != 2 {
		t.Fatalf("inputs = %d, want 2", len(req.Inputs))
	}
	if req.Inputs[0].Name != "IMAGE" || req.Inputs[0].Datatype != "BYTES" {
		t.Errorf("input[0] = %+v", req.Inputs[0])
	}
}

func TestInfer_CustomInferPath(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(wireResponse{Outputs: []wireOutput{fp32Output("OUT", []float32{0})}})
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.InferPath = "/v3/predict"
	_, _ = c.Infer(context.Background(), Request{
		Model:  "ignored-with-custom-path",
		Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	if receivedPath != "/v3/predict" {
		t.Fatalf("path = %q, want /v3/predict", receivedPath)
	}
}

// ---------------------------------------------------------------------------
// FloatScalar helper
// ---------------------------------------------------------------------------

func TestFloatScalar_FP32Output(t *testing.T) {
	srv, _ := mockTritonWithOutputs([]wireOutput{fp32Output("SCORE", []float32{0.42})})
	defer srv.Close()
	c := New(srv.URL)
	resp, _ := c.Infer(context.Background(), Request{
		Model:  "m",
		Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	got, err := resp.FloatScalar("SCORE")
	if err != nil {
		t.Fatal(err)
	}
	if got < 0.41 || got > 0.43 {
		t.Fatalf("FloatScalar = %v, want ≈ 0.42", got)
	}
}

func TestFloatScalar_FP64Output(t *testing.T) {
	f64, _ := json.Marshal([]float64{3.14})
	srv, _ := mockTritonWithOutputs([]wireOutput{{Name: "S", Datatype: "FP64", Data: f64}})
	defer srv.Close()
	c := New(srv.URL)
	resp, _ := c.Infer(context.Background(), Request{
		Model:  "m",
		Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	got, _ := resp.FloatScalar("S")
	if got != 3.14 {
		t.Fatalf("FloatScalar = %v", got)
	}
}

func TestFloatScalar_INT32AndINT64AndUINT8(t *testing.T) {
	i32, _ := json.Marshal([]int32{7})
	i64, _ := json.Marshal([]int64{99})
	u8, _ := json.Marshal([]uint8{42})
	srv, _ := mockTritonWithOutputs([]wireOutput{
		{Name: "I32", Datatype: "INT32", Data: i32},
		{Name: "I64", Datatype: "INT64", Data: i64},
		{Name: "U8", Datatype: "UINT8", Data: u8},
	})
	defer srv.Close()
	c := New(srv.URL)
	resp, _ := c.Infer(context.Background(), Request{
		Model:  "m",
		Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	for name, want := range map[string]float64{"I32": 7, "I64": 99, "U8": 42} {
		got, err := resp.FloatScalar(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func TestFloatScalar_MissingOutput(t *testing.T) {
	resp := Response{}
	if _, err := resp.FloatScalar("x"); !errors.Is(err, ErrOutputMissing) {
		t.Fatalf("err = %v, want ErrOutputMissing", err)
	}
}

func TestFloatScalar_EmptyOutput(t *testing.T) {
	resp := Response{Outputs: []Output{{Name: "EMPTY", Datatype: "FP32"}}}
	if _, err := resp.FloatScalar("EMPTY"); !errors.Is(err, ErrWrongDatatype) {
		t.Fatalf("empty = %v, want ErrWrongDatatype", err)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestInfer_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	_, err := c.Infer(context.Background(), Request{Model: "m", Inputs: []Input{{Name: "x"}}})
	if !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("err = %v, want ErrEmptyEndpoint", err)
	}
}

func TestInfer_EmptyModelError(t *testing.T) {
	c := New("http://localhost")
	_, err := c.Infer(context.Background(), Request{Inputs: []Input{{Name: "x"}}})
	if !errors.Is(err, ErrEmptyModel) {
		t.Fatalf("err = %v, want ErrEmptyModel", err)
	}
}

func TestInfer_NoInputsError(t *testing.T) {
	c := New("http://localhost")
	_, err := c.Infer(context.Background(), Request{Model: "m"})
	if !errors.Is(err, ErrNoInputs) {
		t.Fatalf("err = %v, want ErrNoInputs", err)
	}
}

func TestInfer_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Infer(context.Background(), Request{
		Model: "m", Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP 503 not propagated: %v", err)
	}
}

func TestInfer_MalformedResponseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Infer(context.Background(), Request{
		Model: "m", Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	if err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestInfer_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Infer(ctx, Request{Model: "m", Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}}})
	if err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestInfer_InvalidEndpointURLError(t *testing.T) {
	c := &Client{Endpoint: "ht!tp://bad\x00url"}
	_, err := c.Infer(context.Background(), Request{
		Model: "m", Inputs: []Input{{Name: "IN", Datatype: "FP32", Float32Data: []float32{0}}},
	})
	if err == nil {
		t.Fatal("invalid URL should fail")
	}
}

// ---------------------------------------------------------------------------
// Input wire encoding — every *Data branch
// ---------------------------------------------------------------------------

func TestToWireRequest_AllDataBranches(t *testing.T) {
	req := Request{
		Model: "m",
		Inputs: []Input{
			{Name: "s", Datatype: "BYTES", StringData: []string{"a"}},
			{Name: "f32", Datatype: "FP32", Float32Data: []float32{1}},
			{Name: "f64", Datatype: "FP64", Float64Data: []float64{1}},
			{Name: "i32", Datatype: "INT32", Int32Data: []int32{1}},
			{Name: "i64", Datatype: "INT64", Int64Data: []int64{1}},
			{Name: "u8", Datatype: "UINT8", Uint8Data: []uint8{1}},
			{Name: "b", Datatype: "BOOL", BoolData: []bool{true}},
			{Name: "empty", Datatype: "FP32"}, // no *Data set → empty array
		},
	}
	w := toWireRequest(req)
	if len(w.Inputs) != 8 {
		t.Fatalf("inputs = %d, want 8", len(w.Inputs))
	}
	// Sanity: each one has a non-nil Data field.
	for i, in := range w.Inputs {
		if in.Data == nil {
			t.Errorf("input[%d] (%q) has nil Data", i, in.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Wire response edge case
// ---------------------------------------------------------------------------

func TestFromWireResponse_UnknownDatatypeBestEffort(t *testing.T) {
	// Unknown datatype with float-looking data should land in
	// Float64Data via the best-effort fallback.
	data, _ := json.Marshal([]float64{1.5})
	wire := wireResponse{
		Outputs: []wireOutput{{Name: "X", Datatype: "CUSTOM_TYPE", Data: data}},
	}
	r := fromWireResponse(wire)
	if len(r.Outputs) != 1 {
		t.Fatal("unexpected outputs")
	}
	if len(r.Outputs[0].Float64Data) != 1 || r.Outputs[0].Float64Data[0] != 1.5 {
		t.Fatalf("best-effort Float64 = %v", r.Outputs[0].Float64Data)
	}
}

func TestFromWireResponse_UnknownDatatypeStringFallback(t *testing.T) {
	// Unknown datatype with string-looking data — fallback to StringData.
	data, _ := json.Marshal([]string{"foo"})
	wire := wireResponse{
		Outputs: []wireOutput{{Name: "X", Datatype: "CUSTOM_TYPE", Data: data}},
	}
	r := fromWireResponse(wire)
	if len(r.Outputs[0].StringData) != 1 || r.Outputs[0].StringData[0] != "foo" {
		t.Fatalf("best-effort String = %v", r.Outputs[0].StringData)
	}
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

