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

---

## pkg/autonomous

Session coordination for the Autonomous QA Session. Manages the 4-phase lifecycle, platform workers, and phase transitions.

### Types

```go
// SessionCoordinator manages the entire autonomous QA session lifecycle.
// It orchestrates setup, doc-driven verification, curiosity-driven exploration,
// and report generation across multiple platforms in parallel.
type SessionCoordinator struct {
    config       *SessionConfig
    verifier     LLMsVerifierClient   // selects and scores LLMs
    docProcessor DocProcessorClient   // builds feature maps from docs
    orchestrator AgentPool            // manages CLI agent pool
    visionEngine Analyzer             // VisionEngine screen analysis
    featureMap   *FeatureMap          // built from project docs
    workers      map[string]*PlatformWorker
    phaseManager *PhaseManager
    session      *SessionRecorder
    mu           sync.Mutex
}

// SessionConfig holds all configuration for an autonomous session.
type SessionConfig struct {
    ProjectRoot       string
    Platforms         []string          // "android", "desktop", "web"
    Timeout           time.Duration
    CoverageTarget    float64           // 0.0 - 1.0
    CuriosityEnabled  bool
    CuriosityTimeout  time.Duration
    OutputDir         string
    ReportFormats     []string          // "markdown", "html", "json"
    EnvFile           string
}

// SessionResult contains all outputs from a completed session.
type SessionResult struct {
    SessionID      string
    Status         SessionStatus
    StartTime      time.Time
    EndTime        time.Time
    Duration       time.Duration
    Phases         []Phase
    PlatformResults map[string]*PlatformResult
    Coverage       CoverageReport
    Tickets        []*Ticket
    ReportPaths    []string
    VideoPaths     map[string]string    // platform -> video path
    NavGraphs      map[string]GraphSnapshot
}

// SessionStatus represents the overall session state.
type SessionStatus string
const (
    SessionPending    SessionStatus = "pending"
    SessionRunning    SessionStatus = "running"
    SessionPaused     SessionStatus = "paused"
    SessionCompleted  SessionStatus = "completed"
    SessionFailed     SessionStatus = "failed"
    SessionCancelled  SessionStatus = "cancelled"
)

// ProgressReport provides real-time progress information.
type ProgressReport struct {
    CurrentPhase   string
    PhaseProgress  float64  // 0.0 - 1.0
    OverallProgress float64
    FeaturesVerified int
    FeaturesTotal    int
    IssuesFound      int
    PlatformStatus   map[string]string
}
```

### Functions

```go
func NewSessionCoordinator(config *SessionConfig, opts ...SessionOption) (*SessionCoordinator, error)
func (sc *SessionCoordinator) Run(ctx context.Context) (*SessionResult, error)
func (sc *SessionCoordinator) Pause(ctx context.Context) error
func (sc *SessionCoordinator) Resume(ctx context.Context) error
func (sc *SessionCoordinator) Cancel(ctx context.Context) error
func (sc *SessionCoordinator) Status() SessionStatus
func (sc *SessionCoordinator) Progress() ProgressReport
```

### PlatformWorker

```go
// PlatformWorker executes both doc-driven and curiosity-driven phases
// for a single platform. Each worker holds its own agent, analyzer,
// navigator, and issue detector.
type PlatformWorker struct {
    platform      string           // "android", "desktop", "web"
    agent         Agent            // acquired from AgentPool
    analyzer      Analyzer         // VisionEngine analyzer
    navigator     *NavigationEngine
    issueDetector *IssueDetector
    coverage      CoverageTracker  // from DocProcessor
    navGraph      NavigationGraph  // from VisionEngine/pkg/graph
    detector      CrashDetector    // existing HelixQA detector
    session       *SessionRecorder
    executor      ActionExecutor   // platform-specific (ADB/Playwright/X11)
    mu            sync.Mutex
}

// StepResult captures the outcome of a single verification or exploration step.
type StepResult struct {
    StepName       string
    Platform       string
    FeatureID      string
    Action         Action
    BeforeScreen   *ScreenAnalysis
    AfterScreen    *ScreenAnalysis
    Passed         bool
    Issues         []Issue
    ScreenshotPaths []string
    VideoOffset    time.Duration
    Duration       time.Duration
    Error          error
}
```

