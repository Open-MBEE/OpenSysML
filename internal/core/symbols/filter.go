package symbols

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// ElementFilter is one element-filter condition, as it applies to the elements
// an enumeration would surface: the condition of a `filter <expr>;` member of a
// namespace (KerML 8.2.4 ElementFilterMembership) or of an import's `[...]`
// clause (`import P::*[@Safety]`, SysML v2 7.4.4).
//
// A condition is a predicate over one candidate element, not a value expression,
// and evaluating it needs the metadata annotating the candidate — knowledge the
// symbol layer does not have. So it is carried as parsed: the expression,
// together with the Scope its names resolve against, which the evaluator in
// semantics compiles to a predicate over resolved elements.
type ElementFilter struct {
	Expr  ast.Node
	Scope *Scope
	Span  source.Span
}

// IsZero reports whether there is no condition to apply.
func (f ElementFilter) IsZero() bool { return f.Expr == nil }

// String describes a filter for a snapshot of the index, rendering only whether
// a condition applies: which elements it selects is asserted over resolution
// rather than over this text.
func (f ElementFilter) String() string {
	if f.IsZero() {
		return "unfiltered"
	}
	return "filtered"
}

// FilterOp is the operation a compiled filter predicate node performs.
type FilterOp uint8

const (
	// FilterUnsupported is a condition, or part of one, outside the subset the
	// filter evaluator implements. It has no truth value: it is reported, and
	// never silently taken to be true or false.
	FilterUnsupported FilterOp = iota
	FilterAnd
	FilterOr
	FilterXor
	FilterImplies
	FilterNot
	// FilterClassify is `@T`: the candidate is annotated by metadata
	// conforming to TypeFQN.
	FilterClassify
	// FilterMetaClassify is `@@T`: the candidate's own metaclass conforms to
	// TypeFQN.
	FilterMetaClassify
	FilterEq
	FilterNeq
	FilterLt
	FilterLe
	FilterGt
	FilterGe
	// FilterConst is a literal, or an element reference used as one (an
	// enumeration literal), held in Value.
	FilterConst
	// FilterFeature is the value the candidate's annotation of TypeFQN binds
	// Feature to.
	FilterFeature
)

// FilterUnsupportedKind distinguishes a specification fault from a limitation
// of the evaluator for a FilterUnsupported predicate.
type FilterUnsupportedKind uint8

const (
	// FilterUnsupportedNotEvaluable is a model-level evaluability fault.
	FilterUnsupportedNotEvaluable FilterUnsupportedKind = iota
	// FilterUnsupportedEvaluator is a specification-clean evaluator limitation.
	FilterUnsupportedEvaluator
	// FilterUnsupportedNotBoolean is an unresolved value whose result cannot be
	// shown to be Boolean.
	FilterUnsupportedNotBoolean
)

// FilterPredicate is an element-filter condition compiled to the form the
// evaluator runs: a tree over the candidate element, with every element the
// condition names resolved to its fully-qualified name. Compiling before
// evaluating is what lets one condition be evaluated against many candidates,
// and lets a condition outlive the declaration it was written in.
type FilterPredicate struct {
	Op       FilterOp
	Operands []*FilterPredicate

	// TypeFQN is the metadata type a classification tests against, or whose
	// annotation a feature is read from.
	TypeFQN string

	// Feature is the annotation feature FilterFeature reads.
	Feature string

	// Value is the constant FilterConst yields.
	Value FilterValue

	// Reason says why a FilterUnsupported node cannot be evaluated, phrased for
	// a diagnostic message.
	Reason string

	// UnsupportedKind says whether Reason is a model-level evaluability fault
	// or a limitation of the OpenSysML evaluator.
	UnsupportedKind FilterUnsupportedKind

	// ResultType is the resolved terminal feature for a reference or chain. It
	// lets semantic validation determine Boolean-ness without evaluating it.
	ResultType *Symbol

	// Span locates the part of the condition this node came from.
	Span source.Span
}

// FilterValueKind discriminates a constant a filter predicate compares.
type FilterValueKind uint8

const (
	FilterValueUnknown FilterValueKind = iota
	FilterValueBool
	FilterValueInt
	FilterValueReal
	FilterValueString
	// FilterValueRef is a reference to an element used as a value, such as an
	// enumeration literal, compared by identity of the element it names.
	FilterValueRef
	// FilterValueEmpty is the absence of a value — the empty sequence a feature
	// bound to nothing has. It is a result, unlike FilterValueUnknown, which
	// says a value could not be determined.
	FilterValueEmpty
	// FilterValueInstance is an instance a condition constructs (`new A(…)`).
	// It is a value, and never a truth value.
	FilterValueInstance
	// FilterValueQuantity is a magnitude in a measurement unit (`5 [kg]`),
	// carried in Quantity.
	FilterValueQuantity
)

// FilterValue is a constant a filter predicate yields or compares: a literal, or
// the element an enumeration-literal reference names.
type FilterValue struct {
	Kind     FilterValueKind
	Bool     bool
	Int      int64
	Real     float64
	Str      string
	RefFQN   string
	Quantity QuantityValue
}

// QuantityValue is a constant quantity as the semantic layer represents it;
// units are resolved there, so the symbol layer carries the value opaquely.
type QuantityValue interface {
	// String renders the quantity as `magnitude [unit]`.
	fmt.Stringer
}

