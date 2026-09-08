package runtime

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// holdingFeatures names the features of typ whose stated value may hold the objects of the
// feature at path — a feature's name, or a chain of names from one of typ's features into the
// objects it holds — so their types classify them (KerML 1.0 §7.3.4.1); memoized per type.
func (ctx *Context) holdingFeatures(typ *symbols.Symbol, path string) []string {
	index, ok := ctx.holders[typ]
	if !ok {
		index = make(map[string][]string)
		features := ctx.FeaturesOf(typ)
		data := make(map[string]bool, len(features))
		for i := range features {
			data[features[i].Name] = ctx.holdsData(&features[i])
		}
		for i := range features {
			feat := &features[i]
			if feat.DefaultValue == nil || !ctx.valueBinds(feat) || data[feat.Name] {
				continue
			}
			for _, held := range ctx.mentionedFeatures(typ, feat) {
				if root, _, _ := strings.Cut(held, "."); root != feat.Name && !data[root] {
					index[held] = append(index[held], feat.Name)
				}
			}
		}
		ctx.holders[typ] = index
	}
	return index[path]
}

// holdsData reports a feature whose values are data, never objects: one declared an attribute
// by keyword (`:>> x` alone declares no kind), or typed by a data type. It classifies nothing.
func (ctx *Context) holdsData(feat *EffectiveFeature) bool {
	if feat.Symbol != nil {
		usage, ok := feat.Symbol.Decl.(*ast.Usage)
		if ok && usage.Kind == ast.UsageAttribute && usage.Keyword != "" {
			return true
		}
	}
	return ctx.model.IsDataType(feat.Type)
}

