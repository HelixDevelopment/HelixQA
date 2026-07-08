// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-rag is the RUNNABLE analyzer for the HelixQA
// CPU RAG bank (banks/helixllm_rag.yaml) — it drives a real end-to-end
// Retrieval-Augmented-Generation pipeline against a live CPU embeddings
// endpoint (HF TEI, default :18435) + the live coder LLM (default :18434,
// read-only), reusing the boot mechanism proven in
// submodules/helix_llm/docs/qa/phase3_rag_20260707/harness (§11.4.74).
// It embeds a fixture corpus + a query on TEI, retrieves the real-cosine
// top-1 document, grounds the coder on the top-k retrieved docs, generates,
// and asserts BOTH: (1) retrieval top-1 == the expected fact-bearing doc,
// AND (2) the generated answer genuinely contains the INVENTED fact token
// the coder cannot know from training. Mirrors the CLI convention of
// cmd/helixqa-verify-vision (--out/--conduit-dir/--challenge-id/
// --expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.115/§11.4.123). The
// invented fact (e.g. "Borealis-9") appears in exactly ONE corpus doc and
// nowhere in the model's training — so a PASS is unfakeable proof the
// generation is genuinely GROUNDED in retrieval, not recalled or hardcoded.
// --no-context skips retrieval and asks the bare question (the §11.4.115 RED
// scenario: "broken" = no grounding provided → the coder cannot produce the
// invented token → raw fail); combined with --expect-fail it is the
// golden-bad self-validation case (the analyzer MUST correctly report the
// invented token is absent).
//
// Usage:
//
//	helixqa-verify-rag \
//	  --query "What is the internal codename for the 2027 HelixCode release?" \
//	  --expect-token "Borealis-9" --expect-doc doc_codename \
//	  --out qa-results/helixllm_rag/q1_verdict.json \
//	  [--embed-endpoint http://localhost:18435/v1/embeddings] \
//	  [--chat-endpoint http://localhost:18434/v1/chat/completions] \
//	  [--top-k 2] [--conduit-dir ...] [--challenge-id RAG-Q1-001]
//	  [--no-context] [--expect-fail] [--timeout 120s]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"digital.vasic.helixqa/pkg/conduit"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitInfra = 2
)

// corpusDoc — a fixture document. Two carry an INVENTED fact the served LLM
// cannot know from training; four are topic-adjacent/unrelated distractors so
// retrieval must genuinely discriminate. Identical to the phase-3 RAG proof.
type corpusDoc struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

