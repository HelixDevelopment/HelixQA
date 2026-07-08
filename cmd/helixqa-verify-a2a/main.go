// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-a2a is the RUNNABLE analyzer for the HelixQA A2A
// (Google Agent2Agent) capability bank (banks/helixllm_a2a.yaml) — it drives
// a real JSON-RPC 2.0 "message/send" Task against the live HelixLLM A2A
// server (submodules/helix_llm/cmd/a2a-server, default :18441, spec
// docs/research/07.2026/00_master/ACP_A2A_PROVIDER.md), which itself routes
// the Task to the live coder (:18434, read-only, §11.4.119/§11.4.122 — this
// analyzer never starts/stops/restarts the coder or the A2A server; the
// conductor owns that boot/teardown lifecycle). Mirrors the CLI convention
// of cmd/helixqa-verify-translate-nllb (--out/--conduit-dir/--challenge-id/
// --expect-fail, exit 0/1/2) — a thin, project-agnostic dispatches_to
// analyzer (CONST-051(B), no HelixQA core-engine change).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). The verdict
// requires ALL of: (1) the returned Task's status.state == "completed"
// (never inferred from HTTP 200 alone); (2) every --expect token present
// (case-insensitive substring) in the real agent answer extracted from the
// Task's artifacts/history; (3) the answer is NOT identical to the prompt
// (kills the echo/passthrough bluff). --expect-fail inverts which raw
// outcome counts as case success — the RAW pass is always recorded honestly
// (mirrors helixqa-verify-translate-nllb / helixqa-verify-vision). The full
// raw JSON-RPC response body is embedded in the verdict artefact so a
// reviewer never has to trust the analyzer's parsing blindly.
//
// Usage:
//
//	helixqa-verify-a2a \
//	  --prompt "What is 2+2? Reply with only the digit, nothing else." \
//	  --expect "4" \
//	  --out qa-results/helixllm_a2a/understand_001_verdict.json \
//	  [--endpoint http://localhost:18441/a2a] [--bearer <token>] \
//	  [--conduit-dir qa-results/helixllm_a2a/conduit] \
//	  [--challenge-id A2A-UNDERSTAND-001] [--timeout 5m] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitInfra = 2
)

// ---- A2A wire types (a minimal mirror of internal/a2a/types.go's JSON
// shape — this analyzer speaks HTTP/JSON only, it never imports HelixLLM
// Go packages, per CONST-051(B) decoupling). ----

type a2aPart struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
}

type a2aMessage struct {
	Role  string    `json:"role"`
	Parts []a2aPart `json:"parts"`
}

type a2aArtifact struct {
	Name  string    `json:"name,omitempty"`
	Parts []a2aPart `json:"parts"`
}

