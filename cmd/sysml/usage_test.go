package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// The help states the RDF mapping's status in the same wording a conversion
// reports, wrapped rather than reworded.
func TestPrintUsageStatesTheExperimentalNotice(t *testing.T) {
	var help bytes.Buffer
	printUsage(&help)

	unwrapped := strings.Join(strings.Fields(help.String()), " ")
	if !strings.Contains(unwrapped, convert.ExperimentalNotice) {
		t.Errorf("the help does not state the notice:\n%s", help.String())
	}
	for _, line := range strings.Split(help.String(), "\n") {
		if len(line) > 96 {
			t.Errorf("help line is %d characters wide:\n%s", len(line), line)
		}
	}
}
