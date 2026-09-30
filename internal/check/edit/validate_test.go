package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// An edit is judged under the parse's warnings as every reader is, so what the
// notation passes make of the original's notation is not laid at the edit's
// door: an import without a visibility indicator the original already had is
// not an error the edit introduced.
func TestValidateJudgesTheEditUnderTheOriginalsParseWarnings(t *testing.T) {
	src := "package P {\n" +
		"    package Lib {\n" +
		"        part def Base;\n" +
		"    }\n" +
		"    package Client {\n" +
		"        import Lib::Base;\n" +
		"        part b : Base;\n" +
		"    }\n" +
		"}\n"
	m := loadContent(t, "validate.sysml", src)
	var visibility []diag.Diagnostic
	for _, d := range m.SemDiags {
		if d.Code == "import-visibility" && d.Severity == diag.SeverityError {
			visibility = append(visibility, d)
		}
	}
	if len(visibility) != 1 {
		t.Fatalf("original reports %d import-visibility errors, want 1: %v", len(visibility), m.SemDiags)
	}
	res := applyOne(t, m, AddMember("P::Client", "part def", "Extra"))
	if got := string(res.Content); !strings.Contains(got, "part def Extra;") || !strings.Contains(got, "        import Lib::Base;\n") {
		t.Fatalf("edited source:\n%s", got)
	}
}
