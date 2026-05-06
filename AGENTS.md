# AGENTS.md — HelixCode Authoritative Agent Guide

## HelixCode Agent Guidelines

**Version**: 3.0.0 (Updated with full architecture audit)
**Date**: 2026-04-30
**Scope**: All AI agents, human contributors, and automated processes working on HelixCode
**Authority**: Derived from HelixAgent AGENTS.md with HelixCode-specific enhancements

---

## Project Overview

HelixCode is an enterprise-grade distributed AI development platform built in Go. It enables intelligent task division, work preservation, cross-platform development workflows, and multi-provider LLM integration through a unified REST API, CLI, Terminal UI, Desktop, and Mobile client architecture.

**Current Status**: The `internal/` foundation is largely solid (auth, database, server, worker, task, workflow, tools, editor, notification, MCP, **verifier** are real implementations). Critical bluff and stub areas remain in select entry points and peripheral packages. All agents MUST prioritize zero-bluff implementation.

**LLMsVerifier Integration Status**: `internal/verifier/` package is now implemented with REST API client, two-tier cache, circuit breaker health monitor, background poller, score adapter, and event publisher. BLUFF-002 (hardcoded CLI models) and BLUFF-004 (hardcoded external models) are FIXED. BLUFF-005 (scoring ignores verifier data) is FIXED in `ModelManager.SelectOptimalModel()`.

**Key Features**:
- **Distributed Computing**: SSH-based worker pools with health monitoring, auto-installation, and consensus
- **Multi-Provider LLM Integration**: 15+ providers (OpenAI, Anthropic, Gemini, Ollama, Azure, Bedrock, Groq, Mistral, Cohere, xAI, DeepSeek, Qwen, OpenRouter, HuggingFace, Llama.cpp)
- **Development Workflows**: Automated planning, building, testing, refactoring with real shell execution
- **Task Management**: Intelligent task division with priorities, dependencies, checkpointing, and Redis caching
- **MCP Protocol**: Full Model Context Protocol server over WebSocket with tool dispatch
- **Multi-Client Architecture**: REST API (Gin), Cobra CLI, Terminal UI (tview), Desktop (Fyne), Mobile (gomobile), WebSocket
- **Memory Systems**: In-memory, filesystem, Redis, Memcached, Cognee, ChromaDB, Qdrant, Weaviate integrations
- **Advanced Editor**: Multi-format code editing (diff, whole-file, search/replace, line-based) with backups
- **Tools Ecosystem**: 40+ tools across filesystem, shell, web, browser, mapping, multiedit, confirmation, notebook, git
- **Notifications**: Multi-channel support (Slack, Email, Telegram, Discord, Yandex Messenger, Max)

---

## Technology Stack

**Core Technologies**:
- **Language**: Go 1.24.0 with toolchain go1.24.9
- **Module**: `dev.helix.code`
- **HTTP Framework**: Gin v1.11.0
- **Authentication**: JWT v4.5.2, bcrypt + argon2
- **Database**: PostgreSQL 15+ via pgx/v5 (optional)
- **Cache**: Redis 7+ via go-redis/v9 (optional)
- **Configuration**: Viper v1.21.0
- **CLI Framework**: Cobra v1.8.0
- **Testing**: Testify v1.11.1

**UI Technologies**:
- **Desktop**: Fyne v2.7.0
- **Terminal UI**: tview v0.42.0
- **Mobile**: gomobile bindings

**External Integrations**:
- **Browser Automation**: chromedp v0.14.2
- **Web Scraping**: goquery v1.10.3
- **Tree-sitter**: go-tree-sitter
- **Identity**: Azure SDK, AWS SDK v2
- **Vector/Memory**: Cognee, ChromaDB, Qdrant, Weaviate clients
- **Container Orchestration**: digital.vasic.containers (vasic-digital/Containers submodule)

---

## Working Directory & Build System

**CRITICAL**: All build and test commands must be run from the `HelixCode/` subdirectory, not the repository root.

```bash
cd HelixCode
```

### Build Commands
| Command | Purpose |
|---------|---------|
| `make build` | Build server binary to `bin/helixcode` |
| `make test` | Run `go test -v ./...` |
| `make test-all` | Run tests + coverage + benchmarks + docs |
| `make test-coverage` | Generate coverage report |
| `make test-benchmark` | Run Go benchmarks |
| `make logo-assets` | Generate logo assets (required before first build) |
| `make setup-deps` | Run `go mod tidy` |
| `make fmt` | Run `go fmt ./...` |
| `make lint` | Run `golangci-lint run ./...` |
| `make clean` | Clean build artifacts |
| `make dev` | Start development server |
| `make prod` | Cross-platform production build |
| `make mobile` | Build iOS + Android targets |
| `make aurora-os` | Build Aurora OS target |
| `make harmony-os` | Build Harmony OS target |

### Full Infrastructure Test Commands
| Command | Purpose |
|---------|---------|
| `make test-infra-up` | Start full Docker test infrastructure |
| `make test-infra-down` | Stop full Docker test infrastructure |
| `make test-full` | ALL tests with real infrastructure (zero skips) |
| `make test-unit-full` | Unit tests with real services |
| `make test-integration-full` | Integration tests with `-tags=integration` |
| `make test-e2e-full` | E2E challenge tests via runner |
| `make test-security-full` | Security test suite |
| `make test-load-full` | Load tests |
| `make test-complete` | Sequential run of all full test types |
| `make coverage-full` | Coverage with full infrastructure |

### Containerized Builds (NO Host Dependencies)
| Command | Purpose |
|---------|---------|
| `make container-builder-image` | Build the builder container image |
| `make container-build` | Build application inside container |
| `make container-test` | Run tests inside container |
| `make container-lint` | Run linter inside container |
| `make container-shell` | Interactive shell in builder container |
| `make container-dev-up` | Start containerized dev environment |
| `make container-dev-down` | Stop containerized dev environment |
| `make container-release` | Full release build in container |
| `./scripts/containers/build-in-container.sh` | Convenience wrapper script |

