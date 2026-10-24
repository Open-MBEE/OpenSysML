package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An object declared in one document and read from the prompt scope of
// another is the one object: a behavior performed on it writes what %eval
// reads, rather than %eval materializing a second object of the usage.
func TestEvalReadsObjectDeclaredInAnotherDocument(t *testing.T) {
	s := NewSession()
	res := s.SubmitFiles([]SourceFile{
		{Name: "defs.sysml", Text: "package M {\n    part def P {\n        attribute runs : ScalarValues::Integer default = 0;\n        action fill {\n            assign runs := 1;\n        }\n    }\n}\n", Kind: source.KindSysML},
		{Name: "parts.sysml", Text: "part p : M::P;\n", Kind: source.KindSysML},
	})
	if res.Refused != nil {
		t.Fatalf("submit: %v", res.Refused)
	}
	for _, line := range []string{"%instantiate p", "%action M::P::fill p", "%continue"} {
		if _, _, err := s.RunMeta(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}
	out, _, err := s.RunMeta("%eval p.runs")
	if err != nil {
		t.Fatalf("%%eval p.runs: %v", err)
	}
	if got := strings.Join(out, "\n"); !strings.HasSuffix(got, "= 1") {
		t.Errorf("%%eval p.runs after fill = %q, want the written 1", got)
	}
}
