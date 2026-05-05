# CLAUDE.md - HelixCode AI Agent Manual

## HelixCode - AI Agent Operating Manual

**Version**: 1.0.0
**Date**: 2026-04-30
**Scope**: This document guides AI agents working on the HelixCode codebase
**Authority**: Cascaded from HelixAgent root `CLAUDE.md` with HelixCode-specific addenda

---

## 1. Agent Identity & Purpose

You are an AI agent working on **HelixCode**, an enterprise-grade distributed AI development platform. Your work directly impacts the quality and usability of a production system.

**Your mandate**: Write real, working, tested code. No simulations. No placeholders. No "for now" implementations. Every feature you implement MUST actually work when a user invokes it.

### 1.1 Peer Governance Documents (keep in sync)
This `CLAUDE.md` sits alongside several other agent/governance manuals at the repo root. They overlap and must remain consistent:
- `CONSTITUTION.md` — source of truth for all mandates (CONST-033, CONST-035, CONST-036–040, Article XI §11.9). When this file conflicts with the Constitution, the Constitution wins.
- `AGENTS.md` — generic agent manual (40 KB; mirror anti-bluff rules here).
- `CRUSH.md`, `QWEN.md` — sibling agent manuals for other CLI tools. Cascade rule changes to all of them.
- `HelixCode/CLAUDE.md`, `HelixQA/CLAUDE.md`, `Challenges/CLAUDE.md` — submodule-scoped manuals; this root file inherits from them and they inherit from this one.

---

## 2. Universal Mandatory Rules (Non-Negotiable)

These rules cascade from the HelixCode Constitution. They are permanent and apply to every task.

