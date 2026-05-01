# SPDX-FileCopyrightText: 2026 Milos Vasic
# SPDX-License-Identifier: Apache-2.0

.PHONY: all build test test-race vet lint clean help install \
	anti-bluff anti-bluff-scan anti-bluff-anchors anti-bluff-mutation \
	anti-bluff-mutation-changed update-baseline qa-all challenge

# Default target
all: vet test build

## Build the helixqa binary
build:
	go build -o bin/helixqa ./cmd/helixqa

## Install the helixqa binary
install:
	go install ./cmd/helixqa

## Run all tests
test:
	go test ./... -count=1

## Run all tests with race detection
test-race:
	go test ./... -race -count=1

## Run tests with coverage report
test-cover:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -html=coverage.out -o coverage.html

## Run go vet
vet:
	go vet ./...

## Run static analysis (requires golangci-lint)
lint:
	golangci-lint run ./...

## Format all Go files
fmt:
	gofmt -w .

## Tidy go.mod
tidy:
	go mod tidy

## Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html qa-results/ results/ evidence/

## Show help
help:
	@echo "HelixQA - QA Orchestration Framework"
	@echo ""
	@echo "Targets:"
	@echo "  all        Build, vet, and test (default)"
	@echo "  build      Build the helixqa binary"
	@echo "  install    Install the helixqa binary"
	@echo "  test       Run all tests"
	@echo "  test-race  Run tests with race detection"
	@echo "  test-cover Run tests with coverage report"
	@echo "  vet        Run go vet"
	@echo "  lint       Run golangci-lint"
	@echo "  fmt        Format Go files"
	@echo "  tidy       Tidy go.mod"
	@echo "  clean      Clean build artifacts"
	@echo "  help       Show this help"
	@echo ""
	@echo "  anti-bluff                    All CONST-035 gates (scan+anchors+mutation-changed)"
	@echo "  anti-bluff-scan               Static scanner (full tree)"
	@echo "  anti-bluff-anchors            Behavior-anchor manifest validator"
	@echo "  anti-bluff-mutation           go-mutesting full project (slow)"
	@echo "  anti-bluff-mutation-changed   go-mutesting on changed files only"
	@echo "  update-baseline               Print instructions to refresh the baseline"
	@echo "  qa-all                        Full QA: existing challenges + anti-bluff"
	@echo "  challenge                     Run all challenges/scripts/*.sh"

# === CONST-035 anti-bluff gates ===

anti-bluff-scan:
	@bash scripts/anti-bluff/bluff-scanner.sh --mode all

anti-bluff-anchors:
	@bash challenges/scripts/anchor_manifest_challenge.sh

anti-bluff-mutation:
	@bash challenges/scripts/mutation_ratchet_challenge.sh --mode all

anti-bluff-mutation-changed:
	@bash challenges/scripts/mutation_ratchet_challenge.sh

anti-bluff: anti-bluff-scan anti-bluff-anchors anti-bluff-mutation-changed

update-baseline:
	@echo "Manual baseline update — see docs/ANTI_BLUFF.md"
	@echo "1. Run scanner: bash scripts/anti-bluff/bluff-scanner.sh --mode all"
	@echo "2. Run mutation: bash challenges/scripts/mutation_ratchet_challenge.sh --mode all"
	@echo "3. Edit challenges/baselines/bluff-baseline.txt to reflect new state."

# Run all challenge scripts under challenges/scripts/.
challenge:
	@set -e; \
	for s in challenges/scripts/*.sh; do \
		echo "==> $$s"; \
		bash "$$s" || exit 1; \
	done

# qa-all bundles existing test suite + the challenge scripts.
qa-all: vet test challenge anti-bluff
