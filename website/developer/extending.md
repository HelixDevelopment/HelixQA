# Extending HelixQA

HelixQA exposes several extension points through Go interfaces. You can add new platform executors, crash detectors, LLM providers, report formats, and tool bridges without modifying the core framework. This guide walks through each extension point with implementation patterns and testing guidance.

## Adding a New Detector

The detector package monitors for crashes and ANRs during test execution. Each platform has its own detector implementation. To add detection for a new platform (e.g., iOS), implement the detection logic using the `CommandRunner` interface.

### Step 1: Create the Detector File

Create a new file in `pkg/detector/`:

```go
// SPDX-FileCopyrightText: 2026 Your Name
// SPDX-License-Identifier: Apache-2.0

package detector

import (
    "context"
    "strings"
    "time"

    "digital.vasic.helixqa/pkg/config"
)

// detectIOS performs crash detection for an iOS device
// by parsing syslog output.
func (d *Detector) detectIOS(
    ctx context.Context,
) *DetectionResult {
    result := &DetectionResult{
        Platform:  config.PlatformIOS,
        Timestamp: time.Now(),
    }

    // Capture recent syslog entries
    out, err := d.runner.Run(ctx,
        "idevicesyslog", "--no-colors",
    )
    if err != nil {
        result.Error = err.Error()
        return result
    }

    log := string(out)

    // Check for crash indicators
    if strings.Contains(log, "crashed") ||
        strings.Contains(log, "EXC_BAD_ACCESS") {
        result.HasCrash = true
        result.StackTrace = extractIOSStackTrace(log)
    }

    result.ProcessAlive = !result.HasCrash
    return result
}
```

### Step 2: Wire It Into the Detector

Add a case to the `Detect()` method in `detector.go`:

```go
func (d *Detector) Detect(
    ctx context.Context,
) *DetectionResult {
    switch d.platform {
    case config.PlatformAndroid:
        return d.detectAndroid(ctx)
    case config.PlatformWeb:
        return d.detectWeb(ctx)
    case config.PlatformDesktop:
        return d.detectDesktop(ctx)
    case config.PlatformIOS:
        return d.detectIOS(ctx)
    default:
        return &DetectionResult{
            Error: "unsupported platform",
        }
    }
}
```

### Step 3: Test With a Mock Runner

The `CommandRunner` interface makes testing straightforward. Create a mock that returns known output:

```go
type mockRunner struct {
    output []byte
    err    error
}

func (m *mockRunner) Run(
    ctx context.Context,
    name string,
    args ...string,
) ([]byte, error) {
    return m.output, m.err
}

func TestDetector_iOS_CrashDetected(t *testing.T) {
    runner := &mockRunner{
        output: []byte(
            "Process crashed with EXC_BAD_ACCESS",
        ),
    }
    d := NewDetector(config.PlatformIOS, "", runner)
    result := d.Detect(context.Background())

    assert.True(t, result.HasCrash)
    assert.False(t, result.ProcessAlive)
}

func TestDetector_iOS_NoCrash(t *testing.T) {
    runner := &mockRunner{
        output: []byte("normal log output"),
    }
    d := NewDetector(config.PlatformIOS, "", runner)
    result := d.Detect(context.Background())

    assert.False(t, result.HasCrash)
    assert.True(t, result.ProcessAlive)
}
```

---

## Adding a New Validator

Validators wrap detectors to provide step-level validation with evidence. The existing `Validator` struct in `pkg/validator/` handles most cases. To add custom validation logic (e.g., performance threshold checks), extend the validator:

```go
// SPDX-FileCopyrightText: 2026 Your Name
// SPDX-License-Identifier: Apache-2.0

package validator

// PerformanceValidator adds response time validation
// on top of the standard crash/ANR detection.
type PerformanceValidator struct {
    *Validator
    maxResponseMs int64
}

func NewPerformanceValidator(
    v *Validator,
    maxResponseMs int64,
) *PerformanceValidator {
    return &PerformanceValidator{
        Validator:     v,
        maxResponseMs: maxResponseMs,
    }
}

func (pv *PerformanceValidator) ValidateStep(
    ctx context.Context,
    stepName string,
    responseMs int64,
) *StepResult {
    // Run standard crash detection first
    result := pv.Validator.ValidateStep(ctx, stepName)

    // Add performance check
    if result.Status == StepPassed &&
        responseMs > pv.maxResponseMs {
        result.Status = StepFailed
        result.Error = fmt.Sprintf(
            "response time %dms exceeds threshold %dms",
            responseMs, pv.maxResponseMs,
        )
    }
    return result
}
```

