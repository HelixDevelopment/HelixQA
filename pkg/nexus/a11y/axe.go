package a11y

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Impact names an axe-core severity level.
type Impact string

const (
	ImpactMinor    Impact = "minor"
	ImpactModerate Impact = "moderate"
	ImpactSerious  Impact = "serious"
	ImpactCritical Impact = "critical"
)

// Violation is a single axe-core rule failure.
type Violation struct {
	ID          string   `json:"id"`
	Impact      Impact   `json:"impact"`
	Description string   `json:"description"`
	Help        string   `json:"help"`
	HelpURL     string   `json:"helpUrl"`
	Tags        []string `json:"tags"`
	Nodes       []Node   `json:"nodes"`
}

// Node pinpoints where a violation occurs.
type Node struct {
	Target []string `json:"target"`
	HTML   string   `json:"html"`
	Impact Impact   `json:"impact"`
}

// Report is the parsed axe-core output.
type Report struct {
	URL        string      `json:"url,omitempty"`
	Violations []Violation `json:"violations"`
	Passes     []string    `json:"passes"`
	Incomplete []string    `json:"incomplete"`
}

// Level names a compliance bar.
type Level string

const (
	LevelA   Level = "A"
	LevelAA  Level = "AA"
	LevelAAA Level = "AAA"
)

// Parse consumes axe-core JSON output and returns a typed Report.
func Parse(raw []byte) (*Report, error) {
	if len(raw) == 0 {
		return nil, errors.New("a11y: empty axe output")
	}
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("a11y: parse axe: %w", err)
	}
	return &r, nil
}

// Assert returns a non-nil error when the report breaches level. The
// mapping matches the axe-core rule tags:
//   - level A   -> any critical
//   - level AA  -> any critical or serious
//   - level AAA -> any critical, serious, or moderate
// Minor issues are logged but do not fail the assertion at any level.
func (r *Report) Assert(level Level) error {
	if r == nil {
		return errors.New("a11y: nil report")
	}
	breach := []Violation{}
	for _, v := range r.Violations {
		switch level {
		case LevelA:
			if v.Impact == ImpactCritical {
				breach = append(breach, v)
			}
		case LevelAA:
			if v.Impact == ImpactCritical || v.Impact == ImpactSerious {
				breach = append(breach, v)
			}
		case LevelAAA:
			if v.Impact == ImpactCritical || v.Impact == ImpactSerious || v.Impact == ImpactModerate {
				breach = append(breach, v)
			}
		default:
			return fmt.Errorf("a11y: unknown level %q", level)
		}
	}
	if len(breach) == 0 {
		return nil
	}
	sort.SliceStable(breach, func(i, j int) bool {
		return breach[i].Impact > breach[j].Impact || (breach[i].Impact == breach[j].Impact && breach[i].ID < breach[j].ID)
	})
	return fmt.Errorf("a11y: %s compliance failed (%d violations): %s",
		level, len(breach), summarise(breach))
}

// Section508 returns the subset of violations tagged as Section 508.
func (r *Report) Section508() []Violation {
	if r == nil {
		return nil
	}
	out := []Violation{}
	for _, v := range r.Violations {
		for _, t := range v.Tags {
			if strings.EqualFold(t, "section508") {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// Summary reports the count of violations by severity.
func (r *Report) Summary() map[Impact]int {
	out := map[Impact]int{ImpactMinor: 0, ImpactModerate: 0, ImpactSerious: 0, ImpactCritical: 0}
	if r == nil {
		return out
	}
	for _, v := range r.Violations {
		out[v.Impact]++
	}
	return out
}

func summarise(vs []Violation) string {
	names := make([]string, 0, len(vs))
	for _, v := range vs {
		names = append(names, fmt.Sprintf("%s/%s", v.Impact, v.ID))
		if len(names) >= 10 {
			names = append(names, "...")
			break
		}
	}
	return strings.Join(names, ", ")
}

// InjectionScript returns the JavaScript the browser Engine should
// evaluate to load axe-core from a vendored location (not a CDN) and
// run it against the current document.
//
// Operators ship axe-core at docs/nexus/vendor/axe.min.js and mount it
// at the configured assetBase URL. The script races a load-timeout so
// broken deployments surface a clear error instead of hanging.
func InjectionScript(assetBase string) string {
	return fmt.Sprintf(`(async () => {
  if (typeof axe === 'undefined') {
    await new Promise((resolve, reject) => {
      const s = document.createElement('script');
      s.src = %q;
      s.onload = resolve;
      s.onerror = () => reject(new Error('axe-core load failed'));
      document.head.appendChild(s);
      setTimeout(() => reject(new Error('axe-core load timeout')), 5000);
    });
  }
  return JSON.stringify(await axe.run());
})()`, assetBase+"/axe.min.js")
}
