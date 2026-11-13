package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ExprTable is a table of lowered expressions by id, index zero "none".
type ExprTable struct {
	entries []*ExprEntry
}

// ExprEntry is one lowered expression with where it came from beside it.
type ExprEntry struct {
	program.Expr
	// Tree is the syntax the expression was lowered from.
	Tree ast.Node
	// Scope is where its names were resolved: the only scope its symbols answer for.
	Scope *symbols.Scope
	// Operands are the entries behind Args, in order.
	Operands []*ExprEntry
}

// Entry is the expression behind an id with its tree and scope, nil for none.
func (t *ExprTable) Entry(id program.ExprID) *ExprEntry {
	if t == nil || int(id) >= len(t.entries) {
		return nil
	}
	return t.entries[id]
}

// At is the lowered expression behind an id, nil for none.
func (t *ExprTable) At(id program.ExprID) *program.Expr {
	if entry := t.Entry(id); entry != nil {
		return &entry.Expr
	}
	return nil
}

// Tree is the syntax the expression behind an id was lowered from, nil for none.
func (t *ExprTable) Tree(id program.ExprID) ast.Node {
	if entry := t.Entry(id); entry != nil {
		return entry.Tree
	}
	return nil
}

// LowerExpr lowers the expression node, written in scope, into a table of its
// own: every name it reads is resolved here, once, and recorded beside its spelling.
func LowerExpr(node ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (*ExprTable, program.ExprID) {
	table := &ExprTable{}
	return table, table.Lower(node, scope, resolver)
}

// Lower adds the expression node, written in scope, to the table and returns its
// id. It never fails: a name resolving to nothing is for the run reading it to report.
func (t *ExprTable) Lower(node ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) program.ExprID {
	l := exprLowering{table: t, scope: scope, resolver: resolver, doc: symbols.DocNameOf(scope)}
	return l.lower(node)
}

// exprLowering lowers the expressions of one tree, all written in one scope.
type exprLowering struct {
	table    *ExprTable
	scope    *symbols.Scope
	resolver *resolve.Resolver
	doc      string
}

// lower appends the record of node, then those of its operands.
func (l *exprLowering) lower(node ast.Node) program.ExprID {
	t := l.table
	if len(t.entries) == 0 {
		t.entries = make([]*ExprEntry, 1, 4)
	}
	entry := &ExprEntry{Tree: node, Scope: l.scope}
	id := program.ExprID(tableIndex(len(t.entries)))
	t.entries = append(t.entries, entry)
	expr := &entry.Expr
	expr.Origin.Doc = l.doc
	var operands []ast.Node
	switch n := node.(type) {
	case *ast.LiteralInteger:
		expr.Kind, expr.Text = program.ExprInt, n.Value
		expr.Const, _ = semantics.ParseInteger(n.Value)
	case *ast.LiteralReal:
		// A literal beyond the default budget is left for a run granted more to read.
		expr.Kind, expr.Text = program.ExprReal, n.Value
		expr.Const, _ = semantics.ParseRational(n.Value, semantics.DefaultMaxIntegerBits)
	case *ast.LiteralBool:
		expr.Kind, expr.Const = program.ExprBool, semantics.Value{Kind: semantics.ValBool, Bool: n.Value}
	case *ast.LiteralString:
		expr.Kind, expr.Text = program.ExprString, source.StringValue(n.Value)
	case *ast.LiteralInfinity:
		expr.Kind, expr.Const = program.ExprInfinity, semantics.Value{Kind: semantics.ValInfinity}
	case *ast.NullExpr:
		expr.Kind = program.ExprNull
	case *ast.FeatureReference:
		expr.Kind = program.ExprRef
		if n == nil {
			return id
		}
		l.reference(expr, n.Name)
	case *ast.QualifiedName:
		expr.Kind = program.ExprRef
		l.reference(expr, n)
	case *ast.FeatureChainExpr:
		expr.Kind = program.ExprChain
		if n == nil {
			return id
		}
		if n.Member != nil && len(n.Member.Parts) > 0 {
			base, parts := ast.FeatureChainBase(n)
			operands, expr.Chain = []ast.Node{base}, spelled(parts)
		}
	case *ast.OperatorExpr:
		expr.Kind, expr.Op, operands = program.ExprOp, OperatorOf(n.Operator), n.Operands
		if n.TypeRef != nil {
			l.typeRef(&expr.Ref, n.TypeRef)
		}
	case *ast.SequenceExpr:
		expr.Kind, operands = program.ExprSequence, n.Elements
	case *ast.IndexExpr:
		expr.Kind, expr.Bracket, operands = program.ExprIndex, n.Bracket, []ast.Node{n.Operand, n.Index}
	case *ast.CollectExpr:
		expr.Kind, operands = program.ExprCollect, []ast.Node{n.Operand, n.Body}
	case *ast.SelectExpr:
		expr.Kind, operands = program.ExprSelect, []ast.Node{n.Operand, n.Body}
	case *ast.InvocationExpr:
		expr.Kind = program.ExprInvoke
		operands = l.invocation(expr, n)
	case *ast.ConstructorExpr:
		expr.Kind = program.ExprConstruct
		written(&expr.Ref, n.Type)
		operands = l.arguments(expr, n.Args, n.NamedArgs)
	case *ast.BodyExpr:
		// A body's result is written in the scope of its parameters, and lowered there when applied.
		expr.Kind = program.ExprBody
	case *ast.MetadataAccessExpr:
		expr.Kind = program.ExprMetadata
		written(&expr.Ref, n.Ref)
	case nil:
		return id
	}
	expr.Origin.Span = node.Span()
	for _, operand := range operands {
		arg := l.lower(operand)
		expr.Args = append(expr.Args, arg)
		entry.Operands = append(entry.Operands, t.entries[arg])
	}
	return id
}

// invocation records what a call applies and returns its operands: the receiver of
// `x->F(a)` first, or for `x.f(a)` the chain applied and what its last member is read from.
func (l *exprLowering) invocation(expr *program.Expr, n *ast.InvocationExpr) []ast.Node {
	written(&expr.Ref, n.Type)
	var operands []ast.Node
	if chain := semantics.ChainCallee(n); chain != nil {
		expr.Applied = true
		operands = []ast.Node{chain, chain.Operand}
	} else if n.Operand != nil {
		expr.Receiver = true
		operands = []ast.Node{n.Operand}
	}
	return append(operands, l.arguments(expr, n.Args, n.NamedArgs)...)
}

// arguments records the labels of an argument list and returns the arguments as
// written, those bound by position before those bound by name.
func (l *exprLowering) arguments(expr *program.Expr, args []ast.Node, named []ast.NamedArg) []ast.Node {
	operands := append([]ast.Node(nil), args...)
	if len(named) == 0 {
		return operands
	}
	expr.Named = make([]program.SymbolRef, len(named))
	for i, arg := range named {
		written(&expr.Named[i], arg.Name)
		operands = append(operands, arg.Value)
	}
	return operands
}

// written records a name as spelled, for the run to resolve.
func written(ref *program.SymbolRef, qn *ast.QualifiedName) {
	if qn == nil || len(qn.Parts) == 0 {
		return
	}
	ref.Name, ref.Span = qn.Text(), qn.Span()
}

// typeRef records the type an operator names (`x as T`, `x istype T`), resolved
// as a reference written in the scope is.
func (l *exprLowering) typeRef(ref *program.SymbolRef, qn *ast.QualifiedName) {
	ref.Name, ref.Span = qn.Text(), qn.Span()
	if l.resolver == nil {
		return
	}
	if sym, ok := l.resolver.ResolveQualified(l.scope, qn); ok {
		ref.Sym = sym
	}
}

// spelled is the segments of a name as written, each for the run to resolve.
func spelled(parts []ast.NameSegment) []program.SymbolRef {
	refs := make([]program.SymbolRef, len(parts))
	for i, part := range parts {
		refs[i].Name, refs[i].Span = part.Text, part.Span
	}
	return refs
}

// reference records a name read as what it names: a simple name by the lookup an
// evaluator makes outward from the scope, a qualified one by reading it there.
func (l *exprLowering) reference(expr *program.Expr, qn *ast.QualifiedName) {
	if qn == nil || len(qn.Parts) == 0 {
		return
	}
	expr.Global = qn.Global
	expr.Ref.Name, expr.Ref.Span = qn.Text(), qn.Span()
	if len(qn.Parts) == 1 {
		if l.resolver != nil && l.scope != nil {
			if sym, ok := l.resolver.LookupName(l.scope, expr.Ref.Name); ok {
				expr.Ref.Sym = sym
			}
		}
		return
	}
	// A qualified name keeps its segments, each resolved as far as the name reaches.
	expr.Path = spelled(qn.Parts)
	if l.resolver == nil {
		return
	}
	reading := l.resolver.ReadQualified(l.scope, qn)
	if sym, ok := reading.Symbol(); ok {
		expr.Ref.Sym = sym
	}
	for i := range expr.Path {
		expr.Path[i].Sym, _ = reading.Part(i)
	}
}

// OperatorOf is the IR's operator for the syntax tree's, which it mirrors by number.
func OperatorOf(kind ast.OperatorKind) program.Operator {
	return program.Operator(kind) // #nosec G115 -- the ladder has far fewer than 256 operators
}

// OperatorKindOf is the syntax tree's operator for the IR's.
func OperatorKindOf(op program.Operator) ast.OperatorKind {
	return ast.OperatorKind(op)
}
