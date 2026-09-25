package grpc

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// Query property names, as the SysML v2 API & Services standard's clients write
// them. docs/reference/api.md documents what each one reports.
const (
	QueryPropID                   = corequery.PropertyID
	QueryPropType                 = corequery.PropertyType
	QueryPropName                 = corequery.PropertyName
	QueryPropDeclaredName         = corequery.PropertyDeclaredName
	QueryPropShortName            = corequery.PropertyShortName
	QueryPropDeclaredShortName    = corequery.PropertyDeclaredShortName
	QueryPropDocumentation        = corequery.PropertyDocumentation
	QueryPropQualifiedName        = corequery.PropertyQualifiedName
	QueryPropOwner                = corequery.PropertyOwner
	QueryPropIsAbstract           = corequery.PropertyIsAbstract
	QueryPropElementType          = corequery.PropertyElementType
	QueryPropMultiplicityLower    = corequery.PropertyMultiplicityLower
	QueryPropMultiplicityUpper    = corequery.PropertyMultiplicityUpper
	QueryPropSatisfiedRequirement = corequery.PropertySatisfiedRequirement
	QueryPropSatisfyingFeature    = corequery.PropertySatisfyingFeature
)

// QueryErrorKind classifies why a query could not be evaluated. Every kind fails
// the call: answering with no elements would read as "nothing matched".
type QueryErrorKind int

const (
	// QueryErrUnknownProperty is a property no queryable property names.
	QueryErrUnknownProperty QueryErrorKind = iota + 1
	// QueryErrMalformedConstraint is a constraint with no form, no operator or
	// no operand.
	QueryErrMalformedConstraint
	// QueryErrUnorderedProperty is > or < on a property that is not ordered.
	QueryErrUnorderedProperty
	// QueryErrUnparsableValue is an operand the operator cannot compare.
	QueryErrUnparsableValue
	// QueryErrUnknownScope is a scope naming an element the model does not have.
	QueryErrUnknownScope
)

// QueryError is a query the service refuses to evaluate. It reports itself as
// INVALID_ARGUMENT, since every kind is a fault in the query, not in the model.
type QueryError struct {
	Kind    QueryErrorKind
	Message string
}

func (e *QueryError) Error() string { return e.Message }

func queryErrorf(kind QueryErrorKind, format string, args ...any) *QueryError {
	return &QueryError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// QueryPropertyNames returns every queryable property name, sorted. It is what
// an unknown-property error lists, and what docs/reference/api.md's table documents.
func QueryPropertyNames() []string {
	return corequery.PropertyNames()
}

// Query evaluates a SysML v2 API & Services Query against a parsed model.
func (s *Service) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	if err := s.requireCapability(CapabilityQuery); err != nil {
		return nil, err
	}
	if req.OslcQuery != "" {
		if err := s.requireCapability(CapabilityOSLCQuery); err != nil {
			return nil, err
		}
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", req.ModelHash)
	}
	if req.Query != nil && req.OslcQuery != "" {
		return nil, statusError(connect.CodeInvalidArgument, "query and oslc_query are mutually exclusive")
	}
	if req.Query == nil && req.OslcQuery == "" {
		return nil, queryStatus(queryErrorf(QueryErrMalformedConstraint, "query is unset"))
	}

	sc := cached.SymbolContext()
	defer sc.Lock()()
	eval := &queryEval{sc: sc, cached: cached}
	eval.reader = corequery.NewPropertyReader(sc.Index, sc.Resolver, sc.Semantics).WithIdentity(eval.identity)
	if req.OslcQuery != "" {
		parsed, err := corequery.ParseOSLC(req.OslcQuery)
		if err != nil {
			return nil, queryStatus(err)
		}
		elements, err := corequery.Evaluate(&coreQueryModel{eval: eval, cached: cached}, parsed)
		if err != nil {
			return nil, queryStatus(err)
		}
		out := make([]*pb.QueryResultElement, 0, len(elements))
		for _, element := range elements {
			out = append(out, &pb.QueryResultElement{
				Id: element.ID, Type: element.Type, Properties: element.Properties,
			})
		}
		return &pb.QueryResponse{Elements: out}, nil
	}

	// The query is judged before any element is: whether it is one the service
	// can evaluate cannot depend on how many elements it happens to consider.
	if err := validateConstraint(req.Query.Where); err != nil {
		return nil, queryStatus(err)
	}
	projection, err := projectedProperties(req.Query.Select)
	if err != nil {
		return nil, queryStatus(err)
	}
	candidates, err := eval.candidates(cached, req.Query.Scope)
	if err != nil {
		return nil, queryStatus(err)
	}

	elements := make([]*pb.QueryResultElement, 0, len(candidates))
	for _, sym := range candidates {
		if !eval.matches(sym, req.Query.Where) {
			continue
		}
		elements = append(elements, eval.project(sym, projection))
	}
	return &pb.QueryResponse{Elements: elements}, nil
}