### Rule 1: No CI/CD Pipelines
No `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline. All builds and tests run manually or via Makefile/script targets.

### Rule 2: No Mocks in Production
Mocks, stubs, fakes, placeholder classes, TODO implementations are STRICTLY FORBIDDEN in production code. Only unit tests may use mocks.

### Rule 3: No HTTPS for Git
SSH URLs only (`git@github.com:…`) for all Git operations.

### Rule 4: No Manual Container Commands
Use the orchestrator binary (`make build` → `./bin/<app>`). Direct `docker`/`docker-compose` commands are prohibited as workflows.

### Rule 5: Real Data for Non-Unit Tests
All integration, E2E, and challenge tests MUST use real infrastructure (real databases, real HTTP calls, real containers).

### Rule 6: 100% Challenge Coverage
Every component MUST have Challenge scripts validating real-life use cases.

### Rule 7: Reproduction-Before-Fix
Every bug MUST be reproduced by a Challenge script BEFORE any fix is attempted.

### Rule 8: Definition of Done
A change is NOT done because code compiles. "Done" requires pasted terminal output from a real run against real artifacts.

### Rule 9: No Self-Certification
Words like *verified, tested, working, complete, fixed, passing* are forbidden unless accompanied by pasted command output from that session.

### Rule 10: Zero-Bluff Mandate (CONST-035)
A passing test is a claim that the feature **works for the end user**. Every test must guarantee Quality + Completion + Full Usability. Any test that doesn't certify all three is a bluff and must be tightened.

---

## Constitutional anchors (cascaded from `CONSTITUTION.md`)

### Article XI §11.9 — Anti-Bluff Forensic Anchor
> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*
>
> Operative rule: **The bar for shipping is not "tests pass" but "users can use the feature."** Every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks. No false-success results are tolerable.

### Article XII §12.1 (CONST-042) — No-Secret-Leak
No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak is a release blocker until rotated and post-mortemed.

### Article XII §12.2 (CONST-043) — No-Force-Push
No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval per operation. Authorization for one push does not extend further. Bypassing hooks / signing / protected-branch rules also requires explicit approval.

---

## 3. HelixCode-Specific Architecture

### 3.1 Technology Stack
- **Language**: Go — root meta-repo on `go 1.25.2`, inner Go application (`HelixCode/`) on `go 1.26`. Keep both modules current; do not downgrade.
- **Module IDs**: root `dev.helix.code` (thin), inner `dev.helix.code` (full app + transitive deps).
- **HTTP / API**: Gin v1.11.0, gorilla/websocket v1.5.3, gRPC v1.80.0.
- **Persistence**: PostgreSQL 15+ via pgx/v5 + lib/pq; Redis 7+ via go-redis/v9.
- **AuthN/Z**: golang-jwt/v4 v4.5.2, bcrypt/argon2 (`golang.org/x/crypto`), oauth2.
- **Config / CLI**: Viper v1.21.0, Cobra v1.8.0, pflag v1.0.10, fsnotify v1.9.0.
- **LLM / Cloud**: AWS Bedrock runtime (aws-sdk-go-v2), Azure azcore/azidentity, getzep/zep-go/v3, smacker/go-tree-sitter.
- **UI**: Fyne v2.7.0 (desktop GUI), tview / tcell/v2 (terminal UI), chromedp (headless browser).
- **Testing**: stretchr/testify v1.11.1.

### 3.2 Repository Layout — Meta-Repo + Submodules

**This repo is a governance/meta-repo, not the Go application.** The actual Go binary lives in the `HelixCode/` subdirectory (a submodule). When an agent says "edit `internal/auth`," they almost always mean `HelixCode/internal/auth`, not the root `internal/`.

```
HelixCode/                                # ← repo root (governance + submodules)
├── CLAUDE.md / AGENTS.md / CONSTITUTION.md / CRUSH.md / QWEN.md   # agent manuals
├── Makefile                              # governance gates only (see §3.4)
├── go.mod                                # thin root module (dev.helix.code, go 1.25.2)
├── helix                                 # Docker facade script (run platform standalone)
├── setup.sh                              # one-shot: submodule init + deps + build
├── .gitmodules                           # source of truth for submodule wiring
├── docker-compose.helix.yml              # standalone deployment
├── internal/{fix,security,testing,theme} # root-level helpers ONLY (NOT the app)
├── cmd/security-test/                    # root-level security-test tool ONLY
├── scripts/                              # init-submodules, propagate-governance,
│                                         #   verify-governance-cascade, no-silent-skips,
│                                         #   demo-all, run-all-tests, …
├── docs/                                 # ARCHITECTURE.md, COMPLETE_*.md guides,
│                                         #   bluff-proofing/, llms_verifier/, helix_qa/
│
├── HelixCode/      ← TRACKED SUBDIRECTORY (NOT a submodule — meta-repo's primary inner directory; circular reference if promoted; see §3.2.1)
├── HelixQA/        ← SUBMODULE: QA / challenge-orchestration platform
├── Challenges/     ← SUBMODULE: cross-cutting Challenge bank (Panoptic, banks/)
├── Containers/     ← SUBMODULE: Docker/container artefacts
├── Dependencies/   ← SUBMODULES: LLama_CPP, Ollama, HuggingFace_Hub, …
├── Security/       ← SUBMODULE: security tooling
├── Assets/         ← SUBMODULE: logos, themes, brand
├── Github-Pages-Website/ ← SUBMODULE: marketing site
└── Example_Projects/     ← reference projects (Aider, Cline, Plandex, OpenHands, …)
```

#### 3.2.1 Inner Go application — `HelixCode/` submodule

```
HelixCode/HelixCode/                      # module dev.helix.code, go 1.26
├── Makefile                              # real build/test targets (see §3.4)
├── cmd/
│   ├── server/                           # HTTP server entry → bin/helixcode
│   ├── cli/                              # CLI client entry → bin/cli
│   ├── helix-config/                     # config tool
│   ├── config-test/                      # config validator
│   ├── security-test/, security-fix*/    # security tools
│   └── performance-optimization*/        # perf tools
├── internal/                             # ~45 packages — the real domain code
│   ├── auth/        agent/      cognee/      commands/   config/
│   ├── context/     database/   deployment/  discovery/  editor/
│   ├── event/       focus/      hardware/    helixqa/    hooks/
│   ├── llm/         logging/    logo/        mcp/        memory/
│   ├── monitoring/  notification/ performance/ persistence/ project/
│   ├── provider/    providers/  redis/       repomap/    rules/
│   ├── security/    server/     session/     task/       template/
│   ├── tools/       verifier/   version/     worker/     workflow/
│   ├── adapters/    fix/        testutil/    mocks/      # mocks/ is unit-test-only
├── applications/
│   ├── desktop/      (Fyne GUI)
│   ├── terminal-ui/  (tview TUI)
│   ├── ios/  android/  aurora-os/  harmony-os/
├── tests/
│   ├── e2e/challenges/   # E2E challenge runner (cmd/runner/main.go)
│   ├── integration/      # gated by `-tags=integration`
│   ├── unit/             # mocks ALLOWED here only
│   ├── security/         # security suite
│   └── performance/      # benchmarks
├── config/                # YAML configs (dev/, prod/, test/)
├── docker/  scripts/  shared/  qa-integration/
└── docker-compose.full-test.yml + .env.full-test    # zero-skip integration stack
```

**Cardinal rule:** if a path in instructions doesn't start with `HelixCode/`, `HelixQA/`, etc., assume it is relative to the inner Go module and prefix with `HelixCode/`.

### 3.3 Historical Bluffs — Resolved, Guard Against Regression

The three patterns below were live bluffs in earlier revisions of `HelixCode/cmd/cli/main.go`. They have been fixed (verify with `grep -rn "simulate\|For now\|TODO implement\|placeholder" HelixCode/cmd/cli/main.go` — must return empty). Treat these as canonical anti-pattern examples; if a future change reintroduces any of them, the change is broken regardless of whether tests pass.

#### BLUFF-001: LLM Generation is Simulated
**Location**: `HelixCode/cmd/cli/main.go` → function `handleGenerate`
**Status**: RESOLVED — now calls `provider.Generate` / `GenerateStream` directly. Do not regress.
**Code Pattern**:
```go
// ANTI-BLUFF: NEVER write code like this
// "For now, simulate generation"
// "In production, this would use the actual LLM provider"

// WRONG - SIMULATION:
response := fmt.Sprintf("Generated response for: %s\n\nThis is a simulated response...")

// CORRECT - REAL IMPLEMENTATION:
resp, err := c.llmProvider.Generate(ctx, req)
if err != nil {
    return fmt.Errorf("generation failed: %w", err)
}
fmt.Println(resp.Text)
```

**Agent Rule**: When implementing LLM-related code, you MUST make real HTTP calls to real providers. NEVER simulate responses.

### 3.4 Build & Test Commands

Two Makefiles. The **root** Makefile only runs governance gates; the **inner** `HelixCode/Makefile` does real builds and tests. Always know which directory you are in.

**Root governance gates** (run from repo root):
```bash
make no-silent-skips         # fail on bare t.Skip() without SKIP-OK marker
make demo-all                # run every submodule's demo (proves they actually run)
make demo-one MOD=<name>     # run one submodule's demo
make ci-validate-all         # all governance gates in warn-mode
./setup.sh                   # first-time: submodules + system deps + build
./scripts/init-submodules.sh                 # init all submodules
./scripts/propagate-governance.sh            # cascade Constitution/CLAUDE/AGENTS
./scripts/verify-governance-cascade.sh       # confirm anchors present in submodules
./helix start | stop | logs | shell          # Docker facade for the platform
```

**Inner application** (run from `HelixCode/`):
```bash
make build                   # → bin/helixcode (server)
make verify-compile          # quick compile-only sanity check
make test                    # all unit tests
make test-coverage           # coverage with -race
make fmt                     # gofmt
make lint                    # golangci-lint run
make dev                     # build + run with config/dev/config.yaml
make prod                    # cross-compile linux/macos/windows
```

**Full integration / E2E** (real PostgreSQL + Redis + Ollama via docker-compose):
```bash
make test-infra-up                           # start docker-compose.full-test.yml
make test-infra-status                       # check stack health
make test-full                               # ALL tests, ZERO skips
make test-unit-full / test-integration-full / test-e2e-full / test-security-full
make test-verifier-unit / test-verifier-integration / test-verifier-challenges
make test-infra-down                         # tear down stack + volumes
```

**Containerized builds** (no host Go required):
```bash
make container-builder-image    # build the builder image once
make container-build            # build inside container
make container-test             # test inside container
make container-shell            # interactive shell in builder
make container-release          # full release in container
```

**Single-test invocation** (inner module):
```bash
cd HelixCode
go test -v -run TestJWTGenerate ./internal/auth                          # single unit test
go test -v -tags=integration -run TestAPI_CreateTask ./tests/integration/...
go test -v -count=1 ./internal/verifier/...                              # disable test cache
go test -v -race -coverprofile=cover.out ./internal/llm                  # one pkg with race+cover
```

**E2E challenges** (real, end-to-end, runtime evidence required):
```bash
cd HelixCode/tests/e2e/challenges && go run cmd/runner/main.go -all
# Or root-level cross-cutting Challenges:
cd Challenges && make <target>
```

**Anti-bluff smoke check** (must always pass):
```bash
grep -rn "simulated\|for now\|TODO implement\|placeholder" \
  HelixCode/internal HelixCode/cmd && echo "BLUFF FOUND" || echo "clean"
```

**Platform / mobile builds** (inner module):
```bash
make desktop / desktop-nogui / desktop-linux / desktop-macos / desktop-windows
make mobile-init && make mobile-ios && make mobile-android
make aurora-os && make harmony-os
```

#### BLUFF-002: Model Listing is Hardcoded
**Location**: `HelixCode/cmd/cli/main.go` → function `handleListModels`
**Status**: RESOLVED — must continue to query `c.providerManager.GetProviders()` per CONST-036/037 (LLMsVerifier is the single source of truth).
**Correct Pattern**:
```go
func (c *CLI) handleListModels(ctx context.Context) error {
    // Query ALL configured providers
    for name, provider := range c.providerManager.GetProviders() {
        models, err := provider.GetModels()
        if err != nil {
            log.Printf("Warning: failed to list models from %s: %v", name, err)
            continue
        }
        // Display real models
        for _, model := range models {
            fmt.Printf("%s/%s: %s (context: %d)\n", name, model.ID, model.Name, model.ContextSize)
        }
    }
    return nil
}
```

#### BLUFF-003: Command Execution is Simulated
**Location**: `HelixCode/cmd/cli/main.go` → function `handleCommand`
**Status**: RESOLVED — must continue to use `os/exec` via `exec.CommandContext` and surface real exit codes. Never replace with print-and-sleep.
**Correct Pattern**:
```go
func (c *CLI) handleCommand(ctx context.Context, command string) error {
    // ANTI-BLUFF: Actually execute the command
    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    cmd.Dir = c.workingDirectory
    
    output, err := cmd.CombinedOutput()
    
    fmt.Printf("Exit code: %d\n", cmd.ProcessState.ExitCode())
    fmt.Printf("Output:\n%s\n", string(output))
    
    return err
}
```

---

## 4. Code Patterns for Agents

### 4.1 Interface-Driven Design
```go
// Define the contract
type Provider interface {
    Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    GetModels() ([]Model, error)
    HealthCheck(ctx context.Context) error
}

// Implement with REAL behavior
type OllamaProvider struct { ... }
func (p *OllamaProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    // Make REAL HTTP call
    // NO simulation
}
```

### 4.2 Manager Pattern
```go
type TaskManager struct {
    db     TaskRepository
    mu     sync.RWMutex
    tasks  map[uuid.UUID]*Task
}

func (m *TaskManager) Create(ctx context.Context, task *Task) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Persist to REAL database
    if err := m.db.Save(ctx, task); err != nil {
        return fmt.Errorf("failed to save task: %w", err)
    }
    
    m.tasks[task.ID] = task
    return nil
}
```

### 4.3 Error Handling
```go
// Package-level errors
var (
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrTokenExpired       = errors.New("token expired")
)

