package symbols

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/pack"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ErrNeedsHydration reports a question only a document's syntax tree answers,
// asked of an interface-record symbol: the document has to be hydrated first.
var ErrNeedsHydration = errors.New("symbols: the document must be hydrated to answer")

// NeedsHydration is ErrNeedsHydration naming the document and the question.
type NeedsHydration struct {
	Doc      string
	Question string
}

func (e *NeedsHydration) Error() string {
	return fmt.Sprintf("%s: %s needs the tree of %s", ErrNeedsHydration.Error(), e.Question, e.Doc)
}

// Unwrap makes errors.Is(err, ErrNeedsHydration) true.
func (e *NeedsHydration) Unwrap() error { return ErrNeedsHydration }

// NeedsTree returns the typed answer for question asked of a recorded symbol,
// and nil when sym carries its declaration.
func NeedsTree(sym *Symbol, question string) error {
	if !sym.Recorded() {
		return nil
	}
	return &NeedsHydration{Doc: sym.DocName, Question: question}
}

// DocumentRecord is the interface record of one document: what another
// document can observe of it through the language, as a tree-less scope tree
// whose symbols carry facts in place of declarations, and the imports and
// filters its namespaces state. Scopes[0] is the document root.
type DocumentRecord struct {
	Scopes  []ScopeRecord
	Symbols []SymbolRecord
	// Gathered holds what the workspace-wide audits read from the body.
	Gathered *GatheredRelationships
	// Nodes is the astcodec table of the import declarations and filter
	// conditions the scopes refer to by table index.
	Nodes []byte
}

// ScopeRecord is one namespace of a recorded document.
type ScopeRecord struct {
	Owner    int32 // index into Symbols, -1 for the document root
	Members  []MemberRecord
	Children []int32
	Imports  []uint64 // node ids of the *ast.Import declarations
	Filters  []uint64 // node ids of the `filter` conditions
}

// MemberRecord is one registration in a scope: Name "" registers the symbol
// anonymously.
type MemberRecord struct {
	Name   string
	Symbol int32
}

// SymbolRecord is one symbol of a recorded document.
type SymbolRecord struct {
	Name       string
	Kind       SymbolKind
	Visibility ast.Visibility
	DeclSpan   source.Span
	NameSpan   source.Span
	Scope      int32 // index into Scopes, -1 for a leaf
	ShortName  string
	Naming     Naming
	Facts      LibraryFacts
}

// RecordScope writes the interface record of a loaded scope tree. keep says
// which members the record carries; facts states what each kept symbol's
// declaration yields. A kept symbol's scope is recorded with it; a scope no
// kept symbol owns is not.
func RecordScope(root *Scope, keep func(*Symbol) bool, facts func(*Symbol) LibraryFacts) (*DocumentRecord, error) {
	w := &recordWriter{rec: &DocumentRecord{}, keep: keep, facts: facts, syms: map[*Symbol]int32{}}
	w.scope(root, -1)
	if len(w.nodes) == 0 {
		return w.rec, nil
	}
	pw := pack.NewWriter()
	enc := astcodec.NewEncoder(pw)
	if err := enc.Encode(w.nodes); err != nil {
		return nil, err
	}
	ids := func(nodes []ast.Node) ([]uint64, error) {
		out := make([]uint64, 0, len(nodes))
		for _, n := range nodes {
			id, ok := enc.ID(n)
			if !ok || id == 0 {
				return nil, fmt.Errorf("symbols: record: %T not encoded", n)
			}
			out = append(out, id)
		}
		return out, nil
	}
	for i, nodes := range w.scopeNodes {
		var err error
		if w.rec.Scopes[i].Imports, err = ids(nodes.imports); err != nil {
			return nil, err
		}
		if w.rec.Scopes[i].Filters, err = ids(nodes.filters); err != nil {
			return nil, err
		}
	}
	w.rec.Nodes = pw.Bytes()
	return w.rec, nil
}

type recordWriter struct {
	rec        *DocumentRecord
	keep       func(*Symbol) bool
	facts      func(*Symbol) LibraryFacts
	syms       map[*Symbol]int32
	nodes      []ast.Node
	scopeNodes []scopeNodes // parallel to rec.Scopes
}

// scopeNodes are the declarations a scope refers into the node table for.
type scopeNodes struct {
	imports, filters []ast.Node
}

func (w *recordWriter) scope(s *Scope, owner int32) int32 {
	id := int32(len(w.rec.Scopes)) // #nosec G115 -- one document declares far fewer than MaxInt32 scopes.
	w.rec.Scopes = append(w.rec.Scopes, ScopeRecord{Owner: owner})
	w.scopeNodes = append(w.scopeNodes, scopeNodes{})
	// Registrations keep their declaration order, so a member ordinal in an
	// ElementRef means the same in the loaded and the recorded scope.
	var members []MemberRecord
	named := 0
	for _, sym := range s.members {
		name := ""
		if named < len(s.syms) && s.syms[named] == sym {
			name = s.names[named]
			named++
		}
		if !w.keep(sym) {
			continue
		}
		members = append(members, MemberRecord{Name: name, Symbol: w.symbol(sym)})
	}
	var children []int32
	for _, child := range s.children {
		ownerSym := child.owner
		if ownerSym == nil || child.bodyLocal || child.annotated != nil || ownerSym.Scope != child || !w.keep(ownerSym) {
			continue
		}
		symID := w.symbol(ownerSym)
		childID := w.scope(child, symID)
		w.rec.Symbols[symID].Scope = childID
		children = append(children, childID)
	}
	var nodes scopeNodes
	for _, imp := range s.Imports() {
		nodes.imports = append(nodes.imports, imp)
		w.nodes = append(w.nodes, imp)
	}
	for _, f := range NamespaceFiltersIn(s) {
		nodes.filters = append(nodes.filters, f.Expr)
		w.nodes = append(w.nodes, f.Expr)
	}
	w.rec.Scopes[id].Members, w.rec.Scopes[id].Children = members, children
	w.scopeNodes[id] = nodes
	return id
}

