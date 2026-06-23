// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
//
// helixqa-concrete-runner — runs CONCRETE-ACTION banks (as opposed to
// the prose-step banks the legacy `helixqa run` consumed). Closes the
// CONST-035 §11.4 bluff documented in a consuming project's docs/qa/iter-32/README.md:
//
//   > helixqa run reports PASSED for 22 challenges that NEVER EXECUTED.
//   > The validator is a 200µs crash-observer presented as a test
//   > executor. Step-validation rows mark every test PASSED in microseconds.
//
// The concrete-action bank YAML schema (see schema.go) replaces the
// human-prose "action" + "expected" pair with structured action records
// that this runner can execute directly via adb. Each step has a concrete
// effect on a connected device + a concrete assertion against the
// post-step UI hierarchy. No LLM, no Appium server — just adb + the Go
// stdlib + a small UI-hierarchy parser.
//
// Each PASS in this runner carries POSITIVE runtime evidence (the actual
// dumped UI hierarchy is saved to evidence-dir and the assertion's
// matching text/content-desc is referenced in the result JSON). This is
// the CONST-035 §11.4.2 captured-evidence floor.
//
// Usage:
//
//	helixqa-concrete-runner \
//	    -bank      banks/yole-concrete/file-browser-launch.yaml \
//	    -device    emulator-5554 \
//	    -adb       /opt/homebrew/share/android-commandlinetools/platform-tools/adb \
//	    -evidence  /tmp/yole-qa-evidence \
//	    -package   digital.vasic.yole.android
//
// Exit codes:
//   0 — all cases passed
//   1 — at least one case failed
//   2 — invocation error (bank parse, adb missing, device not reachable)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	bankPath := flag.String("bank", "", "path to a concrete-action bank YAML file (required)")
	deviceID := flag.String("device", "emulator-5554", "adb device ID")
	adbBin := flag.String("adb", "adb", "path to the adb binary")
	evidenceDir := flag.String("evidence", "", "directory for screenshots + UI dumps (required)")
	pkg := flag.String("package", "", "Android package name under test (required)")
	timeout := flag.Duration("timeout", 5*time.Minute, "max total run time")
	flag.Parse()

	if *bankPath == "" || *evidenceDir == "" || *pkg == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-concrete-runner -bank PATH -evidence DIR -package PKG [-device ID] [-adb PATH] [-timeout DUR]")
		os.Exit(2)
	}

	bank, err := loadBank(*bankPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: load bank: %v\n", err)
		os.Exit(2)
	}

	if err := os.MkdirAll(*evidenceDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: create evidence dir: %v\n", err)
		os.Exit(2)
	}

	adb := &ADB{Path: *adbBin, Device: *deviceID}
	if err := adb.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: adb not reachable: %v\n", err)
		os.Exit(2)
	}

	runner := &Runner{
		ADB:         adb,
		Bank:        bank,
		Package:     *pkg,
		EvidenceDir: *evidenceDir,
		Timeout:     *timeout,
	}

	results := runner.Run()

	// Summary block — CONST-035 honest reporting: distinguish what was
	// executed from what merely loaded. The legacy `helixqa run` collapsed
	// these into a single "PASSED" line; this runner does not.
	passed := 0
	failed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Bank:     %s\n", filepath.Base(*bankPath))
	fmt.Printf("Cases:    %d\n", len(results))
	fmt.Printf("Passed:   %d\n", passed)
	fmt.Printf("Failed:   %d\n", failed)
	fmt.Printf("Evidence: %s\n", *evidenceDir)
	fmt.Println(strings.Repeat("=", 60))

	if failed > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}
