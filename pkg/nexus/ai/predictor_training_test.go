package ai

import (
	"strings"
	"testing"
)

const sampleCSV = `test_id,platform,pass,duration_s,retries,hour_of_day,runner_rss_bytes
t1,web,true,1.2,0,10,1073741824
t2,web,false,45.0,3,2,8589934592
t3,android,true,2.5,0,14,2147483648
t4,android,false,60.1,5,23,12884901888
`

func TestPredictor_TrainFromCSV_ObservesEveryRow(t *testing.T) {
	p := NewPredictor()
	n, skipped, err := p.TrainFromCSV(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("observed = %d, want 4", n)
	}
	if len(skipped) != 0 {
		t.Errorf("unexpected skipped rows: %+v", skipped)
	}
	if got := len(p.History()); got != 4 {
		t.Errorf("history len = %d, want 4", got)
	}
}

func TestPredictor_TrainFromCSV_EmptyInput(t *testing.T) {
	p := NewPredictor()
	n, _, err := p.TrainFromCSV(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("empty CSV should produce 0 samples, got %d", n)
	}
}

func TestPredictor_TrainFromCSV_MissingPassColumn(t *testing.T) {
	csv := "test_id,platform\nt1,web\n"
	p := NewPredictor()
	if _, _, err := p.TrainFromCSV(strings.NewReader(csv)); err == nil {
		t.Fatal("missing 'pass' column must error")
	}
}

func TestPredictor_TrainFromCSV_TolerantToBadNumerics(t *testing.T) {
	csv := `test_id,platform,pass,duration_s,retries
t1,web,true,not-a-float,0
`
	p := NewPredictor()
	_, skipped, err := p.TrainFromCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if skipped["bad_duration"] != 1 {
		t.Errorf("bad duration should be tallied, got %+v", skipped)
	}
}

func TestPredictor_AUC_SeparatesClasses(t *testing.T) {
	p := NewPredictor()
	// Obvious positives (Retries high, late hour, big RSS) vs clean
	// negatives. The default weights already rank these correctly so
	// AUC should be comfortably above 0.75.
	samples := []FlakeSample{
		{Retries: 8, DurationS: 200, HourOfDay: 3, RunnerRSS: 8 * 1 << 30, Pass: false},
		{Retries: 7, DurationS: 180, HourOfDay: 2, RunnerRSS: 8 * 1 << 30, Pass: false},
		{Retries: 0, DurationS: 1, HourOfDay: 12, Pass: true},
		{Retries: 0, DurationS: 1, HourOfDay: 13, Pass: true},
	}
	auc := p.AUC(samples)
	if auc < 0.75 {
		t.Errorf("AUC = %f, want >= 0.75", auc)
	}
}

func TestPredictor_AUC_EmptyInput(t *testing.T) {
	p := NewPredictor()
	if got := p.AUC(nil); got != 0 {
		t.Errorf("AUC(nil) = %f, want 0", got)
	}
}

func TestPredictor_AUC_SingleClass(t *testing.T) {
	p := NewPredictor()
	samples := []FlakeSample{
		{Retries: 0, Pass: true},
		{Retries: 0, Pass: true},
	}
	got := p.AUC(samples)
	if !isNaN(got) {
		t.Errorf("AUC with single class should be NaN, got %f", got)
	}
}

func isNaN(v float64) bool { return v != v }
