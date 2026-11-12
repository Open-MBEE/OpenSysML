package program

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestOriginString(t *testing.T) {
	cases := []struct {
		name   string
		origin Origin
		want   string
	}{
		{"named", Origin{Name: "step", Kind: "*ast.Usage"}, "node step"},
		{"final", Origin{Name: "done", Kind: "*ast.FinalNode"}, "node done"},
		{"unnamed", Origin{Kind: "*ast.IfActionNode"}, "the *ast.IfActionNode"},
		{"vertex", Origin{Kind: "state"}, "the state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.origin.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOriginIsPlainData(t *testing.T) {
	a := Origin{Doc: "m.sysml", Span: source.Span{Offset: 3, Len: 4}, Path: "A::b", Name: "b", Kind: "*ast.Usage"}
	b := a
	if a != b {
		t.Fatal("Origin must be comparable and copy by value")
	}
}

func TestNodeKindString(t *testing.T) {
	for k := NodeNone; k <= NodeStatement; k++ {
		if s := k.String(); s == "" || strings.HasPrefix(s, "NodeKind(") {
			t.Errorf("NodeKind %d has no name", k)
		}
	}
	if got := NodeKind(200).String(); got != "NodeKind(200)" {
		t.Errorf("unknown kind renders %q", got)
	}
	if NodeNone != 0 {
		t.Error("the zero kind must be none")
	}
}

func TestGraphIDZero(t *testing.T) {
	var id GraphID
	if !id.IsZero() {
		t.Error("the zero GraphID must report IsZero")
	}
	id[0] = 1
	if id.IsZero() || id.String() != "01000000000000000000000000000000" {
		t.Errorf("IsZero()=%v String()=%q", id.IsZero(), id)
	}
}

func TestIDsAreGraphLocalIndices(t *testing.T) {
	// Zero is "none" for every id type; the first record of a table is 1.
	if NodeID(0) != 0 || EdgeID(0) != 0 || StmtID(0) != 0 || ExprID(0) != 0 ||
		StateID(0) != 0 || RegionID(0) != 0 || PseudoID(0) != 0 || TransID(0) != 0 {
		t.Fatal("zero ids")
	}
}