The builder container includes: Go 1.24, gcc, postgresql-client, redis, docker-cli, golangci-lint, and all build tools. The only host requirement is Docker/Podman.

### Standalone Test Scripts
| Script | Purpose |
|--------|---------|
| `./run_tests.sh --unit` | Unit tests |
| `./run_tests.sh --integration` | Integration tests |
| `./run_tests.sh --e2e` | E2E tests |
| `./run_tests.sh --coverage` | Coverage analysis |
| `./run_tests.sh --security` | Security tests |
| `./run_all_tests.sh` | Orchestrates ALL suites sequentially |
| `./run_integration_tests.sh` | DB integration tests with Docker |

### Single Test Execution
```bash
go test -v -run TestName ./path/to/package
go test -v -tags=integration ./internal/database
cd tests/e2e/challenges && go run cmd/runner/main.go -challenge ascii-art-generator-001 -providers ollama
```

---

## Architecture & Code Organization

```
HelixCode/
├── cmd/                          # Application entry points
│   ├── server/main.go            # HTTP server entry point
│   ├── cli/main.go               # Legacy flag-based CLI client
│   ├── root.go                   # Cobra root command (`helix`)
│   ├── main_commands.go          # `helix start`, `helix auto`
│   ├── other_commands.go         # `helix server`, `helix version`, etc.
│   ├── local-llm.go              # `helix local-llm` command tree
│   ├── local-llm-advanced.go     # Advanced local-llm commands
│   ├── helix-config/main.go      # Dedicated config management CLI
│   ├── security-test/main.go     # Simulated security test runner
│   ├── security-fix/main.go      # Security fix wrapper
│   ├── security-fix-standalone/main.go  # Standalone security scanner
│   ├── performance-optimization/main.go # Performance optimizer
│   ├── performance-optimization-standalone/main.go # Standalone perf simulator
│   └── config-test/main.go       # Config hot-reload test utility
│
├── internal/                     # Internal packages (~40 packages)
│   ├── auth/                     # JWT authentication, bcrypt/argon2, sessions
│   ├── llm/                      # LLM provider implementations (15+ providers)
│   │   ├── providers/            # Per-provider HTTP clients
│   │   ├── compression/          # Context compression
│   │   └── vision/               # Vision/multimodal support
│   ├── provider/                 # Provider abstractions
│   ├── providers/                # Provider management
│   ├── worker/                   # SSH-based worker pool, health checks
│   ├── task/                     # Task queues, dependencies, checkpoints
│   ├── server/                   # Gin HTTP server, routes, middleware
│   ├── database/                 # PostgreSQL pgx pool, schema initialization
│   ├── redis/                    # go-redis wrapper with graceful degradation
│   ├── tools/                    # 40+ tool ecosystem registry
│   │   ├── filesystem/           # fs_read, fs_write, fs_edit, glob, grep
│   │   ├── shell/                # shell, shell_background with sandbox
│   │   ├── web/                  # web_fetch, web_search
│   │   ├── browser/              # browser_launch, browser_navigate, browser_screenshot
│   │   ├── multiedit/            # Transactional multi-file editing
│   │   └── git/                  # Git automation
│   ├── editor/                   # Multi-format code editing with backups
│   ├── memory/                   # Memory providers (in-mem, filesystem, Redis, etc.)
│   ├── cognee/                   # Cognee.ai memory integration
│   ├── context/                  # Hierarchical context management with TTL
│   ├── notification/             # Multi-channel notification engine
│   ├── mcp/                      # Model Context Protocol WebSocket server
│   ├── workflow/                 # Development workflow execution
│   ├── config/                   # Viper-based configuration management
│   ├── event/                    # Pub/sub event bus
│   ├── logging/                  # Structured logging wrapper
│   ├── monitoring/               # Metric collection framework
│   ├── security/                 # Security scanning (stubbed)
│   ├── session/                  # Development session management
│   ├── agent/                    # Agent orchestration
│   ├── project/                  # Project management
│   ├── rules/                    # Rules engine
│   ├── hooks/                    # Hook system
│   ├── focus/                    # Focus chain management
│   ├── template/                 # Template system
│   ├── persistence/              # State persistence
│   ├── deployment/               # Deployment management
│   ├── discovery/                # Service/model discovery
│   ├── hardware/                 # Hardware abstraction
│   ├── repomap/                  # Repository mapping
│   ├── version/                  # Version management
│   ├── fix/                      # Security fix engine
│   ├── performance/              # Performance optimization
│   ├── testutil/                 # Test utilities
│   └── mocks/                    # Shared mocks
│
├── applications/                 # Platform-specific applications
│   ├── desktop/                  # Fyne desktop app
│   ├── terminal-ui/              # tview terminal UI
│   ├── android/                  # Android app
│   ├── ios/                      # iOS app
│   ├── aurora-os/                # Aurora OS client
│   └── harmony-os/               # Harmony OS client
│
├── api/                          # OpenAPI specification
│   └── openapi.yaml              # Full REST API spec (OpenAPI 3.0.3)
│
├── config/                       # Configuration files
│   ├── config.yaml               # Primary application config
│   ├── production-config.yaml    # Enterprise production config
│   ├── minimal-config.yaml       # Minimal test config (DB/Redis disabled)
│   ├── test-config.yaml          # Test-specific config
│   ├── working-config.yaml       # Working variant
│   ├── azure_example.yaml        # Azure-specific example
│   └── model-aliases.example.yaml# Model alias examples
│
├── tests/                        # New test framework
│   ├── e2e/challenges/           # Challenge-based E2E tests
│   └── automation/               # Hardware automation tests
│
├── test/                         # Legacy/parallel test suites
│   ├── integration/              # Integration tests
│   ├── e2e/                      # Legacy E2E tests
│   ├── automation/               # Provider automation tests
│   └── load/                     # Load tests
│
├── benchmarks/                   # Performance benchmarks
├── security/                     # Security tests
├── standalone_tests/             # Standalone CLI tests
├── docker/                       # Docker assets and extended compose
├── scripts/                      # Build and deployment scripts
└── assets/                       # Logo and image assets
```

