package reject

import (
	"path/filepath"
	"strings"
	"testing"
)

// The written files and the headline are announced on the writer the caller
// supplies, so an embedding program captures the whole run.
func TestWriteReportsAnnouncesOnTheSuppliedWriter(t *testing.T) {
	report := &Report{}
	report.summarize()

	var log strings.Builder
	if _, err := writeReports(filepath.Join(t.TempDir(), "out"), report, &log); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"wrote ", "pilot-reject.json", "0 case(s): 0 both reject"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}
