package repl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// runQueryUsage is what %run-query accepts: a document query's name and its
// entry bindings.
const runQueryUsage = "usage: %run-query <name> [<parameter>=<expression> ...]"

// RunDocumentQuery compiles the named document query, binds its entry
// parameters and executes it. invocation is what `%run-query` takes: a name
// followed by `<parameter>=<expression>` bindings.
func (s *Session) RunDocumentQuery(invocation string) Verdict {
	defer s.enter()()
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 {
		return s.withTrace(unresolvedVerdict(invocation, "a document query to run must be named"))
	}
	name := fields[0]
	lines, values, err := s.runDocumentQuery(name, fields[1:])
	if err != nil {
		return s.withTrace(unresolvedVerdict(name, err.Error()))
	}
	return s.withTrace(Verdict{Subject: name, Status: VerdictHolds, Lines: lines, Values: values})
}

// doRunQuery carries out %run-query, reporting a query that could not be run
// the way the prompt reports any command it cannot carry out.
func (s *Session) doRunQuery(invocation string) ([]string, bool, error) {
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 {
		return []string{runQueryUsage}, false, nil
	}
	lines, _, err := s.runDocumentQuery(fields[0], fields[1:])
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return lines, false, nil
}

// splitQueryArgs splits a %run-query invocation on whitespace, keeping quoted
// text together with its quotes so a binding expression parses as written.
func splitQueryArgs(line string) []string {
	var args []string
	var current strings.Builder
	quote := rune(0)
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			current.WriteRune(r)
			escaped = true
		case quote != 0:
			current.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
			current.WriteRune(r)
		case r == ' ' || r == '\t':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// regroupBindings joins tokens back into `<parameter>=<expression>` bindings,
// so an expression may contain unquoted spaces. A new binding starts at a
// `name=` token outside any bracket; other tokens extend the one before.
func regroupBindings(tokens []string) []string {
	var out []string
	depth := 0
	for _, token := range tokens {
		if depth <= 0 && (startsBinding(token) || len(out) == 0) {
			out = append(out, token)
		} else {
			out[len(out)-1] += " " + token
		}
		depth += bracketDelta(token)
	}
	return out
}

// startsBinding reports whether a token opens a `<parameter>=<expression>`
// binding: an identifier followed by a single `=`.
func startsBinding(token string) bool {
	i := strings.IndexRune(token, '=')
	if i <= 0 || (i+1 < len(token) && token[i+1] == '=') {
		return false
	}
	for j, r := range token[:i] {
		alpha := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !alpha && (j == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// bracketDelta counts a token's unquoted bracket openings minus closings.
func bracketDelta(token string) int {
	delta := 0
	quote := rune(0)
	escaped := false
	for _, r := range token {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '(' || r == '[' || r == '{':
			delta++
		case r == ')' || r == ']' || r == '}':
			delta--
		}
	}
	return delta
}

// runDocumentQuery resolves the named query definition, compiles it to a plan,
// binds the arguments and executes the plan against the session's model.
func (s *Session) runDocumentQuery(name string, args []string) ([]string, []NamedValue, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return nil, nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, nil, fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	model, resolver := ctx.Semantics(), ctx.Resolver()
	if !queryplan.IsQueryDefinition(idx, model, sym) {
		return nil, nil, fmt.Errorf("%s is not a document query: one is a calc def specializing DocumentQueries::Query", notationName(fqn))
	}
	program, err := queryplan.Compile(idx, model, resolver, sym)
	if err != nil {
		return nil, nil, err
	}
	bindings, err := s.queryBindings(ctx, regroupBindings(args))
	if err != nil {
		return nil, nil, err
	}
	result, err := queryexec.Execute(program,
		queryexec.Context{Index: idx, Resolver: resolver, Model: model},
		bindings, queryexec.Options{})
	if err != nil {
		return nil, nil, err
	}
	lines, values := renderRowSet(notationName(fqn), result)
	return lines, values, nil
}

// queryBindings reads the `<parameter>=<expression>` arguments of %run-query.
// Repeating a parameter appends to its binding, so a 0..* parameter can be
// given several values.
func (s *Session) queryBindings(ctx *runtime.Context, args []string) (queryexec.Bindings, error) {
	if len(args) == 0 {
		return nil, nil
	}
	bindings := make(queryexec.Bindings, len(args))
	for _, arg := range args {
		param, expr, ok := strings.Cut(arg, "=")
		param, expr = strings.TrimSpace(param), strings.TrimSpace(expr)
		if !ok || param == "" || expr == "" {
			return nil, fmt.Errorf("binding %q is not written as <parameter>=<expression>", arg)
		}
		values, err := s.bindingValues(ctx, param, expr)
		if err != nil {
			return nil, err
		}
		bindings[param] = append(bindings[param], values...)
	}
	return bindings, nil
}

// bindingValues reads one binding: a name denoting a model element binds that
// element, and anything else is evaluated as an expression at the prompt.
func (s *Session) bindingValues(ctx *runtime.Context, param, expr string) ([]queryexec.Value, error) {
	sym, _, lerr := s.lookupSymbol(expr)
	if lerr == nil && sym != nil {
		return []queryexec.Value{queryexec.ElementValue(sym)}, nil
	}
	var ambiguous *AmbiguousNameError
	if errors.As(lerr, &ambiguous) {
		return nil, fmt.Errorf("binding %s: %w", param, lerr)
	}
	node, diags := parseExprAlone(expr)
	if len(diags) > 0 {
		return nil, exprError(expr, diags[0].Message, diags[0].Span, len(exprPrefix))
	}
	if node == nil {
		return nil, fmt.Errorf("binding %s: could not parse %q", param, expr)
	}
	value, err := ctx.EvalWithScope(node, s.promptScope())
	if err != nil {
		return nil, fmt.Errorf("binding %s: %w", param, err)
	}
	values, err := queryValues(value)
	if err != nil {
		return nil, fmt.Errorf("binding %s: %w", param, err)
	}
	return values, nil
}

// queryValues converts an evaluated prompt value into query binding values. A
// collection binds its elements in order; a null binds nothing.
func queryValues(value runtime.Value) ([]queryexec.Value, error) {
	switch value.Kind {
	case runtime.ValConst:
		switch value.Const.Kind {
		case semantics.ValInt:
			return []queryexec.Value{queryexec.IntegerValue(value.Const.Int)}, nil
		case semantics.ValReal:
			return []queryexec.Value{queryexec.RealValue(value.Const.Real)}, nil
		case semantics.ValBool:
			return []queryexec.Value{queryexec.BooleanValue(value.Const.Bool)}, nil
		}
		return nil, fmt.Errorf("%s cannot be bound to a query parameter", runtime.FormatValue(value))
	case runtime.ValString:
		return []queryexec.Value{queryexec.StringValue(value.Str())}, nil
	case runtime.ValNull:
		return nil, nil
	case runtime.ValEnumLiteral:
		return []queryexec.Value{queryexec.ElementValue(value.Literal())}, nil
	case runtime.ValSequence:
		if value.Sequence() == nil {
			return nil, nil
		}
		return queryValueList(value.Sequence().Elements())
	case runtime.ValSet:
		if value.Set() == nil {
			return nil, nil
		}
		return queryValueList(value.Set().Elements())
	case runtime.ValArray:
		return queryValueList(value.Array().Elements)
	case runtime.ValVector:
		components := value.Vector().Elements
		elements := make([]runtime.Value, len(components))
		for i, c := range components {
			elements[i] = runtime.Value{Kind: runtime.ValConst, Const: c}
		}
		return queryValueList(elements)
	case runtime.ValMeasurementRef:
		// A reference to one declared unit is that element; a composed unit names none.
		if decl := value.MeasurementRef().Declaration(); decl != nil {
			return []queryexec.Value{queryexec.ElementValue(decl)}, nil
		}
		return nil, fmt.Errorf("the measurement reference %s names no single declaration to bind to a query parameter", runtime.FormatValue(value))
	case runtime.ValTensorQuantity:
		return nil, fmt.Errorf("a tensor quantity %s cannot be bound to a query parameter: its components are measured, and a query takes bare scalars or elements", runtime.FormatValue(value))
	case runtime.ValCoordinateFrame:
		// A declared frame or scale is that element; one composed by arithmetic names none.
		if decl := value.CoordinateFrame().Decl; decl != nil && value.CoordinateFrame().Object != 0 {
			return []queryexec.Value{queryexec.ElementValue(decl)}, nil
		}
		return nil, fmt.Errorf("the coordinate frame %s names no single declaration to bind to a query parameter", runtime.FormatValue(value))
	case runtime.ValCoordinateTransformation:
		if decl := value.CoordinateTransformation().Decl; decl != nil {
			return []queryexec.Value{queryexec.ElementValue(decl)}, nil
		}
		return nil, fmt.Errorf("the coordinate transformation %s names no single declaration to bind to a query parameter", runtime.FormatValue(value))
	default:
		return nil, fmt.Errorf("a %s cannot be bound to a query parameter", value.Kind)
	}
}

func queryValueList(elements []runtime.Value) ([]queryexec.Value, error) {
	var out []queryexec.Value
	for _, element := range elements {
		values, err := queryValues(element)
		if err != nil {
			return nil, err
		}
		out = append(out, values...)
	}
	return out, nil
}

// renderRowSet reports an executed query's ordered rows and projected cells as
// the prompt prints them, with the row and column counts as reportable values.
func renderRowSet(name string, result *queryexec.RowSet) ([]string, []NamedValue) {
	rows := result.Rows()
	columns := result.Columns()
	lines := []string{fmt.Sprintf("✓ Query %s returned %s", name, countOf(len(rows), "row", "rows"))}
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = column.Name()
	}
	if len(names) > 0 {
		lines = append(lines, "  Columns: "+strings.Join(names, ", "))
	}
	for i, row := range rows {
		lines = append(lines, fmt.Sprintf("  Row %d: %s", i+1, formatQueryValue(row.Element())))
		for j, cell := range row.Cells() {
			if j >= len(names) {
				break
			}
			lines = append(lines, fmt.Sprintf("    %s = %s", names[j], formatQueryCell(cell)))
		}
	}
	values := []NamedValue{{Name: "rows", Value: strconv.Itoa(len(rows))}}
	if len(names) > 0 {
		values = append(values, NamedValue{Name: "columns", Value: strings.Join(names, ", ")})
	}
	return lines, values
}

// formatQueryCell renders one projected cell: nothing, one value, or a
// bracketed sequence of values.
func formatQueryCell(cell queryexec.Cell) string {
	values := cell.Values()
	switch len(values) {
	case 0:
		return "(none)"
	case 1:
		return formatQueryValue(values[0])
	default:
		parts := make([]string, len(values))
		for i, value := range values {
			parts[i] = formatQueryValue(value)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
}

// formatQueryValue renders one query value with the notation the prompt uses
// for runtime results.
func formatQueryValue(value queryexec.Value) string {
	if sym, ok := value.Element(); ok {
		if fqn := symbols.FQNOf(sym); fqn != "" {
			return notationName(fqn)
		}
		return sym.Name
	}
	if text, ok := value.String(); ok {
		return strconv.Quote(text)
	}
	if integer, ok := value.Integer(); ok {
		return strconv.FormatInt(integer, 10)
	}
	if real, ok := value.Real(); ok {
		return semantics.FormatReal(real)
	}
	if boolean, ok := value.Boolean(); ok {
		return strconv.FormatBool(boolean)
	}
	if value.Kind() == queryexec.ValueInfinity {
		return "*"
	}
	if quantity, ok := value.Quantity(); ok {
		return quantity.String()
	}
	return string(value.Kind())
}
