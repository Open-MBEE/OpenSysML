package semantics

import (
	"reflect"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Argument is one invocation argument as the checker types it; Exact holds of
// a literal, whose type is written out rather than only bounded.
type Argument struct {
	Prim  PrimType
	Type  *symbols.Symbol // the declared type of the feature or result named, nil when none
	Exact bool
	Name  *ast.QualifiedName // the name a named argument binds, as written; nil for positional
}

// known reports whether the checker knows anything about the argument's type.
func (a Argument) known() bool {
	return a.Prim != PrimUnknown || a.Type != nil
}

// ArgumentTyper types a call's arguments as the checker does, so a call read
// anywhere in the model selects the overload the checker selects.
type ArgumentTyper interface {
	InvocationArguments(scope *symbols.Scope, e *ast.InvocationExpr) []Argument
}

// SetArgumentTyper installs the checker's argument typing on the model. Selections
// memoized under a different typing are dropped, so none outlives the typing it
// read; installing the typing already in place keeps them.
func (m *Model) SetArgumentTyper(t ArgumentTyper) {
	if sameTyping(m.arguments, t) {
		return
	}
	m.arguments = t
	clear(m.invocations)
}

// sameTyping reports whether two typers are one typing: equal values of a
// comparable type. A typer that cannot be compared is taken as new.
func sameTyping(a, b ArgumentTyper) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ta, tb := reflect.TypeOf(a), reflect.TypeOf(b)
	return ta == tb && ta.Comparable() && a == b
}

// SelectCall selects what e calls, its arguments typed as the checker types them; with no
// checker's typing installed the arguments are unknown, so arity and names alone decide.
func (m *Model) SelectCall(scope *symbols.Scope, e *ast.InvocationExpr, performs Performs) *InvocationSelection {
	if m == nil || e == nil {
		return &InvocationSelection{}
	}
	if m.typingArgs[e] {
		// Typing an argument of e led back to e: select on arity alone, unmemoized,
		// so the typed selection is not answered from this provisional one.
		if e.Type == nil {
			return &InvocationSelection{}
		}
		return m.selectAmong(scope, m.resolver.InvocationCandidates(scope, e.Type), untypedArguments(e), performs)
	}
	return m.SelectInvocation(scope, e, m.callArguments(scope, e), performs)
}

// SelectCallAmong is SelectCall were e's name to denote named, in that order, as a
// trial spelling would: an alias stands for its target. Not memoized.
func (m *Model) SelectCallAmong(scope *symbols.Scope, e *ast.InvocationExpr, named []*symbols.Symbol, performs Performs) *InvocationSelection {
	if m == nil || e == nil {
		return &InvocationSelection{}
	}
	return m.selectAmong(scope, named, m.callArguments(scope, e), performs)
}

// callArguments types e's arguments as the checker does when its typing is
// installed, else lists them untyped.
func (m *Model) callArguments(scope *symbols.Scope, e *ast.InvocationExpr) []Argument {
	if m.arguments == nil || m.typingArgs[e] {
		return untypedArguments(e)
	}
	m.typingArgs[e] = true
	defer delete(m.typingArgs, e)
	return m.arguments.InvocationArguments(scope, e)
}

// untypedArguments lists e's arguments, the receiver first, with nothing known of their types.
func untypedArguments(e *ast.InvocationExpr) []Argument {
	args := make([]Argument, 0, len(e.Args)+len(e.NamedArgs)+1)
	if e.Operand != nil {
		args = append(args, Argument{})
	}
	for range e.Args {
		args = append(args, Argument{})
	}
	for _, arg := range e.NamedArgs {
		if arg.Name != nil && len(arg.Name.Parts) > 0 {
			args = append(args, Argument{Name: arg.Name})
		}
	}
	return args
}

// Performs is what a call site does with the declaration it names, which
// decides the kinds of declaration it may select.
type Performs int

const (
	// PerformsBehavior evaluates an expression: a calc answers; another behavior only
	// when no calc is named.
	PerformsBehavior Performs = iota
	// PerformsAction runs an action, as `action a = tag(x);` does: only actions answer.
	PerformsAction
)

