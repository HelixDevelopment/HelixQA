package fixtures
import "testing"
func TestBluffG003OnlyLogs(t *testing.T) {
	t.Logf("this test only logs and never checks the result")
	t.Logf("more logging without any kind of result-verification")
}