---

## Verified Real Implementations

### AUTH-001: Authentication System (VERIFIED REAL)
**File**: `internal/auth/auth.go` (~470 lines)
**Assessment**: Production-ready
- User registration with validation
- Password hashing with bcrypt + argon2 fallback
- JWT token generation and verification (JWT v4)
- Session management with crypto-random tokens
- Constant-time comparison for timing attack prevention
- Full test coverage in `internal/auth/auth_test.go` (~777 lines)

### DB-001: Database Layer (VERIFIED REAL)
**File**: `internal/database/database.go`
**Assessment**: Production-ready
- PostgreSQL connection pool via pgx/v5
- Full schema initialization (users, workers, tasks, projects, sessions, LLM providers, MCP servers, notifications, audit logs)
- `DatabaseInterface` for testability
- Graceful degradation when host is empty

### SRV-001: HTTP Server (VERIFIED REAL)
**File**: `internal/server/server.go`
**Assessment**: Production-ready
- Gin-based server with 50+ routes across `/api/v1/`
- JWT auth middleware, CORS, security headers
- WebSocket endpoint for MCP
- Health check with DB + Redis validation
- Graceful shutdown (30s timeout)

### LLM-001: LLM Providers (VERIFIED REAL)
**File**: `internal/llm/` (~5000+ lines across providers)
**Assessment**: Real HTTP clients
- `AnthropicProvider` (~752 lines): Full SSE streaming, prompt caching, extended thinking, tool calls
- `OpenAIProvider` (~431+ lines): Full HTTP API client
- `ModelManager`: Multi-provider orchestration, selection strategy, fallback chain
- 16 provider subdirectories with real HTTP implementations
- **Note**: The `internal/llm/` package is genuine. Bluff areas are at `cmd/cli/main.go` only.

### WRK-001: Worker Pool (VERIFIED REAL)
**File**: `internal/worker/` (~800+ lines)
**Assessment**: Real distributed worker management
- `WorkerManager`: Register, heartbeat, assign tasks, complete tasks
- SSH config parsing, capability matching, resource tracking
- Health checks with TTL

### TSK-001: Task Management (VERIFIED REAL)
**File**: `internal/task/` (~1000+ lines)
**Assessment**: Real task lifecycle
- Priority queues, dependency validation, checkpointing
- Redis caching with graceful degradation
- Retry logic and cleanup

### WFL-001: Workflow Engine (VERIFIED REAL)
**File**: `internal/workflow/` (~1100+ lines)
**Assessment**: Real shell execution
- `Executor` dispatches to real `exec.CommandContext()` calls
- Security filtering via `isDangerousCommand()` (rm, dd, mkfs, fork bombs, etc.)
- LLM integration with real `LLMRequest`
- Supports Go, Node, Python, Rust project types

### TOO-001: Tools Ecosystem (VERIFIED REAL)
**File**: `internal/tools/` (~2000+ lines)
**Assessment**: Real tool registry
- 8 categories: filesystem, shell, web, browser, mapping, multiedit, confirmation, notebook
- Real chromedp browser automation
- Transactional multi-file editing

### EDT-001: Code Editor (VERIFIED REAL)
**File**: `internal/editor/` (~600+ lines)
**Assessment**: Real file I/O
- Diff, whole-file, search/replace, line-based editors
- Automatic file backup with `io.Copy`
- `EditApplier` / `EditValidator` interfaces

### NOT-001: Notification Engine (VERIFIED REAL)
**File**: `internal/notification/` (~800+ lines)
**Assessment**: Real HTTP/SMTP calls
- Slack (webhook HTTP POST), Email (SMTP via `net/smtp`), Telegram (Bot API), Discord (webhook)
- Yandex Messenger (OAuth API), Max (enterprise API)
- Rate limiting, retry, queue, metrics

### MCP-001: MCP Protocol Server (VERIFIED REAL)
**File**: `internal/mcp/` (~400+ lines)
**Assessment**: Real WebSocket server
- gorilla/websocket concurrent session handling
- JSON-RPC-like message format
- Tool execution dispatch

### CFG-001: Configuration Management (VERIFIED REAL)
**File**: `internal/config/` (~1700+ lines)
**Assessment**: Full Viper integration
- Environment variable binding (`HELIX_*`)
- Config file search (`.`, `$HOME/.helixcode`, `/etc/helixcode`)
- Validation rules, default config creation
- `ConfigManager` for load/save/merge

### QA-001: HelixQA Integration (VERIFIED REAL)
**Files**: `internal/helixqa/`, `internal/server/qa_handlers.go`, `applications/terminal-ui/main.go`
**Assessment**: Full embedded QA engine with real session lifecycle
- `Engine` struct manages QA sessions with map + sync.RWMutex
- `StartSession()`, `CancelSession()`, `GetSession()`, `ListSessions()` with real state tracking
- REST API: `POST /api/v1/qa/session`, `GET /api/v1/qa/session/:id/status`, `GET /api/v1/qa/session/:id/report`, `GET /api/v1/qa/session/:id/screenshot/:name`, `DELETE /api/v1/qa/session/:id`
- CLI flags: `--qa-run`, `--qa-list`, `--qa-report`, `--qa-screenshot`, `--qa-cancel`
- TUI dashboard with session table, stats panel, refresh/cancel actions
- Screenshot pipeline: 8 platform engines (Linux, Web, iOS, Android, CLI, TUI, macOS, Windows)
- Tests: `internal/helixqa/wrapper_test.go`, `internal/server/qa_handlers_test.go`, `pkg/screenshot/*_test.go`