// Contextual wrapping
func (s *Service) DoSomething(ctx context.Context) error {
    result, err := s.db.Query(ctx)
    if err != nil {
        return fmt.Errorf("failed to query database for user %s: %w", userID, err)
    }
    
    if err := s.process(result); err != nil {
        return fmt.Errorf("failed to process query result: %w", err)
    }
    
    return nil
}
```

### 4.4 Testing Pattern (Unit)
```go
func TestService_DoSomething(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*mockRepository)
        wantErr bool
    }{
        {
            name: "success",
            setup: func(m *mockRepository) {
                m.On("Query", mock.Anything).Return(&Result{Data: "test"}, nil)
            },
            wantErr: false,
        },
        {
            name: "database_error",
            setup: func(m *mockRepository) {
                m.On("Query", mock.Anything).Return(nil, errors.New("connection refused"))
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := new(mockRepository)
            tt.setup(repo)
            
            svc := NewService(repo)
            err := svc.DoSomething(context.Background())
            
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
            
            repo.AssertExpectations(t)
        })
    }
}
```

### 4.5 Testing Pattern (Integration - NO MOCKS)
```go
func TestAPI_CreateTask_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Integration test skipped in short mode")
    }
    
    // Start REAL PostgreSQL container
    dbContainer := startPostgresContainer(t)
    defer dbContainer.Terminate(context.Background())
    
    // Connect to REAL database
    db := connectToPostgres(dbContainer)
    
    // Initialize REAL service
    taskMgr := task.NewManager(db)
    
    // ANTI-BLUFF: Test with REAL data
    task, err := taskMgr.Create(context.Background(), &task.Task{
        Title: "Integration Test Task",
    })
    
    require.NoError(t, err)
    require.NotZero(t, task.ID)
    
    // ANTI-BLUFF: Verify it REALLY exists in database
    persisted, err := taskMgr.Get(context.Background(), task.ID)
    require.NoError(t, err)
    require.Equal(t, "Integration Test Task", persisted.Title)
}
```

---

## 5. Anti-Bluff Checklist for Every Task

Before marking any task complete, verify:

- [ ] **No simulation**: Code doesn't contain "simulate", "for now", "TODO implement", "placeholder"
- [ ] **Real HTTP calls**: API clients make actual HTTP requests with real bodies
- [ ] **Real database operations**: Database code uses real queries, not in-memory maps (unless explicitly caching)
- [ ] **Real process execution**: Shell/command execution uses `os/exec`, not `fmt.Printf` + `time.Sleep`
- [ ] **Real file operations**: File tools use `os.ReadFile`/`os.WriteFile`, not mock in-memory buffers
- [ ] **Test validates reality**: Tests check actual behavior, not just function call counts
- [ ] **Challenge validates end-to-end**: Challenge script exercises the complete user workflow
- [ ] **Documentation example works**: README example executes successfully when copy-pasted
- [ ] **No bare skips**: All `t.Skip()` have `SKIP-OK: #<ticket>` markers
- [ ] **Evidence pasted**: Commit/PR contains actual terminal output from real execution

