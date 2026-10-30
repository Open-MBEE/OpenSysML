package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The document counts its `typed by` spellings (RootNamespace.TypedBy), on
// declarations, prefix metadata and body-expression parameters alike, so a
// pass that reports the spelling knows when it has found them all.
func TestRootNamespaceCountsTypedBy(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"none", "package P { part def A; part a : A; }", 0},
		{"typed_as_name", "package P { attribute typed; }", 0},
		{"declaration", "package P { part def A; part a typed by A; }", 1},
		{"two_declarations", "package P { part def A; part a typed by A; part b typed by A, A; }", 2},
		{"prefix_metadata", "package P { metadata def M; @ m typed by M; }", 1},
		{"body_parameter", "package P { part def A; calc c { in x; return y = x.{ in v typed by A; v }; } }", 1},
		{"kerml_feature", "package P { classifier A; feature f typed by A; }", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			root := p.ParseFile()
			if root.TypedBy != tt.want {
				t.Errorf("RootNamespace.TypedBy = %d, want %d", root.TypedBy, tt.want)
			}
		})
	}
}
