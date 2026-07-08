// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-mcp-gateway is the RUNNABLE analyzer for the
// HelixQA MCP-gateway capability bank (banks/helixllm_mcp_gateway.yaml) — it
// drives real requests against the live HelixLLM MCP gateway
// (submodules/helix_llm/cmd/mcp-gateway, Streamable-HTTP per go-sdk v1.6.1,
// default :18444, spec
// docs/research/07.2026/05_mcp_acp_protocols/MCP_OKF_GATEWAY_MEMO.md), which
// proxies real HTTP calls to the live coder (:18434, read-only,
// §11.4.119/§11.4.122 — this analyzer never starts/stops/restarts the coder
// or the gateway; the conductor owns that boot/teardown lifecycle). Mirrors
// the CLI convention of cmd/helixqa-verify-translate-nllb
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2) — a thin,
// project-agnostic dispatches_to analyzer (CONST-051(B), no HelixQA
// core-engine change).
//
// A single --case flag selects one of four real wire interactions (mirrors
// helixqa-verify-rag's --no-context mode-select convention):
//
//   - unauth401  — raw JSON-RPC POST with NO Authorization header MUST get
//     HTTP 401 (the gateway's auth.RequireBearerToken middleware runs
//     BEFORE any MCP protocol logic, so a raw POST — no MCP session
//     handshake needed — genuinely exercises the real rejection path).
//   - toolslist  — a real go-sdk mcp.Client Streamable-HTTP session lists
//     tools and asserts helixllm_generate + helixllm_list_models are BOTH
//     registered (sourced live from the server, never hardcoded — CONST-036).
//   - generate   — a real tools/call of helixllm_generate proxies to the
//     live coder's /v1/chat/completions; asserts the returned content
//     contains every --expect fact token AND is not identical to the prompt.
//   - listmodels — a real tools/call of helixllm_list_models proxies to the
//     live coder's /v1/models; asserts every --expect model-id substring is
//     present in the returned model list.
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). Every case records
// the RAW pass honestly; --expect-fail only inverts which raw outcome
// counts as case success (golden-bad self-validation fixtures). The full
// raw wire response (tool result / raw HTTP body) is embedded in the
// verdict artefact so a reviewer never has to trust the analyzer's parsing
// blindly.
//
// Usage:
//
//	helixqa-verify-mcp-gateway --case unauth401 \
//	  --out qa-results/helixllm_mcp_gateway/unauth_verdict.json \
//	  [--endpoint http://localhost:18444]
//
//	helixqa-verify-mcp-gateway --case toolslist --bearer <token> \
//	  --out qa-results/helixllm_mcp_gateway/toolslist_verdict.json
//
//	helixqa-verify-mcp-gateway --case generate --bearer <token> \
//	  --prompt "What is 2+2? Reply with only the digit, nothing else." \
//	  --expect "4" \
//	  --out qa-results/helixllm_mcp_gateway/generate_verdict.json
//
//	helixqa-verify-mcp-gateway --case listmodels --bearer <token> \
//	  --expect "Qwen3-Coder" \
//	  --out qa-results/helixllm_mcp_gateway/listmodels_verdict.json
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"digital.vasic.helixqa/pkg/conduit"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitInfra = 2
)