### Functions

```go
func NewPlatformWorker(platform string, opts ...WorkerOption) (*PlatformWorker, error)
func (pw *PlatformWorker) RunDocDriven(ctx context.Context, features []Feature) ([]StepResult, error)
func (pw *PlatformWorker) RunCuriosityDriven(ctx context.Context, timeout time.Duration) ([]StepResult, error)
```

### PhaseManager

```go
// PhaseManager tracks phase transitions with listener notifications.
// Thread-safe via sync.Mutex.
type PhaseManager struct {
    phases    []Phase
    current   int
    listeners []PhaseListener
    mu        sync.Mutex
}

// Phase represents one of the 4 session phases.
type Phase struct {
    Name     string      // "setup", "doc-driven", "curiosity", "report"
    Status   PhaseStatus // pending, running, completed, failed, skipped
    StartAt  time.Time
    EndAt    time.Time
    Progress float64     // 0.0 - 1.0
    Error    error
}

type PhaseStatus string
const (
    PhasePending   PhaseStatus = "pending"
    PhaseRunning   PhaseStatus = "running"
    PhaseCompleted PhaseStatus = "completed"
    PhaseFailed    PhaseStatus = "failed"
    PhaseSkipped   PhaseStatus = "skipped"
)

// PhaseListener receives notifications on phase transitions.
type PhaseListener interface {
    OnPhaseStart(phase Phase)
    OnPhaseComplete(phase Phase)
    OnPhaseError(phase Phase, err error)
}
```

### Functions

```go
func NewPhaseManager(phases ...string) *PhaseManager
func (pm *PhaseManager) AddListener(listener PhaseListener)
func (pm *PhaseManager) Start(name string) error     // pending -> running
func (pm *PhaseManager) Complete(name string) error   // running -> completed
func (pm *PhaseManager) Fail(name string, err error) error
func (pm *PhaseManager) Skip(name string) error
func (pm *PhaseManager) Current() Phase
func (pm *PhaseManager) All() []Phase
```

---

## pkg/navigator

Navigation engine for autonomous app traversal. Provides platform-specific action execution via the ActionExecutor interface.

### Types

```go
// NavigationEngine coordinates agent-driven navigation through an app's UI.
// It maintains a NavigationGraph (from VisionEngine) and uses an ActionExecutor
// for platform-specific interactions.
type NavigationEngine struct {
    agent    Agent            // LLM agent for navigation decisions
    analyzer Analyzer         // VisionEngine for screen analysis
    executor ActionExecutor   // platform-specific action execution
    graph    NavigationGraph  // directed graph of screens + transitions
    state    *StateTracker    // current navigation state
}

// ActionResult captures the outcome of a single UI action.
type ActionResult struct {
    Action       Action
    BeforeScreen *ScreenAnalysis
    AfterScreen  *ScreenAnalysis
    Success      bool
    NewScreen    bool   // true if action led to a new screen
    ScreenID     string
    Duration     time.Duration
    Error        error
}

// ExploreResult captures the outcome of one exploration step.
type ExploreResult struct {
    ScreensVisited   int
    ActionsPerformed int
    IssuesFound      []Issue
    NewScreenIDs     []string
    Coverage         float64
    Duration         time.Duration
}

// StateTracker maintains the current navigation state.
type StateTracker struct {
    CurrentScreenID string
    History         []string   // visited screen IDs in order
    BackStack       []string   // screens for Back navigation
    FailedPaths     [][]string // paths that failed to reach target
}
```

### Functions

```go
func NewNavigationEngine(agent Agent, analyzer Analyzer, executor ActionExecutor, graph NavigationGraph) *NavigationEngine
func (ne *NavigationEngine) NavigateTo(ctx context.Context, target string) error
func (ne *NavigationEngine) PerformAction(ctx context.Context, action Action) (*ActionResult, error)
func (ne *NavigationEngine) ExploreUnknown(ctx context.Context) (*ExploreResult, error)
func (ne *NavigationEngine) CurrentScreen(ctx context.Context) (*ScreenAnalysis, error)
func (ne *NavigationEngine) GoBack(ctx context.Context) error
func (ne *NavigationEngine) GoHome(ctx context.Context) error
```

