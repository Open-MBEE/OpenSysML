// Package queryplan compiles native SysML document queries into immutable plans.
package queryplan

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Operation is one closed document-query planning operation.
type Operation string

const (
	OperationParameter     Operation = "parameter"
	OperationElement       Operation = "element"
	OperationLiteral       Operation = "literal"
	OperationSequence      Operation = "sequence"
	OperationInvoke        Operation = "invoke"
	OperationOwnedElements Operation = "owned-elements"
	OperationDescendants   Operation = "descendants"
	OperationAncestors     Operation = "ancestors"
	// OperationObjects enumerates the objects a session holds, by type.
	OperationObjects Operation = "objects"
	// OperationVerdicts checks the assertions about each source row's object.
	OperationVerdicts Operation = "verdicts"
	// OperationStates lists the active states of each source row's object.
	OperationStates Operation = "states"
	// OperationInState lists the session's objects whose machine is in a state.
	OperationInState Operation = "in-state"
	// OperationEvents reads the session's trace as time-ordered rows.
	OperationEvents          Operation = "events"
	OperationRelatedElements Operation = "related-elements"
	OperationWhereType       Operation = "where-type"
	OperationWhereMetadata   Operation = "where-metadata"
	OperationWhereName       Operation = "where-name"
	OperationWhereFeature    Operation = "where-feature"
	OperationOrderBy         Operation = "order-by"
	OperationProject         Operation = "project"
	OperationColumn          Operation = "column"
	OperationRowProperty     Operation = "row-property"
	OperationColumnOperator  Operation = "column-operator"
	// OperationRelatedColumn projects the elements a relationship reaches from each row.
	OperationRelatedColumn Operation = "related-column"
	// OperationWhereRelated keeps the source rows by whether a related element exists.
	OperationWhereRelated Operation = "where-related"
	// OperationExcept and OperationUnion are the ordered set operations over rows.
	OperationExcept Operation = "except"
	OperationUnion  Operation = "union"
)

// LiteralKind classifies a literal retained in a query plan.
type LiteralKind string

const (
	LiteralString   LiteralKind = "string"
	LiteralInteger  LiteralKind = "integer"
	LiteralReal     LiteralKind = "real"
	LiteralBoolean  LiteralKind = "boolean"
	LiteralInfinity LiteralKind = "infinity"
	LiteralNull     LiteralKind = "null"
	// LiteralQuantity is a magnitude in a unit, `2.5 [s]`, folded at planning.
	LiteralQuantity LiteralKind = "quantity"
)

// Multiplicity is a query parameter's effective cardinality.
type Multiplicity struct {
	Lower         int64
	Upper         int64
	UpperInfinite bool
	Known         bool
}

// Parameter is one typed query input or result. A defaulted input carries its
// compiled default and the query whose declaration supplied it.
type Parameter struct {
	Name         string
	Type         string
	Multiplicity Multiplicity
	HasDefault   bool
	Default      Expression
	DefaultQuery string
	Origin       provenance.Origin
}

func (p Parameter) clone() Parameter {
	p.Default = p.Default.clone()
	return p
}

// Argument is one normalized invocation argument.
type Argument struct {
	Name  string
	Named bool
	Value Expression
}

// Expression is one immutable node of a compiled query plan.
type Expression struct {
	operation Operation
	target    string
	literal   LiteralKind
	value     string
	quantity  *semantics.Quantity
	element   *symbols.Symbol
	arguments []Argument
	origin    provenance.Origin
}

// Operation returns the operation this expression performs.
func (e Expression) Operation() Operation { return e.operation }

// Target returns the parameter or query definition named by the expression.
func (e Expression) Target() string { return e.target }

// Literal returns the kind and source value of a literal expression.
func (e Expression) Literal() (LiteralKind, string) { return e.literal, e.value }

// Quantity returns an independent copy of a quantity literal's folded value and
// whether the expression is one.
func (e Expression) Quantity() (semantics.Quantity, bool) {
	if e.literal != LiteralQuantity || e.quantity == nil {
		return semantics.Quantity{}, false
	}
	return e.quantity.Clone(), true
}

// Element returns the model element an element expression binds.
func (e Expression) Element() (*symbols.Symbol, bool) {
	return e.element, e.operation == OperationElement && e.element != nil
}

// Arguments returns an independent copy of the expression's arguments.
func (e Expression) Arguments() []Argument {
	out := make([]Argument, len(e.arguments))
	for i, arg := range e.arguments {
		out[i] = Argument{Name: arg.Name, Named: arg.Named, Value: arg.Value.clone()}
	}
	return out
}

// Origin returns the source expression that produced this plan node.
func (e Expression) Origin() provenance.Origin { return e.origin }

func (e Expression) clone() Expression {
	e.arguments = e.Arguments()
	return e
}

// Definition is one reusable query definition in a compiled program.
type Definition struct {
	name         string
	parameters   []Parameter
	result       Parameter
	expression   Expression
	dependencies []string
	origin       provenance.Origin
}

// Name returns the definition's fully qualified name.
func (d Definition) Name() string { return d.name }

// Parameters returns the definition's ordered effective inputs.
func (d Definition) Parameters() []Parameter {
	return cloneParameters(d.parameters)
}

func cloneParameters(parameters []Parameter) []Parameter {
	out := make([]Parameter, len(parameters))
	for i, parameter := range parameters {
		out[i] = parameter.clone()
	}
	return out
}

// Result returns the definition's typed result parameter.
func (d Definition) Result() Parameter { return d.result }

// Expression returns an independent copy of the compiled result expression.
func (d Definition) Expression() Expression { return d.expression.clone() }

// Dependencies returns the invoked query definitions in encounter order.
func (d Definition) Dependencies() []string {
	return append([]string(nil), d.dependencies...)
}

// Origin returns the definition's declaration location.
func (d Definition) Origin() provenance.Origin { return d.origin }

func (d Definition) clone() Definition {
	d.parameters = d.Parameters()
	d.expression = d.Expression()
	d.dependencies = d.Dependencies()
	return d
}

// Program is an immutable dependency-ordered query plan.
type Program struct {
	entry       string
	definitions []Definition
}

// Entry returns the fully qualified entry query name.
func (p *Program) Entry() string {
	if p == nil {
		return ""
	}
	return p.entry
}

// Definitions returns dependencies before their consumers, each exactly once.
func (p *Program) Definitions() []Definition {
	if p == nil {
		return nil
	}
	out := make([]Definition, len(p.definitions))
	for i, definition := range p.definitions {
		out[i] = definition.clone()
	}
	return out
}

// The aggregates a related column reduces its traversal to.
const (
	RelatedAggregateList  = "list"
	RelatedAggregateCount = "count"
	RelatedAggregateAny   = "any"
)

// RelatedAggregateSupported reports whether aggregate names a related-column aggregate.
func RelatedAggregateSupported(aggregate string) bool {
	switch aggregate {
	case RelatedAggregateList, RelatedAggregateCount, RelatedAggregateAny:
		return true
	}
	return false
}