---

## Adding a New Reporter Format

The reporter package generates session reports. To add a new format (e.g., JUnit XML), implement a formatter function and wire it into the report generation flow.

### Step 1: Create the Formatter

Create a new file in `pkg/reporter/`:

```go
// SPDX-FileCopyrightText: 2026 Your Name
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
    "encoding/xml"
    "fmt"
    "os"
    "path/filepath"
)

// JUnitTestSuites is the root element.
type JUnitTestSuites struct {
    XMLName xml.Name         `xml:"testsuites"`
    Suites  []JUnitTestSuite `xml:"testsuite"`
}

// JUnitTestSuite represents a platform result.
type JUnitTestSuite struct {
    XMLName  xml.Name        `xml:"testsuite"`
    Name     string          `xml:"name,attr"`
    Tests    int             `xml:"tests,attr"`
    Failures int             `xml:"failures,attr"`
    Cases    []JUnitTestCase `xml:"testcase"`
}

// JUnitTestCase represents a single test result.
type JUnitTestCase struct {
    XMLName   xml.Name       `xml:"testcase"`
    Name      string         `xml:"name,attr"`
    ClassName string         `xml:"classname,attr"`
    Time      string         `xml:"time,attr"`
    Failure   *JUnitFailure  `xml:"failure,omitempty"`
}

// JUnitFailure describes why a test failed.
type JUnitFailure struct {
    Message string `xml:"message,attr"`
    Type    string `xml:"type,attr"`
    Content string `xml:",chardata"`
}

// GenerateJUnit writes a JUnit XML report.
func (r *Reporter) GenerateJUnit(
    report *QAReport,
    outputDir string,
) error {
    suites := convertToJUnit(report)
    data, err := xml.MarshalIndent(suites, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal junit: %w", err)
    }

    path := filepath.Join(
        outputDir, "pipeline-report.xml",
    )
    return os.WriteFile(path, data, 0644)
}
```

### Step 2: Register the Format

Add the format to the report generation dispatch in `reporter.go` or `enhanced.go`:

```go
case "junit":
    return r.GenerateJUnit(report, outputDir)
```

---

## Adding a New LLM Provider

All LLM providers implement the `Provider` interface from `pkg/llm/provider.go`. Most new providers are OpenAI-compatible and can reuse the `openai.go` client with different base URLs.

### For OpenAI-Compatible Providers

Most Tier 2 providers use the OpenAI `chat/completions` API format. Add them to `providers_registry.go`:

```go
{
    name:    "newprovider",
    envKey:  "NEWPROVIDER_API_KEY",
    baseURL: "https://api.newprovider.com/v1",
    model:   "default-model-name",
},
```

The registry creates an OpenAI-compatible client with the specified base URL automatically.

### For Non-Compatible Providers

If the provider has a unique API format, create a new implementation file:

```go
// SPDX-FileCopyrightText: 2026 Your Name
// SPDX-License-Identifier: Apache-2.0

package llm

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

// CustomProvider implements the Provider interface for
// a provider with a non-standard API.
type CustomProvider struct {
    apiKey  string
    baseURL string
    model   string
    client  *http.Client
}

func NewCustomProvider(apiKey string) *CustomProvider {
    return &CustomProvider{
        apiKey:  apiKey,
        baseURL: "https://api.custom.ai/v1",
        model:   "custom-default",
        client:  &http.Client{},
    }
}

func (p *CustomProvider) Chat(
    ctx context.Context,
    messages []Message,
) (*Response, error) {
    // Build request body in provider's format
    body := buildCustomRequest(messages, p.model)
    // Send request, parse response
    resp, err := p.doRequest(ctx, "/chat", body)
    if err != nil {
        return nil, fmt.Errorf("custom chat: %w", err)
    }
    return parseCustomResponse(resp)
}

func (p *CustomProvider) Vision(
    ctx context.Context,
    image []byte,
    prompt string,
) (*Response, error) {
    // Implement vision API call
    return nil, fmt.Errorf("vision not supported")
}

func (p *CustomProvider) Name() string {
    return "custom"
}

func (p *CustomProvider) SupportsVision() bool {
    return false
}
```