### ActionExecutor Interface

```go
// ActionExecutor provides platform-specific UI interaction capabilities.
// Three implementations: ADBExecutor (Android), PlaywrightExecutor (Web),
// X11Executor (Desktop Linux).
type ActionExecutor interface {
    Click(ctx context.Context, x, y int) error
    Type(ctx context.Context, text string) error
    Scroll(ctx context.Context, direction string, amount int) error
    LongPress(ctx context.Context, x, y int) error
    Swipe(ctx context.Context, fromX, fromY, toX, toY int) error
    KeyPress(ctx context.Context, key string) error
    Back(ctx context.Context) error
    Home(ctx context.Context) error
    Screenshot(ctx context.Context) ([]byte, error)
}
```

### Implementations

```go
// ADBExecutor executes actions on Android devices via adb shell input.
type ADBExecutor struct { /* device string, commandRunner */ }
func NewADBExecutor(device string, opts ...ADBOption) *ADBExecutor

// PlaywrightExecutor executes actions in web browsers via Playwright API.
type PlaywrightExecutor struct { /* browserURL string, browser string */ }
func NewPlaywrightExecutor(browserURL string, opts ...PlaywrightOption) *PlaywrightExecutor

// X11Executor executes actions on desktop Linux via xdotool.
type X11Executor struct { /* display string, processName string */ }
func NewX11Executor(display string, opts ...X11Option) *X11Executor
```

---

## pkg/issuedetector

LLM-powered bug detection across visual, UX, accessibility, functional, performance, and crash categories.

### Types

```go
// IssueDetector uses an LLM agent and VisionEngine analyzer to identify
// issues by comparing before/after screen states and analyzing navigation patterns.
type IssueDetector struct {
    agent     Agent            // LLM agent for analysis
    analyzer  Analyzer         // VisionEngine for screen comparison
    ticketGen *ticket.Generator
    session   *SessionRecorder
}

// Issue represents a detected problem in the application.
type Issue struct {
    ID          string
    Type        IssueType    // visual, ux, accessibility, functional, performance, crash
    Severity    string       // critical, high, medium, low
    Title       string
    Description string
    Platform    string
    ScreenID    string
    FeatureID   string       // if linked to a documented feature
    Evidence    []string     // screenshot paths
    VideoOffset time.Duration
    LLMAnalysis string       // LLM explanation and suggested fix
    DetectedAt  time.Time
}

type IssueType string
const (
    IssueVisual        IssueType = "visual"
    IssueUX            IssueType = "ux"
    IssueAccessibility IssueType = "accessibility"
    IssueFunctional    IssueType = "functional"
    IssuePerformance   IssueType = "performance"
    IssueCrash         IssueType = "crash"
)

// IssueCategory provides classification and prompt templates per issue type.
type IssueCategory struct {
    Type         IssueType
    Name         string
    Description  string
    PromptHint   string   // hint for LLM analysis prompt
    MinSeverity  string   // minimum severity to report
}
```

### Functions

```go
func NewIssueDetector(agent Agent, analyzer Analyzer, opts ...IssueDetectorOption) *IssueDetector
func (id *IssueDetector) AnalyzeAction(ctx context.Context, before, after ScreenAnalysis, action Action) ([]Issue, error)
func (id *IssueDetector) AnalyzeUX(ctx context.Context, navGraph NavigationGraph) ([]Issue, error)
func (id *IssueDetector) AnalyzeAccessibility(ctx context.Context, screen ScreenAnalysis) ([]Issue, error)
func (id *IssueDetector) CreateTicket(ctx context.Context, issue Issue) (*Ticket, error)
```

---

## pkg/session

Recording and timeline management for autonomous QA sessions. Manages video capture (ffmpeg, adb screenrecord, Playwright API), screenshot indexing, and event timeline.