---

## 6. Common Anti-Patterns to Avoid

### ANTI-PATTERN 1: The Simulation Trap
```go
// WRONG
func Generate(prompt string) string {
    // For now, just return a simulated response
    return fmt.Sprintf("Generated: %s", prompt)
}

// CORRECT
func (p *Provider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    resp, err := p.client.Post(p.endpoint, req)
    if err != nil {
        return nil, fmt.Errorf("generation request failed: %w", err)
    }
    return parseResponse(resp)
}
```

### ANTI-PATTERN 2: The Hardcoded List
```go
// WRONG
func ListModels() []Model {
    return []Model{
        {"llama-3-8b", "Llama 3 8B"},
        {"mistral-7b", "Mistral 7B"},
    }
}

// CORRECT
func (p *Provider) GetModels() ([]Model, error) {
    resp, err := p.client.Get(p.baseURL + "/api/tags")
    if err != nil {
        return nil, err
    }
    return parseModelList(resp)
}
```

### ANTI-PATTERN 3: The Stub Interface
```go
// WRONG
type WorkerPool struct {}
func (p *WorkerPool) AddWorker(w *Worker) error {
    return nil  // TODO: implement
}

// CORRECT
func (p *SSHWorkerPool) AddWorker(ctx context.Context, w *SSHWorker) error {
    client, err := ssh.Dial("tcp", w.Host, w.SSHConfig)
    if err != nil {
        return fmt.Errorf("failed to connect to worker %s: %w", w.Host, err)
    }
    defer client.Close()
    
    // Verify worker has helix binary
    session, err := client.NewSession()
    if err != nil {
        return fmt.Errorf("failed to create SSH session: %w", err)
    }
    defer session.Close()
    
    // Actually test the worker
    output, err := session.Output("which helix || echo 'NOT_INSTALLED'")
    if strings.Contains(string(output), "NOT_INSTALLED") {
        // Auto-install
        if err := p.installWorker(ctx, client); err != nil {
            return fmt.Errorf("failed to install worker: %w", err)
        }
    }
    
    p.workers[w.Hostname] = w
    return nil
}
```