---

## Verified Bluff & Stub Areas (MUST FIX)

### BLUFF-001: LLM Generation is Simulated in Legacy CLI (CRITICAL) — FIXED
**File**: `cmd/cli/main.go` lines ~236-284
**Evidence**: Previously returned `fmt.Sprintf("Generated response for: %s...", prompt)` without calling any provider.
**Fix**: `handleGenerate()` now constructs a real `llm.LLMRequest` with user messages and calls `provider.Generate()` / `provider.GenerateStream()`. Errors are propagated to the user if the provider is unavailable.
**Verification**: `go build -tags nogui ./cmd/cli/` compiles; provider call is real (returns error if Ollama/etc. is not running).
**Fix Priority**: P0 — RESOLVED

### BLUFF-002: Model Listing is Hardcoded in Legacy CLI (CRITICAL) — FIXED
**File**: `cmd/cli/main.go` lines ~101-128
**Evidence**: Previously only 3 hardcoded models. No dynamic discovery.
**Fix**: Replaced with verifier-aware `handleListModels()` that queries LLMsVerifier adapter first, falls back to provider discovery, then to constitutional `FallbackModels` (7 models with scores and verification status).
**Verification**: `go test -v ./internal/verifier/...` passes; `go build ./cmd/cli/...` compiles.
**Fix Priority**: P0 — RESOLVED

### BLUFF-003: Command Execution is Simulated in Legacy CLI (HIGH) — FIXED
**File**: `cmd/cli/main.go` lines ~310-324
**Evidence**: Previously printed the command and slept for 1 second without executing anything.
**Fix**: `handleCommand()` uses `exec.CommandContext(ctx, "sh", "-c", command)` with real `os.Stdout`/`os.Stderr` redirection. Exit codes are reported.
**Verification**: `go build -tags nogui ./cmd/cli/` compiles.
**Fix Priority**: P0 — RESOLVED

### STUB-001: Security Scanning is Simulated
**File**: `internal/security/security.go` (~132 lines)
**Evidence**: `ScanFeature()` contains explicit "Simulate security scanning logic" comment. Always returns `Success=true, Score=95` with empty issues.
**Fix Priority**: P1

### STUB-002: Memory Redis/Memcached Providers Store Locally
**File**: `internal/memory/` (~1800+ lines)
**Evidence**: `RedisMemoryProvider` and `MemcachedMemoryProvider` store data in local maps with comments like "Redis client would be used in production." Connection config is parsed but not used.
**Fix Priority**: P2

### STUB-003: Security-Test Entry Point is Entirely Simulated
**File**: `cmd/security-test/main.go`
**Evidence**: Hardcoded list of 12 simulated security tests. `simulateSecurityScan()` returns pre-canned issue lists per category.
**Fix Priority**: P2

### STUB-004: Several `helix` Subcommands are Print-Only
**File**: `cmd/other_commands.go`
**Evidence**: `server`, `generate`, `test`, `worker`, `notify` commands are stubbed (print placeholder messages).
**Fix Priority**: P2

### STUB-005: Several `helix-config` Subcommands are Placeholders
**File**: `cmd/helix-config/main.go`
**Evidence**: Many template/history/schema subcommands print placeholder messages.
**Fix Priority**: P3

### BLUFF-004: LLMsVerifier Integration is Stubbed or Bypassed (CRITICAL)
**File Pattern**: `internal/verifier/*.go` containing empty structs, `// TODO`, or methods that return hardcoded data instead of calling the verifier.
**Evidence**:
- `VerificationService` methods return hardcoded `VerificationResult{OverallScore: 8.5}` instead of querying the verifier database
- `ModelDiscoveryService` returns an empty slice instead of calling provider APIs
- The verifier client returns fallback models without attempting a real HTTP call
**Fix Priority**: P0 - Immediate
**Verification Command**:
```bash
make test-verifier-integration
# This MUST pass with real verifier data, not mocked scores
```

### BLUFF-005: Provider Discovery Uses Hardcoded Env Var Names (HIGH)
**File Pattern**: `internal/verifier/startup.go` or provider adapter files containing hardcoded strings like `"OPENAI_API_KEY"` without checking `SupportedProviders[provider].EnvVars`.
**Fix Priority**: P1 - High

### BLUFF-006: Model Capabilities Are Hardcoded (HIGH)
**File Pattern**: `internal/llm/*.go` containing `SupportsToolUse: true` as a struct literal for specific models, or `Provider.GetCapabilities()` returning a static slice.
**Fix Priority**: P1 - High
**Constitutional Impact**: Violates CONST-041 (MCP/LSP/ACP/Embedding/RAG/Skills/Plugins Integration Mandate).

### BLUFF-007: Test Claims Integration But Uses Mocked Verifier (CRITICAL)
**File Pattern**: `*_test.go` files with `testify/mock` or `testMode: true` in non-unit test files.
**Fix Priority**: P0 - Immediate
**Constitutional Impact**: Violates CONST-038 (Model Provider Anti-Bluff Guarantee) and CONST-035 (Zero-Bluff Testing).

### BLUFF-008: Scoring Weights Do Not Sum to 1.0 (MEDIUM)
**File Pattern**: `configs/verifier.yaml` or `internal/verifier/config.go` where scoring weights are misconfigured.
**Fix Priority**: P2 - Medium

### BLUFF-009: `/metrics` Endpoint Returns Hardcoded Zeros (CRITICAL) — FIXED
**File**: `internal/server/handlers.go` lines ~834-855
**Evidence**: All dynamic metrics (goroutines, memory, database connections) were hardcoded to `0`.
**Fix**: `getMetrics()` now calls `runtime.ReadMemStats()`, `runtime.NumGoroutine()`, and `s.db.Pool.Stat()` to return real values.
**Fix Priority**: P0 — RESOLVED

