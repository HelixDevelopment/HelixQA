// Package observability provides the OpenTelemetry + metrics surface
// that Helix Nexus emits. The package keeps the OTel dependency at
// arm's length: concrete spans and metrics are exposed through narrow
// interfaces so consumers (any HTTP service, CLI runner, or
// dashboard) can swap the real OTel exporter for a no-op tracer
// during tests.
package observability