var defaultCorpus = []corpusDoc{
	{"doc_codename", "The internal codename for the 2027 HelixCode release is Borealis-9."},
	{"doc_sidecar", "The internal service-mesh sidecar used internally for HelixLLM routing is called Quillfeather-7."},
	{"doc_cat", "The cat sat on the mat."},
	{"doc_revenue", "Quarterly revenue rose four percent."},
	{"doc_coffee", "Coffee brewing techniques improve flavor extraction using controlled water temperature."},
	{"doc_ci", "The HelixCode continuous integration pipeline runs unit tests before every merge to main."},
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}
type embeddingItem struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}
type embeddingResponse struct {
	Data  []embeddingItem `json:"data"`
	Model string          `json:"model"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Messages    []chatMessage `json:"messages"`
}
type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type rankedDoc struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

type verdict struct {
	Query         string      `json:"query"`
	ExpectToken   string      `json:"expect_token"`
	ExpectDoc     string      `json:"expect_doc"`
	EmbedEndpoint string      `json:"embed_endpoint"`
	ChatEndpoint  string      `json:"chat_endpoint"`
	NoContext     bool        `json:"no_context"`
	TopK          int         `json:"top_k"`
	Ranking       []rankedDoc `json:"ranking,omitempty"`
	Top1ID        string      `json:"top1_id,omitempty"`
	Top1Score     float64     `json:"top1_score,omitempty"`
	RetrievalOK   bool        `json:"retrieval_ok"`
	Answer        string      `json:"answer"`
	TokenFound    bool        `json:"token_found"`
	Pass          bool        `json:"pass"`
	ExpectFail    bool        `json:"expect_fail"`
	CaseResult    bool        `json:"case_result"`
	LatencyMS     int64       `json:"latency_ms"`
	Error         string      `json:"error,omitempty"`
}

func main() { os.Exit(run()) }

func run() int {
	var (
		query      = flag.String("query", "", "the question to ask (required)")
		expectTok  = flag.String("expect-token", "", "the invented fact token the grounded answer must contain (required)")
		expectDoc  = flag.String("expect-doc", "", "the corpus doc id retrieval top-1 must equal (required unless --no-context)")
		corpusFile = flag.String("corpus-file", "", "optional JSON file [{id,text},...] overriding the built-in fixture corpus")
		embedEP    = flag.String("embed-endpoint", envOr("HELIXLLM_EMBED_ENDPOINT", "http://localhost:18435/v1/embeddings"), "TEI /v1/embeddings endpoint")
		chatEP     = flag.String("chat-endpoint", envOr("HELIXLLM_CHAT_ENDPOINT", "http://localhost:18434/v1/chat/completions"), "coder /v1/chat/completions endpoint")
		topK       = flag.Int("top-k", 2, "number of retrieved docs to stuff into the grounded prompt")
		noContext  = flag.Bool("no-context", false, "skip retrieval; ask the bare question (RED scenario — the coder cannot know the invented fact)")
		out        = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID    = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout    = flag.Duration("timeout", 120*time.Second, "per-request timeout")
		expectFail = flag.Bool("expect-fail", false, "invert case-level exit code — for the golden-bad self-validation fixture")
	)
	flag.Parse()

	if *query == "" || *expectTok == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-rag --query <q> --expect-token <tok> --expect-doc <id> --out <verdict.json> [--no-context] [--expect-fail]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_rag", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "rag")

	corpus := defaultCorpus
	if *corpusFile != "" {
		b, err := os.ReadFile(*corpusFile)
		if err != nil {
			return failInfra(sink, cid, &verdict{Query: *query, ExpectToken: *expectTok, ExpectDoc: *expectDoc, EmbedEndpoint: *embedEP, ChatEndpoint: *chatEP}, *out, fmt.Sprintf("read corpus-file: %v", err))
		}
		if err := json.Unmarshal(b, &corpus); err != nil {
			return failInfra(sink, cid, &verdict{Query: *query, ExpectToken: *expectTok, ExpectDoc: *expectDoc, EmbedEndpoint: *embedEP, ChatEndpoint: *chatEP}, *out, fmt.Sprintf("parse corpus-file: %v", err))
		}
	}

	v := verdict{
		Query:         *query,
		ExpectToken:   *expectTok,
		ExpectDoc:     *expectDoc,
		EmbedEndpoint: *embedEP,
		ChatEndpoint:  *chatEP,
		NoContext:     *noContext,
		TopK:          *topK,
	}

	client := &http.Client{Timeout: *timeout}
	ctx := context.Background()
	start := time.Now()

	var groundedDocs []corpusDoc
	if !*noContext {
		// Embed corpus + query (single batched request keeps the vectors in
		// the SAME model space — no cross-model drift).
		inputs := make([]string, 0, len(corpus)+1)
		for _, d := range corpus {
			inputs = append(inputs, d.Text)
		}
		inputs = append(inputs, *query)
		er, err := doEmbed(ctx, client, *embedEP, inputs)
		if err != nil {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("embed: %v", err))
		}
		if len(er.Data) != len(corpus)+1 {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("expected %d vectors, got %d", len(corpus)+1, len(er.Data)))
		}
		byIdx := map[int][]float64{}
		for _, it := range er.Data {
			byIdx[it.Index] = it.Embedding
		}
		qVec := byIdx[len(corpus)]
		ranked := make([]rankedDoc, len(corpus))
		for i, d := range corpus {
			ranked[i] = rankedDoc{ID: d.ID, Score: cosine(byIdx[i], qVec)}
		}
		sort.Slice(ranked, func(a, b int) bool { return ranked[a].Score > ranked[b].Score })
		v.Ranking = ranked
		v.Top1ID = ranked[0].ID
		v.Top1Score = ranked[0].Score
		v.RetrievalOK = v.Top1ID == v.ExpectDoc

		n := *topK
		if n > len(ranked) {
			n = len(ranked)
		}
		for i := 0; i < n; i++ {
			groundedDocs = append(groundedDocs, corpusDoc{ID: ranked[i].ID, Text: corpusText(corpus, ranked[i].ID)})
		}
	}

	// Generate on the live coder.
	var msgs []chatMessage
	if *noContext {
		msgs = []chatMessage{{Role: "user", Content: *query + " Answer in one short sentence."}}
	} else {
		var ctxLines []string
		for i, d := range groundedDocs {
			ctxLines = append(ctxLines, fmt.Sprintf("%d. %s", i+1, d.Text))
		}
		system := "You are a helpful assistant. Answer the user's question using ONLY the information " +
			"in the CONTEXT below. Quote the exact term or name from the context in your answer. " +
			"If the context does not contain the answer, say \"I don't know.\" Do not use any information " +
			"beyond the given context."
		user := "CONTEXT:\n" + strings.Join(ctxLines, "\n") + "\n\nQUESTION: " + *query
		msgs = []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}
	}
	cr, err := doChat(ctx, client, *chatEP, msgs)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("chat: %v", err))
	}
	v.LatencyMS = time.Since(start).Milliseconds()
	if len(cr.Choices) > 0 {
		v.Answer = cr.Choices[0].Message.Content
	}
	v.TokenFound = containsToken(v.Answer, *expectTok)

	// PASS requires: (with context) retrieval top-1 correct AND token found;
	// (no-context) the raw pass is definitionally false (no grounding).
	if *noContext {
		v.Pass = v.TokenFound // will be false for a genuine invented fact
	} else {
		v.Pass = v.RetrievalOK && v.TokenFound && strings.TrimSpace(v.Answer) != ""
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

	conduit.EvidenceCaptured(sink, cid, "rag_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s top1=%s(ok=%v) tokenFound=%v no_context=%v expect_fail=%v raw_pass=%v answer=%q\n",
			cid, v.Top1ID, v.RetrievalOK, v.TokenFound, v.NoContext, v.ExpectFail, v.Pass, v.Answer)
		return exitPass
	}
	reason := fmt.Sprintf("top1=%s(ok=%v) tokenFound=%v no_context=%v expect_fail=%v raw_pass=%v", v.Top1ID, v.RetrievalOK, v.TokenFound, v.NoContext, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s answer=%q\n", cid, reason, v.Answer)
	return exitFail
}

func doEmbed(ctx context.Context, client *http.Client, endpoint string, inputs []string) (embeddingResponse, error) {
	body, _ := json.Marshal(embeddingRequest{Model: "helix-embed", Input: inputs})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return embeddingResponse{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return embeddingResponse{}, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
	}
	var er embeddingResponse
	if err := json.Unmarshal(b, &er); err != nil {
		return embeddingResponse{}, err
	}
	return er, nil
}

func doChat(ctx context.Context, client *http.Client, endpoint string, msgs []chatMessage) (chatResponse, error) {
	body, _ := json.Marshal(chatRequest{Model: "coder", Temperature: 0, MaxTokens: 200, Messages: msgs})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return chatResponse{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return chatResponse{}, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
	}
	var cr chatResponse
	if err := json.Unmarshal(b, &cr); err != nil {
		return chatResponse{}, err
	}
	return cr, nil
}

func corpusText(corpus []corpusDoc, id string) string {
	for _, d := range corpus {
		if d.ID == id {
			return d.Text
		}
	}
	return ""
}

func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.NaN()
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return math.NaN()
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func normalizeToken(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func containsToken(haystack, needle string) bool {
	if strings.TrimSpace(needle) == "" {
		return false
	}
	return strings.Contains(normalizeToken(haystack), normalizeToken(needle))
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
