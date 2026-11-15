package program

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Calc is a calculation's lowered invocation interface: its positional inputs, its outputs
// and its result, each resolved across the specialization chain the usage inherits from.
type Calc struct {
	Sym *symbols.Symbol
	// Origin names the calc the way a message does: Path and Name are its
	// qualified name, Kind the word diagnostics name the behavior by.
	Origin Origin
	// Name is the qualified name; Kind the behavior's keyword (`calc`, `analysis`,
	// `verification`, a predicate's kind); Label is Kind then Name, as diagnostics say it.
	Name  string
	Kind  string
	Label string
	// Chain is the calcs the invoked one takes members from, most general first,
	// then the invoked calc itself.
	Chain  []*symbols.Symbol
	Params []CalcParam
	// ParamNames are the Params' names by position: the slots an invocation binds.
	ParamNames []string
	// Aliases map the name of a parameter or output redeclared under a new name
	// (`in g :>> x`) to that name, which an inherited body reads it by.
	Aliases map[string]string
	// Outputs are the calc's output features — its `out` parameters and the
	// result parameter a `return` declares — in declaration order.
	Outputs []CalcOutput
	// BodyOwner is the calc whose body the invocation runs: the invoked calc's own
	// when it states one, else the closest inherited body.
	BodyOwner *symbols.Symbol
	// BodyOutputs are the output features some statement of the body assigns on any
	// path: a calc binding its outputs this way states a computation without returning.
	BodyOutputs map[string]bool
	// Result is the expression a binding connector binds to `result`, 0 when none.
	Result ExprID
	// Returns reports a body that returns a value on some path; StepsReturn says
	// the same of its steps alone, the bindings of its outputs left out.
	Returns     bool
	StepsReturn bool
	// StepCount counts the action nodes a case's body performs as its steps.
	StepCount int
}

// CalcParam is one input parameter of a calc, in positional order.
type CalcParam struct {
	Name string // parameter name arguments bind to
	// Default is the value-binding expression used when no argument is passed, 0 if none.
	Default ExprID
	Owner   *symbols.Symbol // the calc that declares the default (a supertype for an inherited one)
	Sym     *symbols.Symbol // the parameter's own symbol, which its effective multiplicity is read off
	// Declarations are the parameter's declarations along the chain, most general
	// first: what a bound value answers to, the closest to the invoked calc winning.
	Declarations []CalcMember
	// IsSubject marks a case's subject parameter, the object the case is about.
	IsSubject bool
	// IsActor marks an actor parameter (SysML v2 §7.19.2, §7.24.2): bound by name
	// like the subject, never by position, and left unvalued when nothing binds it.
	IsActor bool
	// IsCalc marks a calc usage parameter (`in calc f`), which a function value binds.
	IsCalc bool
}

// CalcOutput is one output feature of a calc — an `out` parameter, or the result
// parameter a `return` declares — read by name rather than run as a statement.
type CalcOutput struct {
	Name string // feature name the output answers to, empty for an anonymous result parameter
	// Value is the value-binding expression, 0 when the calc gives the output no value.
	Value ExprID
	Owner *symbols.Symbol // the calc declaring the binding, whose scope it is written in
	// Declarations are the output's declarations along the chain, most general first.
	Declarations []CalcMember
	IsResult     bool // declared with `return`: the result an invocation of the calc yields
	// IsInOut marks an `inout` parameter: an output feature (KerML 7.4.9) bound by
	// the invocation rather than by a declaration or an assignment.
	IsInOut bool
	// IsInitial marks a value declared with `:=`: what the output holds when the
	// body starts, which the body's assignments may then replace.
	IsInitial bool
}

// CalcMember is one declaration of a parameter or output: the feature declared
// and the calc declaring it, under the name the declaration binds.
type CalcMember struct {
	Feature *symbols.Symbol
	Owner   *symbols.Symbol
	Name    string
}

// ResultOutputName is the name an unnamed result parameter answers to.
const ResultOutputName = "result"

