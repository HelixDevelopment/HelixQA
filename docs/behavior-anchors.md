---
schema_version: 1
constitution_rule: CONST-035
last_audit: 2026-05-01
---

# Behavior Anchor Manifest — HelixQA

Every row is a user-facing capability and the single anchor test that
proves it works end-to-end. See CONST-035 in `CONSTITUTION.md`.

## Status legend

- `active` — anchor exists and is callable; capability is verified.
- `pending-anchor` — capability declared, anchor test does not yet
  exist. Listed in `challenges/baselines/bluff-baseline.txt` Section 3.
  Reducing this state is the work of campaign sub-project 4.
- `retired` — capability removed; row kept for history.

## Path format

For Go tests: `<path>.go::<TestFuncName>`. Verifier greps for
`func <TestFuncName>\b` in the file.

## Capabilities

| id | layer | capability | anchor_test_path | verifies | status |
|----|-------|------------|------------------|----------|--------|
| CAP-001 | submodule:HelixQA | Construct orchestrator with default options | pkg/orchestrator/orchestrator_test.go::TestNew | New() returns a usable Orchestrator with default detector/validator/reporter | active |
| CAP-002 | submodule:HelixQA | Construct platform-default crash/ANR detector | pkg/detector/detector_test.go::TestNew_DefaultPlatform | Detector.New() defaults to the host's primary platform (linux/android/web) | active |
| CAP-003 | submodule:HelixQA | Detect Android process-alive without false-positive crash | pkg/detector/android_test.go::TestCheckAndroid_ProcessAlive_NoCrash | Android detector reports process alive when no crash signals are present in logcat | active |
| CAP-004 | submodule:HelixQA | Validate LLM crash-analysis output schema | pkg/detector/llm_analyzer_test.go::TestCrashAnalysis_Validate_Valid | CrashAnalysis JSON schema validation passes on well-formed LLM responses | active |
| CAP-005 | submodule:HelixQA | Construct step validator with defaults | pkg/validator/validator_test.go::TestNew_Defaults | Validator.New() returns a configured validator with default evidence collector | active |
| CAP-006 | submodule:HelixQA | Construct evidence collector with defaults | pkg/evidence/collector_test.go::TestCollector_New_Defaults | Evidence collector exposes types: screenshot, video, log, ticket | active |
| CAP-007 | submodule:HelixQA | Generate Markdown ticket from a crashing test step (with evidence path) | pkg/ticket/ticket_test.go::TestGenerator_GenerateFromStep_Crash | Ticket carries non-empty evidence_paths, session_id, step_number, repro_steps | active |
| CAP-008 | submodule:HelixQA | Construct reporter with defaults | pkg/reporter/reporter_test.go::TestNew_Defaults | Reporter.New() returns a usable reporter with default Markdown/JSON outputs | active |
| CAP-009 | submodule:HelixQA | Load test bank file (YAML/JSON) | pkg/testbank/manager_test.go::TestManager_LoadFile | TestBankManager.LoadFile() parses test bank and registers cases | active |
| CAP-010 | submodule:HelixQA | Default config exposes sensible platform/speed/format defaults | pkg/config/config_test.go::TestDefaultConfig | DefaultConfig() returns a non-empty config with required fields | active |
| CAP-011 | submodule:HelixQA | userflow-runner replay subcommand parses replay DSL from session log | cmd/helixqa/replay_test.go::TestExtractReplayDSL_Present | ExtractReplayDSL detects a present replay block in session-log Markdown | active |
| CAP-012 | submodule:HelixQA | FindingsBridge persists two distinct findings with non-empty evidence | pkg/autonomous/findings_bridge_test.go::TestFindingsBridge_Process_TwoFindings | Process() persists 2 findings as 2 tickets, each with evidence_paths | active |
| CAP-013 | submodule:HelixQA | LLM fallback chain returns first provider's success without trying next | pkg/autonomous/fallback_test.go::TestFallbackChain_FirstSucceeds | FallbackChain.Execute() short-circuits on first provider's success | active |
| CAP-014 | submodule:HelixQA | Adaptive LLM provider selects first ranked provider | pkg/llm/adaptive_test.go::TestAdaptiveProvider_SelectsFirst | AdaptiveProvider.Chat() routes to highest-scored provider | active |
| CAP-015 | submodule:HelixQA | Test plan generator produces test cases from knowledge base | pkg/planning/planner_test.go::TestTestPlanGenerator_Generate | TestPlanGenerator.Generate() returns a non-empty test plan with priority ordering | active |
| CAP-016 | submodule:HelixQA | Vision analyzer produces analysis from a screenshot | pkg/analysis/vision_test.go::TestVisionAnalyzer_AnalyzeScreenshot | VisionAnalyzer.AnalyzeScreenshot() returns AnalysisResult with non-empty findings | active |
| CAP-017 | submodule:HelixQA | scrcpy recorder reports start state correctly | pkg/video/scrcpy_test.go::TestScrcpyRecorder_StartState | ScrcpyRecorder transitions from Idle → Recording on Start() | active |

(More capabilities — geo-restriction probe, device-state preservation,
LLM cost tracking, llama.cpp RPC distributed inference, websocket
streaming, full e2e autonomous session — populated in subsequent
iterations of sub-project 3.)
