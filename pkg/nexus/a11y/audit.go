package a11y

import (
	"context"
	"errors"
	"fmt"
)

// Evaluator is the narrow browser-facing contract the Auditor needs to
// execute axe-core against a live page. The browser Engine satisfies it
// via its own adapter; tests inject a fake.
type Evaluator interface {
	// Eval runs script in the current page context and returns the
	// string-encoded result (axe-core serialises to JSON).
	Eval(ctx context.Context, script string) (string, error)
}

// Auditor drives a Report end-to-end: inject axe-core from a vendored
// assetBase, invoke `axe.run()`, parse the JSON, apply a WCAG level
// assertion. A single Auditor can be reused across sessions.
type Auditor struct {
	assetBase string
	level     Level
}

// NewAuditor returns an Auditor that pulls axe-core from assetBase and
// enforces the given compliance level when Run() completes.
func NewAuditor(assetBase string, level Level) (*Auditor, error) {
	if assetBase == "" {
		return nil, errors.New("a11y: assetBase is required")
	}
	if level != LevelA && level != LevelAA && level != LevelAAA {
		return nil, fmt.Errorf("a11y: unknown level %q", level)
	}
	return &Auditor{assetBase: assetBase, level: level}, nil
}

// Run injects axe-core, evaluates it in the page, parses the response,
// and returns the Report. The level assertion is applied to *Report on
// demand via Report.Assert so callers can record non-passing runs
// without failing the whole test.
func (a *Auditor) Run(ctx context.Context, e Evaluator) (*Report, error) {
	if e == nil {
		return nil, errors.New("a11y: nil evaluator")
	}
	raw, err := e.Eval(ctx, InjectionScript(a.assetBase))
	if err != nil {
		return nil, fmt.Errorf("a11y: eval axe: %w", err)
	}
	return Parse([]byte(raw))
}

// RunAndAssert returns a non-nil error on breach at the configured
// level. Use Run() directly if the caller wants to record violations
// without failing the test.
func (a *Auditor) RunAndAssert(ctx context.Context, e Evaluator) (*Report, error) {
	report, err := a.Run(ctx, e)
	if err != nil {
		return nil, err
	}
	if err := report.Assert(a.level); err != nil {
		return report, err
	}
	return report, nil
}

// Level returns the configured compliance level.
func (a *Auditor) Level() Level { return a.level }