// CallSite is the kind of call site a reference is: an action performance when
// the call is an action usage's value, else an expression.
func CallSite(ref resolve.Reference) Performs {
	if ref.Performed {
		return PerformsAction
	}
	return PerformsBehavior
}

// Performable reports whether a call site of kind p can run sym: a behavior, or for an
// evaluated call a feature typed by one, which performs that behavior.
func (m *Model) Performable(p Performs, sym *symbols.Symbol) bool {
	if p == PerformsAction {
		return sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage
	}
	return m.performs(sym, behaviorLike)
}

// Evaluates reports whether calling sym yields a result: it is a calc (a KerML
// function), or a feature typed by one, which is what an expression evaluates.
func (m *Model) Evaluates(sym *symbols.Symbol) bool {
	return m.performs(sym, calcLike)
}

// performs reports whether sym, or a behavior it is typed by, satisfies is.
func (m *Model) performs(sym *symbols.Symbol, is func(*symbols.Symbol) bool) bool {
	visited := map[*symbols.Symbol]bool{}
	for sym != nil && !visited[sym] {
		if is(sym) {
			return true
		}
		visited[sym] = true
		sym = m.featureType(sym)
	}
	return false
}

// calcLike reports whether sym declares a calc (a KerML function).
func calcLike(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefCalc
	case *ast.Usage:
		return d.Kind == ast.UsageCalc
	}
	return sym.Kind == symbols.SymbolCalcDef || sym.Kind == symbols.SymbolCalcUsage
}

// InvocationSelection is which declaration an invocation calls, out of every
// declaration its written name is visible as.
type InvocationSelection struct {
	Candidates []*symbols.Symbol // what the name denotes, in lookup order, aliases followed
	Applicable []*symbols.Symbol // the candidates the arguments bind to
	Selected   *symbols.Symbol   // the one called; nil when none applies or they tie
	Ambiguous  bool              // several applicable candidates are equally specific
	Tied       []*symbols.Symbol // the applicable candidates none is more specific than, when Ambiguous
	callable   *symbols.Symbol   // the first candidate the call site can run, when any is
}

// Resolved reports whether the name denotes at least one declaration.
func (s *InvocationSelection) Resolved() bool {
	return s != nil && len(s.Candidates) > 0
}

// Called is the declaration a call runs: the one selected, or when no candidate fits the
// first callable one (else the first at all), which then reports the mismatch; nil when none.
func (s *InvocationSelection) Called() *symbols.Symbol {
	switch {
	case s == nil:
		return nil
	case s.Selected != nil:
		return s.Selected
	case s.callable != nil:
		return s.callable
	case len(s.Candidates) > 0:
		return s.Candidates[0]
	}
	return nil
}

// invocationKey identifies an invocation expression in the scope and the kind of
// call site it is read in.
type invocationKey struct {
	node     *ast.InvocationExpr
	scope    *symbols.Scope
	performs Performs
}

// SelectInvocation chooses the declaration e calls: the most specific candidate the arguments
// fit, or the first in lookup order when an argument's type is unknown. Memoized per node/scope.
func (m *Model) SelectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, args []Argument, performs Performs) *InvocationSelection {
	if m == nil || e == nil || e.Type == nil {
		return &InvocationSelection{}
	}
	key := invocationKey{node: e, scope: scope, performs: performs}
	if sel, ok := m.invocations[key]; ok {
		return sel
	}
	sel := m.selectAmong(scope, m.resolver.InvocationCandidates(scope, e.Type), args, performs)
	m.resolver.Journal(e, func() { delete(m.invocations, key) })
	m.invocations[key] = sel
	return sel
}

// MemoSize is the number of invocation selections the model retains by node.
func (m *Model) MemoSize() int {
	return len(m.invocations)
}

