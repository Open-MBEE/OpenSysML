// Package astcodec writes syntax trees to a pack stream and reads them back.
// Nodes form a table, grouped by type, and refer to each other by table index.
package astcodec

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ErrUnknownNode reports a node of a type the codec does not know, which it
// refuses to encode rather than drop.
var ErrUnknownNode = errors.New("astcodec: node of a type the codec does not know")

// Encoder writes one node table.
type Encoder struct {
	w          *pack.Writer
	ids        map[ast.Node]uint64
	byKind     [numKinds][]ast.Node
	collecting bool
	last       int // offset of the last span written, for delta coding
	err        error
}

// NewEncoder returns an Encoder writing to w.
func NewEncoder(w *pack.Writer) *Encoder {
	return &Encoder{w: w, ids: make(map[ast.Node]uint64)}
}

// Encode writes the table of every node reachable from roots: the count of
// each type, then a section holding the nodes' fields.
func (e *Encoder) Encode(roots []ast.Node) error {
	body := e.w
	e.w, e.collecting = pack.NewWriter(), true
	for _, root := range roots {
		e.node(root)
	}
	e.w, e.collecting, e.last = body, false, 0

	id := uint64(1)
	for _, nodes := range e.byKind {
		for _, n := range nodes {
			e.ids[n] = id
			id++
		}
	}
	e.w.Len(int(numKinds))
	for _, nodes := range e.byKind {
		e.w.Len(len(nodes))
	}
	body.Section(func(w *pack.Writer) {
		e.w = w
		for _, nodes := range e.byKind {
			for _, n := range nodes {
				e.encodeFields(n)
			}
		}
	})
	e.w = body
	return e.err
}

// ID returns the table index of an encoded node; nil is 0. It reports false
// for a node Encode did not reach.
func (e *Encoder) ID(n ast.Node) (uint64, bool) {
	if _, ok := kindOf(n); !ok {
		return 0, true
	}
	id, ok := e.ids[n]
	return id, ok
}

// node writes a reference to n, or while collecting, records n and walks it.
func (e *Encoder) node(n ast.Node) {
	k, ok := kindOf(n)
	if !ok {
		if k == numKinds && n != nil && e.err == nil {
			e.err = fmt.Errorf("%w: %T", ErrUnknownNode, n)
		}
		e.w.Uint(0)
		return
	}
	if !e.collecting {
		e.w.Uint(e.ids[n])
		return
	}
	if _, seen := e.ids[n]; seen {
		return
	}
	e.ids[n] = 0
	e.byKind[k] = append(e.byKind[k], n)
	e.encodeFields(n)
}

func (e *Encoder) base(b *ast.NodeBase) {
	e.span(b.Span())
	e.trivia(b.LeadingTrivia())
	e.trivia(b.TrailingTrivia())
}

func (e *Encoder) trivia(ts []ast.Trivia) {
	e.w.Len(len(ts))
	for _, t := range ts {
		e.w.Int(int64(t.Kind))
		e.span(t.Span)
	}
}

// span writes a span with its offset relative to the last one written, since
// a tree's spans mostly advance in small steps.
func (e *Encoder) span(s source.Span) {
	e.w.Int(int64(s.Offset - e.last))
	e.w.Int(int64(s.Len))
	e.last = s.Offset
}

func (e *Encoder) ident(id ast.Identification) {
	e.w.String(id.ShortName)
	e.span(id.ShortNameSpan)
	e.w.String(id.Name)
	e.span(id.NameSpan)
}

func (e *Encoder) nodes(ns []ast.Node) {
	e.w.Len(len(ns))
	for _, n := range ns {
		e.node(n)
	}
}

func (e *Encoder) rels(ns []*ast.Relationship) {
	e.w.Len(len(ns))
	for _, n := range ns {
		e.node(n)
	}
}

func (e *Encoder) prefixes(ns []*ast.PrefixMetadata) {
	e.w.Len(len(ns))
	for _, n := range ns {
		e.node(n)
	}
}

func (e *Encoder) qnames(ns []*ast.QualifiedName) {
	e.w.Len(len(ns))
	for _, n := range ns {
		e.node(n)
	}
}

func (e *Encoder) regions(ns []*ast.StateRegion) {
	e.w.Len(len(ns))
	for _, n := range ns {
		e.node(n)
	}
}

