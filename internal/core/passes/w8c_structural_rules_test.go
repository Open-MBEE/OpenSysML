package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// w8cLibraryMessages analyzes src as KerML against the standard library, which
// the rules keyed off library types need.
func w8cLibraryMessages(t *testing.T, src string) []string {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New("<t>.kerml", []byte(src))).ParseFile()
	idx.AddDocument("<t>.kerml", root)
	idx.ExpandWildcardImports()

	var out []string
	for _, d := range Analyze("<t>.kerml", root, nil, idx) {
		out = append(out, d.Message)
	}
	return out
}

func TestW8CReferenceSubsettingAtMostOne(t *testing.T) {
	src := `package P {
	feature a;
	feature b references a;
	feature c references a subsets b;
	feature d references b references c;
}`
	msgs := w8cMessages(t, src)
	if w8cCount(msgs, msgReferenceSubsettingAtMostOne) != 1 {
		t.Errorf("want one %q, got %v", msgReferenceSubsettingAtMostOne, msgs)
	}
}

func TestW8CReferenceSubsettingLegal(t *testing.T) {
	src := `package P {
	feature a;
	feature b references a;
	feature c references a subsets b;
	binding a = b;
}`
	if msgs := w8cMessages(t, src); w8cCount(msgs, msgReferenceSubsettingAtMostOne) != 0 {
		t.Errorf("unexpected %q in %v", msgReferenceSubsettingAtMostOne, msgs)
	}
}

func TestW8CTopLevelImportMustBePrivate(t *testing.T) {
	src := "public import ScalarValues::*;\npackage P {\n\tpublic import ScalarValues::*;\n}"
	var got []diag.Diagnostic
	for _, d := range w8cLibraryDiagnostics(t, "import-public.kerml", src) {
		if d.Message == msgTopLevelImportPrivate {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one %q, got %v", msgTopLevelImportPrivate, got)
	}
	text := strings.TrimSpace(src[got[0].Span.Offset:got[0].Span.End()])
	if text != "public import ScalarValues::*;" {
		t.Errorf("span covers %q", text)
	}
}

func TestW8CTopLevelImportProtectedIsReported(t *testing.T) {
	src := "protected import ScalarValues::*;\n"
	n := w8cCount(w8cLibraryMessagesIn(t, "import-protected.kerml", src), msgTopLevelImportPrivate)
	if n != 1 {
		t.Errorf("want one %q, got %d", msgTopLevelImportPrivate, n)
	}
}

func TestW8CTopLevelImportPrivateIsLegal(t *testing.T) {
	src := "private import ScalarValues::*;\nexpose ScalarValues::*;\npackage P {\n}"
	if w8cCount(w8cLibraryMessagesIn(t, "import-private.kerml", src), msgTopLevelImportPrivate) != 0 {
		t.Errorf("unexpected %q", msgTopLevelImportPrivate)
	}
}

func TestW8CAssociationEndTypes(t *testing.T) {
	src := `package P {
	class C1;
	class C2;
	class C3 specializes C1;
	assoc A1 {
		end x : C1;
		end y : C1, C2;
	}
	assoc A2 specializes A1 {
		end x : C2;
	}
	assoc A3 specializes A1 {
		end x : C3;
	}
}`
	msgs := w8cMessages(t, src)
	if w8cCount(msgs, msgAssociationEndTypes) != 2 {
		t.Errorf("want two %q, got %v", msgAssociationEndTypes, msgs)
	}
}

func TestW8CAssociationEndTypesLegal(t *testing.T) {
	src := `package P {
	class C1;
	assoc A {
		end x;
		end [1..*] feature y : C1;
	}
	assoc B specializes A {
		end x1;
		end [0..*] feature y1 redefines y;
	}
}`
	if msgs := w8cMessages(t, src); w8cCount(msgs, msgAssociationEndTypes) != 0 {
		t.Errorf("unexpected %q in %v", msgAssociationEndTypes, msgs)
	}
}

func TestW8CVariableFeatureOwner(t *testing.T) {
	src := `package P {
	class C {
		var feature x;
	}
	datatype D {
		var feature z;
	}
}`
	// `class` is an occurrence type; `datatype` is not.
	msgs := w8cLibraryMessages(t, src)
	if w8cCount(msgs, msgVariableFeatureOwner) != 1 {
		t.Errorf("want one %q, got %v", msgVariableFeatureOwner, msgs)
	}
}

func TestW8CResultExpressionAtMostOne(t *testing.T) {
	src := `package P {
	function F {
		1
	}
	function G :> F {
		2
	}
	expr f : F {
		1
	}
	expr g :> f;
}`
	msgs := w8cMessages(t, src)
	if got := w8cCount(msgs, msgResultExpressionAtMostOne); got != 3 {
		t.Errorf("want three %q, got %d in %v", msgResultExpressionAtMostOne, got, msgs)
	}
}

func TestW8CResultExpressionLegal(t *testing.T) {
	src := `package P {
	function F {
		1
	}
	function H;
	expr e : H;
}`
	if msgs := w8cMessages(t, src); w8cCount(msgs, msgResultExpressionAtMostOne) != 0 {
		t.Errorf("unexpected %q in %v", msgResultExpressionAtMostOne, msgs)
	}
}

// One result expression reached along two specialization paths is still one.
func TestW8CResultExpressionDiamondIsLegal(t *testing.T) {
	src := `package P {
	function F {
		1
	}
	function G :> F;
	function H :> F;
	function I :> G, H;
}`
	if msgs := w8cMessages(t, src); w8cCount(msgs, msgResultExpressionAtMostOne) != 0 {
		t.Errorf("unexpected %q in %v", msgResultExpressionAtMostOne, msgs)
	}
}
