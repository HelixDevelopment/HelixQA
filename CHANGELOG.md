# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-18

### Added

- **Orchestrator**: Composable QA brain reusing Challenges runner and bank loader
- **Detector**: Real-time crash/ANR detection for Android (ADB), Web (pgrep), Desktop (pgrep/kill)
- **Validator**: Step-by-step validation with evidence collection (screenshots, logs)
- **Reporter**: Evidence-based QA reports in Markdown, HTML, and JSON formats
- **CLI**: `helixqa` command with platform, device, speed, output, and report flags
- **Configuration**: Type-safe config with validation, platform expansion, speed modes
- Comprehensive test suite with 150+ tests across all packages
- CLAUDE.md, AGENTS.md, CONTRIBUTING.md, and README.md documentation
- Makefile with build, test, lint, and coverage targets