// ResultOutputIndex is the position in Outputs of the result parameter a
// `return` declares, -1 when the calc declares none.
func (c *Calc) ResultOutputIndex() int {
	for i := range c.Outputs {
		if c.Outputs[i].IsResult {
			return i
		}
	}
	return -1
}

// ResultOutput returns the result parameter a `return` declares, if the calc has one.
func (c *Calc) ResultOutput() *CalcOutput {
	if i := c.ResultOutputIndex(); i >= 0 {
		return &c.Outputs[i]
	}
	return nil
}

// HasInitialOutput reports whether some output starts the body holding a value
// (`:=`), which the calc yields when the body leaves it as it is.
func (c *Calc) HasInitialOutput() bool {
	for _, out := range c.Outputs {
		if out.IsInitial {
			return true
		}
	}
	return false
}

// IsCase reports whether the calc is a case — an analysis or a verification —
// rather than a plain calc.
func (c *Calc) IsCase() bool {
	return c.Kind == "analysis" || c.Kind == "verification"
}

// Performs reports whether the body's action nodes are performed as steps: a case
// is an action, so its body is; a calc's body performs nothing.
func (c *Calc) Performs() bool {
	return c.IsCase()
}

// BodyScope is the namespace the body's names resolve in.
func (c *Calc) BodyScope() *symbols.Scope {
	if c.BodyOwner != nil {
		return c.BodyOwner.Scope
	}
	return c.Sym.Scope
}

// Parameter is the calc's parameter of that name, nil when it declares none.
func (c *Calc) Parameter(name string) *CalcParam {
	for i := range c.Params {
		if c.Params[i].Name == name {
			return &c.Params[i]
		}
	}
	return nil
}

// HasParameter reports whether the calc declares an input parameter of that name.
func (c *Calc) HasParameter(name string) bool {
	return c.Parameter(name) != nil
}

// SubjectParameter is the parameter the case's subject binds to, if it declares one.
func (c *Calc) SubjectParameter() (*CalcParam, bool) {
	for i := range c.Params {
		if c.Params[i].IsSubject {
			return &c.Params[i], true
		}
	}
	return nil, false
}

// PositionalIndexes are the indexes in Params a positional argument can bind, in
// declaration order: every parameter but the actors, which bind by name only.
func (c *Calc) PositionalIndexes() []int {
	indexes := make([]int, 0, len(c.Params))
	for i := range c.Params {
		if !c.Params[i].IsActor {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

// PositionalArity counts the parameters a positional argument can bind.
func (c *Calc) PositionalArity() int {
	return len(c.PositionalIndexes())
}

// OutputNames renders the output features the calc declares, for a diagnostic
// about one it does not.
func (c *Calc) OutputNames() string {
	names := make([]string, 0, len(c.Outputs))
	for _, out := range c.Outputs {
		if out.Name != "" {
			names = append(names, out.Name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// UsageSpelling writes the calc usage a modeler declares to read the outputs of
// a calc that designates no result, with the inputs it has to bind.
func (c *Calc) UsageSpelling(output string) string {
	var b strings.Builder
	b.WriteString(c.Kind + " c : " + c.Name + " {")
	for _, param := range c.Params {
		if param.IsActor {
			continue
		}
		b.WriteString(" in " + param.Name + " = ...;")
	}
	b.WriteString(" } then read c." + output)
	return b.String()
}

// Canonical is the name the run binds name under: the alias recorded for it, else itself.
func Canonical(aliases map[string]string, name string) string {
	if alias, ok := aliases[name]; ok {
		return alias
	}
	return name
}

// AliasRedefined records that name now binds the feature redefined was known by.
// Aliases already pointing at redefined follow it to name.
func AliasRedefined(aliases map[string]string, redefined, name string) map[string]string {
	if redefined == "" || name == "" || redefined == name {
		return aliases
	}
	if aliases == nil {
		aliases = make(map[string]string)
	}
	for alias, target := range aliases {
		if target == redefined {
			aliases[alias] = name
		}
	}
	aliases[redefined] = name
	delete(aliases, name)
	return aliases
}