// selectAmong selects among the declarations named, in lookup order.
func (m *Model) selectAmong(scope *symbols.Scope, named []*symbols.Symbol, args []Argument, performs Performs) *InvocationSelection {
	sel := &InvocationSelection{}
	for _, sym := range named {
		target, ok := m.resolver.ResolveAliasTarget(sym)
		if !ok || target == nil {
			continue
		}
		if !containsSymbol(sel.Candidates, target) {
			sel.Candidates = append(sel.Candidates, target)
		}
	}
	if len(sel.Candidates) == 0 {
		return sel
	}
	behaviors := make([]*symbols.Symbol, 0, len(sel.Candidates))
	for _, c := range sel.Candidates {
		if m.Performable(performs, c) {
			behaviors = append(behaviors, c)
		}
	}
	switch {
	case len(behaviors) == 0:
		// Nothing the call site can run: the checker reports the first as it always has.
		sel.Selected = sel.Candidates[0]
		return sel
	case len(sel.Candidates) == 1:
		sel.Applicable = sel.Candidates
		sel.Selected = sel.Candidates[0]
		return sel
	}
	// An expression takes its call's result, which only a calc yields: when the name
	// denotes one, no action competes, however well its inputs fit the arguments.
	if performs == PerformsBehavior {
		if calcs := filterSymbols(behaviors, m.Evaluates); len(calcs) > 0 {
			behaviors = calcs
		}
	}
	sel.callable = behaviors[0]

	signatures := make([]invocationSignature, len(behaviors))
	for i, c := range behaviors {
		signatures[i] = m.signatureOf(c)
	}
	// Candidates binding every default-less parameter come first (the others cannot run), and
	// within each the ones the arguments conform to; a loose fit reports no tie.
	var applicable []*symbols.Symbol
	var sigs []invocationSignature
	var fits []bindFit
	var strict bool
	for _, pass := range bindingPasses {
		applicable, sigs, fits = m.filterApplicable(scope, behaviors, signatures, args, pass)
		if len(applicable) > 0 {
			strict = pass.mode == bindStrict
			break
		}
	}
	sel.Applicable = applicable
	switch len(applicable) {
	case 0:
		// None fits: the one every written name identifies a parameter of reports the mismatch.
		for i, sig := range signatures {
			if m.namesParameters(scope, sig, args) {
				sel.callable = behaviors[i]
				break
			}
		}
		return sel
	case 1:
		sel.Selected = applicable[0]
		return sel
	}
	if !strict || !argumentsKnown(args) {
		sel.Selected = applicable[0]
		return sel
	}
	// Specificity decides only among candidates the arguments surely fit: exact matches, else
	// widenings when no candidate takes them through an undetermined type; else lookup order.
	decisive := indicesWithFit(fits, fitExact)
	if len(decisive) == 0 && !containsFit(fits, fitOpen) {
		decisive = indicesWithFit(fits, fitWiden)
	}
	if len(decisive) == 0 {
		sel.Selected = applicable[0]
		return sel
	}
	decisiveSigs := make([]invocationSignature, len(decisive))
	for i, k := range decisive {
		decisiveSigs[i] = sigs[k]
	}
	if best := m.mostSpecific(scope, decisiveSigs, args); best >= 0 {
		sel.Selected = applicable[decisive[best]]
		return sel
	}
	sel.Ambiguous = true
	for _, k := range m.unbeaten(scope, decisiveSigs, args) {
		sel.Tied = append(sel.Tied, applicable[decisive[k]])
	}
	return sel
}

// bindMode is how closely an argument's type must fit its parameter's.
type bindMode int

const (
	bindLoose  bindMode = iota // the argument's type may only bound its values
	bindStrict                 // the argument's type conforms to the parameter's
)

// bindFit is how surely a candidate's parameters take the arguments, weakest
// parameter deciding.
type bindFit int

const (
	fitExact bindFit = iota // declared types conform, or the same scalar type
	fitWiden                // the argument widens to a wider parameter type, Anything included
	fitOpen                 // a type the checker cannot determine, so anything may fit
)

// bindingPass is one attempt at binding the arguments: how closely their types must fit,
// and whether a candidate may be left with a default-less parameter unbound.
type bindingPass struct {
	mode    bindMode
	partial bool
}

// bindingPasses are tried in order until one admits a candidate.
var bindingPasses = []bindingPass{
	{bindStrict, false},
	{bindLoose, false},
	{bindStrict, true},
	{bindLoose, true},
}

