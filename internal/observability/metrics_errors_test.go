package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

var errMeterInjection = errors.New("meter instrument creation failed")

type failingMeter struct {
	noop.Meter
	failAt int
	call   int
}

func (m *failingMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	m.call++
	if m.call == m.failAt {
		return nil, errMeterInjection
	}
	return noop.Meter{}.Float64Histogram("", nil)
}

func (m *failingMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	m.call++
	if m.call == m.failAt {
		return nil, errMeterInjection
	}
	return noop.Meter{}.Int64Counter("", nil)
}

func (m *failingMeter) Float64Counter(string, ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	m.call++
	if m.call == m.failAt {
		return nil, errMeterInjection
	}
	return noop.Meter{}.Float64Counter("", nil)
}

func (m *failingMeter) Float64Gauge(string, ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	m.call++
	if m.call == m.failAt {
		return nil, errMeterInjection
	}
	return noop.Meter{}.Float64Gauge("", nil)
}

func (m *failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	m.call++
	if m.call == m.failAt {
		return nil, errMeterInjection
	}
	return noop.Meter{}.Int64Gauge("", nil)
}

func TestInitMetrics_ErrorOnFirstInstrument(t *testing.T) {
	m := &failingMeter{failAt: 1}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnSecondInstrument(t *testing.T) {
	m := &failingMeter{failAt: 2}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnThirdInstrument(t *testing.T) {
	m := &failingMeter{failAt: 3}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnFourthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 4}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnFifthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 5}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnSixthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 6}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnSeventhInstrument(t *testing.T) {
	m := &failingMeter{failAt: 7}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnEighthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 8}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnNinthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 9}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnTenthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 10}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnEleventhInstrument(t *testing.T) {
	m := &failingMeter{failAt: 11}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnTwelfthInstrument(t *testing.T) {
	m := &failingMeter{failAt: 12}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitMetrics_ErrorOnLastInstrument(t *testing.T) {
	m := &failingMeter{failAt: 13}
	err := InitMetrics(m)
	if err == nil {
		t.Fatal("expected error from InitMetrics")
	}
	if !errors.Is(err, errMeterInjection) {
		t.Errorf("got %v, want errMeterInjection", err)
	}
}

func TestInitOTel_EmptyConfig(t *testing.T) {
	logger := zap.NewNop()
	cfg := OTelConfig{}

	shutdown, err := InitOTel(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("InitOTel with empty config failed: %v", err)
	}
	defer shutdown()
}

func TestOTelConfig_ZeroValue(t *testing.T) {
	var cfg OTelConfig
	if cfg.ServiceName != "" {
		t.Errorf("zero OTelConfig.ServiceName = %q, want empty", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "" {
		t.Errorf("zero OTelConfig.ServiceVersion = %q, want empty", cfg.ServiceVersion)
	}
	if cfg.Environment != "" {
		t.Errorf("zero OTelConfig.Environment = %q, want empty", cfg.Environment)
	}
}