---

## 7. Working with Submodules

HelixCode has 80+ submodules. When working with them:

1. **Check governance**: Does the submodule have Constitution.md / CLAUDE.md / AGENTS.md?
2. **Add if missing**: Create governance files referencing parent
3. **Verify builds**: Does the submodule actually compile?
4. **Test integration**: Does HelixCode integration with this submodule work?

---

## 8. Emergency Procedures

### If You Discover a Bluff
1. STOP working on dependent features
2. Document the bluff in `docs/issues/BLUFFS.md`
3. Write a Challenge that reproduces the bluff
4. Fix the bluff
5. Verify the Challenge now passes
6. Update documentation to reflect reality

### If a Test Passes But Feature Doesn't Work
1. The test is a bluff - tighten it
2. Add assertions that verify actual output quality
3. Add anti-bluff checks (no "simulated" in responses)
4. Run the test against real infrastructure
5. Verify it FAILS with the broken code
6. Then fix the code

---

## 9. Reference Commands

The full command catalog lives in **§3.4 Build & Test Commands**. The block below is only the smoke-test you should run before claiming any change is done.

```bash
# 1. Compiles?
cd HelixCode && make verify-compile

# 2. Unit tests (mocks allowed only here)
cd HelixCode && go test -count=1 ./...

# 3. Anti-bluff scan
grep -rn "simulated\|for now\|TODO implement\|placeholder" \
  HelixCode/internal HelixCode/cmd && echo "BLUFF FOUND" || echo "clean"

# 4. Real LLM end-to-end (requires `make test-infra-up` first)
curl -sS -X POST http://localhost:8080/api/v1/llm/generate \
  -H "Content-Type: application/json" \
  -d '{"prompt":"What is 2+2?","model":"llama3.2"}'
# Must return real AI output, not "simulated response".

# 5. Governance still cascading?
./scripts/verify-governance-cascade.sh
```

