package symbols

import (
	"math"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// Scope is a node in the immutable per-document scope tree. It owns the
// symbols declared directly within a namespace-like construct and links to
// its parent and child scopes.
type Scope struct {
	parent           *Scope
	owner            *Symbol                            // the symbol that owns this scope (for inheritance lookup)
	node             ast.Node                           // the owning declaration node
	names            []string                           // named members in declaration order
	syms             []*Symbol                          // named members in declaration order
	members          []*Symbol                          // all registrations in declaration order
	memberIndex      atomic.Pointer[map[string][]int32] // lazily built lookup index for larger scopes
	anonymousMembers []*Symbol                          // anonymous symbols (no name)
	children         []*Scope
	childIndex       atomic.Pointer[map[ast.Node]*Scope] // lazily built node -> child scope index for larger scopes
	bodyLocal        bool                                // declarations live only inside the owning body
	docName          string                              // document this scope tree belongs to (stamped by SetDocName)
}

const memberIndexThreshold = 12

// childIndexThreshold is the child count above which ChildFor indexes instead
// of scanning; most scopes stay below it and hold no map at all.
const childIndexThreshold = 12

// DocName is the document this scope belongs to, or "" when none was stamped.
func (s *Scope) DocName() string { return s.docName }

// NewScope creates an empty scope with the given parent and owning node.
func NewScope(parent *Scope, node ast.Node) *Scope {
	return &Scope{
		parent: parent,
		node:   node,
	}
}

// Parent returns the enclosing scope, or nil for the document root.
func (s *Scope) Parent() *Scope { return s.parent }

// Owner returns the symbol that owns this scope, or nil if not set.
func (s *Scope) Owner() *Symbol { return s.owner }

// SetOwner sets the symbol that owns this scope (for inheritance lookup).
func (s *Scope) SetOwner(sym *Symbol) { s.owner = sym }

// Node returns the AST node that owns this scope, or nil for synthetic scopes.
func (s *Scope) Node() ast.Node { return s.node }

// BodyLocal reports whether this scope's names exist only inside the body that
// declares them, so a subtree search such as a recursive import must skip it.
func (s *Scope) BodyLocal() bool { return s.bodyLocal }

// markBodyLocal records that this scope's names do not escape its body.
func (s *Scope) markBodyLocal() { s.bodyLocal = true }

// Children returns the child scopes in definition order.
func (s *Scope) Children() []*Scope { return s.children }

// AddChild appends a child scope.
func (s *Scope) AddChild(c *Scope) {
	s.children = append(s.children, c)
	s.childIndex.Store(nil)
}

// ChildFor returns the child scope the given declaration owns, or nil. It is the
// first such child, as two children of one node are one body scoped twice.
func (s *Scope) ChildFor(node ast.Node) *Scope {
	if node == nil || len(s.children) == 0 {
		return nil
	}
	if len(s.children) > childIndexThreshold {
		return (*s.loadChildIndex())[node]
	}
	for _, c := range s.children {
		if c.node == node {
			return c
		}
	}
	return nil
}

// loadChildIndex returns the node -> child scope index, building it on first use
// after a change.
func (s *Scope) loadChildIndex() *map[ast.Node]*Scope {
	if idx := s.childIndex.Load(); idx != nil {
		return idx
	}
	built := make(map[ast.Node]*Scope, len(s.children))
	for _, c := range s.children {
		if c.node == nil {
			continue
		}
		if _, ok := built[c.node]; !ok {
			built[c.node] = c
		}
	}
	s.childIndex.Store(&built)
	return &built
}

// Define registers sym under the given name key. Multiple symbols may share a
// key (duplicate declarations); all are retained in definition order.
func (s *Scope) Define(name string, sym *Symbol) {
	if name == "" {
		return
	}
	s.names = append(s.names, name)
	s.syms = append(s.syms, sym)
	s.members = append(s.members, sym)
	s.memberIndex.Store(nil)
}

// DefineAnonymous adds an anonymous symbol (without name) to this scope.
func (s *Scope) DefineAnonymous(sym *Symbol) {
	s.anonymousMembers = append(s.anonymousMembers, sym)
	s.members = append(s.members, sym)
}

// HasAnonymousMembers reports whether any member is declared without a name.
func (s *Scope) HasAnonymousMembers() bool {
	return len(s.anonymousMembers) > 0
}

// AnonymousMembers returns the members declared without a name, in declaration
// order. One may still have an effective name, taken from a feature it
// implicitly redefines, which only the semantic model knows (KerML 7.3.4.5).
func (s *Scope) AnonymousMembers() []*Symbol {
	out := make([]*Symbol, len(s.anonymousMembers))
	copy(out, s.anonymousMembers)
	return out
}

// LookupLocal returns the first symbol defined under name in this scope only.
// A declared name wins over an effective one taken from a referenced feature.
func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	syms := s.LookupLocalAll(name)
	if len(syms) == 0 {
		return nil, false
	}
	if len(syms) == 1 {
		return syms[0], true
	}
	return PreferDeclared(syms)[0], true
}