// queryEval evaluates one query over a model's symbol context. It holds no
// element state of its own: every property is read from the index and semantic
// model on demand.
type queryEval struct {
	sc     *SymbolContext
	reader *corequery.PropertyReader
	cached *CachedModel
}

// candidates returns the elements a query considers, in declaration order: every
// loaded document of the model when the scope is empty, else each scoped element
// and everything nested inside it.
func (e *queryEval) candidates(cached *CachedModel, scope []string) ([]*symbols.Symbol, error) {
	if len(scope) == 0 {
		var out []*symbols.Symbol
		for _, root := range cached.DocumentRoots() {
			out = append(out, e.collect(root)...)
		}
		return out, nil
	}

	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	for _, fqn := range scope {
		roots := e.sc.Index.LookupQualified(fqn)
		if len(roots) == 0 {
			e.positionalNames(cached)
			if sym := cached.byPositional[fqn]; sym != nil {
				roots = []*symbols.Symbol{sym}
			}
		}
		if len(roots) == 0 {
			return nil, queryErrorf(QueryErrUnknownScope,
				"query scope names an element the model does not have: %q", fqn)
		}
		for _, root := range roots {
			if seen[root] {
				continue
			}
			seen[root] = true
			out = append(out, root)
			for _, nested := range e.nested(root) {
				if !seen[nested] {
					seen[nested] = true
					out = append(out, nested)
				}
			}
		}
	}
	return out, nil
}

func (e *queryEval) identity(sym *symbols.Symbol) string {
	if e.identifies(sym) {
		return e.sc.Index.GetFQN(sym)
	}
	e.positionalNames(e.cached)
	return e.cached.positional[sym]
}