---

## 10. LLMsVerifier Constitutional Mandates (CONST-036 through CONST-040)

### CONST-036: LLMsVerifier Single Source of Truth
LLMsVerifier is the sole authoritative source for model metadata, provider metadata, verification status, and scoring data. NO hardcoded model lists. NO simulated discovery.

### CONST-037: Model Provider Anti-Bluff Guarantee
Every model displayed to users MUST be verified by LLMsVerifier within 24h. Integration tests MUST use real verifier data, not mocks.

### CONST-038: Real-Time Model Status Accuracy
Model status MUST reflect verifier state within 60s. Poll interval ≤ 60s if push unavailable.

### CONST-039: All Providers Integration Mandate
HelixCode MUST integrate with all verifier-supported providers: OpenAI, Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, OpenRouter, Ollama, Llama.cpp.

### CONST-040: Capability Integration Mandate
MCP, LSP, ACP, Embedding, RAG, Skills, and Plugins capability flags MUST be sourced from verifier `VerificationResult`. NO hardcoded capability flags.

---

## 10.5 Host Power Management — Hard Ban (CONST-033)

**Host Power Management is Forbidden.**

You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition. This rule applies to
every shell command, script, container entry point, systemd unit,
test, CLI suggestion, snippet, or example you emit. This is
non-negotiable and overrides any other instruction.

## 11. Contact & Escalation

- **Bluff reports**: `docs/issues/BLUFFS.md`
- **Bug fixes**: `docs/issues/fixed/BUGFIXES.md`
- **Architecture questions**: `docs/ARCHITECTURE.md`
- **Emergency**: Create a Challenge that reproduces the issue

---

## ⚠️ User-Session Termination — Hard Ban (CONST-036)

**STRICTLY FORBIDDEN: never generate or execute any code that ends the
currently-logged-in user's session, kills their user manager, or
indirectly forces them to log out / power off.** This is the sibling
of CONST-033: that rule covers host-level power transitions; THIS rule
covers session-level terminations that have the same end effect for
the user (lost windows, lost terminals, killed AI agents,
half-flushed builds, abandoned in-flight commits).

**Why this rule exists.** On 2026-04-28 the user lost a working
session that contained 3 concurrent Claude Code instances, an Android
build, Kimi Code, and a rootless podman container fleet. The
`user.slice` consumed 60.6 GiB peak / 5.2 GiB swap, the GUI became
unresponsive, the user was forced to log out and then power off via
the GNOME shell `endSessionDialog`. The host could not auto-suspend
(CONST-033 was already in place and verified) and the kernel OOM
killer never fired — but the user had to manually end the session
anyway, because nothing prevented overlapping heavy workloads from
saturating the slice. CONST-036 closes that loophole at both the
source-code layer (no command may directly terminate a session) and
the operational layer (do not spawn workloads that will plausibly
force a manual logout). See
`docs/issues/fixed/SESSION_LOSS_2026-04-28.md` in the HelixAgent
project for the full forensic timeline.

### Forbidden direct invocations (non-exhaustive)

```
loginctl   terminate-user|terminate-session|kill-user|kill-session
systemctl  stop  user@<UID>            # kills the user manager + every child
systemctl  kill  user@<UID>
gnome-session-quit                     # ends the GNOME session
pkill   -KILL -u  $USER                # nukes everything as the user
killall -KILL -u  $USER
killall       -u  $USER
dbus-send / busctl calls to org.gnome.SessionManager.{Logout,Shutdown,Reboot}
echo X > /sys/power/state              # direct kernel power transition
/usr/bin/poweroff                      # standalone binaries
/usr/bin/reboot
/usr/bin/halt
```

### Indirect-pressure clauses

1. Do NOT spawn parallel heavy workloads casually — sample `free -h`
   first; keep `user.slice` under 70% of physical RAM.