// filterApplicable keeps the candidates whose signature args bind to under pass, with
// how surely each takes them.
func (m *Model) filterApplicable(scope *symbols.Scope, cands []*symbols.Symbol, sigs []invocationSignature, args []Argument, pass bindingPass) ([]*symbols.Symbol, []invocationSignature, []bindFit) {
	var outCands []*symbols.Symbol
	var outSigs []invocationSignature
	var outFits []bindFit
	for i, sig := range sigs {
		if fit, complete, ok := m.applicable(scope, sig, args, pass.mode); ok && (complete || pass.partial) {
			outCands = append(outCands, cands[i])
			outSigs = append(outSigs, sig)
			outFits = append(outFits, fit)
		}
	}
	return outCands, outSigs, outFits
}

func indicesWithFit(fits []bindFit, want bindFit) []int {
	var out []int
	for i, fit := range fits {
		if fit == want {
			out = append(out, i)
		}
	}
	return out
}

func containsFit(fits []bindFit, want bindFit) bool {
	return len(indicesWithFit(fits, want)) > 0
}

// invocationSignature is the input parameters a candidate is invoked with.
type invocationSignature struct {
	owner  *symbols.Symbol // the candidate the parameters belong to
	params []signatureParameter
	known  bool // false when the parameters cannot be determined, so anything fits
}

type signatureParameter struct {
	sym      *symbols.Symbol
	name     string
	typ      *symbols.Symbol // the declared type, nil when untyped or unresolved
	prim     PrimType
	untyped  bool // typed Anything, whether written or by declaring no type
	optional bool // may go without an argument: a default, or a multiplicity admitting none
}

// signatureOf returns sym's effective input parameters, in signature order: the
// subject of a requirement or case first, being its first input parameter (SysML
// v2 §7.19.1, §7.20.1), then its directed `in` and `inout` parameters.
func (m *Model) signatureOf(sym *symbols.Symbol) invocationSignature {
	sig := invocationSignature{owner: sym}
	if subject := m.SubjectParameterOf(sym); subject != nil {
		sig.params = append(sig.params, m.signatureParameterOf(subject, m.EffectiveNameOf(subject)))
	}
	for _, p := range m.BehaviorParametersOf(sym) {
		if p.IsResult || (p.Direction != ast.DirIn && p.Direction != ast.DirInOut) {
			continue
		}
		name := p.Symbol.Name
		u, isUsage := p.Symbol.Decl.(*ast.Usage)
		if isUsage {
			if effective, _ := ast.EffectiveName(u); effective != "" {
				name = effective
			}
		}
		sig.params = append(sig.params, m.signatureParameterOf(p.Symbol, name))
	}
	// A parameterless declaration with supertypes may inherit an unseen signature.
	sig.known = len(sig.params) > 0 || (sym.Decl != nil && len(m.DirectSupertypes(sym)) == 0)
	return sig
}

// signatureParameterOf describes the parameter sym as a call binds it under name.
func (m *Model) signatureParameterOf(sym *symbols.Symbol, name string) signatureParameter {
	param := signatureParameter{
		sym:      sym,
		name:     name,
		typ:      m.featureType(sym),
		prim:     m.PrimTypeOf(sym),
		optional: m.OptionalParameter(sym),
	}
	switch {
	case IsAnything(param.typ):
		param.typ, param.untyped = nil, true
	case param.typ == nil && param.prim == PrimUnknown && !m.declaresType(sym):
		param.untyped = true
	}
	return param
}

// OptionalParameter reports whether a call may omit the parameter: it or a parameter it
// redefines declares a default, or the nearest stated multiplicity admits no value.
func (m *Model) OptionalParameter(sym *symbols.Symbol) bool {
	if value, _ := m.ParameterDefault(sym); value != nil {
		return true
	}
	for _, p := range m.ParameterRedefinitionChain(sym) {
		if _, mult, _ := parameterDeclaration(p); mult != nil {
			r, ok := m.multiplicityRange(mult)
			return ok && r.AllowsNone()
		}
	}
	return false
}