### BLUFF-010: Multi-Edit Conflict Detection is a No-Op (HIGH) — FIXED
**File**: `internal/tools/multiedit/transaction.go` lines ~352-369
**Evidence**: `detectFileConflict()` always returned `nil, nil` with comment "For now, we'll assume no conflicts."
**Fix**: Implemented real conflict detection — reads the file from disk, computes SHA-256, and compares against the `Checksum` field. Returns `ConflictModified` or `ConflictDeleted` when appropriate.
**Fix Priority**: P1 — RESOLVED

---

## Configuration Management

### Primary Configuration
Main config at `config/config.yaml`:

```yaml
server:
  address: "0.0.0.0"
  port: 8080
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 300
  shutdown_timeout: 30

database:
  host: ""          # Empty string disables PostgreSQL
  port: 5432
  user: "helix"
  password: "${HELIX_DATABASE_PASSWORD}"
  dbname: "helixcode_prod"
  sslmode: "disable"

redis:
  host: "redis"
  port: 6379
  password: "${HELIX_REDIS_PASSWORD}"
  db: 0
  enabled: true

auth:
  jwt_secret: "${HELIX_AUTH_JWT_SECRET}"
  token_expiry: 86400
  session_expiry: 604800
  bcrypt_cost: 12

workers:
  health_check_interval: 30
  health_ttl: 120
  max_concurrent_tasks: 10

tasks:
  max_retries: 3
  checkpoint_interval: 300
  cleanup_interval: 3600

llm:
  default_provider: "local"
  max_tokens: 4096
  temperature: 0.7
  timeout: 30
  max_retries: 3
  providers:
    <name>:
      type: <provider-type>
      endpoint: <url>
      enabled: true
      parameters:
        timeout: 30.0
        max_retries: 3
        streaming_support: true
        api_key: ""
  selection:
    strategy: "performance"
    fallback_enabled: true
    health_check_interval: 30

logging:
  level: "info"
  format: "text"
  output: "stdout"

notifications:
  enabled: true
  rules:
    - name: "..."
      condition: "type==error"
      channels: ["slack", "email"]
      priority: urgent
      enabled: true
  channels:
    slack: { enabled, webhook_url, channel, username, timeout }
    telegram: { enabled, bot_token, chat_id, timeout }
    email: { enabled, smtp: { server, port, username, password, tls }, recipients, timeout }
    discord: { enabled, webhook_url, timeout }
```

### Environment Variables
**Required for Production**:
- `HELIX_DATABASE_PASSWORD`
- `HELIX_AUTH_JWT_SECRET`
- `HELIX_REDIS_PASSWORD`

**LLM Provider Keys** (as needed):
- `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`, `XAI_API_KEY`, `DEEPSEEK_API_KEY`, `GROQ_API_KEY`, `MISTRAL_API_KEY`, `COHERE_API_KEY`, `AZURE_OPENAI_API_KEY`, `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`

**Notification Integrations**:
- `HELIX_SLACK_WEBHOOK_URL`
- `HELIX_TELEGRAM_BOT_TOKEN`, `HELIX_TELEGRAM_CHAT_ID`
- `HELIX_EMAIL_SMTP_SERVER`, `HELIX_EMAIL_USERNAME`, `HELIX_EMAIL_PASSWORD`
- `HELIX_DISCORD_WEBHOOK_URL`

---

## Testing Strategy

### Test Categories
1. **Unit tests**: Mocks allowed, `*_test.go`, `-short` flag
2. **Contract tests**: Real API schemas, no mocks
3. **Component tests**: Real subsystems wired together
4. **Integration tests**: Full app with real dependencies (`-tags=integration`)
5. **E2E challenges**: Complete user workflows against real LLM APIs
6. **Security tests**: OWASP compliance
7. **Performance tests**: Benchmarks
8. **Automation tests**: Provider/hardware automation (`-tags=automation`)
9. **Load tests**: Stress testing

### Anti-Bluff Testing Rules
- Unit tests: Mocks OK
- **ALL other tests: Real infrastructure ONLY**
- Every PASS guarantees **Quality + Completion + Usability**
- Challenges fail on simulated/stubbed behavior
- No bare `t.Skip()` without `SKIP-OK: #<ticket>` marker

### Docker Test Infrastructure
- `docker-compose.test.yml`: PostgreSQL 16, Redis 7, Memcached, Cognee, ChromaDB, Qdrant, Ollama, Prometheus, Grafana
- `docker-compose.full-test.yml`: Complete stack with mock-LLM server, Selenium, ChromeDP, SSH server + 3 workers, Cognee, Weaviate, mock-Slack, multicast router

### Challenge Framework (`tests/e2e/challenges/`)
The most rigorous test system validates HelixCode by having it **generate real projects** and testing them:
- **Challenge Definitions**: JSON specs (ASCII art generator, CLI task manager, JSON validator, notes API, tic-tac-toe TUI, URL shortener)
- **Execution Flow**: Load spec → Call real LLM API → Parse generated code → Compile → Test → Runtime validation
- **Validation Layers**: Directory structure, code quality, compilation, testing, functionality, runtime validation with diverse data
- **Test Matrix**: Supports CLI, TUI, REST, WebSocket interfaces across 15+ providers and worker pool distributions

### Test Scripts Summary
```bash
# Basic
cd HelixCode && make test

# Full infrastructure (recommended for validation)
make test-infra-up
make test-complete
make test-infra-down

# Individual categories
make test-unit-full
make test-integration-full
make test-e2e-full
make test-security-full
make test-load-full

# Legacy scripts
./run_tests.sh --all
./run_all_tests.sh
./run_integration_tests.sh
```

---

## Docker Deployment

### Production (`docker-compose.yml`)
Services: helixcode-server (8080, 2222), postgres:15, redis:7, nginx (80, 443), prometheus (9090), grafana (3000)

### Quick Start
```bash
cd HelixCode
cp .env.example .env
# Edit .env with secure passwords
docker compose up -d
docker compose ps
curl http://localhost/health
```

