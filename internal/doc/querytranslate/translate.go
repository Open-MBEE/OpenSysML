package querytranslate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// SourceParameter is the entry query's parameter a frontend binds the elements
// the query considers to: the model's, or the scope's and everything nested.
const SourceParameter = "source"

// Context is what a translation reads of the model: its index and semantics,
// for the property table and the metaclasses a type test may name.
type Context struct {
	Index *symbols.Index
	Model *semantics.Model
	// Table is the property table; built from Index and Model when nil.
	Table *Table
}

// Plan is a translated query: the program to execute, bound at
// SourceParameter, and the columns its rows report, in order.
type Plan struct {
	Program *queryplan.Program
	Columns []string
	// Objects reports a query over the session's held objects, whose root is
	// Objects rather than the bound source.
	Objects bool
}

// Translate compiles a query to a plan named name, refusing before any element
// is read what no model could evaluate: an unknown property, an operator its
// values cannot take, a value it cannot compare.
func Translate(q Query, ctx Context, name string) (*Plan, error) {
	table := ctx.Table
	if table == nil {
		table = NewTable(ctx.Index, ctx.Model)
	}
	t := &translator{ctx: ctx, table: table}
	columns, err := t.columns(q.Select)
	if err != nil {
		return nil, err
	}
	for _, term := range q.OrderBy {
		if err := t.knownProperty(term.Property, "oslc.orderBy"); err != nil {
			return nil, err
		}
	}
	root, where, objects, err := t.root(q.Where)
	if err != nil {
		return nil, err
	}
	if where != nil {
		if err := t.validate(where, ""); err != nil {
			return nil, err
		}
	}
	expression := root
	if where != nil {
		expression = t.constraint(where, root, "")
	}
	if len(q.SearchTerms) > 0 {
		expression = t.search(expression, q.SearchTerms)
	}
	expression = operate(queryplan.OperationProject,
		arg("source", expression),
		arg("properties", stringList(columns)),
		arg("tolerate", queryplan.BooleanLiteral(true, origin)))
	for i := len(q.OrderBy) - 1; i >= 0; i-- {
		term := q.OrderBy[i]
		direction := "ascending"
		if term.Desc {
			direction = "descending"
		}
		expression = operate(queryplan.OperationOrderBy,
			arg("source", expression),
			arg("property", str(term.Property)),
			arg("direction", str(direction)),
			arg("missing", str("first")),
			arg("multiple", str("first")))
	}
	elements := queryplan.Multiplicity{Lower: 0, UpperInfinite: true, Known: true}
	parameters := []queryplan.Parameter{{Name: SourceParameter, Type: "Element", Multiplicity: elements}}
	result := queryplan.Parameter{Name: "result", Type: "Element", Multiplicity: elements}
	return &Plan{
		Program: queryplan.NewProgram(name, parameters, result, expression, origin),
		Columns: columns,
		Objects: objects,
	}, nil
}

var origin symbols.Origin

type translator struct {
	ctx   Context
	table *Table
}

// columns expands the selection to the properties to project: the universal
// properties for the wildcard, dotted paths for nested selections.
func (t *translator) columns(selections []Selection) ([]string, error) {
	if len(selections) == 0 {
		return DefaultProjection(), nil
	}
	var out []string
	seen := map[string]bool{}
	var walk func(prefix string, selections []Selection) error
	walk = func(prefix string, selections []Selection) error {
		for _, selection := range selections {
			if selection.Property == SelectAll {
				for _, property := range DefaultProjection() {
					if !seen[prefix+property] {
						seen[prefix+property] = true
						out = append(out, prefix+property)
					}
				}
				continue
			}
			if err := t.knownProperty(prefix+selection.Property, "select"); err != nil {
				return err
			}
			if len(selection.Nested) > 0 {
				if t.table.kind(selection.Property) != kindElement && t.table.kind(selection.Property) != kindUnknown {
					return errorf(ErrUnorderedProperty,
						"%s holds no elements: only an element-valued property can be selected into", prefix+selection.Property)
				}
				if err := walk(prefix+selection.Property+".", selection.Nested); err != nil {
					return err
				}
				continue
			}
			if seen[prefix+selection.Property] {
				return errorf(ErrMalformed, "select names %q twice", prefix+selection.Property)
			}
			seen[prefix+selection.Property] = true
			out = append(out, prefix+selection.Property)
		}
		return nil
	}
	if err := walk("", selections); err != nil {
		return nil, err
	}
	return out, nil
}

