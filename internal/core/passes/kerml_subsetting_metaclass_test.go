package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Every KerML feature declaration subsets a Feature: a `bool`, an `expr` or a
// `step` naming a data type or a class is the error the pilot reports for
// `feature f :> D` (Subsetting::subsettedFeature is typed Feature), while a
// `predicate` naming one is a classifier specialization judged by the family
// rules. Typing a feature by a classifier is fine.
func TestKerMLSubsettingMetaclassCoversEveryFeatureKeyword(t *testing.T) {
	src := `package P {
	datatype D;
	class C;
	struct S;
	behavior B;
	predicate Pr;
	bool b1 :> D;
	bool b2 :> C;
	bool b3 :> S;
	bool b4 :> B;
	bool b5 : D;
	bool b6 : C;
	bool b7 : Pr;
	predicate p1 :> D;
	expr e1 :> D;
	expr e2 : D;
	step s1 :> D;
	step s2 : D;
	feature f1 :> D;
	feature f2 : D;
	feature g;
	bool b8 :> g;
}`
	var got []string
	for _, d := range diagsIn(t, "a.kerml", src, "type") {
		pos := source.New("a.kerml", []byte(src)).Lines().PosAt(d.Span.Offset)
		got = append(got, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Col, d.Message))
	}
	want := []string{
		"7:13: subsets target must be a feature, found attributeDef",
		"8:13: subsets target must be a feature, found kermlType",
		"9:13: subsets target must be a feature, found kermlType",
		"10:13: subsets target must be a feature, found kermlType",
		"14:18: Cannot specialize data type or association",
		"15:13: subsets target must be a feature, found attributeDef",
		"17:13: subsets target must be a feature, found attributeDef",
		"19:16: subsets target must be a feature, found attributeDef",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