### Other Compose Files
| File | Purpose |
|------|---------|
| `docker-compose-simple.yml` | Minimal dev (postgres + redis only) |
| `docker-compose.test.yml` | Integration/E2E testing stack |
| `docker-compose.full-test.yml` | Zero-skip full test infrastructure |
| `docker-compose.aurora-os.yml` | Security-focused Aurora OS platform |
| `docker-compose.harmony-os.yml` | Distributed Harmony OS platform |
| `docker-compose.specialized-platforms.yml` | Combined Aurora + Harmony |
| `docker/docker-compose.yml` | Extended full-stack with Milvus, Elasticsearch, MLflow, Jaeger, Jupyter, Portainer |

### Deployment Patterns
- Healthchecks on every service
- Docker profiles: `monitoring`, `distributed`, `with-redis`, `production`, `dev`, `server`
- Isolated bridge networks per deployment
- Named persistent volumes for all stateful services
- `.env` file for secrets

---

## Code Style & Development Conventions

### Go Conventions
- Standard Go formatting: `go fmt ./...`
- Linting: `golangci-lint run ./...` (timeout 10m in CI)
- Vet: `go vet ./...`
- Table-driven tests with `t.Run()` subtests
- Build tags for integration/automation tests: `//go:build integration`

### Project Conventions
- **Always work from `HelixCode/` subdirectory**
- **Generate logo assets before first build**: `make logo-assets`
- **Database/Redis optional**: Disable by setting `database.host: ""`
- **Environment variables override config file**
- Use `internal/` for all core packages; no `pkg/` directory in active use
- Error handling: explicit, no silent failures
- Concurrent access: use `sync.RWMutex` or channel patterns

### API Conventions
- REST API documented in `api/openapi.yaml` (OpenAPI 3.0.3)
- Base path: `/api/v1`
- Authentication: Bearer JWT via `Authorization` header
- Health endpoint: `GET /health` (no auth required)

---

## Security Considerations

### Verified Security Features
- Password hashing: bcrypt (cost 12) with argon2 fallback
- JWT with constant-time comparison
- CORS middleware, security headers (X-Frame-Options, CSP, HSTS)
- Rate limiting support in production config
- Session timeout, concurrent session limits, IP binding options
- Workflow `isDangerousCommand()` filter blocks rm, dd, mkfs, fork bombs, etc.
- Input validation in auth and server packages

### Security Testing
- `security/security_test.go`: OWASP Top 10, SAST, DAST, credential scanning, TLS enforcement, input validation (path traversal, XSS, SQL injection, command injection, SSRF)
- File permission checks (0600 for configs)

### Known Security Stubs
- `internal/security/security.go`: Simulated scanning (always returns clean)
- `cmd/security-test/main.go`: Entirely simulated security tests

### Production Hardening
- Use `HELIX_AUTH_JWT_SECRET` with high entropy
- Enable PostgreSQL SSL in production
- Enable Redis authentication
- Configure CORS `allowed_origins` explicitly
- Enable audit logging
- Set `bcrypt_cost: 14` in production

---

## Universal Mandatory Constraints

### Hard Stops (permanent, non-negotiable)
1. **NO CI/CD pipelines** (Note: existing workflow files in `.github/workflows/` are legacy and must not be expanded)
2. **NO HTTPS for Git** (SSH only)
3. **NO manual container commands** (orchestrator-owned)

### Mandatory Development Standards
1. **100% Test Coverage** (unit, integration, E2E, automation, security, benchmark)
2. **Challenge Coverage** (every component)
3. **Real Data** (actual API calls, real DB, live services)
4. **Health & Observability** (health endpoints, circuit breakers)
5. **Documentation & Quality** (update docs with code changes)
6. **Validation Before Release** (full suite + all challenges)
7. **No Mocks in Production**
8. **Comprehensive Verification** (runtime, compile, structure, dependencies, compatibility)
9. **Resource Limits** (30-40% of host resources max)
10. **Bugfix Documentation** (root cause, affected files, fix, verification link)
11. **Real Infrastructure for All Non-Unit Tests**
12. **Reproduction-Before-Fix** (Challenge first, then fix)
13. **Concurrent-Safe Collections**

### Definition of Done
A change is NOT done because code compiles. "Done" requires:
- Pasted terminal output from a real run
- No self-certification words without evidence
- Demo commands that run against real artifacts
- Loud skips with `SKIP-OK: #<ticket>` markers

---

## CONST-035 — End-User Usability Mandate

A test or Challenge that PASSES is a CLAIM that the tested behavior **works for the end user of the product**.

The HelixAgent project has repeatedly hit the failure mode where every test ran green AND every Challenge reported PASS, yet most product features did not actually work — buggy challenge wrappers masked failed assertions, scripts checked file existence without executing the file, "reachability" tests tolerated timeouts, contracts were honest in advertising but broken in dispatch. **This MUST NOT recur in HelixCode.**

Every PASS result MUST guarantee:
a. **Quality** — correct behavior under real inputs, edge cases, concurrency
b. **Completion** — wired end-to-end with no stub/placeholder gaps
c. **Full usability** — a user following documentation succeeds

A passing test that doesn't certify all three is a **bluff** and MUST be tightened.

### Bluff Taxonomy (each pattern observed and now forbidden)

- **Wrapper bluff** — assertions PASS but wrapper's exit-code logic is buggy
- **Contract bluff** — system advertises capability but rejects it in dispatch
- **Structural bluff** — file exists but doesn't contain working code
- **Comment bluff** — comment promises behavior code doesn't have
- **Skip bluff** — `t.Skip("not running yet")` without `SKIP-OK: #<ticket>` marker

The taxonomy is illustrative, not exhaustive. Every Challenge or test added going forward MUST pass an honest self-review against this taxonomy before being committed.

## Constitutional anchors (cascaded from `CONSTITUTION.md`)

