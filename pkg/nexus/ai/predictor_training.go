package ai

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// TrainFromCSV trains the Predictor from a CSV stream. Expected columns:
//
//   test_id,platform,pass,duration_s,retries,hour_of_day,runner_rss_bytes
//
// Rows with malformed numeric fields are skipped. Returns the
// observed sample count and a map tracking skipped rows per reason so
// operators can fix their training data without re-running the
// import.
func (p *Predictor) TrainFromCSV(r io.Reader) (int, map[string]int, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1 // tolerate header row with extra whitespace
	headers, err := cr.Read()
	if err == io.EOF {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, fmt.Errorf("predictor train: read header: %w", err)
	}
	col := indexHeaders(headers)
	if _, ok := col["pass"]; !ok {
		return 0, nil, fmt.Errorf("predictor train: missing 'pass' column")
	}

	observed := 0
	skipped := map[string]int{}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			skipped["parse_error"]++
			continue
		}
		sample := FlakeSample{
			TestID:   fieldAt(rec, col, "test_id"),
			Platform: fieldAt(rec, col, "platform"),
		}
		rawPass := strings.ToLower(fieldAt(rec, col, "pass"))
		sample.Pass = rawPass == "true" || rawPass == "1" || rawPass == "yes"

		if n, ok := parseFloat(fieldAt(rec, col, "duration_s")); ok {
			sample.DurationS = n
		} else {
			skipped["bad_duration"]++
		}
		if n, ok := parseInt(fieldAt(rec, col, "retries")); ok {
			sample.Retries = int(n)
		}
		if n, ok := parseInt(fieldAt(rec, col, "hour_of_day")); ok && n >= 0 && n < 24 {
			sample.HourOfDay = int(n)
		}
		if n, ok := parseInt(fieldAt(rec, col, "runner_rss_bytes")); ok {
			sample.RunnerRSS = int(n)
		}
		p.Observe(sample)
		observed++
	}
	return observed, skipped, nil
}

// AUC computes the area under the ROC curve for the supplied holdout
// set using the current Predictor weights. Callers typically run this
// after Train to validate that the model generalises. Values above 0.75
// meet the success criterion from the gap report.
func (p *Predictor) AUC(holdout []FlakeSample) float64 {
	if len(holdout) == 0 {
		return 0
	}
	positives, negatives := 0, 0
	var sumRanks float64
	scores := make([]aucSample, 0, len(holdout))
	for _, s := range holdout {
		scores = append(scores, aucSample{prob: p.Probability(s), pos: !s.Pass})
	}
	sortAsc(scores)
	for i, sc := range scores {
		rank := float64(i + 1)
		if sc.pos {
			sumRanks += rank
			positives++
		} else {
			negatives++
		}
	}
	if positives == 0 || negatives == 0 {
		return math.NaN()
	}
	return (sumRanks - float64(positives*(positives+1))/2) / float64(positives*negatives)
}

// aucSample is the package-scope type used by AUC's internal sort so
// sortAsc can accept a concrete []aucSample instead of an anonymous
// struct literal (anonymous types are not assignable across function
// boundaries in Go's type identity rules).
type aucSample struct {
	prob float64
	pos  bool
}

func sortAsc(s []aucSample) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].prob < s[j-1].prob; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func indexHeaders(h []string) map[string]int {
	out := map[string]int{}
	for i, raw := range h {
		name := strings.ToLower(strings.TrimSpace(raw))
		out[name] = i
	}
	return out
}

func fieldAt(rec []string, col map[string]int, name string) string {
	idx, ok := col[name]
	if !ok || idx >= len(rec) {
		return ""
	}
	return rec[idx]
}

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func parseInt(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	return v, err == nil
}