func (e *queryEval) positionalNames(cached *CachedModel) {
	if cached == nil {
		return
	}
	cached.positionalOnce.Do(func() {
		candidates := make(map[string]map[*symbols.Symbol]bool)
		type positionalCandidate struct {
			name string
			sym  *symbols.Symbol
		}
		for _, doc := range cached.Documents {
			if doc == nil || doc.Source == nil {
				continue
			}
			file := source.NewWithKind(doc.Source.Name(), doc.Source.Bytes(), doc.Source.Kind())
			p := parser.New(file)
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				continue
			}
			names, err := export.DeclarationNames(file, root)
			if err != nil {
				continue
			}
			scope := cached.Index.DocumentRoot(doc.Source.Name())
			seen := make(map[*symbols.Scope]bool)
			var walk func(*symbols.Scope)
			walk = func(current *symbols.Scope) {
				if current == nil || seen[current] {
					return
				}
				seen[current] = true
				for _, sym := range current.AllMembers() {
					if sym == nil || sym.DocName != doc.Source.Name() ||
						e.identifies(sym) || sym.Decl == nil {
						continue
					}
					if sym.Name != "" && !sym.EffectiveName() && sym.Owner() == nil {
						continue
					}
					name, ok := names[sym.Decl.Span()]
					if !ok || !isPositionalIdentity(name) || len(e.sc.Index.LookupQualified(name)) != 0 {
						continue
					}
					if candidates[name] == nil {
						candidates[name] = make(map[*symbols.Symbol]bool)
					}
					candidates[name][sym] = true
				}
				for _, child := range current.Children() {
					walk(child)
				}
			}
			walk(scope)
		}

		accepted := make([]positionalCandidate, 0, len(candidates))
		for name, claims := range candidates {
			if len(claims) != 1 {
				continue
			}
			for sym := range claims {
				accepted = append(accepted, positionalCandidate{name: name, sym: sym})
			}
		}
		slices.SortFunc(accepted, func(a, b positionalCandidate) int {
			aDepth, bDepth := strings.Count(a.name, "::"), strings.Count(b.name, "::")
			if aDepth < bDepth {
				return -1
			}
			if aDepth > bDepth {
				return 1
			}
			return strings.Compare(a.name, b.name)
		})

		cached.positional = make(map[*symbols.Symbol]string)
		cached.byPositional = make(map[string]*symbols.Symbol)
		for _, candidate := range accepted {
			sym := candidate.sym
			owner := sym.Owner()
			if sym.Name != "" && !sym.EffectiveName() {
				if owner == nil || cached.positional[owner] == "" {
					continue
				}
			} else if owner != nil && !e.identifies(owner) && cached.positional[owner] == "" {
				continue
			}
			cached.positional[sym] = candidate.name
			cached.byPositional[candidate.name] = sym
		}
	})
}

func isPositionalIdentity(name string) bool {
	for _, segment := range strings.Split(name, "::") {
		if len(segment) < 2 || segment[0] != '@' {
			continue
		}
		value, err := strconv.Atoi(segment[1:])
		if err == nil && value >= 0 && strconv.Itoa(value) == segment[1:] {
			return true
		}
	}
	return false
}

// nested returns every element declared inside an element, in declaration
// order. It is what a scope entry expands to.
func (e *queryEval) nested(sym *symbols.Symbol) []*symbols.Symbol {
	w := &elementWalk{eval: e, seen: map[*symbols.Symbol]bool{sym: true}, walked: map[*symbols.Scope]bool{}}
	w.members(sym)
	return w.out
}

// collect returns every element declared in a scope tree, in declaration order,
// parents before the elements they own.
func (e *queryEval) collect(scope *symbols.Scope) []*symbols.Symbol {
	w := &elementWalk{eval: e, seen: map[*symbols.Symbol]bool{}, walked: map[*symbols.Scope]bool{}}
	w.scope(scope)
	return w.out
}

// elementWalk enumerates elements over the scope tree, without duplicates.
type elementWalk struct {
	eval   *queryEval
	out    []*symbols.Symbol
	seen   map[*symbols.Symbol]bool
	walked map[*symbols.Scope]bool
}

// scope walks a scope's members. A scope no symbol owns — a loop body, a body
// expression's parameters — is walked through, since it still declares elements.
func (w *elementWalk) scope(s *symbols.Scope) {
	if s == nil || w.walked[s] {
		return
	}
	w.walked[s] = true
	for _, sym := range s.AllMembers() {
		w.visit(sym)
	}
	for _, child := range s.Children() {
		w.scope(child)
	}
}

// visit reports an element and walks what it declares. One with no queryable
// identity is walked through but not reported: the standard identifies an
// element by `@id`, and it has none to be told apart or named by.
func (w *elementWalk) visit(sym *symbols.Symbol) {
	if w.seen[sym] {
		return
	}
	w.seen[sym] = true
	if w.eval.identity(sym) != "" {
		w.out = append(w.out, sym)
	}
	w.members(sym)
}

// identifies reports whether an element's qualified name is the `@id` the
// standard's clients expect: a real qualified name that names this element back,
// so a later query may use it as a scope.
func (e *queryEval) identifies(sym *symbols.Symbol) bool {
	fqn := e.sc.Index.GetFQN(sym)
	if !hasQualifiedIdentity(fqn) {
		return false
	}
	return slices.Contains(e.sc.Index.LookupQualified(fqn), sym)
}