See [LLM Providers](/developer/llm-providers) for detailed provider configuration.

---

## Adding a New Bridge

Bridges integrate external QA tools. The bridge registry in `pkg/bridges/` discovers which tools are installed on the host.

### Step 1: Add a Tool Probe

In `pkg/bridges/registry.go`, add an entry to `toolProbes`:

```go
var toolProbes = []toolProbe{
    // Existing tools...
    {name: "scrcpy", versionArgs: []string{"--version"}},
    {name: "appium", versionArgs: []string{"--version"}},

    // Your new tool
    {name: "mytool", versionArgs: []string{"--version"}},
}
```

### Step 2: Create a Bridge Package (Optional)

For tools that need more than simple subprocess invocation, create a subdirectory under `pkg/bridges/`:

```
pkg/bridges/mytool/
    mytool.go
    mytool_test.go
```

```go
// SPDX-FileCopyrightText: 2026 Your Name
// SPDX-License-Identifier: Apache-2.0

package mytool

import (
    "context"

    "digital.vasic.helixqa/pkg/bridges"
)

// Runner wraps the mytool binary for QA operations.
type Runner struct {
    path   string
    runner bridges.CommandRunner
}

func NewRunner(
    path string,
    runner bridges.CommandRunner,
) *Runner {
    return &Runner{path: path, runner: runner}
}

func (r *Runner) Execute(
    ctx context.Context,
    args ...string,
) ([]byte, error) {
    return r.runner.Run(ctx, r.path, args...)
}
```

---

## Testing Extensions

### Unit Tests

Every extension should have table-driven tests covering the pass path, fail path, and edge cases:

```go
func TestCustomProvider_Chat(t *testing.T) {
    tests := []struct {
        name     string
        messages []Message
        wantErr  bool
    }{
        {
            name:     "single message",
            messages: []Message{{Role: RoleUser, Content: "hello"}},
            wantErr:  false,
        },
        {
            name:     "empty messages",
            messages: []Message{},
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := NewCustomProvider("test-key")
            _, err := p.Chat(context.Background(), tt.messages)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests

For extensions that interact with external services, use build tags to isolate integration tests:

```go
//go:build integration

func TestCustomProvider_RealAPI(t *testing.T) {
    key := os.Getenv("CUSTOM_API_KEY")
    if key == "" {
        t.Skip("CUSTOM_API_KEY not set")
    }
    // Test against real API
}
```

Run integration tests explicitly:

```bash
go test ./pkg/llm/ -tags=integration -count=1
```

---

## Contributing Guidelines

### Code Style

- Follow standard Go conventions and `gofmt` formatting
- Add SPDX license headers to every `.go` file
- Group imports: stdlib, third-party, internal (blank-line separated)
- Target 80-character line width (100 maximum)
- Wrap errors with `fmt.Errorf("context: %w", err)`

### Naming

- Private: `camelCase`
- Exported: `PascalCase`
- Test functions: `Test<Struct>_<Method>_<Scenario>`
- Test files: `*_test.go` beside the source file

### Pull Request Checklist

1. New code has tests (both pass and fail paths)
2. All existing tests pass: `go test ./... -count=1 -race`
3. No lint issues: `go vet ./...`
4. SPDX header on every new file
5. Documentation updated if adding a new package
6. Commit message follows conventional commits: `feat(detector): add iOS crash detection`

## Related Pages

- [Architecture Reference](/developer/architecture) -- full package structure
- [LLM Providers](/developer/llm-providers) -- provider implementation details
- [Challenges](/guides/challenges) -- challenge development
- [Open-Source Tools](/advanced/tools) -- tools integrated via bridges
