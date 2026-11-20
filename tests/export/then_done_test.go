package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// `then done;` is written only for the `done` the succession's owner inherits; a library `done` of
// another kind, which that notation would not reach, is written back by name.
func TestThenDoneIsWrittenOnlyForTheInheritedDone(t *testing.T) {
	src := "package P {\n" +
		"    action def A {\n        action a;\n        first start then a;\n        then States::StateAction::done;\n    }\n" +
		"    action def B {\n        action a;\n        first start then a;\n        then done;\n    }\n" +
		"    use case def UC {\n        first start;\n        then done;\n    }\n" +
		"    use case def UD {\n        first start;\n        then Actions::Action::done;\n    }\n" +
		"    action def M {\n        action a;\n        then done;\n        action b;\n        then States::StateAction::done;\n    }\n" +
		"}\n"
	graph, err := convert.Convert("done.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if got := string(structuralRoundTrip(t, "done", graph)); strings.TrimSpace(got) != strings.TrimSpace(src) {
		t.Errorf("the graph alone wrote\n%s\nwant\n%s", got, src)
	}
}