// hasQualifiedIdentity reports whether a qualified name identifies an element.
// An unnamed one — a doc note, an anonymous usage — has an empty segment, so its
// name is neither unique nor a name a scope could use.
func hasQualifiedIdentity(fqn string) bool {
	if fqn == "" {
		return false
	}
	for _, segment := range strings.Split(fqn, "::") {
		if segment == "" {
			return false
		}
	}
	return true
}

// members walks what an element declares. A library element restored from cache
// owns no scope, so its members are reached through the index by qualified name.
func (w *elementWalk) members(sym *symbols.Symbol) {
	if sym.Scope != nil {
		w.scope(sym.Scope)
		return
	}
	for _, child := range w.eval.sc.Index.LookupDirectChildren(w.eval.sc.Index.GetFQN(sym)) {
		w.visit(child)
	}
}

// projectedProperties returns the properties to report, validating each name.
// An empty selection reports every property.
func projectedProperties(selected []string) ([]string, error) {
	if len(selected) == 0 {
		return QueryPropertyNames(), nil
	}
	out := make([]string, 0, len(selected))
	for _, name := range selected {
		if !corequery.IsProperty(name) {
			return nil, unknownProperty(name)
		}
		out = append(out, name)
	}
	return out, nil
}

func unknownProperty(name string) *QueryError {
	return queryErrorf(QueryErrUnknownProperty,
		"unknown query property %q; queryable properties are %s",
		name, strings.Join(QueryPropertyNames(), ", "))
}

// project builds the record of a matched element: its identity and type always,
// plus the selected properties it has.
func (e *queryEval) project(sym *symbols.Symbol, selected []string) *pb.QueryResultElement {
	element := &pb.QueryResultElement{
		Id:         e.identity(sym),
		Type:       corequery.MetamodelTypeNameOf(sym),
		Properties: make(map[string]string, len(selected)),
	}
	for _, name := range selected {
		if values, ok := e.reader.Values(sym, name); ok && len(values) != 0 {
			element.Properties[name] = values[0]
		}
	}
	return element
}

// validateConstraint reports whether a constraint is one the service can
// evaluate at all: every fault in a query is found here, before any element is
// read, so an empty scope cannot turn a malformed query into "nothing matched".
func validateConstraint(constraint *pb.Constraint) error {
	if constraint == nil {
		return nil // a query with no `where` selects its whole scope
	}
	switch form := constraint.Constraint.(type) {
	case *pb.Constraint_Primitive:
		return validatePrimitive(form.Primitive)
	case *pb.Constraint_Composite:
		return validateComposite(form.Composite)
	default:
		return queryErrorf(QueryErrMalformedConstraint,
			"constraint is neither a PrimitiveConstraint nor a CompositeConstraint")
	}
}

// validatePrimitive checks a comparison names a queryable property, has an
// operator, and has operands that operator can compare.
func validatePrimitive(c *pb.PrimitiveConstraint) error {
	if c == nil {
		return queryErrorf(QueryErrMalformedConstraint, "PrimitiveConstraint is unset")
	}
	if !corequery.IsProperty(c.Property) {
		return unknownProperty(c.Property)
	}
	if len(c.Value) == 0 {
		return queryErrorf(QueryErrMalformedConstraint,
			"PrimitiveConstraint on %q has no value to compare against", c.Property)
	}
	switch c.Operator {
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:
		return nil
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER,
		pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:
		return validateOrdered(c)
	default:
		return queryErrorf(QueryErrMalformedConstraint,
			"PrimitiveConstraint on %q has no operator", c.Property)
	}
}

// validateComposite checks a combination has an operator, something to combine,
// and no malformed constraint anywhere beneath it, decisive or not.
func validateComposite(c *pb.CompositeConstraint) error {
	if c == nil {
		return queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint is unset")
	}
	if len(c.Constraint) == 0 {
		return queryErrorf(QueryErrMalformedConstraint,
			"CompositeConstraint has no constraints to combine")
	}
	switch c.Operator {
	case pb.CompositeOperator_COMPOSITE_OPERATOR_AND, pb.CompositeOperator_COMPOSITE_OPERATOR_OR:
	default:
		return queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint has no operator")
	}
	for _, nested := range c.Constraint {
		if err := validateConstraint(nested); err != nil {
			return err
		}
	}
	return nil
}