### Types

```go
// SessionRecorder manages video recording and timeline events for an
// autonomous QA session. Thread-safe via sync.Mutex.
type SessionRecorder struct {
    sessionID     string
    outputDir     string
    videos        map[string]*VideoManager  // platform -> video manager
    timeline      *Timeline
    screenshotIdx int
    mu            sync.Mutex
}

// Screenshot captures a single screenshot with metadata.
type Screenshot struct {
    Path      string
    Platform  string
    Name      string
    Index     int
    Timestamp time.Time
    Size      int64
}

// TimelineEvent records a single event in the session timeline.
type TimelineEvent struct {
    ID             string
    Type           EventType
    Platform       string
    Timestamp      time.Time
    VideoOffset    time.Duration  // offset into the platform video
    ScreenID       string
    Description    string
    ScreenshotPath string
    IssueID        string
    FeatureID      string
    Metadata       map[string]string
}

type EventType string
const (
    EventAction       EventType = "action"
    EventScreenshot   EventType = "screenshot"
    EventIssue        EventType = "issue"
    EventPhaseChange  EventType = "phase_change"
    EventCrash        EventType = "crash"
    EventNavigation   EventType = "navigation"
)

// Timeline stores and queries session events.
type Timeline struct {
    events []TimelineEvent
    mu     sync.RWMutex
}

// VideoManager handles video recording lifecycle for a single platform.
type VideoManager struct {
    platform   string
    outputPath string
    startTime  time.Time
    recording  bool
    process    *os.Process
    mu         sync.Mutex
}
```

### Functions

```go
// SessionRecorder
func NewSessionRecorder(sessionID, outputDir string) *SessionRecorder
func (sr *SessionRecorder) StartRecording(ctx context.Context, platform string) error
func (sr *SessionRecorder) StopRecording(ctx context.Context, platform string) (string, error)
func (sr *SessionRecorder) CaptureScreenshot(ctx context.Context, platform, name string) (Screenshot, error)
func (sr *SessionRecorder) RecordEvent(event TimelineEvent)
func (sr *SessionRecorder) VideoTimestamp(platform string) time.Duration
func (sr *SessionRecorder) ExportTimeline() []TimelineEvent

// Timeline
func NewTimeline() *Timeline
func (t *Timeline) Add(event TimelineEvent)
func (t *Timeline) Events() []TimelineEvent
func (t *Timeline) EventsByPlatform(platform string) []TimelineEvent
func (t *Timeline) EventsByType(eventType EventType) []TimelineEvent
func (t *Timeline) EventsInRange(start, end time.Time) []TimelineEvent

// VideoManager
func NewVideoManager(platform, outputPath string) *VideoManager
func (vm *VideoManager) Start(ctx context.Context) error
func (vm *VideoManager) Stop(ctx context.Context) (string, error)
func (vm *VideoManager) IsRecording() bool
func (vm *VideoManager) Elapsed() time.Duration
```

---

## Resilience Patterns

All LLM interactions in the autonomous session use the following resilience patterns.

### Retry with Exponential Backoff

```go
// RetryConfig configures retry behavior for LLM calls.
type RetryConfig struct {
    MaxRetries    int           // default: 3
    InitialDelay  time.Duration // default: 1s
    MaxDelay      time.Duration // default: 30s
    BackoffFactor float64       // default: 2.0
}

func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error
```

### Fallback Chain

```go
// FallbackChain tries providers in score-ranked order until one succeeds.
type FallbackChain struct {
    providers []VisionProvider
    current   int
}

func NewFallbackChain(providers ...VisionProvider) *FallbackChain
func (fc *FallbackChain) Execute(ctx context.Context, fn func(VisionProvider) error) error
```

### Response Sanitization

```go
// SanitizeResponse removes path traversal patterns, shell metacharacters,
// and excessively long content from LLM responses before use as file paths,
// shell commands, or ticket content.
func SanitizeResponse(raw string, maxLength int) string
func SanitizePath(raw string) (string, error)
func SanitizeCommand(raw string) (string, error)
```