// verdict is the machine-readable artefact the ContentAssertingResolver
// gates on — embeds the raw wire response, never trust-the-analyzer-blindly.
type verdict struct {
	Case          string          `json:"case"`
	Endpoint      string          `json:"endpoint"`
	BearerSet     bool            `json:"bearer_set"`
	HTTPStatus    int             `json:"http_status,omitempty"`
	ToolNames     []string        `json:"tool_names,omitempty"`
	Prompt        string          `json:"prompt,omitempty"`
	Content       string          `json:"content,omitempty"`
	Models        []string        `json:"models,omitempty"`
	ExpectedFacts []string        `json:"expected_facts,omitempty"`
	MatchedFacts  int             `json:"matched_facts"`
	ExpectedCount int             `json:"expected_count"`
	NotIdentity   bool            `json:"not_identity"`
	IsToolError   bool            `json:"is_tool_error"`
	Pass          bool            `json:"pass"`
	ExpectFail    bool            `json:"expect_fail"`
	CaseResult    bool            `json:"case_result"`
	LatencyMS     int64           `json:"latency_ms"`
	RawResponse   json.RawMessage `json:"raw_response,omitempty"`
	Error         string          `json:"error,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		caseName   = flag.String("case", "", "unauth401 | toolslist | generate | listmodels (required)")
		endpoint   = flag.String("endpoint", envOr("HELIX_MCP_GATEWAY_ENDPOINT", "http://localhost:18444"), "MCP gateway Streamable-HTTP endpoint")
		bearer     = flag.String("bearer", os.Getenv("HELIX_MCP_GATEWAY_TOKEN"), "Bearer token (§11.4.10 — never logged verbatim; required for toolslist/generate/listmodels)")
		prompt     = flag.String("prompt", "", "prompt for --case generate")
		expectCSV  = flag.String("expect", "", "comma-separated required fact tokens (case-insensitive substring match)")
		maxTokens  = flag.Int("max-tokens", 16, "max_tokens argument for --case generate")
		out        = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID    = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout    = flag.Duration("timeout", 60*time.Second, "request timeout")
		expectFail = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *caseName == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-mcp-gateway --case <unauth401|toolslist|generate|listmodels> --out <verdict.json>")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_mcp_gateway", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "mcp-gateway")

	v := verdict{
		Case:          *caseName,
		Endpoint:      *endpoint,
		BearerSet:     *bearer != "",
		ExpectedFacts: splitCSV(*expectCSV),
	}
	v.ExpectedCount = len(v.ExpectedFacts)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var runErr error
	start := time.Now()
	switch *caseName {
	case "unauth401":
		runErr = runUnauth401(ctx, &v, *endpoint)
	case "toolslist":
		runErr = runToolsList(ctx, &v, *endpoint, *bearer)
	case "generate":
		if *prompt == "" || *expectCSV == "" {
			return failInfra(sink, cid, &v, *out, "--case generate requires --prompt and --expect")
		}
		v.Prompt = *prompt
		runErr = runGenerate(ctx, &v, *endpoint, *bearer, *prompt, *maxTokens)
	case "listmodels":
		if *expectCSV == "" {
			return failInfra(sink, cid, &v, *out, "--case listmodels requires --expect")
		}
		runErr = runListModels(ctx, &v, *endpoint, *bearer)
	default:
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("unknown --case %q", *caseName))
	}
	v.LatencyMS = time.Since(start).Milliseconds()
	if runErr != nil {
		return failInfra(sink, cid, &v, *out, runErr.Error())
	}

	v.ExpectFail = *expectFail
	if v.ExpectFail {
		v.CaseResult = !v.Pass
	} else {
		v.CaseResult = v.Pass
	}

	if writeErr := writeVerdict(*out, &v); writeErr != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write verdict: %v\n", writeErr)
		return exitInfra
	}
	conduit.EvidenceCaptured(sink, cid, "mcp_gateway_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s case=%s expect_fail=%v raw_pass=%v\n", cid, *caseName, v.ExpectFail, v.Pass)
		return exitPass
	}
	reason := fmt.Sprintf("case=%s expect_fail=%v raw_pass=%v", *caseName, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s\n", cid, reason)
	return exitFail
}

// runUnauth401 sends a raw JSON-RPC POST with NO Authorization header and
// asserts the gateway's auth middleware rejects it with HTTP 401 — the
// real, non-simulated rejection path (auth runs before any MCP session
// logic reaches server.go, so this raw POST genuinely exercises it).
func runUnauth401(ctx context.Context, v *verdict, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	v.HTTPStatus = resp.StatusCode
	v.RawResponse = json.RawMessage(mustJSONString(buf.String()))
	v.Pass = resp.StatusCode == http.StatusUnauthorized
	return nil
}

// runToolsList opens a real MCP client session over Streamable-HTTP with a
// valid Bearer token and lists tools, asserting BOTH capabilities the
// design memo promises are registered (sourced live, never hardcoded).
func runToolsList(ctx context.Context, v *verdict, endpoint, bearer string) error {
	if bearer == "" {
		return fmt.Errorf("--bearer (or HELIX_MCP_GATEWAY_TOKEN) is required for --case toolslist")
	}
	session, closeFn, err := connectMCP(ctx, endpoint, bearer)
	if err != nil {
		return err
	}
	defer closeFn()

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	names := make([]string, 0, len(toolsResult.Tools))
	for _, t := range toolsResult.Tools {
		names = append(names, t.Name)
	}
	v.ToolNames = names
	raw, _ := json.Marshal(toolsResult.Tools)
	v.RawResponse = json.RawMessage(raw)
	v.Pass = containsString(names, "helixllm_generate") && containsString(names, "helixllm_list_models")
	return nil
}

// runGenerate performs a real tools/call of helixllm_generate — proxies to
// the live coder's /v1/chat/completions — and fact-matches + not-identity
// checks the returned content (kills the echo/passthrough bluff).
func runGenerate(ctx context.Context, v *verdict, endpoint, bearer, prompt string, maxTokens int) error {
	if bearer == "" {
		return fmt.Errorf("--bearer (or HELIX_MCP_GATEWAY_TOKEN) is required for --case generate")
	}
	session, closeFn, err := connectMCP(ctx, endpoint, bearer)
	if err != nil {
		return err
	}
	defer closeFn()

	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "helixllm_generate",
		Arguments: map[string]any{"prompt": prompt, "max_tokens": maxTokens},
	})
	if err != nil {
		return fmt.Errorf("tools/call helixllm_generate: %w", err)
	}
	raw, _ := json.Marshal(callResult)
	v.RawResponse = json.RawMessage(raw)
	v.IsToolError = callResult.IsError
	v.Content = contentText(callResult)

	fwdNorm := normalize(v.Content)
	srcNorm := normalize(prompt)
	v.NotIdentity = strings.TrimSpace(v.Content) != "" && fwdNorm != srcNorm

	lowerContent := strings.ToLower(v.Content)
	matched := 0
	for _, f := range v.ExpectedFacts {
		if strings.Contains(lowerContent, strings.ToLower(f)) {
			matched++
		}
	}
	v.MatchedFacts = matched

	v.Pass = !v.IsToolError && v.NotIdentity && v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount
	return nil
}

// runListModels performs a real tools/call of helixllm_list_models —
// proxies to the live coder's /v1/models — and asserts every --expect
// model-id substring is present in the real returned model list.
func runListModels(ctx context.Context, v *verdict, endpoint, bearer string) error {
	if bearer == "" {
		return fmt.Errorf("--bearer (or HELIX_MCP_GATEWAY_TOKEN) is required for --case listmodels")
	}
	session, closeFn, err := connectMCP(ctx, endpoint, bearer)
	if err != nil {
		return err
	}
	defer closeFn()

	callResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "helixllm_list_models",
		Arguments: map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("tools/call helixllm_list_models: %w", err)
	}
	raw, _ := json.Marshal(callResult)
	v.RawResponse = json.RawMessage(raw)
	v.IsToolError = callResult.IsError
	v.Content = contentText(callResult)

	// Parse the structured output when present; fall back to the text
	// content (server.go emits `%v` of the []string models on the text
	// content, so a substring check works either way).
	models, _ := decodeStructuredModels(callResult.StructuredContent)
	v.Models = models

	lowerContent := strings.ToLower(v.Content)
	matched := 0
	for _, f := range v.ExpectedFacts {
		if strings.Contains(lowerContent, strings.ToLower(f)) {
			matched++
		}
	}
	v.MatchedFacts = matched
	v.Pass = !v.IsToolError && v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount
	return nil
}

func decodeStructuredModels(raw any) ([]string, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var s struct {
		Models []string `json:"models"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return s.Models, nil
}