// Same reports whether two filters are the same condition, so that two routes to
// one re-exported name can be compared. A condition is identified by the
// declaration it was written in: the expression node and the scope its names
// resolve against.
func (f ElementFilter) Same(g ElementFilter) bool {
	return f.Expr == g.Expr && f.Scope == g.Scope
}

// NamespaceFiltersOf returns the filter conditions declared by the namespace
// registered under fqn, over the documents declaring it in name order.
func (idx *Index) NamespaceFiltersOf(fqn string) []ElementFilter {
	byDoc := idx.nsFilters.at(fqn)
	if len(byDoc) == 0 {
		return nil
	}
	var out []ElementFilter
	for _, doc := range sortedKeys(byDoc) {
		out = append(out, byDoc[doc]...)
	}
	return out
}

// namespaceFiltersGating returns the conditions gating an import doc states into
// the namespace under fqn: every document's for a named namespace, only doc's
// for a root namespace, which each document owns separately.
func (idx *Index) namespaceFiltersGating(fqn, doc string) []ElementFilter {
	if fqn != "" {
		return idx.NamespaceFiltersOf(fqn)
	}
	return idx.nsFilters.at(fqn)[doc]
}

// SetNamespaceFilters records the filter conditions doc declares for the
// namespace registered under fqn. A library restored from an index cache states
// them this way, having no declaration left to read them from.
func (idx *Index) SetNamespaceFilters(fqn, doc string, filters []ElementFilter) {
	idx.mustBeWritable("SetNamespaceFilters")
	if len(filters) == 0 {
		idx.forgetNamespaceFilters(fqn, doc)
		return
	}
	writableMap(idx.nsFilters, fqn)[doc] = filters
	idx.refilter(fqn)
}

// dropNamespaceFilters forgets the filters doc declared, for every namespace.
func (idx *Index) dropNamespaceFilters(doc string) {
	for _, fqn := range idx.nsFilters.keys() {
		idx.forgetNamespaceFilters(fqn, doc)
	}
}

// forgetNamespaceFilters drops the filters doc declared for the namespace
// registered under fqn, if it declared any.
func (idx *Index) forgetNamespaceFilters(fqn, doc string) {
	if _, had := idx.nsFilters.at(fqn)[doc]; !had {
		return
	}
	byDoc := writableMap(idx.nsFilters, fqn)
	delete(byDoc, doc)
	if len(byDoc) == 0 {
		idx.nsFilters.del(fqn)
	}
	idx.refilter(fqn)
}

// refilter drops what the namespace registered under fqn re-exports, because the
// conditions gating those routes have changed: the next expansion records them
// under the filters the namespace now declares, and the members it takes back
// meanwhile mark the namespaces importing it onward for expansion too.
func (idx *Index) refilter(fqn string) {
	idx.lastTargets.del(fqn)
	idx.purgeReexportsUnder(fqn)
}

// NamespaceFiltersIn returns the filter conditions the namespace owning scope
// declares. It is what a lookup through an import consults: the namespace's
// `filter` members restrict its imported memberships, whichever route reaches
// them (KerML 8.2.4).
//
// Unlike NamespaceFiltersOf, which answers for a namespace registered in the
// index, this reads the declaration a scope was built from, so it also answers
// for a `filter` member of a definition or usage body — where the training
// corpus puts one, in a view definition restricting what its views expose.
func NamespaceFiltersIn(scope *Scope) []ElementFilter {
	if scope == nil {
		return nil
	}
	return extractNamespaceFilters(scope.Node(), scope)
}

// ImportFiltersIn returns the filter clauses of the imports the namespace owning
// scope declares (`import P::*[@Safety]`), in declaration order. It is what a
// validation pass reads: the conditions an import states are checked where they
// are written, whichever lookup later applies them.
func ImportFiltersIn(scope *Scope) []ElementFilter {
	if scope == nil {
		return nil
	}
	var out []ElementFilter
	for _, m := range namespaceMembers(scope.Node()) {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		imp, ok := m.(*ast.Import)
		if !ok || imp.FilterExpr == nil {
			continue
		}
		out = append(out, ElementFilter{
			Expr:  imp.FilterExpr,
			Scope: scope,
			Span:  imp.FilterExpr.Span(),
		})
	}
	return out
}

// extractNamespaceFilters returns the conditions of the `filter` members a
// namespace-bearing declaration states, in declaration order.
func extractNamespaceFilters(decl ast.Node, scope *Scope) []ElementFilter {
	var out []ElementFilter
	for _, m := range namespaceMembers(decl) {
		if mem, ok := m.(*ast.Membership); ok {
			m = mem.Member
		}
		f, ok := m.(*ast.FilterMember)
		if !ok || f.Condition == nil {
			continue
		}
		out = append(out, ElementFilter{Expr: f.Condition, Scope: scope, Span: f.Span()})
	}
	return out
}

// namespaceMembers returns the members of a namespace-bearing declaration. A
// definition or usage body is a namespace too (KerML 7.2), and can hold both
// imports and filters.
func namespaceMembers(decl ast.Node) []ast.Node {
	switch d := decl.(type) {
	case *ast.Package:
		return d.Members
	case *ast.Namespace:
		return d.Members
	case *ast.RootNamespace:
		return d.Members
	case *ast.Definition:
		return d.Members
	case *ast.Usage:
		return d.Members
	default:
		return nil
	}
}

// sortedKeys returns a map's keys in name order, so that reading it does not
// depend on map iteration order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
