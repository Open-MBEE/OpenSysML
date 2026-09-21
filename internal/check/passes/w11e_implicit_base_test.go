package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// implicitBaseFindings returns "line: message" for each diagnostic of code that
// src draws, in order.
func implicitBaseFindings(t *testing.T, name, src, code string) []string {
	t.Helper()
	var out []string
	for _, d := range only(w8cLibraryDiagnostics(t, name, src), code) {
		out = append(out, fmt.Sprintf("%d: %s", strings.Count(src[:d.Span.Offset], "\n")+1, d.Message))
	}
	return out
}

func TestW11ESysMLMissingLibraryBase(t *testing.T) {
	const name = "<t>.sysml"
	const src = `part def P;`
	idx := symbols.NewIndex()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx.AddDocument(name, root)
	got := only(ImplicitBasePass{}.Run(NewContextWithKind(name, source.KindSysML, idx, nil), name, root), "classifier-default-supertype")
	if len(got) != 1 || got[0].Message != "Must directly or indirectly specialize Parts::Part" {
		t.Fatalf("classifier-default-supertype = %+v, want missing Parts::Part diagnostic", got)
	}
}

// A conjugated classifier owns no specialization, so it reaches its kind's
// default supertype only through the type it conjugates
// (validateClassifierDefaultSupertype); a two-end association is judged
// against the binary base, which implies the generic one.
func TestW11EConjugatedClassifierDefaultSupertype(t *testing.T) {
	const src = `package P {
		class D;
		behavior B;
		struct S ~ D;
		function F ~ B;
		assoc A ~ B { end feature a : D; end feature b : D; }
		struct S2 ~ Objects::Object;
		class C ~ D;
		assoc A2 ~ Links::BinaryLink { end feature a : D; end feature b : D; }
	}`
	want := []string{
		"4: Must directly or indirectly specialize Objects::Object",
		"5: Must directly or indirectly specialize Performances::Evaluation",
		"6: Must directly or indirectly specialize Links::BinaryLink",
	}
	got := implicitBaseFindings(t, "<t>.kerml", src, "classifier-default-supertype")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("classifier-default-supertype = %q, want %q", got, want)
	}
}

// A conjugated feature loses the implicit subsetting that would type it, so it
// is typed only by the types of what it conjugates (validateFeatureHasType).
func TestW11EConjugatedFeatureHasType(t *testing.T) {
	const src = `package P {
		class D;
		feature f ~ D;
		class C { feature g ~ D; }
		feature f1 : D;
		feature f2 ~ f1;
		feature f3;
		step s ~ D;
	}`
	want := []string{
		"3: " + msgFeatureNoType,
		"4: " + msgFeatureNoType,
		"8: " + msgFeatureNoType,
	}
	got := implicitBaseFindings(t, "<t>.kerml", src, "feature-has-type")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("feature-has-type = %q, want %q", got, want)
	}
}
