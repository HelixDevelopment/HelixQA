# HelixQA API Reference

## pkg/orchestrator

### Types

```go
type Orchestrator struct { /* unexported fields */ }
type Result struct {
    Report     *reporter.QAReport
    ReportPath string
    Success    bool
    StartTime  time.Time
    EndTime    time.Time
    Duration   time.Duration
}
type Option func(*Orchestrator)
```

### Functions

```go
func New(cfg *config.Config, opts ...Option) *Orchestrator
func (o *Orchestrator) Run(ctx context.Context) (*Result, error)
```

### Options

```go
func WithLogger(logger logging.Logger) Option
func WithRunner(r runner.Runner) Option
func WithDetector(d *detector.Detector) Option
func WithValidator(v *validator.Validator) Option
func WithReporter(r *reporter.Reporter) Option
func WithBank(b *bank.Bank) Option
```

---

## pkg/testbank

### Types

```go
type TestCase struct {
    ID                string
    Name              string
    Description       string
    Category          string
    Priority          Priority  // critical|high|medium|low
    Platforms         []config.Platform
    Steps             []TestStep
    Dependencies      []string
    DocumentationRefs []DocRef
    Tags              []string
    EstimatedDuration string
    ExpectedResult    string
}

type TestStep struct {
    Name     string
    Action   string
    Expected string
    Platform config.Platform  // optional
}

type DocRef struct {
    Type    string  // user_guide|api_spec|video_course|architecture
    Section string
    Path    string
}

type BankFile struct {
    Version   string
    Name      string
    Description string
    TestCases []TestCase
    Metadata  map[string]string
}

type Manager struct { /* unexported fields */ }
type Priority string  // critical|high|medium|low
```

### Functions

```go
// Loader
func LoadFile(path string) (*BankFile, error)
func LoadDir(dir string) ([]*BankFile, error)
func SaveFile(path string, bf *BankFile) error

// Manager
func NewManager() *Manager
func (m *Manager) LoadFile(path string) error
func (m *Manager) LoadDir(dir string) error
func (m *Manager) Get(id string) (*TestCase, bool)
func (m *Manager) All() []*TestCase
func (m *Manager) ForPlatform(p config.Platform) []*TestCase
func (m *Manager) ByCategory(category string) []*TestCase
func (m *Manager) ByPriority(p Priority) []*TestCase
func (m *Manager) ByTag(tag string) []*TestCase
func (m *Manager) Count() int
func (m *Manager) Sources() []string
func (m *Manager) Banks() []*BankFile
func (m *Manager) ToDefinitions(platform config.Platform) []*challenge.Definition

// TestCase methods
func (tc *TestCase) ToDefinition() *challenge.Definition
func (tc *TestCase) AppliesToPlatform(p config.Platform) bool
func (tc *TestCase) IsValid() string
```

---

## pkg/detector

### Types

```go
type Detector struct { /* unexported fields */ }
type DetectionResult struct {
    Platform       config.Platform
    HasCrash       bool
    HasANR         bool
    ProcessAlive   bool
    StackTrace     string
    LogEntries     []string
    ScreenshotPath string
    Timestamp      time.Time
    Error          string
}
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
type Option func(*Detector)
```

### Functions

```go
func New(platform config.Platform, opts ...Option) *Detector
func (d *Detector) Check(ctx context.Context) (*DetectionResult, error)
func (d *Detector) CheckApp(ctx context.Context, platform config.Platform) (*DetectionResult, error)
func (d *Detector) Platform() config.Platform
```

### Options

```go
func WithDevice(device string) Option
func WithPackageName(pkg string) Option
func WithBrowserURL(url string) Option
func WithProcessName(name string) Option
func WithProcessPID(pid int) Option
func WithEvidenceDir(dir string) Option
func WithCommandRunner(runner CommandRunner) Option
```

---

## pkg/validator

### Types

```go
type Validator struct { /* unexported fields */ }
type StepResult struct {
    StepName       string
    Status         StepStatus
    Platform       config.Platform
    Detection      *detector.DetectionResult
    PreScreenshot  string
    PostScreenshot string
    StartTime      time.Time
    EndTime        time.Time
    Duration       time.Duration
    Error          string
}
type StepStatus string  // passed|failed|skipped|error
type ScreenshotFunc func(ctx context.Context, name string) (string, error)
type Option func(*Validator)
```

### Functions

```go
func New(det *detector.Detector, opts ...Option) *Validator
func (v *Validator) ValidateStep(ctx context.Context, stepName string, platform config.Platform) (*StepResult, error)
func (v *Validator) Results() []*StepResult
func (v *Validator) PassedCount() int
func (v *Validator) FailedCount() int
func (v *Validator) TotalCount() int
func (v *Validator) EvidenceDir() string
func (v *Validator) EvidencePath(name string) string
func (v *Validator) Reset()
```

