package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// own makes the document declaring sym the owner of what is memoized until
// LeaveDoc is called on the returned resolver (see resolve.Resolver.EnterDoc):
// a method memoizing per symbol computes under `defer m.own(sym).LeaveDoc()`.
func (m *Model) own(sym *symbols.Symbol) *resolve.Resolver {
	doc := ""
	if sym != nil {
		doc = sym.DocName
	}
	m.resolver.EnterDoc(doc)
	return m.resolver
}

// ownScope is own for a method memoizing per scope.
func (m *Model) ownScope(scope *symbols.Scope) *resolve.Resolver {
	m.resolver.EnterDoc(symbols.DocNameOf(scope))
	return m.resolver
}

// journal registers the deletion of table[key], about to be written for the
// first time, with the frame owning node (see resolve.JournalNew).
func journal[K comparable, V any](m *Model, table map[K]V, key K, node ast.Node) {
	resolve.JournalNew(m.resolver, table, key, node)
}

// shared runs build inside the frame named name unless ready reports its result
// still stands, and journals reset to run when that frame is dropped. The
// caller's frame depends on name's, so a reader is dropped with the result.
func (m *Model) shared(name string, ready func() bool, build func(), reset func()) {
	m.resolver.EnterDoc(name)
	defer m.resolver.LeaveDoc()
	m.resolver.ReadName(name)
	if ready() {
		return
	}
	build()
	m.resolver.Journal(nil, reset)
}