func (e *Encoder) ends(ns []*ast.ConnectorEnd) {
	e.w.Len(len(ns))
	for _, n := range ns {
		e.node(n)
	}
}

func (e *Encoder) segments(segs []ast.NameSegment) {
	e.w.Len(len(segs))
	for _, s := range segs {
		e.w.String(s.Text)
		e.span(s.Span)
		e.w.Bool(s.Chained)
	}
}

func (e *Encoder) namedArgs(args []ast.NamedArg) {
	e.w.Len(len(args))
	for _, a := range args {
		e.node(a.Name)
		e.node(a.Value)
	}
}

func (e *Encoder) params(ps []ast.BodyParam) {
	e.w.Len(len(ps))
	for i := range ps {
		p := &ps[i]
		e.w.String(p.Name)
		e.node(p.Type)
		e.node(p.Multiplicity)
		e.node(p.Value)
		e.w.Bool(p.IsReference)
		e.nodes(p.Members)
		e.rels(p.Relationships)
		e.span(p.Span)
	}
}

// Decoder reads one node table: Allocate creates the nodes, Fill reads their
// fields, and Node may hand out addresses while Fill runs elsewhere.
type Decoder struct {
	r      *pack.Reader // the reader in use: head, then fields
	head   *pack.Reader
	fields *pack.Reader // the section holding the fields, after Allocate
	table  []ast.Node   // by table index; table[0] is nil
	last   int

	nodeSlices pack.Arena[ast.Node]
	relSlices  pack.Arena[*ast.Relationship]
	prefSlices pack.Arena[*ast.PrefixMetadata]
	nameSlices pack.Arena[*ast.QualifiedName]
	regSlices  pack.Arena[*ast.StateRegion]
	endSlices  pack.Arena[*ast.ConnectorEnd]
	segArena   pack.Arena[ast.NameSegment]
	argArena   pack.Arena[ast.NamedArg]
	paramArena pack.Arena[ast.BodyParam]
	trivArena  pack.Arena[ast.Trivia]
}

// NewDecoder returns a Decoder reading from r.
func NewDecoder(r *pack.Reader) *Decoder {
	return &Decoder{r: r, head: r}
}

// Decode reads the node table Encode wrote: Allocate then Fill. Errors are
// left on the reader for the caller to check.
func (d *Decoder) Decode() {
	d.Allocate()
	d.Fill()
}

// Allocate reads the node counts, allocates every node in one block per type,
// and moves the reader past the table; the nodes' fields are zero until Fill.
func (d *Decoder) Allocate() {
	if d.r.Len() != int(numKinds) {
		d.r.Fail("node kind count")
		return
	}
	var counts [numKinds]int
	total := 0
	for k := range counts {
		counts[k] = d.r.Len()
		total += counts[k]
	}
	d.fields = d.r.Section()
	if d.r.Err() != nil {
		return
	}
	d.table = make([]ast.Node, 1, total+1)
	for k, count := range counts {
		d.table = alloc(kind(k), count, d.table)
	}
}

// Fill reads the fields of the nodes Allocate made. Its error is on Err.
func (d *Decoder) Fill() {
	if d.fields == nil || len(d.table) == 0 {
		return
	}
	d.r = d.fields
	for _, n := range d.table[1:] {
		d.decodeFields(n)
		if d.r.Err() != nil {
			return
		}
	}
	if !d.r.Done() {
		d.r.Fail("bytes after the node fields")
	}
}

// Err reports the first malformation Allocate or Fill met, or nil.
func (d *Decoder) Err() error {
	if err := d.head.Err(); err != nil || d.fields == nil {
		return err
	}
	return d.fields.Err()
}

// Node returns the node at a table index, and whether the index is in the
// table; 0 is nil. It is safe to call while Fill runs.
func (d *Decoder) Node(id uint64) (ast.Node, bool) {
	if id >= uint64(len(d.table)) {
		return nil, false
	}
	return d.table[id], true
}

func (d *Decoder) node() ast.Node {
	n, ok := d.Node(d.r.Uint())
	if !ok {
		d.r.Fail("node index")
	}
	return n
}

// typed reads a node reference that must be a T, or nil.
func typed[T ast.Node](d *Decoder) T {
	var zero T
	n := d.node()
	if n == nil {
		return zero
	}
	t, ok := n.(T)
	if !ok {
		d.r.Fail("node type")
		return zero
	}
	return t
}