// PreferDeclared drops symbols whose name was borrowed from a referenced
// feature when the key also has a declared one, which is how a `perform p;`
// shorthand and a declaration named p coexist in one scope. A `first x` label
// borrows its name the same way, from the member it starts at.
func PreferDeclared(syms []*Symbol) []*Symbol {
	declared := make([]*Symbol, 0, len(syms))
	for _, sym := range syms {
		if declaresName(sym) {
			declared = append(declared, sym)
		}
	}
	if len(declared) == 0 {
		return syms
	}
	return declared
}

// declaresName reports whether sym bears a name of its own rather than one
// borrowed from another member.
func declaresName(sym *Symbol) bool {
	if sym.EffectiveName() {
		return false
	}
	_, label := sym.Decl.(*ast.InitialNode)
	return !label
}

// LookupLocalAll returns every symbol defined under name in this scope only.
func (s *Scope) LookupLocalAll(name string) []*Symbol {
	if len(s.names) == 0 {
		return nil
	}
	if len(s.names) <= memberIndexThreshold {
		first := -1
		count := 0
		for i, memberName := range s.names {
			if memberName != name {
				continue
			}
			if first == -1 {
				first = i
			}
			count++
		}
		if first == -1 {
			return nil
		}
		if count == 1 {
			return s.syms[first : first+1 : first+1]
		}
		out := make([]*Symbol, 0, count)
		for i, memberName := range s.names {
			if memberName == name {
				out = append(out, s.syms[i])
			}
		}
		return out
	}
	index := s.loadMemberIndex()
	indices := (*index)[name]
	if len(indices) == 0 {
		return nil
	}
	if len(indices) == 1 {
		i := int(indices[0])
		return s.syms[i : i+1 : i+1]
	}
	out := make([]*Symbol, len(indices))
	for i, memberIndex := range indices {
		out[i] = s.syms[memberIndex]
	}
	return out
}

func (s *Scope) loadMemberIndex() *map[string][]int32 {
	if index := s.memberIndex.Load(); index != nil {
		return index
	}
	built := make(map[string][]int32)
	for i, name := range s.names {
		built[name] = append(built[name], memberIndexOf(i))
	}
	if s.memberIndex.CompareAndSwap(nil, &built) {
		return &built
	}
	return s.memberIndex.Load()
}

func memberIndexOf(i int) int32 {
	if i > math.MaxInt32 {
		panic("scope member index exceeds int32 capacity")
	}
	return int32(i) // #nosec G115 -- bounds checked against math.MaxInt32 above.
}

// MemberNames returns the distinct member keys of this scope in declaration order.
func (s *Scope) MemberNames() []string {
	if len(s.names) == 0 {
		return nil
	}
	out := make([]string, 0, len(s.names))
	if len(s.names) > memberIndexThreshold {
		index := s.loadMemberIndex()
		for i, name := range s.names {
			indices := (*index)[name]
			if len(indices) > 0 && int(indices[0]) == i {
				out = append(out, name)
			}
		}
		return out
	}
	for _, name := range s.names {
		duplicate := false
		for _, existing := range out {
			if existing == name {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		out = append(out, name)
	}
	return out
}

// MemberDeclaring returns the member of s that decl declared, or nil.
func (s *Scope) MemberDeclaring(decl ast.Node) *Symbol {
	if s == nil || decl == nil {
		return nil
	}
	sym, _ := memberDeclaring(s, decl)
	return sym
}

// AllMembers returns named and anonymous symbol registrations in declaration order.
func (s *Scope) AllMembers() []*Symbol {
	var all []*Symbol
	s.ForEachMember(func(sym *Symbol) bool {
		all = append(all, sym)
		return true
	})
	return all
}

// ForEachMember visits named and anonymous symbol registrations in declaration order.
func (s *Scope) ForEachMember(yield func(*Symbol) bool) {
	if s == nil || yield == nil {
		return
	}
	for _, sym := range s.members {
		if !yield(sym) {
			return
		}
	}
}