func (w *recordWriter) symbol(sym *Symbol) int32 {
	if id, ok := w.syms[sym]; ok {
		return id
	}
	id := int32(len(w.rec.Symbols)) // #nosec G115 -- one document declares far fewer than MaxInt32 symbols.
	w.syms[sym] = id
	facts := w.facts(sym)
	facts.Recorded = true
	w.rec.Symbols = append(w.rec.Symbols, SymbolRecord{
		Name:       sym.Name,
		Kind:       sym.Kind,
		Visibility: sym.Visibility,
		DeclSpan:   sym.DeclSpan,
		NameSpan:   sym.NameSpan,
		Scope:      -1,
		ShortName:  sym.ShortName,
		Naming:     sym.Naming,
		Facts:      facts,
	})
	return id
}

// BuildRecorded rebuilds the tree-less scope tree of a record, stamped as
// name's. Every symbol has Decl unset and Facts set, with Recorded true.
func BuildRecorded(rec *DocumentRecord, name string) (*Scope, error) {
	if len(rec.Scopes) == 0 {
		return nil, fmt.Errorf("%w: record without a root scope", pack.ErrCorrupt)
	}
	var dec *astcodec.Decoder
	if len(rec.Nodes) > 0 {
		r, err := pack.NewReader(rec.Nodes)
		if err != nil {
			return nil, err
		}
		dec = astcodec.NewDecoder(r)
		dec.Decode()
		if err := dec.Err(); err != nil {
			return nil, err
		}
	}
	node := func(id uint64) (ast.Node, error) {
		if dec == nil {
			return nil, fmt.Errorf("%w: record refers to a node table it has none of", pack.ErrCorrupt)
		}
		n, ok := dec.Node(id)
		if !ok || n == nil {
			return nil, fmt.Errorf("%w: record node index %d", pack.ErrCorrupt, id)
		}
		return n, nil
	}
	scopes := make([]*Scope, len(rec.Scopes))
	for i := range scopes {
		scopes[i] = &Scope{recorded: true, docName: name}
	}
	syms := make([]*Symbol, len(rec.Symbols))
	in := interner{}
	for i, sr := range rec.Symbols {
		facts := sr.Facts
		facts.Recorded = true
		in.facts(&facts)
		sym := &Symbol{
			Name:       in.str(sr.Name),
			Kind:       sr.Kind,
			Visibility: sr.Visibility,
			DeclSpan:   sr.DeclSpan,
			NameSpan:   sr.NameSpan,
			ShortName:  in.str(sr.ShortName),
			Naming:     sr.Naming,
			DocName:    name,
			Facts:      &facts,
		}
		if sr.Scope >= 0 {
			if int(sr.Scope) >= len(scopes) {
				return nil, fmt.Errorf("%w: record scope index %d", pack.ErrCorrupt, sr.Scope)
			}
			sym.Scope = scopes[sr.Scope]
			sym.Scope.owner = sym
		}
		syms[i] = sym
	}
	for i, sr := range rec.Scopes {
		s := scopes[i]
		for _, m := range sr.Members {
			if int(m.Symbol) < 0 || int(m.Symbol) >= len(syms) {
				return nil, fmt.Errorf("%w: record symbol index %d", pack.ErrCorrupt, m.Symbol)
			}
			sym := syms[m.Symbol]
			sym.OwnerScope = s
			if m.Name == "" {
				s.DefineAnonymous(sym)
			} else {
				s.Define(in.str(m.Name), sym)
			}
		}
		for _, c := range sr.Children {
			if int(c) <= 0 || int(c) >= len(scopes) {
				return nil, fmt.Errorf("%w: record child index %d", pack.ErrCorrupt, c)
			}
			scopes[c].parent = s
			s.AddChild(scopes[c])
		}
		for _, id := range sr.Imports {
			n, err := node(id)
			if err != nil {
				return nil, err
			}
			imp, ok := n.(*ast.Import)
			if !ok {
				return nil, fmt.Errorf("%w: record import is a %T", pack.ErrCorrupt, n)
			}
			s.imports = append(s.imports, imp)
		}
		for _, id := range sr.Filters {
			n, err := node(id)
			if err != nil {
				return nil, err
			}
			s.filters = append(s.filters, ElementFilter{Expr: n, Scope: s, Span: n.Span()})
		}
	}
	return scopes[0], nil
}

// interner shares one copy of each distinct string a decoded record repeats —
// a type's name under every feature typed by it, a member's name under its
// registration and its symbol — where decoding gave each its own.
type interner map[string]string

func (in interner) str(s string) string {
	if s == "" {
		return ""
	}
	if shared, ok := in[s]; ok {
		return shared
	}
	in[s] = s
	return s
}

func (in interner) ref(r *ElementRef) {
	r.FQN = in.str(r.FQN)
	r.Doc = in.str(r.Doc)
}

func (in interner) refs(rs []ElementRef) {
	for i := range rs {
		in.ref(&rs[i])
	}
}

func (in interner) facts(f *LibraryFacts) {
	in.refs(f.Supers)
	in.refs(f.Redefines)
	in.refs(f.About)
	in.ref(&f.Alias)
	in.ref(&f.References)
	in.ref(&f.BaseType)
	for i := range f.Relationships {
		in.ref(&f.Relationships[i].Target)
	}
	for i := range f.Annotations {
		f.Annotations[i].TypeFQN = in.str(f.Annotations[i].TypeFQN)
	}
	f.Keyword = in.str(f.Keyword)
}