// knownProperty refuses a property the table does not know, naming the nearest
// spelling it does.
func (t *translator) knownProperty(property, where string) error {
	if t.table.Known(property) {
		return nil
	}
	names := t.table.Names()
	message := fmt.Sprintf("unknown query property %q in %s", property, where)
	if strings.Contains(property, ".") {
		message = fmt.Sprintf("unknown query property %q in %s: a segment of the path names no property", property, where)
	}
	return &Error{
		Kind:       ErrUnknownProperty,
		Message:    suggest.With(message, property, suggest.Nearest(property, names)),
		Candidates: names,
	}
}

// root is the row set the filter narrows: the bound source, or the objects an
// instance selection at the top of the conjunction names, with the rest of the
// filter to apply.
func (t *translator) root(where Constraint) (queryplan.Expression, Constraint, bool, error) {
	source := queryplan.ParameterRef(SourceParameter, origin)
	var instances []string
	var rest []Constraint
	switch c := where.(type) {
	case nil:
		return source, nil, false, nil
	case Instances:
		instances = c.Elements
	case Conjunction:
		for _, term := range c.Terms {
			if selection, ok := term.(Instances); ok {
				instances = append(instances, selection.Elements...)
				continue
			}
			rest = append(rest, term)
		}
	default:
		return source, where, false, nil
	}
	if len(instances) == 0 {
		return source, where, false, nil
	}
	var root queryplan.Expression
	for _, element := range instances {
		objects := operate(queryplan.OperationObjects, arg("type", str(element)))
		if root.Operation() == "" {
			root = objects
			continue
		}
		root = operate(queryplan.OperationUnion, arg("source", root), arg("other", objects))
	}
	switch len(rest) {
	case 0:
		return root, nil, true, nil
	case 1:
		return root, rest[0], true, nil
	default:
		return root, Conjunction{Terms: rest}, true, nil
	}
}

// validate refuses, before any element is read, what no model could evaluate.
func (t *translator) validate(c Constraint, prefix string) error {
	switch c := c.(type) {
	case Comparison:
		return t.validateComparison(c, prefix)
	case TypeTest:
		if prefix != "" && !c.Exact {
			return errorf(ErrUnorderedProperty, "a scoped type test compares the metaclass of %s exactly; instanceOf cannot be scoped", strings.TrimSuffix(prefix, "."))
		}
		return t.validateTypes(c.Types, c.Exact)
	case Instances:
		return errorf(ErrUnsupportedInstanceTerm,
			"rdf:type=elmt:… selects the objects of an element; it is a term of the query itself, not of a scoped, negated or alternative part")
	case Scoped:
		if err := t.knownProperty(prefix+c.Property, "a scoped term"); err != nil {
			return err
		}
		if kind := t.table.kind(c.Property); kind != kindElement && kind != kindUnknown {
			return errorf(ErrUnorderedProperty,
				"%s holds no elements: only an element-valued property can scope a term", prefix+c.Property)
		}
		return t.validate(c.Inner, prefix+c.Property+".")
	case Conjunction:
		for _, term := range c.Terms {
			if err := t.validate(term, prefix); err != nil {
				return err
			}
		}
		return nil
	case Disjunction:
		for _, term := range c.Terms {
			if err := t.validate(term, prefix); err != nil {
				return err
			}
		}
		return nil
	default:
		return errorf(ErrMalformed, "constraint has no form")
	}
}

