package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Diagnostic codes of the trigger argument rules (SysML v2 §8.3.17,
// TriggerInvocationExpression): the argument of `after` is a DurationValue,
// of `at` a TimeInstantValue, of `when` a Boolean.
const (
	codeTriggerAfterDuration = "trigger-after-duration"
	codeTriggerAtTimeInstant = "trigger-at-time-instant"
	codeTriggerWhenBoolean   = "trigger-when-boolean"

	msgTriggerAfterDuration = "an 'after' trigger's delay must be a DurationValue, found %s: write it with a duration unit (`after 5 [s]`) or name a feature typed by DurationValue"
	msgTriggerAtTimeInstant = "an 'at' trigger's time must be a TimeInstantValue, found %s: name a feature typed by TimeInstantValue or a clock reading such as `TimeOf(this)`"
	msgTriggerWhenBoolean   = "a 'when' trigger's condition must be Boolean, found %s: compare the value (`when x > 3`) or name a feature typed by Boolean"
)

// TriggerArgumentPass types the argument of every `after`, `at` or `when`
// trigger, each gated on its own argument rather than the whole document.
type TriggerArgumentPass struct{}

func (TriggerArgumentPass) Level() PassLevel { return LevelType }

func (TriggerArgumentPass) ElementScoped() { /* marker: per-element gating */ }

func (TriggerArgumentPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	expr := &exprChecker{resolver: ctx.Resolver(), model: ctx.Model(), lang: ctx.Kind}
	// An argument's body members are typed as the type pass does.
	bodies := &typeChecker{resolver: ctx.Resolver(), expr: expr, lang: ctx.Kind}
	expr.walkMembers = bodies.walk
	c := &triggerArgumentChecker{ctx: ctx, expr: expr}
	c.walk(rootScope, root.Members)
	return append(bodies.diags, expr.diagnostics()...)
}

type triggerArgumentChecker struct {
	ctx  *Context
	expr *exprChecker
}

// walk checks every trigger among members, descending through every member
// list they own — the body of a `{ … }` value included.
func (c *triggerArgumentChecker) walk(scope *symbols.Scope, members []ast.Node) {
	w := symbols.ExprWalker{Body: symbols.BodyExprScope, Members: c.walk}
	for _, member := range members {
		node := unwrapType(member)
		switch n := node.(type) {
		case *ast.Usage:
			if n.IsAccept {
				c.check(scope, n)
			}
		case *ast.TransitionMember:
			c.check(scope, n.Trigger)
		case *ast.DeferMember:
			for _, trigger := range n.Triggers {
				c.check(scope, trigger)
			}
		}
		w.Decl(scope, node)
	}
}

// check types one trigger unless its argument rests on a lower-tier fault.
func (c *triggerArgumentChecker) check(scope *symbols.Scope, trigger ast.Node) {
	trigger, arg := triggerArgument(trigger)
	if arg == nil || c.ctx.DownstreamOfFailure(arg) {
		return
	}
	if t, ok := trigger.(*ast.TimeEvent); ok {
		c.expr.checkTimeEvent(scope, t)
		return
	}
	c.expr.checkCondition(scope, arg, codeTriggerWhenBoolean, msgTriggerWhenBoolean, true)
}

// triggerArgument returns a trigger and the expression its rule judges; a
// signal or call trigger (a bare name included) names an event and has none.
func triggerArgument(trigger ast.Node) (ast.Node, ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return nil, nil
	case *ast.ChangeEvent:
		return t, t.Condition
	case *ast.TimeEvent:
		return t, t.Duration
	case *ast.Usage:
		if t.IsAccept && isTriggerValue(t.Value) {
			return triggerArgument(t.Value)
		}
		return nil, nil
	case *ast.FeatureReference, *ast.QualifiedName, *ast.AcceptEvent, *ast.CallEvent:
		return nil, nil
	}
	return trigger, trigger
}

// isTriggerValue reports whether a usage's value is the trigger of an accept
// (`accept after 5 [s]`), checked by the body walk, rather than a bound value.
func isTriggerValue(value ast.Node) bool {
	switch value.(type) {
	case *ast.TimeEvent, *ast.ChangeEvent:
		return true
	}
	return false
}

// checkTimeEvent checks the argument of a time trigger against the library type
// it must be a value of. An argument whose type only evaluation determines is
// not reported.
func (ec *exprChecker) checkTimeEvent(scope *symbols.Scope, t *ast.TimeEvent) {
	if t.Duration == nil {
		return
	}
	ec.infer(scope, t.Duration)
	code, format := codeTriggerAfterDuration, msgTriggerAfterDuration
	if t.Absolute {
		code, format = codeTriggerAtTimeInstant, msgTriggerAtTimeInstant
	}
	c := ec.model.TimeEventConforms(scope, t)
	if !c.Known || c.Holds {
		return
	}
	ec.errorCode(code, t.Duration.Span(), format, c.Found)
}