### Options

```go
func WithEvidenceDir(dir string) Option
func WithScreenshotFunc(fn ScreenshotFunc) Option
```

---

## pkg/evidence

### Types

```go
type Collector struct { /* unexported fields */ }
type Item struct {
    Type      Type  // screenshot|video|logcat|stacktrace|console_log
    Path      string
    Platform  config.Platform
    Step      string
    Timestamp time.Time
    Size      int64
}
type Type string
type Option func(*Collector)
```

### Functions

```go
func New(opts ...Option) *Collector
func (c *Collector) CaptureScreenshot(ctx context.Context, name string) (*Item, error)
func (c *Collector) CaptureLogcat(ctx context.Context, name string, lines int) (*Item, error)
func (c *Collector) StartRecording(ctx context.Context, name string) error
func (c *Collector) StopRecording(ctx context.Context) (*Item, error)
func (c *Collector) IsRecording() bool
func (c *Collector) Items() []Item
func (c *Collector) ItemsByType(t Type) []Item
func (c *Collector) Count() int
func (c *Collector) Reset()
```

### Options

```go
func WithOutputDir(dir string) Option
func WithPlatform(p config.Platform) Option
func WithCommandRunner(r detector.CommandRunner) Option
```

---

## pkg/ticket

### Types

```go
type Generator struct { /* unexported fields */ }
type Ticket struct {
    ID               string
    Title            string
    Severity         Severity  // critical|high|medium|low
    Platform         config.Platform
    TestCaseID       string
    Description      string
    StepsToReproduce []string
    ExpectedBehavior string
    ActualBehavior   string
    Detection        *detector.DetectionResult
    StepResult       *validator.StepResult
    Screenshots      []string
    Logs             []string
    StackTrace       string
    CreatedAt        time.Time
    Labels           []string
}
type Severity string
type Option func(*Generator)
```

### Functions

```go
func New(opts ...Option) *Generator
func (g *Generator) GenerateFromStep(sr *validator.StepResult, testCaseID string) *Ticket
func (g *Generator) GenerateFromDetection(dr *detector.DetectionResult, context string) *Ticket
func (g *Generator) WriteTicket(t *Ticket) (string, error)
func (g *Generator) WriteAll(tickets []*Ticket) ([]string, error)
func (g *Generator) RenderMarkdown(t *Ticket) []byte
```

### Options

```go
func WithOutputDir(dir string) Option
```

---

## pkg/reporter

### Types

```go
type Reporter struct { /* unexported fields */ }
type QAReport struct {
    Title            string
    GeneratedAt      time.Time
    PlatformResults  []*PlatformResult
    TotalChallenges  int
    PassedChallenges int
    FailedChallenges int
    TotalCrashes     int
    TotalANRs        int
    TotalDuration    time.Duration
    OutputDir        string
}
type PlatformResult struct {
    Platform         config.Platform
    ChallengeResults []*challenge.Result
    StepResults      []*validator.StepResult
    StartTime        time.Time
    EndTime          time.Time
    Duration         time.Duration
    CrashCount       int
    ANRCount         int
    EvidenceDir      string
}
type Option func(*Reporter)
```

### Functions

```go
func New(opts ...Option) *Reporter
func (r *Reporter) GenerateQAReport(results []*PlatformResult) (*QAReport, error)
func (r *Reporter) WriteMarkdown(qa *QAReport, path string) error
func (r *Reporter) WriteJSON(qa *QAReport, path string) error
func (r *Reporter) WriteReport(qa *QAReport, baseDir string) (string, error)
func (r *Reporter) GenerateChallengeReport(result *challenge.Result) ([]byte, error)
```

### Options

```go
func WithOutputDir(dir string) Option
func WithReportFormat(format config.ReportFormat) Option
func WithChallengeReporter(cr report.Reporter) Option
```

---

## pkg/config

### Types

```go
type Config struct {
    Banks          []string
    Platforms      []Platform
    Device         string
    PackageName    string
    OutputDir      string
    Speed          SpeedMode
    ReportFormat   ReportFormat
    ValidateSteps  bool
    Record         bool
    Verbose        bool
    Timeout        time.Duration
    StepTimeout    time.Duration
    BrowserURL     string
    DesktopProcess string
    DesktopPID     int
}
type Platform string      // android|web|desktop|all
type SpeedMode string     // slow|normal|fast
type ReportFormat string  // markdown|html|json
```

### Functions

```go
func DefaultConfig() *Config
func (c *Config) Validate() error
func (c *Config) ExpandedPlatforms() []Platform
func (c *Config) StepDelay() time.Duration
func ParsePlatforms(s string) ([]Platform, error)
func ParseBanks(s string) []string
```