// ParameterDefault is the value the parameter takes when a call binds none: the nearest
// declared along its redefinitions, with the scope it resolves in. Nil when none is.
func (m *Model) ParameterDefault(sym *symbols.Symbol) (ast.Node, *symbols.Scope) {
	for _, p := range m.ParameterRedefinitionChain(sym) {
		if value, _, _ := parameterDeclaration(p); value != nil {
			return value, p.OwnerScope
		}
	}
	return nil, nil
}

// parameterDeclaration is the value and multiplicity a parameter declares as its own,
// whether written as a usage or as a requirement's or case's subject.
func parameterDeclaration(sym *symbols.Symbol) (value ast.Node, mult *ast.Multiplicity, ok bool) {
	if sym == nil {
		return nil, nil, false
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		return decl.Value, decl.Multiplicity, true
	case *ast.SubjectMember:
		return decl.BindingExpr, decl.Multiplicity, true
	}
	return nil, nil, false
}

// ParameterRedefinitionChain is sym followed by the parameters it redefines, explicitly
// or by position, nearest first; each declares a usage or a subject.
func (m *Model) ParameterRedefinitionChain(sym *symbols.Symbol) []*symbols.Symbol {
	var chain []*symbols.Symbol
	visited := map[*symbols.Symbol]bool{}
	for sym != nil && !visited[sym] {
		visited[sym] = true
		if _, _, ok := parameterDeclaration(sym); !ok {
			break
		}
		chain = append(chain, sym)
		redefined := m.RedefinedFeatures(sym)
		if len(redefined) == 0 {
			redefined = m.ImplicitParameterRedefinitions(sym)
		}
		if len(redefined) != 1 {
			break
		}
		sym = redefined[0]
	}
	return chain
}

// declaresType reports whether sym writes a type or generalization, or implicitly
// redefines a parameter it may take one from.
func (m *Model) declaresType(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && GeneralizationKind(rel.Kind) {
			return true
		}
	}
	return len(m.ImplicitParameterRedefinitions(sym)) > 0
}

// applicable reports whether args bind to sig by count, name and type, how surely, and
// whether they leave no default-less parameter unbound (complete).
func (m *Model) applicable(scope *symbols.Scope, sig invocationSignature, args []Argument, mode bindMode) (fit bindFit, complete, ok bool) {
	if !sig.known {
		return fitOpen, true, true
	}
	bound := make([]bool, len(sig.params))
	positional := 0
	fit = fitExact
	for _, arg := range args {
		var i int
		if arg.Name == nil {
			i = positional
			positional++
		} else {
			i = m.parameterIndex(scope, sig, arg.Name)
		}
		if i < 0 || i >= len(sig.params) || bound[i] {
			return fitOpen, false, false
		}
		bound[i] = true
		argFit, binds := m.argumentBinds(arg, sig.params[i], mode)
		if !binds {
			return fitOpen, false, false
		}
		if argFit > fit {
			fit = argFit
		}
	}
	complete = true
	for i, p := range sig.params {
		if !bound[i] && !p.optional {
			complete = false
			break
		}
	}
	return fit, complete, true
}

// namesParameters reports whether every named argument identifies a parameter of sig.
func (m *Model) namesParameters(scope *symbols.Scope, sig invocationSignature, args []Argument) bool {
	if !sig.known {
		return false
	}
	for _, arg := range args {
		if arg.Name != nil && m.parameterIndex(scope, sig, arg.Name) < 0 {
			return false
		}
	}
	return true
}

// parameterIndex is the parameter in sig a named argument binds, or -1: the one
// named as written, else the one the name resolves to within the candidate.
func (m *Model) parameterIndex(scope *symbols.Scope, sig invocationSignature, name *ast.QualifiedName) int {
	if len(name.Parts) == 1 {
		for i, p := range sig.params {
			if p.name == name.Parts[0].Text {
				return i
			}
		}
	}
	target, ok := m.LookupBinding(scope, sig.owner, name)
	if !ok {
		return -1
	}
	for i, p := range sig.params {
		if p.sym == target || (p.sym.Decl != nil && p.sym.Decl == target.Decl) {
			return i
		}
	}
	return -1
}