// matches reports whether an element satisfies a constraint, which
// validateConstraint has already found evaluable. An unset constraint matches
// every element, as a query with no `where` selects its whole scope.
func (e *queryEval) matches(sym *symbols.Symbol, constraint *pb.Constraint) bool {
	if constraint == nil {
		return true
	}
	switch form := constraint.Constraint.(type) {
	case *pb.Constraint_Primitive:
		return e.matchesPrimitive(sym, form.Primitive)
	case *pb.Constraint_Composite:
		return e.matchesComposite(sym, form.Composite)
	default:
		return true
	}
}

// matchesPrimitive compares one property of an element, negating the verdict
// when the constraint is inverse.
func (e *queryEval) matchesPrimitive(sym *symbols.Symbol, c *pb.PrimitiveConstraint) bool {
	values, has := e.reader.Values(sym, c.Property)
	value := ""
	if len(values) != 0 {
		value = values[0]
	}
	var verdict bool
	if c.Operator == pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL {
		verdict = has && equalsAny(value, c.Value)
	} else {
		verdict = has && compareOrdered(c, value)
	}
	if c.Inverse {
		return !verdict
	}
	return verdict
}

// equalsAny reports whether the property's value is one of the constraint's.
// The standard writes a single value, and its clients also write a list for
// `@type`; one value is the degenerate case of the same rule.
func equalsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

// validateOrdered checks > or < has an ordered property and one numeric operand:
// either fault is a fault in the query, never a false verdict.
func validateOrdered(c *pb.PrimitiveConstraint) error {
	if !corequery.IsOrdered(c.Property) {
		return queryErrorf(QueryErrUnorderedProperty,
			"query property %q is not ordered, so %s cannot compare it",
			c.Property, operatorText(c.Operator))
	}
	if len(c.Value) != 1 {
		return queryErrorf(QueryErrMalformedConstraint,
			"%s on %q compares against exactly one value, got %d",
			operatorText(c.Operator), c.Property, len(c.Value))
	}
	if _, err := parseOrdered(c.Value[0]); err != nil {
		return queryErrorf(QueryErrUnparsableValue,
			"%s on %q needs a number to compare against, got %q",
			operatorText(c.Operator), c.Property, c.Value[0])
	}
	return nil
}

// compareOrdered evaluates > or < against an element's own value. A value that
// is not a number fails the comparison: that is a fact about the element.
func compareOrdered(c *pb.PrimitiveConstraint, value string) bool {
	operand, err := parseOrdered(c.Value[0])
	if err != nil {
		return false
	}
	actual, err := parseOrdered(value)
	if err != nil {
		return false
	}
	if c.Operator == pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER {
		return actual > operand
	}
	return actual < operand
}

// parseOrdered reads a number, accepting the unbounded multiplicity "*" as the
// infinity it denotes.
func parseOrdered(text string) (float64, error) {
	if text == "*" {
		return math.Inf(1), nil
	}
	return strconv.ParseFloat(text, 64)
}

// operatorText renders an operator as the standard writes it.
func operatorText(op pb.PrimitiveOperator) string {
	switch op {
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:
		return "="
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER:
		return ">"
	case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:
		return "<"
	default:
		return "(no operator)"
	}
}

// matchesComposite combines the verdicts of nested constraints, short-circuiting
// once one is decisive.
func (e *queryEval) matchesComposite(sym *symbols.Symbol, c *pb.CompositeConstraint) bool {
	and := c.Operator == pb.CompositeOperator_COMPOSITE_OPERATOR_AND
	for _, nested := range c.Constraint {
		if e.matches(sym, nested) != and {
			return !and
		}
	}
	return and
}