### Article XI §11.9 — Anti-Bluff Forensic Anchor
> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*
>
> Operative rule: **The bar for shipping is not "tests pass" but "users can use the feature."** Every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks. No false-success results are tolerable.

### Article XII §12.1 (CONST-042) — No-Secret-Leak
No API key, token, password, certificate, or other credential may be committed to any repository owned by HelixDevelopment or vasic-digital. All secrets live in `.env` files (mode 0600) listed in `.gitignore`. Any leak is a release blocker until rotated and post-mortemed.

### Article XII §12.2 (CONST-043) — No-Force-Push
No force push, force-with-lease push, history rewrite, branch deletion of `main`/`master`, or upstream-overwriting operation may be performed without explicit, in-conversation user approval per operation. Authorization for one push does not extend further. Bypassing hooks / signing / protected-branch rules also requires explicit approval.

### Article XIII §13.1 (CONST-044) — Continuation Document Maintenance Mandate
The meta-repo's `docs/CONTINUATION.md` MUST be maintained in sync with actual programme state at all times. It is the authoritative resumption record for any CLI agent or LLM picking up the CLI-Agent Fusion programme. Every commit that advances state — task completion, feature close-out, push, known-issue discovery, deferred-item resolution, phase transition, submodule/remote add or remove — MUST update CONTINUATION in the same commit. Out-of-sync CONTINUATION is a **CRITICAL DEFECT** — same severity as a false-success test result under CONST-035 / Article XI §11.9. Cascade applies to every owned-by-us repo's three governance files. See `CONSTITUTION.md` Article XIII §13.1 for the full mandate.

---

## CONST-036: LLMsVerifier Single Source of Truth Mandate

**Rule**: LLMsVerifier SHALL BE the sole authoritative source for:
1. All model metadata (names, IDs, context windows, capabilities)
2. All provider metadata (endpoints, auth types, supported models)
3. All verification status (verified, partial, failed, pending)
4. All scoring data (overall scores, capability scores, tier rankings)

**Prohibition**: NO hardcoded model lists, NO hardcoded provider lists, NO simulated model discovery. Any code path that presents a model or provider listing to a user MUST fetch that listing from the LLMsVerifier subsystem or its cached replica.

**Anti-Bluff Verification**:
- Challenge script `challenges/scripts/verifier_hardcode_check.sh` scans all Go source files for hardcoded model arrays.
- The only permitted hardcoded data is the 7-entry fallback list in `internal/verifier/fallback_models.go`.

---

## CONST-037: Model Provider Anti-Bluff Guarantee

**Rule**: Every model displayed to an end user MUST have been verified by LLMsVerifier within the last 24h. Models older than this MUST display a "stale" indicator and be deprioritized.

**Anti-Bluff Testing**:
- Unit tests MAY mock the verifier client.
- Integration tests MUST start the verifier server and perform real provider discovery.
- The Makefile target `make test-verifier-integration` MUST exist and run without mocks.

---

## CONST-038: Real-Time Model Status Accuracy

**Rule**: Model status (available, rate-limited, cooldown, offline, deprecated) displayed to users MUST reflect the actual state as known by LLMsVerifier within 60 seconds.

**Polling vs. Push**:
- If WebSocket/SSE push is unavailable, the system MUST poll LLMsVerifier at most every 60s.
- The TUI MUST display a "last updated" timestamp with every model listing.
- Models in "cooldown" or "rate-limited" state MUST show the estimated recovery time if known.

---

## CONST-039: All Providers and Models Integration Mandate

**Rule**: HelixCode MUST integrate with ALL providers that LLMsVerifier supports, subject only to:
1. The provider being explicitly disabled in configuration (`enabled: false`)
2. The API key being absent and the provider requiring one
3. The provider being marked `deprecated` in the verifier database

**Minimum Provider Set** (SHALL NOT be reduced without constitutional amendment):
OpenAI, Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, OpenRouter, Ollama, Llama.cpp.

---

## CONST-040: MCP / LSP / ACP / Embedding / RAG / Skills / Plugins Integration Mandate

**Rule**: LLMsVerifier integration SHALL extend beyond basic model listing to cover ALL capability dimensions:

1. **MCP**: The verifier MUST report which models support MCP tool calling.
2. **LSP**: The verifier MUST report code-analysis capabilities.
3. **ACP**: The verifier MUST report multi-agent coordination support.
4. **Embedding**: The verifier MUST report `supports_embeddings` for each model.
5. **RAG**: The verifier MUST report context-window sizes for chunking strategies.
6. **Skills / Plugins**: The verifier MUST track plugin compatibility.

**Prohibition**: Capability flags MUST NOT be hardcoded. The `Provider.GetCapabilities()` method MUST return data sourced from the verifier's `VerificationResult` fields.

---

## Free AI Providers

- **XAI (Grok)**: `grok-3-fast-beta`, `grok-3-mini-fast-beta`
- **OpenRouter**: Free models from various providers
- **GitHub Copilot**: `gpt-4o`, `claude-3.5-sonnet` (with subscription)
- **Qwen**: 2,000 requests/day free tier

---

## Host Power Management — Hard Ban (CONST-033)

**Host Power Management is Forbidden.**

You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition. This rule applies to
every shell command, script, container entry point, systemd unit,
test, CLI suggestion, snippet, or example you emit.

## Common Issues

1. **Build fails**: Run `make logo-assets` then `make build`
2. **Database errors**: Check `HELIX_DATABASE_PASSWORD`
3. **Worker SSH failures**: Verify SSH key authentication
4. **LLM timeouts**: Check provider status and config
5. **Redis connection failures**: Check `HELIX_REDIS_PASSWORD` and `redis.enabled`
6. **Test skips**: Ensure `SKIP-OK: #<ticket>` marker is present for any intentional skips

---

## Resources & References