// argumentBinds reports whether and how surely arg may be passed to p: strictly by conformance
// alone; loosely a non-literal's type only bounds its values, so either direction fits, but
// unrelated declared types never bind, except to a Collection or to Element, which every element is.
func (m *Model) argumentBinds(arg Argument, p signatureParameter, mode bindMode) (bindFit, bool) {
	if arg.Type != nil && p.typ != nil {
		switch {
		case m.Conforms(arg.Type, p.typ):
			return fitExact, true
		case IsElementType(p.typ):
			return fitWiden, true
		case mode == bindLoose && (!arg.Exact && m.Conforms(p.typ, arg.Type) || IsCollection(p.typ)):
			return fitOpen, true
		}
		return fitOpen, false
	}
	if p.untyped {
		return fitWiden, true
	}
	if PrimConforms(arg.Prim, p.prim) {
		switch {
		case arg.Prim == PrimUnknown || p.prim == PrimUnknown:
			return fitOpen, true
		case arg.Prim == p.prim:
			return fitExact, true
		}
		return fitWiden, true
	}
	if mode == bindLoose && !arg.Exact && PrimConforms(p.prim, arg.Prim) {
		return fitOpen, true
	}
	return fitOpen, false
}

func argumentsKnown(args []Argument) bool {
	for _, arg := range args {
		if !arg.known() {
			return false
		}
	}
	return true
}

// mostSpecific returns the index of the one signature at least as specific as
// every other at the bound parameters, or -1 when none or several are.
func (m *Model) mostSpecific(scope *symbols.Scope, sigs []invocationSignature, args []Argument) int {
	best := -1
	for i := range sigs {
		leads := true
		for j := range sigs {
			if i != j && !m.atLeastAsSpecific(scope, sigs[i], sigs[j], args) {
				leads = false
				break
			}
		}
		if !leads {
			continue
		}
		if best >= 0 {
			return -1
		}
		best = i
	}
	return best
}

// unbeaten returns the indices of the signatures no other is strictly more specific
// than at the bound parameters; every index when each is beaten by another.
func (m *Model) unbeaten(scope *symbols.Scope, sigs []invocationSignature, args []Argument) []int {
	var out []int
	for i := range sigs {
		beaten := false
		for j := range sigs {
			if i != j && m.atLeastAsSpecific(scope, sigs[j], sigs[i], args) && !m.atLeastAsSpecific(scope, sigs[i], sigs[j], args) {
				beaten = true
				break
			}
		}
		if !beaten {
			out = append(out, i)
		}
	}
	if len(out) == 0 {
		for i := range sigs {
			out = append(out, i)
		}
	}
	return out
}

// atLeastAsSpecific reports whether a's parameter types conform to b's at every
// parameter the arguments bind; an unknown signature is the most general.
func (m *Model) atLeastAsSpecific(scope *symbols.Scope, a, b invocationSignature, args []Argument) bool {
	if !b.known {
		return true
	}
	if !a.known {
		return false
	}
	positional := 0
	for _, arg := range args {
		var pa, pb int
		if arg.Name == nil {
			pa, pb = positional, positional
			positional++
		} else {
			pa, pb = m.parameterIndex(scope, a, arg.Name), m.parameterIndex(scope, b, arg.Name)
		}
		if pa < 0 || pb < 0 || pa >= len(a.params) || pb >= len(b.params) {
			return false
		}
		if !m.parameterConforms(a.params[pa], b.params[pb]) {
			return false
		}
	}
	return true
}

// parameterConforms reports whether a's type conforms to b's: by declared type where
// both declare one, by scalar lattice otherwise; the untyped parameter is the top type.
func (m *Model) parameterConforms(a, b signatureParameter) bool {
	if b.untyped {
		return true
	}
	if a.untyped {
		return false
	}
	if IsElementType(b.typ) {
		return true
	}
	if a.typ != nil && b.typ != nil {
		return m.Conforms(a.typ, b.typ)
	}
	if b.prim == PrimUnknown {
		return true
	}
	return a.prim != PrimUnknown && PrimConforms(a.prim, b.prim)
}

func containsSymbol(syms []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, s := range syms {
		if s == sym {
			return true
		}
	}
	return false
}

func filterSymbols(syms []*symbols.Symbol, keep func(*symbols.Symbol) bool) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, s := range syms {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}
