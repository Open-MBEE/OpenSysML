package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// The help states the RDF mapping's status in the same wording a conversion
// reports, wrapped rather than reworded.
func TestPrintUsageStatesTheExperimentalNotice(t *testing.T) {
	var help bytes.Buffer
	printUsage(&help, docFlags())

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

// Every flag is listed under exactly one heading, so a flag added without a
// place in the help is caught here rather than by a reader who cannot find it.
func TestEveryFlagIsInOneOptionGroup(t *testing.T) {
	// A left-out feature's flags are hidden stand-ins, which no group documents.
	declared := flag.NewFlagSet("sysml", flag.ContinueOnError)
	docFlags().VisitAll(func(f *flag.Flag) {
		if _, standIn := f.Value.(*omittedFlag); !standIn {
			declared.Var(f.Value, f.Name, f.Usage)
		}
	})
	if err := doc().CheckOptions(declared); err != nil {
		t.Error(err)
	}
}
