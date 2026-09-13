package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// frameLayout assigns frame slots to the names a body reads: parameters first,
// then each local as declared; the latest binding of a name masks earlier ones.
type frameLayout struct {
	bindings []frameBinding
	// next is the first free slot; size is the widest the frame ever gets.
	next, size int
}

// frameBinding is one visible name and the slot holding its value.
type frameBinding struct {
	name string
	slot int
}

// newFrameLayout lays out a frame holding every parameter, of which the first n
// are bound: a default sees the parameters bound before it, a body all of them.
// A bound parameter answers to each alias of its name as well.
func newFrameLayout(params []string, aliases map[string]string, n int) *frameLayout {
	layout := &frameLayout{next: len(params), size: len(params)}
	for i := 0; i < n && i < len(params); i++ {
		layout.bindings = append(layout.bindings, frameBinding{name: params[i], slot: i})
		for alias, name := range aliases {
			if name == params[i] {
				layout.bindings = append(layout.bindings, frameBinding{name: alias, slot: i})
			}
		}
	}
	return layout
}

// lookup answers the slot the innermost binding of name reads.
func (l *frameLayout) lookup(name string) (int, bool) {
	for i := len(l.bindings) - 1; i >= 0; i-- {
		if l.bindings[i].name == name {
			return l.bindings[i].slot, true
		}
	}
	return 0, false
}

// declare binds name to a fresh slot, masking any earlier binding.
func (l *frameLayout) declare(name string) int {
	slot := l.next
	l.next++
	if l.next > l.size {
		l.size = l.next
	}
	l.bindings = append(l.bindings, frameBinding{name: name, slot: slot})
	return slot
}

// layoutMark is the layout on entering a block, restored on leaving it.
type layoutMark struct{ bindings, next int }

// enter notes the bindings in force; leave forgets the ones a block declared
// and frees their slots for the next block.
func (l *frameLayout) enter() layoutMark { return layoutMark{len(l.bindings), l.next} }

func (l *frameLayout) leave(mark layoutMark) {
	l.bindings = l.bindings[:mark.bindings]
	l.next = mark.next
}

// compiledStmt runs one statement over the frame and reports whether it
// returned, with the value it returned.
type compiledStmt func(ctx *Context, frame []scalar) (scalar, bool, error)

// compileStatements compiles a statement list, reporting whether every path
// through it returns, so a body that may run off its end keeps the evaluator.
func (c *calcCompiler) compileStatements(stmts []lower.Statement, layout *frameLayout, result *scalarCheck) (compiledStmt, bool, error) {
	compiled := make([]compiledStmt, 0, len(stmts))
	returns := false
	for _, stmt := range stmts {
		one, always, err := c.compileStatement(stmt, layout, result)
		if err != nil {
			return nil, false, err
		}
		compiled = append(compiled, one)
		returns = returns || always
	}
	return sequenceStmt(compiled), returns, nil
}

// compileStatement compiles one lowered statement of the subset: a scalar local
// declared with a value, a return, a conditional, a nested block.
func (c *calcCompiler) compileStatement(stmt lower.Statement, layout *frameLayout, result *scalarCheck) (compiledStmt, bool, error) {
	switch s := stmt.(type) {
	case lower.Declare:
		if s.Value == nil {
			return nil, false, ineligible(fmt.Sprintf("local %q declares no value", s.Name))
		}
		if err := plainScope(s.Scope); err != nil {
			return nil, false, err
		}
		// The initializer runs before the name is bound, so it reads what the
		// name meant before this declaration.
		value, err := c.compileNode(s.Value, s.Scope, layout)
		if err != nil {
			return nil, false, err
		}
		check, ok := c.localCheckFor(s)
		if !ok {
			return nil, false, ineligible(fmt.Sprintf("local %q declares a type outside the scalar lattice", s.Name))
		}
		return declareStmt(s, layout.declare(s.Name), value.expr(), check), false, nil
	case lower.Return:
		if s.Value == nil {
			return nil, false, ineligible("return without a value")
		}
		if err := plainScope(s.Scope); err != nil {
			return nil, false, err
		}
		value, err := c.compileNode(s.Value, s.Scope, layout)
		if err != nil {
			return nil, false, err
		}
		return returnStmt(value.expr(), result), true, nil
	case lower.If:
		if s.Condition == nil {
			return nil, false, ineligible("if without a condition")
		}
		if err := plainScope(s.Scope); err != nil {
			return nil, false, err
		}
		cond, err := c.compileNode(s.Condition, s.Scope, layout)
		if err != nil {
			return nil, false, err
		}
		then, thenReturns, err := c.compileBlock(s.Then, layout, result)
		if err != nil {
			return nil, false, err
		}
		if s.Else == nil {
			return ifStmt(cond.expr(), then, nil), false, nil
		}
		els, elseReturns, err := c.compileBlock(*s.Else, layout, result)
		if err != nil {
			return nil, false, err
		}
		return ifStmt(cond.expr(), then, els), thenReturns && elseReturns, nil
	case lower.Block:
		return c.compileBlock(s, layout, result)
	default:
		return nil, false, ineligible(fmt.Sprintf("statement %q is outside the compiled subset", stmtLabel(stmt)))
	}
}