// mentionedFeatures names the features of typ whose values feat's value may pass on as its
// own, wherever its expression refers to them: a feature by name, a chain into the objects
// one holds as the names along it.
func (ctx *Context) mentionedFeatures(typ *symbols.Symbol, feat *EffectiveFeature) []string {
	scope := feat.DefaultScope()
	if scope == nil {
		return nil
	}
	var paths []string
	seen := make(map[string]bool)
	for _, passed := range ctx.passedFeatureReferences(scope, feat.DefaultValue) {
		sym := passed.symbol(ctx)
		if sym == nil {
			continue
		}
		if name, ok := ctx.denotedFeature(typ, sym); ok {
			path := strings.Join(append([]string{name}, passed.members...), ".")
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// passedReference is a feature reference an expression may answer the values of, in the
// scope it is written in: a body's parameters are looked up in the body's own. members
// are the names a chain reads from the objects the reference holds, none for the
// reference's own values.
type passedReference struct {
	scope   *symbols.Scope
	ref     *ast.FeatureReference
	members []string
}

// symbol resolves the reference where it is written.
func (p passedReference) symbol(ctx *Context) *symbols.Symbol {
	return ctx.referencedSymbol(p.scope, p.ref.Name)
}

// through is the reference read on through further members.
func (p passedReference) through(members []string) passedReference {
	if len(members) == 0 {
		return p
	}
	p.members = append(append([]string{}, p.members...), members...)
	return p
}

// readThrough is refs each read on through members.
func readThrough(refs []passedReference, members []string) []passedReference {
	if len(members) == 0 {
		return refs
	}
	out := make([]passedReference, len(refs))
	for i, ref := range refs {
		out[i] = ref.through(members)
	}
	return out
}

// passedFeatureReferences lists the feature references an expression written in scope
// may answer the values of as its own: a branch chosen, an element indexed or selected,
// an argument a call returns, a collected element the body answers, a chain's last member
// read from what its operand passes on — not a condition or an operand computed from.
func (ctx *Context) passedFeatureReferences(scope *symbols.Scope, expr ast.Node) []passedReference {
	var refs []passedReference
	var visit func(nodes ...ast.Node)
	visit = func(nodes ...ast.Node) {
		for _, n := range nodes {
			switch n := n.(type) {
			case *ast.FeatureReference:
				refs = append(refs, passedReference{scope: scope, ref: n})
			case *ast.FeatureChainExpr:
				if n.Member == nil || len(n.Member.Parts) == 0 {
					continue
				}
				base, parts := chainBase(n)
				members := make([]string, len(parts))
				for i, part := range parts {
					members[i] = part.Text
				}
				refs = append(refs, readThrough(ctx.passedFeatureReferences(scope, base), members)...)
			case *ast.SequenceExpr:
				visit(n.Elements...)
			case *ast.OperatorExpr:
				switch n.Operator {
				case ast.OpConditional:
					if len(n.Operands) > 1 {
						visit(n.Operands[1:]...)
					}
				case ast.OpNullCoalesce, ast.OpAs:
					visit(n.Operands...)
				}
			case *ast.IndexExpr:
				visit(n.Operand)
			case *ast.InvocationExpr:
				visit(ctx.returnedArguments(scope, n)...)
			case *ast.CollectExpr:
				refs = append(refs, ctx.collectedReferences(scope, n.Operand, n.Body)...)
			case *ast.SelectExpr:
				visit(n.Operand)
			case *ast.BodyExpr:
				refs = append(refs, ctx.bodyReferences(scope, n, nil)...)
			case *ast.Usage:
				visit(n.Value)
			}
		}
	}
	visit(expr)
	return refs
}

// collectedReferences lists what `operand.body` may answer: what the body's result
// answers, the operand's values where that is an element; every value of the operand
// for a body not written there, whose result is not known here.
func (ctx *Context) collectedReferences(scope *symbols.Scope, operand, body ast.Node) []passedReference {
	b, ok := body.(*ast.BodyExpr)
	if !ok {
		return ctx.passedFeatureReferences(scope, operand)
	}
	return ctx.bodyReferences(scope, b, operand)
}

// bodyReferences lists what a body expression's result may answer, its parameters
// standing for the elements of operand — for none when operand is nil — and the
// features it declares for their values.
func (ctx *Context) bodyReferences(scope *symbols.Scope, body *ast.BodyExpr, operand ast.Node) []passedReference {
	bodyScope := symbols.BodyExprScope(scope, body)
	if bodyScope == scope {
		return ctx.passedFeatureReferences(scope, body.Result)
	}
	var refs []passedReference
	seen := make(map[*symbols.Symbol]bool)
	var expand func(passed []passedReference)
	expand = func(passed []passedReference) {
		for _, p := range passed {
			sym := p.symbol(ctx)
			if sym == nil || sym.OwnerScope != bodyScope {
				refs = append(refs, p)
				continue
			}
			if seen[sym] {
				continue
			}
			seen[sym] = true
			if sym.Decl == body {
				if operand != nil {
					refs = append(refs, readThrough(ctx.passedFeatureReferences(scope, operand), p.members)...)
				}
				continue
			}
			if value := ctx.extractDefaultValue(sym); value != nil {
				expand(readThrough(ctx.passedFeatureReferences(bodyScope, value), p.members))
			}
		}
	}
	expand(ctx.passedFeatureReferences(bodyScope, body.Result))
	return refs
}

// returnedArguments lists the arguments of a call its result may consist of: those bound
// to parameters a calc's returns pass on; every one for a function whose body is not
// written in the model; none for a call that denotes nothing or computes no result.
func (ctx *Context) returnedArguments(scope *symbols.Scope, call *ast.InvocationExpr) []ast.Node {
	var target *invocationTarget
	if chain := passes.ChainCallee(call); chain != nil {
		target = ctx.chainTarget(scope, chain, call.NamedArgs)
	} else {
		target = NewEvalContext(ctx, scope).invocationTarget(call)
	}
	positional := passes.InvocationArgs(call)
	var args []ast.Node
	switch {
	case target.shape != nil:
		passed := ctx.returnedParameters(target.shape)
		for i, arg := range positional {
			if i < len(target.shape.ParamNames) && passed[target.shape.ParamNames[i]] {
				args = append(args, arg)
			}
		}
		for i, named := range call.NamedArgs {
			if passed[target.names[i]] {
				args = append(args, named.Value)
			}
		}
	case target.builtin != nil || target.library != nil:
		args = append(args, positional...)
		for _, named := range call.NamedArgs {
			args = append(args, named.Value)
		}
	}
	return args
}

// chainTarget is how a call `x.f(a)` is applied as far as the model states it: by the
// shape of the calc feature the chain denotes, which the named arguments bind parameters of.
func (ctx *Context) chainTarget(scope *symbols.Scope, chain *ast.FeatureChainExpr, named []ast.NamedArg) *invocationTarget {
	target := &invocationTarget{qualName: chainText(chain)}
	if sym, ok := ctx.resolver.ResolveTarget(scope, chain); ok && sym != nil {
		if shape, err := ctx.calcShapeOf(sym); err == nil {
			target.calc, target.shape = sym, shape
		}
	}
	if len(named) > 0 {
		target.names, target.unbound = ctx.boundParameterNames(scope, target.calc, named)
	}
	return target
}

// returnedAnalysis is the parameters a calc shape's returns pass on, as far as known.
// Shapes calling each other are analysed together: each is provisional until the first
// of them entered — the root of the cycle — is stable, when all of them are final.
type returnedAnalysis struct {
	passed map[string]bool
	done   bool
	// active is the shape's position on the analysis stack while its returns are
	// followed, -1 otherwise; low is the lowest position a call reached from it.
	active, low int
}

// returnedParameters names the parameters of a calc whose values its result may consist
// of: those its return expressions pass on, a `for` variable standing for its collection.
// Memoized per shape; a call within a cycle reads the set as far as it is known, and the
// cycle's root iterates every shape of it until none grows.
func (ctx *Context) returnedParameters(shape *calcShape) map[string]bool {
	a, ok := ctx.returnedParams[shape]
	if !ok {
		a = &returnedAnalysis{passed: make(map[string]bool), active: -1}
		ctx.returnedParams[shape] = a
	}
	if a.done {
		return a.passed
	}
	if a.active >= 0 {
		top := ctx.returnedStack[len(ctx.returnedStack)-1]
		top.low = min(top.low, a.active)
		return a.passed
	}
	a.active, a.low = len(ctx.returnedStack), len(ctx.returnedStack)
	ctx.returnedStack = append(ctx.returnedStack, a)
	provisional := len(ctx.returnedProvisional)
	params := make(map[string]bool, len(shape.ParamNames))
	for _, name := range shape.ParamNames {
		params[name] = true
	}
	for {
		known := len(a.passed)
		ctx.collectReturnedParameters(shape, a.passed, params)
		if len(a.passed) == known {
			break
		}
	}
	index := a.active
	ctx.returnedStack = ctx.returnedStack[:index]
	a.active = -1
	if a.low < index {
		caller := ctx.returnedStack[index-1]
		caller.low = min(caller.low, a.low)
		ctx.returnedProvisional = append(ctx.returnedProvisional, a)
		return a.passed
	}
	for _, member := range ctx.returnedProvisional[provisional:] {
		member.done = true
	}
	ctx.returnedProvisional = ctx.returnedProvisional[:provisional]
	a.done = true
	return a.passed
}

// valueSource is an expression a local's value may come from, in the scope it is written in.
type valueSource struct {
	scope *symbols.Scope
	expr  ast.Node
}

// collectReturnedParameters adds to passed the parameters of shape its returns and result
// binding pass on, directly or through the locals its body writes them to.
func (ctx *Context) collectReturnedParameters(shape *calcShape, passed, params map[string]bool) {
	var returns []valueSource
	locals := make(map[*symbols.Symbol][]valueSource)
	written := func(in *symbols.Scope, name string, source valueSource) {
		if source.expr == nil {
			return
		}
		if sym, ok := ctx.resolver.LookupName(in, name); ok {
			locals[sym] = append(locals[sym], source)
		}
	}
	var walk func(stmts []lower.Statement)
	walk = func(stmts []lower.Statement) {
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case lower.Return:
				returns = append(returns, valueSource{scope: s.Scope, expr: s.Value})
			case lower.Declare:
				written(s.Scope, s.Name, valueSource{scope: s.Scope, expr: s.Value})
			case lower.Assign:
				if s.Chain == nil {
					written(s.Scope, s.Target, valueSource{scope: s.Scope, expr: s.Value})
				}
			case lower.If:
				walk(s.Then.Steps())
				if s.Else != nil {
					walk(s.Else.Steps())
				}
			case lower.Loop:
				if s.Variable != "" {
					written(s.Body.Scope, s.Variable, valueSource{scope: s.Scope, expr: s.Collection})
				}
				walk(s.Body.Steps())
			case lower.Block:
				walk(s.Steps())
			}
		}
	}
	walk(shape.Body)
	for _, binding := range shape.Bindings {
		for i := range binding.Ends {
			if binding.Ends[i].Path == "result" && binding.Ends[1-i].Expr != nil {
				returns = append(returns, valueSource{scope: binding.Scope, expr: binding.Ends[1-i].Expr})
			}
		}
	}

	// A chain from a parameter passes on objects the parameter holds, not the parameter;
	// a local written from itself is followed once.
	following := make(map[*symbols.Symbol]bool)
	var follow func(source valueSource, members []string)
	follow = func(source valueSource, members []string) {
		if source.scope == nil {
			return
		}
		for _, ref := range readThrough(ctx.passedFeatureReferences(source.scope, source.expr), members) {
			sym := ref.symbol(ctx)
			if sym == nil || following[sym] {
				continue
			}
			if name, ok := ctx.denotedFeature(shape.Sym, sym); ok && params[name] {
				if len(ref.members) == 0 {
					passed[name] = true
				}
				continue
			}
			following[sym] = true
			for _, from := range locals[sym] {
				follow(from, ref.members)
			}
			delete(following, sym)
		}
	}
	for _, ret := range returns {
		follow(ret, nil)
	}
}

// referencedSymbol resolves a name the way the evaluator does: a single part by
// lookup in the scope, more through the qualified-name reader.
func (ctx *Context) referencedSymbol(scope *symbols.Scope, qn *ast.QualifiedName) *symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}
	if len(qn.Parts) == 1 && !qn.Global {
		if sym, ok := ctx.resolver.LookupName(scope, qn.Parts[0].Text); ok {
			return sym
		}
		return nil
	}
	sym, _ := ctx.resolver.ReadQualified(scope, qn).Symbol()
	return sym
}

// materializeHolders reads the features whose value lists the named feature of inst — on inst,
// or on an object holding it as a chain through the features leading to inst — before its
// objects are made, so every holding feature classifies them whichever is read first.
// A holder that fails to materialize holds nothing: its error is its own, reported when it is read.
func (ctx *Context) materializeHolders(inst *Instance, name string) {
	if fv, ok := inst.FeatureValues[name]; !ok || ctx.holdsData(fv.Feature) {
		return
	}
	path := name
	for holding := inst; holding != nil; {
		for _, typ := range holding.types() {
			for _, holder := range ctx.holdingFeatures(typ, path) {
				fv, ok := holding.FeatureValues[holder]
				if !ok || fv.Materialized ||
					ctx.derivingFeatureValues[featureValueRef{instance: holding.ID, feature: holder}] {
					continue
				}
				_, _ = holding.GetFeatureValue(ctx, holder)
			}
		}
		owner, feature := holding.Owner()
		if owner == nil || feature == "" {
			break
		}
		path, holding = feature+"."+path, owner
	}
}