// connectMCP opens a real go-sdk mcp.Client Streamable-HTTP session with a
// Bearer-token-injecting http.Client — the SAME transport shape the
// existing docs/qa/phase1_mcp_gateway_20260708T074850Z/harness proof used.
func connectMCP(ctx context.Context, endpoint, bearer string) (*mcp.ClientSession, func(), error) {
	httpClient := &http.Client{Transport: &bearerRoundTripper{token: bearer, base: http.DefaultTransport}}
	transport := &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: httpClient}
	client := mcp.NewClient(&mcp.Implementation{Name: "helixqa-verify-mcp-gateway", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("MCP client Connect: %w", err)
	}
	return session, func() { _ = session.Close() }, nil
}

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (rt *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.base.RoundTrip(req)
}

func contentText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func mustJSONString(s string) []byte {
	b, err := json.Marshal(s)
	if err != nil {
		return []byte(`""`)
	}
	return b
}

func failInfra(sink conduit.Sink, cid string, v *verdict, out, reason string) int {
	v.Error = reason
	v.Pass = false
	if writeErr := writeVerdict(out, v); writeErr != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write verdict (after infra error %q): %v\n", reason, writeErr)
	}
	conduit.Errorf(sink, cid, reason)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Fprintf(os.Stderr, "INFRA-ERROR: %s: %s\n", cid, reason)
	return exitInfra
}

func writeVerdict(path string, v *verdict) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