func (t *translator) validateComparison(c Comparison, prefix string) error {
	property := prefix + c.Property
	if c.Property == query.PropertyType && prefix == "" {
		exact := c.Operator != InstanceOf
		if c.Operator != Equal && c.Operator != In && c.Operator != NotEqual && c.Operator != InstanceOf {
			return errorf(ErrUnorderedProperty, "query property %q is not ordered, so %s cannot compare it", c.Property, c.Operator)
		}
		if len(c.Values) == 0 {
			return errorf(ErrMalformed, "constraint on %q has no value to compare against", c.Property)
		}
		return t.validateTypes(c.Values, exact)
	}
	if err := t.knownProperty(property, "a comparison"); err != nil {
		return err
	}
	kind := t.table.kind(property)
	switch c.Operator {
	case Equal, NotEqual, In:
		if len(c.Values) == 0 {
			if c.Operator == In {
				return errorf(ErrMalformed, "in on %q needs a list of values to compare against", property)
			}
			return errorf(ErrMalformed, "constraint on %q has no value to compare against", property)
		}
		if kind == kindBoolean {
			for _, value := range c.Values {
				if _, err := strconv.ParseBool(value); err != nil {
					return errorf(ErrUnparsableValue, "%s on %q needs true or false to compare against, got %q", c.Operator, property, value)
				}
			}
		}
		return nil
	case Less, LessEqual, Greater, GreaterEqual:
		if kind != kindNumber && kind != kindUnknown {
			return errorf(ErrUnorderedProperty, "query property %q is not ordered, so %s cannot compare it", property, c.Operator)
		}
		if len(c.Values) != 1 {
			return errorf(ErrMalformed, "%s on %q compares against exactly one value, got %d", c.Operator, property, len(c.Values))
		}
		if !numeric(c.Values[0]) {
			return errorf(ErrUnparsableValue, "%s on %q needs a number to compare against, got %q", c.Operator, property, c.Values[0])
		}
		return nil
	case InstanceOf:
		return errorf(ErrUnorderedProperty, "instanceOf applies to %q, the element's metaclass, not to %q", query.PropertyType, property)
	default:
		return errorf(ErrMalformed, "constraint on %q has no operator", property)
	}
}

// validateTypes refuses a type test naming no metaclass of the metamodel.
func (t *translator) validateTypes(names []string, exact bool) error {
	operator := "instanceOf"
	if exact {
		operator = "="
	}
	if len(names) == 0 {
		return errorf(ErrMalformed, "constraint on %q has no value to compare against", query.PropertyType)
	}
	for _, name := range names {
		if name == "" {
			return errorf(ErrUnparsableValue, "%s on %q needs a metaclass name to compare against, got %q", operator, query.PropertyType, name)
		}
		if query.IsMetamodelTypeName(name) || (t.ctx.Model != nil && t.ctx.Model.Metaclass(name) != nil) {
			continue
		}
		return errorf(ErrUnparsableValue, "%s on %q needs a metaclass the library declares, got %q", operator, query.PropertyType, name)
	}
	return nil
}

// numeric reports whether text is a number a comparison can take: as the
// evaluator reads one, `*` for the unbounded upper multiplicity included.
func numeric(text string) bool {
	if text == "*" {
		return true
	}
	text = strings.ReplaceAll(text, "_", "")
	if _, ok := semantics.ParseInteger(text); ok {
		return true
	}
	if _, err := semantics.ParseRational(text, semantics.DefaultMaxIntegerBits); err == nil {
		return true
	}
	_, err := strconv.ParseFloat(text, 64)
	return err == nil
}

// constraint translates a filter over source into the operations that keep
// the rows satisfying it, in source order.
func (t *translator) constraint(c Constraint, source queryplan.Expression, prefix string) queryplan.Expression {
	switch c := c.(type) {
	case Comparison:
		return t.comparison(c, source, prefix)
	case TypeTest:
		if prefix != "" {
			// A type test of a related element reads its metaclass as a property.
			return t.comparison(Comparison{Property: query.PropertyType, Operator: In, Values: c.Types, Inverse: c.Inverse}, source, prefix)
		}
		return negated(c.Inverse, source, t.typeTest(c.Types, c.Exact, source))
	case Scoped:
		return t.constraint(c.Inner, source, prefix+c.Property+".")
	case Conjunction:
		expression := source
		for _, term := range c.Terms {
			expression = t.constraint(term, expression, prefix)
		}
		return expression
	case Disjunction:
		return t.disjunction(c.Terms, source, func(term Constraint, source queryplan.Expression) queryplan.Expression {
			return t.constraint(term, source, prefix)
		})
	default:
		return source
	}
}