func (d *Decoder) base(b *ast.NodeBase) {
	b.NodeSpan = d.span()
	b.SetLeadingTrivia(d.triviaList())
	b.SetTrailingTrivia(d.triviaList())
}

func (d *Decoder) triviaList() []ast.Trivia {
	out := d.trivArena.Take(d.r.Len())
	for i := range out {
		out[i].Kind = ast.TriviaKind(d.r.Int())
		out[i].Span = d.span()
	}
	return out
}

func (d *Decoder) span() source.Span {
	offset := d.last + int(d.r.Int())
	length := int(d.r.Int())
	d.last = offset
	return source.Span{Offset: offset, Len: length}
}

func (d *Decoder) ident() ast.Identification {
	var id ast.Identification
	id.ShortName = d.r.String()
	id.ShortNameSpan = d.span()
	id.Name = d.r.String()
	id.NameSpan = d.span()
	return id
}

func (d *Decoder) nodes() []ast.Node {
	out := d.nodeSlices.Take(d.r.Len())
	for i := range out {
		out[i] = d.node()
	}
	return out
}

func (d *Decoder) rels() []*ast.Relationship {
	out := d.relSlices.Take(d.r.Len())
	for i := range out {
		out[i] = typed[*ast.Relationship](d)
	}
	return out
}

func (d *Decoder) prefixes() []*ast.PrefixMetadata {
	out := d.prefSlices.Take(d.r.Len())
	for i := range out {
		out[i] = typed[*ast.PrefixMetadata](d)
	}
	return out
}

func (d *Decoder) qnames() []*ast.QualifiedName {
	out := d.nameSlices.Take(d.r.Len())
	for i := range out {
		out[i] = typed[*ast.QualifiedName](d)
	}
	return out
}

func (d *Decoder) regions() []*ast.StateRegion {
	out := d.regSlices.Take(d.r.Len())
	for i := range out {
		out[i] = typed[*ast.StateRegion](d)
	}
	return out
}

func (d *Decoder) ends() []*ast.ConnectorEnd {
	out := d.endSlices.Take(d.r.Len())
	for i := range out {
		out[i] = typed[*ast.ConnectorEnd](d)
	}
	return out
}

func (d *Decoder) segments() []ast.NameSegment {
	return d.segmentsN(d.r.Len())
}

func (d *Decoder) segmentsN(count int) []ast.NameSegment {
	out := d.segArena.Take(count)
	for i := range out {
		out[i] = d.segment()
	}
	return out
}

func (d *Decoder) segment() ast.NameSegment {
	var s ast.NameSegment
	s.Text = d.r.String()
	s.Span = d.span()
	s.Chained = d.r.Bool()
	return s
}

// parts fills a name's segments, a lone one into the node's own storage as
// the parser does.
func (d *Decoder) parts(n *ast.QualifiedName) {
	if count := d.r.Len(); count == 1 {
		n.SetSingleton(d.segment())
	} else {
		n.Parts = d.segmentsN(count)
	}
}

func (d *Decoder) namedArgs() []ast.NamedArg {
	out := d.argArena.Take(d.r.Len())
	for i := range out {
		out[i].Name = typed[*ast.QualifiedName](d)
		out[i].Value = d.node()
	}
	return out
}

func (d *Decoder) params() []ast.BodyParam {
	out := d.paramArena.Take(d.r.Len())
	for i := range out {
		p := &out[i]
		p.Name = d.r.String()
		p.Type = typed[*ast.QualifiedName](d)
		p.Multiplicity = typed[*ast.Multiplicity](d)
		p.Value = d.node()
		p.IsReference = d.r.Bool()
		p.Members = d.nodes()
		p.Relationships = d.rels()
		p.Span = d.span()
	}
	return out
}

// Reachable is the set of nodes reachable from roots, roots included: what
// Encode would write for them.
func Reachable(roots ...ast.Node) map[ast.Node]bool {
	e := &Encoder{w: pack.NewWriter(), ids: make(map[ast.Node]uint64), collecting: true}
	for _, root := range roots {
		e.node(root)
	}
	set := make(map[ast.Node]bool, len(e.ids))
	for n := range e.ids {
		set[n] = true
	}
	return set
}