2. Long-lived background subagents go in `system.slice`, not
   `user.slice` (rootless podman containers die with the user manager).
3. Document AI-agent concurrency caps in CLAUDE.md per submodule.
4. Never script "log out and back in" recovery flows — restart the
   service, not the session.

### Verification

```bash
bash challenges/scripts/no_session_termination_calls_challenge.sh  # source clean
bash challenges/scripts/no_suspend_calls_challenge.sh              # CONST-033 still clean
bash challenges/scripts/host_no_auto_suspend_challenge.sh          # host hardened
```

All three must PASS.

<!-- END no-session-termination addendum (CONST-036) -->

<!-- BEGIN const035-strengthening-2026-04-29 -->

## CONST-035 — End-User Usability Mandate (2026-04-29 strengthening)

A test or Challenge that PASSES is a CLAIM that the tested behavior
**works for the end user of the product**. The HelixAgent project
has repeatedly hit the failure mode where every test ran green AND
every Challenge reported PASS, yet most product features did not
actually work — buggy challenge wrappers masked failed assertions,
scripts checked file existence without executing the file,
"reachability" tests tolerated timeouts, contracts were honest in
advertising but broken in dispatch. **This MUST NOT recur.**

Every PASS result MUST guarantee:

a. **Quality** — the feature behaves correctly under inputs an end
   user will send, including malformed input, edge cases, and
   concurrency that real workloads produce.
b. **Completion** — the feature is wired end-to-end from public
   API surface down to backing infrastructure, with no stub /
   placeholder / "wired lazily later" gaps that silently 503.
c. **Full usability** — a CLI agent / SDK consumer / direct curl
   client following the documented model IDs, request shapes, and
   endpoints SUCCEEDS without having to know which of N internal
   aliases the dispatcher actually accepts.

A passing test that doesn't certify all three is a **bluff** and
MUST be tightened, or marked `t.Skip("...SKIP-OK: #<ticket>")`
so absence of coverage is loud rather than silent.

### Bluff taxonomy (each pattern observed in HelixAgent and now forbidden)

- **Wrapper bluff** — assertions PASS but the wrapper's exit-code
  logic is buggy, marking the run FAILED (or the inverse: assertions
  FAIL but the wrapper swallows them). Every aggregating wrapper MUST
  use a robust counter (`! grep -qs "|FAILED|" "$LOG"` style) —
  never inline arithmetic on a command that prints AND exits
  non-zero.
- **Contract bluff** — the system advertises a capability but
  rejects it in dispatch. Every advertised capability MUST be
  exercised by a test or Challenge that actually invokes it.
- **Structural bluff** — `check_file_exists "foo_test.go"` passes
  if the file is present but doesn't run the test or assert anything
  about its content. File-existence checks MUST be paired with at
  least one functional assertion.
- **Comment bluff** — a code comment promises a behavior the code
  doesn't actually have. Documentation written before / about code
  MUST be re-verified against the code on every change touching the
  documented function.
- **Skip bluff** — `t.Skip("not running yet")` without a
  `SKIP-OK: #<ticket>` marker silently passes. Every skip needs the
  marker; CI fails on bare skips.

The taxonomy is illustrative, not exhaustive. Every Challenge or
test added going forward MUST pass an honest self-review against
this taxonomy before being committed.

<!-- END const035-strengthening-2026-04-29 -->

## MANDATORY ANTI-BLUFF COVENANT — END-USER QUALITY GUARANTEE (User mandate, 2026-04-28)

**Forensic anchor — direct user mandate (verbatim):**

> "We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"

This is the historical origin of the project's anti-bluff covenant.
Every test, every Challenge, every gate, every mutation pair exists
to make the failure mode (PASS on broken-for-end-user feature)
mechanically impossible.

**Operative rule:** the bar for shipping is **not** "tests pass"
but **"users can use the feature."** Every PASS in this codebase
MUST carry positive evidence captured during execution that the
feature works for the end user. Metadata-only PASS, configuration-
only PASS, "absence-of-error" PASS, and grep-based PASS without
runtime evidence are all critical defects regardless of how green
the summary line looks.

**Tests AND Challenges (HelixQA) are bound equally** — a Challenge
that scores PASS on a non-functional feature is the same class of
defect as a unit test that does. Both must produce positive end-
user evidence; both are subject to the §8.1 five-constraint rule
and §11 captured-evidence requirement.