type a2aTaskStatus struct {
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

type a2aTask struct {
	ID        string        `json:"id"`
	Status    a2aTaskStatus `json:"status"`
	History   []a2aMessage  `json:"history,omitempty"`
	Artifacts []a2aArtifact `json:"artifacts,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// verdict is the machine-readable artefact the ContentAssertingResolver
// gates on — embeds the raw request+response, never trust-the-analyzer-blindly.
type verdict struct {
	Endpoint       string          `json:"endpoint"`
	Prompt         string          `json:"prompt"`
	BearerSet      bool            `json:"bearer_set"`
	TaskID         string          `json:"task_id,omitempty"`
	TaskState      string          `json:"task_state,omitempty"`
	Answer         string          `json:"answer"`
	ExpectedFacts  []string        `json:"expected_facts"`
	ForbiddenFacts []string        `json:"forbidden_facts,omitempty"`
	MatchedFacts   int             `json:"matched_facts"`
	ExpectedCount  int             `json:"expected_count"`
	NotIdentity    bool            `json:"not_identity"`
	Hallucinated   bool            `json:"hallucinated"`
	TaskCompleted  bool            `json:"task_completed"`
	Pass           bool            `json:"pass"`
	ExpectFail     bool            `json:"expect_fail"`
	CaseResult     bool            `json:"case_result"`
	LatencyMS      int64           `json:"latency_ms"`
	HTTPStatus     int             `json:"http_status"`
	RawResponse    json.RawMessage `json:"raw_response,omitempty"`
	Error          string          `json:"error,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		prompt     = flag.String("prompt", "", "the user prompt to send in the A2A Task (required)")
		expectCSV  = flag.String("expect", "", "comma-separated required fact tokens (case-insensitive substring match, required)")
		forbidCSV  = flag.String("forbid", "", "comma-separated forbidden fact tokens")
		endpoint   = flag.String("endpoint", envOr("HELIX_A2A_ENDPOINT", "http://localhost:18441/a2a"), "A2A JSON-RPC dispatch endpoint")
		bearer     = flag.String("bearer", os.Getenv("HELIX_A2A_BEARER_TOKEN"), "optional Bearer token (§11.4.10 — never logged verbatim)")
		out        = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID    = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout    = flag.Duration("timeout", 5*time.Minute, "request timeout (code generation is not instantaneous)")
		expectFail = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *prompt == "" || *expectCSV == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-a2a --prompt <text> --expect <a,b> --out <verdict.json> [--forbid c,d] [--bearer tok]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_a2a", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "a2a")

	v := verdict{
		Endpoint:       *endpoint,
		Prompt:         *prompt,
		BearerSet:      *bearer != "",
		ExpectedFacts:  splitCSV(*expectCSV),
		ForbiddenFacts: splitCSV(*forbidCSV),
	}
	v.ExpectedCount = len(v.ExpectedFacts)

	reqBody, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "message/send",
		Params: map[string]any{
			"message": a2aMessage{Role: "user", Parts: []a2aPart{{Kind: "text", Text: *prompt}}},
		},
	})

	client := &http.Client{Timeout: *timeout}
	start := time.Now()
	httpReq, err := http.NewRequest(http.MethodPost, *endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if *bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+*bearer)
	}
	resp, err := client.Do(httpReq)
	latency := time.Since(start)
	v.LatencyMS = latency.Milliseconds()
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("http request: %v", err))
	}
	defer resp.Body.Close()
	v.HTTPStatus = resp.StatusCode
	rawBody, _ := io.ReadAll(resp.Body)
	v.RawResponse = json.RawMessage(rawBody)

	if resp.StatusCode != http.StatusOK {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("non-200 status=%d body=%s", resp.StatusCode, string(rawBody)))
	}

	var rr rpcResponse
	if err := json.Unmarshal(rawBody, &rr); err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode JSON-RPC envelope: %v", err))
	}
	if rr.Error != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("JSON-RPC error: code=%d message=%s", rr.Error.Code, rr.Error.Message))
	}

	var task a2aTask
	if err := json.Unmarshal(rr.Result, &task); err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode Task result: %v", err))
	}
	v.TaskID = task.ID
	v.TaskState = task.Status.State
	v.TaskCompleted = task.Status.State == "completed"

	// --- extract the agent's answer text: prefer the first artifact's text
	// parts (handlers.go always sets Artifacts[0]={"generated-code",...} on
	// success), fall back to the last agent-role history message. ---
	v.Answer = extractAnswer(task)

	// --- strict fact-matching + not-identity (§11.4.6/§11.4.107(10)) ---
	fwdNorm := normalize(v.Answer)
	srcNorm := normalize(v.Prompt)
	v.NotIdentity = strings.TrimSpace(v.Answer) != "" && fwdNorm != srcNorm

	lowerAns := strings.ToLower(v.Answer)
	matched := 0
	for _, f := range v.ExpectedFacts {
		if strings.Contains(lowerAns, strings.ToLower(f)) {
			matched++
		}
	}
	v.MatchedFacts = matched

	hallucinated := false
	for _, f := range v.ForbiddenFacts {
		if f != "" && strings.Contains(lowerAns, strings.ToLower(f)) {
			hallucinated = true
			break
		}
	}
	v.Hallucinated = hallucinated

	v.Pass = v.TaskCompleted && v.NotIdentity && v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && !v.Hallucinated

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
	conduit.EvidenceCaptured(sink, cid, "a2a_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s state=%s notIdentity=%v matched=%d/%d hallucinated=%v expect_fail=%v raw_pass=%v answer=%q\n",
			cid, v.TaskState, v.NotIdentity, v.MatchedFacts, v.ExpectedCount, v.Hallucinated, v.ExpectFail, v.Pass, v.Answer)
		return exitPass
	}
	reason := fmt.Sprintf("state=%s notIdentity=%v matched=%d/%d hallucinated=%v expect_fail=%v raw_pass=%v", v.TaskState, v.NotIdentity, v.MatchedFacts, v.ExpectedCount, v.Hallucinated, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s answer=%q\n", cid, reason, v.Answer)
	return exitFail
}

// extractAnswer prefers the first artifact's concatenated text parts
// (handlers.go's handleMessageSend always sets Artifacts on a completed
// Task), falling back to the last agent-role History message.
func extractAnswer(task a2aTask) string {
	for _, art := range task.Artifacts {
		var sb strings.Builder
		for _, p := range art.Parts {
			if p.Kind == "text" {
				sb.WriteString(p.Text)
			}
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}
	for i := len(task.History) - 1; i >= 0; i-- {
		msg := task.History[i]
		if msg.Role != "agent" {
			continue
		}
		var sb strings.Builder
		for _, p := range msg.Parts {
			if p.Kind == "text" {
				sb.WriteString(p.Text)
			}
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}
	return ""
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
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
