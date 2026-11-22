package astcodec

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
)

// everyNodeType is one filled node of each type, nested two levels deep.
func everyNodeType() []ast.Node {
	f := &filler{}
	var roots []ast.Node
	for k := kind(0); k < numKinds; k++ {
		n := alloc(k, 1, nil)[0]
		f.fill(reflect.ValueOf(n), 2)
		roots = append(roots, n)
	}
	return roots
}

// TestReachableKeepsNothingItWalks pins that the walk which only gathers nodes
// writes into a discarding stream.
func TestReachableKeepsNothingItWalks(t *testing.T) {
	roots := everyNodeType()
	enc := &Encoder{w: pack.NewDiscard(), ids: make(map[ast.Node]uint64), collecting: true}
	for _, root := range roots {
		enc.node(root)
	}
	if got, empty := enc.w.Bytes(), pack.NewDiscard().Bytes(); len(got) != len(empty) {
		t.Fatalf("the collecting walk kept %d bytes, want the %d of an empty stream", len(got), len(empty))
	}
	if len(enc.ids) < len(roots) {
		t.Fatalf("the collecting walk reached %d nodes of %d roots", len(enc.ids), len(roots))
	}
}

func BenchmarkEncode(b *testing.B) {
	roots := everyNodeType()
	b.ReportAllocs()
	for b.Loop() {
		w := pack.NewWriter()
		if err := NewEncoder(w).Encode(roots); err != nil {
			b.Fatal(err)
		}
		_ = w.Bytes()
	}
}

func BenchmarkReachable(b *testing.B) {
	roots := everyNodeType()
	b.ReportAllocs()
	for b.Loop() {
		_ = Reachable(roots...)
	}
}
