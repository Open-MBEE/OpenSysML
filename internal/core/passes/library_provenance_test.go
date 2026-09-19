package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// constraintDiagsOverLibrary analyses src in an index where libSrc is marked as
// library content, which is how a bundled definition reaches a model.
func constraintDiagsOverLibrary(t *testing.T, libSrc, src string) []diag.Diagnostic {
	t.Helper()
	idx := newTestIndex()
	idx.AddDocument("frame.sysml", parser.New(source.New("frame.sysml", []byte(libSrc))).ParseFile())
	idx.MarkLibrary("frame.sysml")
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()

	var out []diag.Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == "constraint" {
			out = append(out, d)
		}
	}
	return out
}

// A library objective is the frame a case objective redefines, so a case owning
// one objective is silent even though it inherits the library's.
func TestW7GLibraryObjectiveIsNotACompetingObjective(t *testing.T) {
	const frame = `package Frame {
		case def FramedCase {
			objective frameObj;
		}
	}`
	diags := only(constraintDiagsOverLibrary(t, frame, `package C {
		case def Analysis :> Frame::FramedCase {
			objective own;
		}
	}`), "only-one-objective")
	if len(diags) != 0 {
		t.Fatalf("a library objective competed with the model's own, got %v", diags)
	}
}

// Where a case owns no objective, provenance decides which inherited objectives
// compete: two library ones are the frame, two of the model's own are a defect.
func TestW7GInheritedObjectivesCompeteByProvenance(t *testing.T) {
	const frame = `package Frame {
		case def FramedCase {
			objective frameObj;
		}
		case def AlsoFramed :> FramedCase {
			objective otherFrameObj;
		}
	}`
	if diags := only(constraintDiagsOverLibrary(t, frame, `package C {
		case def Analysis :> Frame::AlsoFramed;
	}`), "only-one-objective"); len(diags) != 0 {
		t.Fatalf("library objectives competed with each other, got %v", diags)
	}
	diags := only(constraintDiagsOverLibrary(t, frame, `package C {
		case def Base {
			objective inheritedObj;
			objective anotherObj;
		}
		case def Analysis :> Base;
	}`), "only-one-objective")
	if len(diags) != 2 {
		t.Fatalf("expected the owned excess and the inherited conflict, got %v", diags)
	}
}