// compileBlock compiles a block in a layout of its own, so a local it declares
// is visible to its statements alone.
func (c *calcCompiler) compileBlock(block lower.Block, layout *frameLayout, result *scalarCheck) (compiledStmt, bool, error) {
	if block.Graph != nil {
		return nil, false, ineligible("block with a control flow graph")
	}
	mark := layout.enter()
	defer layout.leave(mark)
	return c.compileStatements(block.Statements, layout, result)
}

// plainScope admits an expression written in a calc body or a block of it, not
// one inside an expression body, whose declarations the frame does not hold.
func plainScope(scope *symbols.Scope) error {
	if scope == nil {
		return ineligible("expression without a scope")
	}
	for s := scope; s != nil && s.BodyLocal(); s = s.Parent() {
		if _, ok := s.Node().(*ast.BodyExpr); ok {
			return ineligible("expression inside an expression body")
		}
	}
	return nil
}

// sequenceStmt runs statements in order, stopping at the first that returns.
func sequenceStmt(stmts []compiledStmt) compiledStmt {
	switch len(stmts) {
	case 0:
		return func(*Context, []scalar) (scalar, bool, error) { return scalar{}, false, nil }
	case 1:
		return stmts[0]
	}
	return func(ctx *Context, frame []scalar) (scalar, bool, error) {
		for _, stmt := range stmts {
			v, returned, err := stmt(ctx, frame)
			if err != nil || returned {
				return v, returned, err
			}
		}
		return scalar{}, false, nil
	}
}

// localCheckFor decides the declaration of a body-local for scalars; a local
// declaring no feature holds anything.
func (c *calcCompiler) localCheckFor(s lower.Declare) (*scalarCheck, bool) {
	target, ok := c.ctx.writeTargetIn(s.Scope, s.Name)
	if !ok {
		return nil, true
	}
	check, ok := c.scalarCheckFor(&calcMemberDecl{Target: target, Owner: c.shape.BodyOwner, multStated: target.multStated})
	if !ok {
		return nil, false
	}
	return &check, true
}

// declareStmt evaluates a local's initializer into its slot and holds it to the
// local's declaration, wording a failure as the evaluator's declaration does.
func declareStmt(s lower.Declare, slot int, value compiledExpr, check *scalarCheck) compiledStmt {
	return func(ctx *Context, frame []scalar) (scalar, bool, error) {
		v, err := value(ctx, frame)
		if err != nil {
			return scalar{}, false, fmt.Errorf("eval declaration %s: %w", s.Name, err)
		}
		if check != nil && !check.accepts(v) {
			boxed := v.boxed()
			if err := ctx.checkBodyDeclaration(s.Scope, calcBodyDescription, s.Name, &boxed); err != nil {
				return scalar{}, false, err
			}
		}
		frame[slot] = v
		return scalar{}, false, nil
	}
}

// returnStmt evaluates the returned expression and holds it to the result
// parameter's declaration, as the evaluator's host does on accepting a return.
func returnStmt(value compiledExpr, result *scalarCheck) compiledStmt {
	return func(ctx *Context, frame []scalar) (scalar, bool, error) {
		v, err := value(ctx, frame)
		if err != nil {
			return scalar{}, false, fmt.Errorf("evaluating the returned expression: %w", err)
		}
		if result != nil && !result.accepts(v) {
			if err := result.refuse(ctx, v, func() string { return "result" }); err != nil {
				return scalar{}, false, err
			}
		}
		return v, true, nil
	}
}

// ifStmt runs the branch the condition selects; a condition that is no Boolean
// is reported as the evaluator's calculation body reports it.
func ifStmt(cond compiledExpr, then, els compiledStmt) compiledStmt {
	return func(ctx *Context, frame []scalar) (scalar, bool, error) {
		cv, err := cond(ctx, frame)
		if err != nil {
			return scalar{}, false, fmt.Errorf("eval condition of 'if': %w", err)
		}
		if cv.kind != scalarBool {
			return scalar{}, false, fmt.Errorf("%s: condition of 'if' must evaluate to a Boolean, got %s",
				calcBodyDescription, ValConst)
		}
		if cv.truth() {
			return then(ctx, frame)
		}
		if els == nil {
			return scalar{}, false, nil
		}
		return els(ctx, frame)
	}
}

// bodyStmt is a statement list every path of which returns, run as a body: the
// value returned is the body's, a fallthrough being unreachable by construction.
func bodyStmt(stmts compiledStmt) compiledExpr {
	return func(ctx *Context, frame []scalar) (scalar, error) {
		v, returned, err := stmts(ctx, frame)
		if err != nil {
			return scalar{}, err
		}
		if !returned {
			return scalar{}, fmt.Errorf("%w: compiled body ended without a return", ErrNoResultExpression)
		}
		return v, nil
	}
}
