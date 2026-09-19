package astcodec

import (
	"errors"
	"fmt"
	goast "go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// nodeTypeNames lists the concrete node types package ast declares, read from
// its source, so a new type that the codec does not know fails a test here.
func nodeTypeNames(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			gd, ok := d.(*goast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*goast.TypeSpec)
				if !ok {
					continue
				}
				if embedsNodeBase(ts) {
					names = append(names, ts.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names
}

func embedsNodeBase(ts *goast.TypeSpec) bool {
	st, ok := ts.Type.(*goast.StructType)
	if !ok {
		return false
	}
	for _, f := range st.Fields.List {
		if id, ok := f.Type.(*goast.Ident); ok && len(f.Names) == 0 && id.Name == "NodeBase" {
			return true
		}
	}
	return false
}

func TestKindsCoverEveryNodeType(t *testing.T) {
	want := nodeTypeNames(t)
	var got []string
	for k := kind(0); k < numKinds; k++ {
		n := alloc(k, 1, nil)[0]
		if got, ok := kindOf(n); !ok || got != k {
			t.Errorf("kindOf(%T) = %v, %v; want %v", n, got, ok, k)
		}
		got = append(got, strings.TrimPrefix(fmt.Sprintf("%T", n), "*ast."))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("codec kinds differ from the node types of package ast\n got: %v\nwant: %v", got, want)
	}
}

// filler builds nodes with every exported field set to a distinct non-zero
// value, so a field the codec forgets shows up as a difference after decoding.
type filler struct {
	counter int
}

func (f *filler) next() int {
	f.counter++
	return f.counter
}

func (f *filler) span() source.Span {
	return source.Span{Offset: f.next(), Len: f.next()}
}

var (
	nodeType    = reflect.TypeFor[ast.Node]()
	triggerType = reflect.TypeFor[ast.TriggerEvent]()
	nodeBase    = reflect.TypeFor[ast.NodeBase]()
)

// fill sets every exported field of v, a struct, recursing into nested nodes
// up to depth so that a chain of references ends.
func (f *filler) fill(v reflect.Value, depth int) {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Type() == nodeBase {
		v.FieldByName("NodeSpan").Set(reflect.ValueOf(f.span()))
		b := v.Addr().Interface().(*ast.NodeBase)
		b.SetLeadingTrivia([]ast.Trivia{{Kind: ast.TriviaKind(f.next()), Span: f.span()}})
		b.SetTrailingTrivia([]ast.Trivia{{Kind: ast.TriviaKind(f.next()), Span: f.span()}, {Kind: ast.TriviaKind(f.next()), Span: f.span()}})
		return
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if !field.IsExported() {
			continue
		}
		f.fillValue(v.Field(i), depth)
	}
}

func (f *filler) fillValue(v reflect.Value, depth int) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(fmt.Sprintf("s%d", f.next()))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int:
		v.SetInt(int64(f.next()))
	case reflect.Struct:
		if v.Type() == reflect.TypeFor[source.Span]() {
			v.Set(reflect.ValueOf(f.span()))
			return
		}
		f.fill(v, depth)
	case reflect.Ptr:
		if depth == 0 {
			return
		}
		n := reflect.New(v.Type().Elem())
		f.fill(n, depth-1)
		v.Set(n)
	case reflect.Interface:
		if depth == 0 {
			return
		}
		var n reflect.Value
		switch v.Type() {
		case nodeType:
			n = reflect.New(reflect.TypeFor[ast.LiteralString]())
		case triggerType:
			n = reflect.New(reflect.TypeFor[ast.TimeEvent]())
		default:
			panic("unexpected interface " + v.Type().String())
		}
		f.fill(n, depth-1)
		v.Set(n)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := 0; i < 2; i++ {
			f.fillValue(s.Index(i), depth)
		}
		v.Set(s)
	case reflect.Array:
	default:
		panic("unexpected kind " + v.Kind().String())
	}
}

func roundTrip(t *testing.T, roots []ast.Node) []ast.Node {
	t.Helper()
	w := pack.NewWriter()
	enc := NewEncoder(w)
	if err := enc.Encode(roots); err != nil {
		t.Fatal(err)
	}
	ids := make([]uint64, len(roots))
	for i, r := range roots {
		id, ok := enc.ID(r)
		if !ok {
			t.Fatalf("root %d not encoded", i)
		}
		ids[i] = id
	}
	r, err := pack.NewReader(w.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(r)
	dec.Decode()
	if err := dec.Err(); err != nil {
		t.Fatal(err)
	}
	if !r.Done() {
		t.Fatal("bytes left after the node table")
	}
	out := make([]ast.Node, len(roots))
	for i, id := range ids {
		n, ok := dec.Node(id)
		if !ok {
			t.Fatalf("root %d: index %d not in the decoded table", i, id)
		}
		out[i] = n
	}
	return out
}

func TestRoundTripEveryNodeType(t *testing.T) {
	f := &filler{}
	var roots []ast.Node
	for k := kind(0); k < numKinds; k++ {
		n := alloc(k, 1, nil)[0]
		f.fill(reflect.ValueOf(n), 2)
		roots = append(roots, n)
	}
	got := roundTrip(t, roots)
	for i, want := range roots {
		if !reflect.DeepEqual(got[i], want) {
			t.Errorf("%T does not round-trip:\n got %+v\nwant %+v", want, got[i], want)
		}
	}
}

func TestRoundTripKeepsSharing(t *testing.T) {
	mult := &ast.Multiplicity{Lower: &ast.LiteralInteger{Value: "1"}}
	a := &ast.Usage{Multiplicity: mult}
	b := &ast.Usage{Multiplicity: mult, Value: a.Multiplicity.Lower}
	got := roundTrip(t, []ast.Node{a, b})
	ga, gb := got[0].(*ast.Usage), got[1].(*ast.Usage)
	if ga.Multiplicity != gb.Multiplicity {
		t.Error("shared Multiplicity decoded as two nodes")
	}
	if ga.Multiplicity.Lower != gb.Value {
		t.Error("node reached through two fields decoded as two nodes")
	}
	if ga.Multiplicity == mult {
		t.Error("decoded node aliases the original")
	}
}

func TestRoundTripNilsAndEmpties(t *testing.T) {
	roots := []ast.Node{
		&ast.RootNamespace{},
		&ast.Usage{Members: []ast.Node{nil}},
		&ast.QualifiedName{Parts: []ast.NameSegment{{Text: "A"}, {Text: "B"}}},
		&ast.TransitionEdge{},
	}
	got := roundTrip(t, roots)
	for i, want := range roots {
		if !reflect.DeepEqual(got[i], want) {
			t.Errorf("%T does not round-trip:\n got %+v\nwant %+v", want, got[i], want)
		}
	}
	if got[3].(*ast.TransitionEdge).Trigger != nil {
		t.Error("nil TriggerEvent decoded non-nil")
	}
}

func TestSingletonQualifiedName(t *testing.T) {
	qn := &ast.QualifiedName{}
	qn.SetSingleton(ast.NameSegment{Text: "A", Span: source.Span{Offset: 3, Len: 1}})
	got := roundTrip(t, []ast.Node{qn})[0].(*ast.QualifiedName)
	if !reflect.DeepEqual(got, qn) {
		t.Errorf("got %+v, want %+v", got, qn)
	}
	if cap(got.Parts) != 1 || &got.Parts[0] == &qn.Parts[0] {
		t.Error("a lone segment is not held in the decoded node's own storage")
	}
}

// unknownNode is a node type package ast does not have, which the codec must
// refuse rather than encode as nil.
type unknownNode struct{ ast.NodeBase }

func TestEncodeRejectsUnknownNodeType(t *testing.T) {
	w := pack.NewWriter()
	enc := NewEncoder(w)
	root := &ast.RootNamespace{Members: []ast.Node{&unknownNode{}}}
	if err := enc.Encode([]ast.Node{root}); !errors.Is(err, ErrUnknownNode) {
		t.Errorf("Encode of an unknown node type: got %v, want ErrUnknownNode", err)
	}
	if _, ok := enc.ID(&ast.Package{}); ok {
		t.Error("ID reports a node Encode never saw")
	}
}

func TestDecodeRejectsCorruptStream(t *testing.T) {
	w := pack.NewWriter()
	if err := NewEncoder(w).Encode([]ast.Node{&ast.Package{Members: []ast.Node{&ast.Usage{}}}}); err != nil {
		t.Fatal(err)
	}
	data := w.Bytes()
	for cut := 1; cut < len(data); cut++ {
		r, err := pack.NewReader(data[:cut])
		if err != nil {
			continue
		}
		dec := NewDecoder(r)
		dec.Decode()
		if dec.Err() == nil {
			t.Errorf("truncated to %d bytes: decoded without error", cut)
		}
	}
}

func TestReachable(t *testing.T) {
	lower := &ast.LiteralInteger{Value: "1"}
	mult := &ast.Multiplicity{Lower: lower}
	a := &ast.Usage{Multiplicity: mult}
	got := Reachable(a)
	for _, n := range []ast.Node{a, mult, lower} {
		if !got[n] {
			t.Errorf("%T not reached", n)
		}
	}
	if len(got) != 3 || got[nil] {
		t.Errorf("reached %d nodes, want 3 without nil", len(got))
	}
}