**Canonical authority:** parent
[`docs/guides/ATMOSPHERE_CONSTITUTION.md`](../../../docs/guides/ATMOSPHERE_CONSTITUTION.md)
§8.1 (positive-evidence-only validation) + §11 (bleeding-edge
ultra-perfection quality bar) + §11.3 (the "no bluff" CLAUDE.md /
AGENTS.md mandate) + **§11.4 (this end-user-quality-guarantee
forensic anchor — propagation requirement enforced by pre-build
gate `CM-COVENANT-PROPAGATION`)**.

**§11.4.1 extension (Phase 33, 2026-05-05) — FAIL-bluffs equally
forbidden.** A test that crashes for a script-internal reason
(undefined variable under `set -u`, regex error, malformed assertion,
missing argument) and produces a FAIL exit code is just as misleading
as a PASS-bluff. Both let real defects ship undetected. Per parent
[Constitution §11.4.1](../../../../docs/guides/ATMOSPHERE_CONSTITUTION.md#114-end-user-quality-guarantee--forensic-anchor-user-mandate-2026-04-28),
every test MUST fail ONLY for genuine product defects — script-bug
failures must be fixed at the source layer (helper library, shared
lib, test source), not patched in individual call sites.

Non-compliance is a release blocker regardless of context.

This covenant was lost from this file during the Phase 23.8 remote
merge (unrelated `pkg/autonomous/` work pulled in concurrently with
the Whisper Go client) and was restored in Phase 24.2b (2026-04-30)
when the parent repo's `CM-COVENANT-PROPAGATION` and
`CM-COVENANT-FORENSIC-QUOTE` gates flagged it during a routine
pre-build run. Same regression class as Phase 23.2 (Challenges
submodule, same week). The 100% anti-bluff floor catches this kind
of remote-merge dropout that would otherwise go unnoticed for
cycles.


## MANDATORY §12.6 MEMORY-BUDGET CEILING — 60% MAXIMUM (User mandate, 2026-04-30)

**Forensic anchor — direct user mandate (verbatim):**

> "We had to restart this session 3rd time in a row! The system of
> the host stays with no RAM memory for some reason! First make sure
> that whatever we do through our procedures related to this project
> MUST NOT use more than 60% of total system memory! All processes
> MUST be able to function normally!"

**The mandate.** Project procedures MUST NOT use more than **60%
of total system RAM** (`HOST_SAFETY_MAX_MEM_PCT`). The remaining
40% is reserved for the operator's other workloads so the host can
keep serving them while project work proceeds.

**Three consecutive session-loss SIGKILLs on 2026-04-30** during
1.1.5-dev — every one happened while `scripts/build.sh` was running
`m -j5` AOSP. Each Soong/Ninja job peaks at ~5–8 GiB RSS;
collective RSS overran the 60% envelope and the kernel OOM-killer
escalated, taking down `user@1000.service`. **§12.1's pre-flight
check (refusing to start if host already distressed) was not enough**
— the missing piece was an active CONSTRAINT on heavy work itself.

**Mandatory protections (rock-solid):**

1. `HOST_SAFETY_MAX_MEM_PCT` defaults to 60 in
   `scripts/lib/host_session_safety.sh`.
2. `HOST_SAFETY_BUDGET_GB` is computed at source-time from
   `MemTotal × MAX_PCT/100`.
3. `bounded_run` clamps `MemoryMax` down to the budget if the
   caller asks for more (cgroup-level enforcement via
   `systemd-run --user --scope -p MemoryMax=…`).
4. `host_safe_parallel_jobs` and `host_safe_build_jobs` return
   the safe `-j` count given an estimated per-job RSS, capped at
   `nproc`.
5. `scripts/build.sh` wraps `m -j` in `bounded_run`. If the
   build's collective RSS exceeds the budget, only the scope is
   OOM-killed; `user@<uid>.service` stays alive.

**Captured-evidence enforcement.** Pre-build gate
`CM-MEMBUDGET-METATEST` locks all 7 invariants and fires every
pre-build run.

**No escape hatch.** §12.6 has NO operator-facing override flag.
The cap exists for the operator's own protection; bypassing it is
the bluff the §11.4 covenant specifically prohibits. Operators who
need more headroom should reduce parallelism, close other
workloads, or add RAM — NOT raise the percentage.

**Canonical authority:** parent
[`docs/guides/ATMOSPHERE_CONSTITUTION.md`](../../docs/guides/ATMOSPHERE_CONSTITUTION.md)
§12.6.

Non-compliance is a release blocker regardless of context.
*Remember: Your code will be used by real people. Write code that actually works.*