// disjunction keeps the source rows any term keeps, in source order: the
// rows left once every term's rows are taken away are the ones to exclude.
func (t *translator) disjunction(terms []Constraint, source queryplan.Expression,
	translate func(Constraint, queryplan.Expression) queryplan.Expression) queryplan.Expression {
	if len(terms) == 1 {
		return translate(terms[0], source)
	}
	rest := source
	for _, term := range terms {
		rest = operate(queryplan.OperationExcept, arg("source", rest), arg("exclude", translate(term, source)))
	}
	return operate(queryplan.OperationExcept, arg("source", source), arg("exclude", rest))
}

func (t *translator) comparison(c Comparison, source queryplan.Expression, prefix string) queryplan.Expression {
	if c.Property == query.PropertyType && prefix == "" {
		return negated(c.Inverse, source, t.typeTest(c.Values, c.Operator != InstanceOf, source))
	}
	property := prefix + c.Property
	operator := c.Operator
	if operator == In {
		operator = Equal
	}
	if len(c.Values) > 1 {
		terms := make([]Constraint, len(c.Values))
		for i, value := range c.Values {
			terms[i] = Comparison{Property: c.Property, Operator: operator, Values: []string{value}}
		}
		return negated(c.Inverse, source, t.disjunction(terms, source,
			func(term Constraint, source queryplan.Expression) queryplan.Expression {
				return t.comparison(term.(Comparison), source, prefix)
			}))
	}
	value := c.Values[0]
	var filter queryplan.Expression
	if property == query.PropertyName {
		filter = operate(queryplan.OperationWhereName,
			arg("source", source), arg("operator", str(string(operator))), arg("value", str(value)))
	} else {
		filter = operate(queryplan.OperationWhereFeature,
			arg("source", source),
			arg("feature", str(property)),
			arg("operator", str(string(operator))),
			arg("value", str(value)),
			arg("tolerate", queryplan.BooleanLiteral(true, origin)))
	}
	return negated(c.Inverse, source, filter)
}

func (t *translator) typeTest(names []string, exact bool, source queryplan.Expression) queryplan.Expression {
	return operate(queryplan.OperationWhereType,
		arg("source", source),
		arg("type", stringList(names)),
		arg("exact", queryplan.BooleanLiteral(exact, origin)))
}

// search keeps the rows whose text, in the universal text properties, holds
// any of the terms, case-insensitively.
func (t *translator) search(source queryplan.Expression, terms []string) queryplan.Expression {
	quoted := make([]string, len(terms))
	for i, term := range terms {
		quoted[i] = regexp.QuoteMeta(term)
	}
	projected := operate(queryplan.OperationProject,
		arg("source", source),
		arg("properties", stringList(textProperties)),
		arg("tolerate", queryplan.BooleanLiteral(true, origin)))
	return operate(queryplan.OperationWhereText,
		arg("source", projected),
		arg("operator", str("matches")),
		arg("value", str("(?i)"+strings.Join(quoted, "|"))))
}

// negated keeps the source rows a filter does not, in source order, when inverse.
func negated(inverse bool, source, filter queryplan.Expression) queryplan.Expression {
	if !inverse {
		return filter
	}
	return operate(queryplan.OperationExcept, arg("source", source), arg("exclude", filter))
}

func operate(operation queryplan.Operation, arguments ...queryplan.Argument) queryplan.Expression {
	return queryplan.Operate(operation, origin, arguments...)
}

func arg(name string, value queryplan.Expression) queryplan.Argument {
	return queryplan.Arg(name, value)
}

func str(value string) queryplan.Expression {
	return queryplan.StringLiteral(value, origin)
}

func stringList(values []string) queryplan.Expression {
	items := make([]queryplan.Expression, len(values))
	for i, value := range values {
		items[i] = str(value)
	}
	return queryplan.Sequence(origin, items...)
}
