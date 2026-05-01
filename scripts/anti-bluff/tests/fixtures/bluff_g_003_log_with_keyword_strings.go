package fixtures
import "testing"

// Regression fixture: prior to scanner iter 13 string-literal stripping,
// the `assert.` substring inside the log message would falsely cause this
// test to be counted as having a real assertion, masking the BLUFF-G-003
// hit. The fix strips string literals before pattern matching, so this
// fixture must now correctly produce a BLUFF-G-003 hit.
func TestBluffG003LogWithAssertKeywordInString(t *testing.T) {
	t.Logf("describes assert.something but does not actually assert")
	t.Logf("also mentions t.Fatal in passing without calling it")
}
