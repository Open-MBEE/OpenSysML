package libs

import (
	"testing"
)

func TestTransitionPerformancesContext(t *testing.T) {
	es := &embedSource{}
	data, err := es.Read("Kernel Libraries/Kernel Semantic Library/TransitionPerformances.kerml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// Print context around offset 1371
	start := 1320
	end := 1450
	if end > len(data) {
		end = len(data)
	}

	t.Logf("Context [%d:%d]:\n%s", start, end, string(data[start:end]))
}