- **Constitution**: `CONSTITUTION.md`
- **CLAUDE.md**: `CLAUDE.md`
- **Gap Analysis**: `HELIXCODE_GAP_ANALYSIS.md`
- **Zero-Bluff Plan**: `HELIXCODE_ZERO_BLUFF_PLAN.md`
- **Testing Strategy**: `ANTI_BLUFF_TESTING_STRATEGY.md`
- **OpenAPI Spec**: `HelixCode/api/openapi.yaml`
- **Docker Guide**: `HelixCode/DOCKER_DEPLOYMENT.md`

---

<!-- END host-power-management addendum (CONST-033) -->



<!-- CONST-035 anti-bluff addendum (cascaded) -->

## CONST-035 — Anti-Bluff Tests & Challenges (mandatory; inherits from root)

Tests and Challenges in this submodule MUST verify the product, not
the LLM's mental model of the product. A test that passes when the
feature is broken is worse than a missing test — it gives false
confidence and lets defects ship to users. Functional probes at the
protocol layer are mandatory:

- TCP-open is the FLOOR, not the ceiling. Postgres → execute
  `SELECT 1`. Redis → `PING` returns `PONG`. ChromaDB → `GET
  /api/v1/heartbeat` returns 200. MCP server → TCP connect + valid
  JSON-RPC handshake. HTTP gateway → real request, real response,
  non-empty body.
- Container `Up` is NOT application healthy. A `docker/podman ps`
  `Up` status only means PID 1 is running; the application may be
  crash-looping internally.
- No mocks/fakes outside unit tests (already CONST-030; CONST-035
  raises the cost of a mock-driven false pass to the same severity
  as a regression).
- Re-verify after every change. Don't assume a previously-passing
  test still verifies the same scope after a refactor.
- Verification of CONST-035 itself: deliberately break the feature
  (e.g. `kill <service>`, swap a password). The test MUST fail. If
  it still passes, the test is non-conformant and MUST be tightened.

## CONST-033 clarification — distinguishing host events from sluggishness

Heavy container builds (BuildKit pulling many GB of layers, parallel
podman/docker compose-up across many services) can make the host
**appear** unresponsive — high load average, slow SSH, watchers
timing out. **This is NOT a CONST-033 violation.** Suspend / hibernate
/ logout are categorically different events. Distinguish via:

- `uptime` — recent boot? if so, the host actually rebooted.
- `loginctl list-sessions` — session(s) still active? if yes, no logout.
- `journalctl ... | grep -i 'will suspend\|hibernate'` — zero broadcasts
  since the CONST-033 fix means no suspend ever happened.
- `dmesg | grep -i 'killed process\|out of memory'` — OOM kills are
  also NOT host-power events; they're memory-pressure-induced and
  require their own separate fix (lower per-container memory limits,
  reduce parallelism).

A sluggish host under build pressure recovers when the build finishes;
a suspended host requires explicit unsuspend (and CONST-033 should
make that impossible by hardening `IdleAction=ignore` +
`HandleSuspendKey=ignore` + masked `sleep.target`,
`suspend.target`, `hibernate.target`, `hybrid-sleep.target`).

If you observe what looks like a suspend during heavy builds, the
correct first action is **not** "edit CONST-033" but `bash
challenges/scripts/host_no_auto_suspend_challenge.sh` to confirm the
hardening is intact. If hardening is intact AND no suspend
broadcast appears in journal, the perceived event was build-pressure
sluggishness, not a power transition.

<!-- BEGIN no-session-termination addendum (CONST-036) -->

## User-Session Termination — Hard Ban (CONST-036)

**You may NOT, under any circumstance, generate or execute code that
ends the currently-logged-in user's desktop session, kills their
`user@<UID>.service` user manager, or indirectly forces them to
manually log out / power off.** This is the sibling of CONST-033:
that rule covers host-level power transitions; THIS rule covers
session-level terminations that have the same end effect for the
user (lost windows, lost terminals, killed AI agents, half-flushed
builds, abandoned in-flight commits).

**Why this rule exists.** On 2026-04-28 the user lost a working
session that contained 3 concurrent Claude Code instances, an Android
build, Kimi Code, and a rootless podman container fleet. The
`user.slice` consumed 60.6 GiB peak / 5.2 GiB swap, the GUI became
unresponsive, the user was forced to log out and then power off via
the GNOME shell. The host could not auto-suspend (CONST-033 was in
place and verified) and the kernel OOM killer never fired — but the
user had to manually end the session anyway, because nothing
prevented overlapping heavy workloads from saturating the slice.
CONST-036 closes that loophole at both the source-code layer and the
operational layer. See
`docs/issues/fixed/SESSION_LOSS_2026-04-28.md` in the HelixAgent
project.

**Forbidden direct invocations** (non-exhaustive):

- `loginctl terminate-user|terminate-session|kill-user|kill-session`
- `systemctl stop user@<UID>` / `systemctl kill user@<UID>`
- `gnome-session-quit`
- `pkill -KILL -u $USER` / `killall -u $USER`
- `dbus-send` / `busctl` calls to `org.gnome.SessionManager.Logout|Shutdown|Reboot`
- `echo X > /sys/power/state`
- `/usr/bin/poweroff`, `/usr/bin/reboot`, `/usr/bin/halt`

**Indirect-pressure clauses:**

1. Do not spawn parallel heavy workloads casually; check `free -h`
   first; keep `user.slice` under 70% of physical RAM.
2. Long-lived background subagents go in `system.slice`. Rootless
   podman containers die with the user manager.
3. Document AI-agent concurrency caps in CLAUDE.md.
4. Never script "log out and back in" recovery flows.

**Defence:** every project ships
`scripts/host-power-management/check-no-session-termination-calls.sh`
(static scanner) and
`challenges/scripts/no_session_termination_calls_challenge.sh`
(challenge wrapper). Both MUST be wired into the project's CI /
`run_all_challenges.sh`.

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
*Built with zero-bluff commitment. Every feature actually works.*
