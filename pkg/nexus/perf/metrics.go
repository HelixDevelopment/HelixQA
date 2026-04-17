package perf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Metrics is the unified Core Web Vitals envelope. Values are in ms
// except CLS which is unitless.
type Metrics struct {
	LCP  float64 `json:"lcp"`
	INP  float64 `json:"inp"`
	CLS  float64 `json:"cls"`
	FCP  float64 `json:"fcp"`
	TTFB float64 `json:"ttfb"`
}

// Thresholds lists the fail bars for each metric; zero means no cap.
type Thresholds struct {
	LCPMax  float64 `json:"lcp_max"`
	INPMax  float64 `json:"inp_max"`
	CLSMax  float64 `json:"cls_max"`
	FCPMax  float64 `json:"fcp_max"`
	TTFBMax float64 `json:"ttfb_max"`
}

// DefaultThresholds returns the Core Web Vitals "good" bar.
func DefaultThresholds() Thresholds {
	return Thresholds{
		LCPMax:  2500,
		INPMax:  200,
		CLSMax:  0.1,
		FCPMax:  1800,
		TTFBMax: 800,
	}
}

// Assert fails the test if any metric exceeds the supplied threshold.
// Zero thresholds are ignored so callers can relax individual metrics
// without copying the whole struct.
func (m Metrics) Assert(t Thresholds) error {
	fails := []string{}
	if t.LCPMax > 0 && m.LCP > t.LCPMax {
		fails = append(fails, fmt.Sprintf("LCP %.1fms > %.1fms", m.LCP, t.LCPMax))
	}
	if t.INPMax > 0 && m.INP > t.INPMax {
		fails = append(fails, fmt.Sprintf("INP %.1fms > %.1fms", m.INP, t.INPMax))
	}
	if t.CLSMax > 0 && m.CLS > t.CLSMax {
		fails = append(fails, fmt.Sprintf("CLS %.3f > %.3f", m.CLS, t.CLSMax))
	}
	if t.FCPMax > 0 && m.FCP > t.FCPMax {
		fails = append(fails, fmt.Sprintf("FCP %.1fms > %.1fms", m.FCP, t.FCPMax))
	}
	if t.TTFBMax > 0 && m.TTFB > t.TTFBMax {
		fails = append(fails, fmt.Sprintf("TTFB %.1fms > %.1fms", m.TTFB, t.TTFBMax))
	}
	if len(fails) > 0 {
		return fmt.Errorf("perf: threshold breach: %s", strings.Join(fails, "; "))
	}
	return nil
}

// ParseK6JSON reads a single JSON object or a newline-delimited stream
// produced by `k6 run --out json=...` and extracts the browser web
// vitals channel.
func ParseK6JSON(raw []byte) (*Metrics, error) {
	if len(raw) == 0 {
		return nil, errors.New("perf: empty k6 output")
	}
	out := &Metrics{}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	for dec.More() {
		var rec k6Record
		if err := dec.Decode(&rec); err != nil {
			// k6 streams mix Points, Metrics, and Summary. Ignore decode
			// errors on records we do not care about.
			continue
		}
		if rec.Type != "Point" {
			continue
		}
		switch rec.Metric {
		case "browser_web_vital_lcp":
			out.LCP = rec.Data.Value
		case "browser_web_vital_inp":
			out.INP = rec.Data.Value
		case "browser_web_vital_cls":
			out.CLS = rec.Data.Value
		case "browser_web_vital_fcp":
			out.FCP = rec.Data.Value
		case "browser_web_vital_ttfb":
			out.TTFB = rec.Data.Value
		}
	}
	return out, nil
}

type k6Record struct {
	Type   string `json:"type"`
	Metric string `json:"metric"`
	Data   struct {
		Value float64 `json:"value"`
	} `json:"data"`
}
