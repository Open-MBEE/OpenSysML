package grpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/doc/querytranslate"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// Query property names, as the SysML v2 API & Services standard's clients write
// them. docs/reference/api.md documents what each one reports.
const (
	QueryPropID                    = corequery.PropertyID
	QueryPropType                  = corequery.PropertyType
	QueryPropName                  = corequery.PropertyName
	QueryPropDeclaredName          = corequery.PropertyDeclaredName
	QueryPropShortName             = corequery.PropertyShortName
	QueryPropDeclaredShortName     = corequery.PropertyDeclaredShortName
	QueryPropDocumentation         = corequery.PropertyDocumentation
	QueryPropTextualRepresentation = corequery.PropertyTextualRepresentation
	QueryPropBody                  = corequery.PropertyBody
	QueryPropLocale                = corequery.PropertyLocale
	QueryPropLanguage              = corequery.PropertyLanguage
	QueryPropQualifiedName         = corequery.PropertyQualifiedName
	QueryPropOwner                 = corequery.PropertyOwner
	QueryPropIsAbstract            = corequery.PropertyIsAbstract
	QueryPropElementType           = corequery.PropertyElementType
	QueryPropMultiplicityLower     = corequery.PropertyMultiplicityLower
	QueryPropMultiplicityUpper     = corequery.PropertyMultiplicityUpper
	QueryPropSatisfiedRequirement  = corequery.PropertySatisfiedRequirement
	QueryPropSatisfyingFeature     = corequery.PropertySatisfyingFeature
	QueryPropPayloadArgument       = corequery.PropertyPayloadArgument
	QueryPropSenderArgument        = corequery.PropertySenderArgument
	QueryPropReceiverArgument      = corequery.PropertyReceiverArgument
)

// QueryErrorKind classifies why a query could not be evaluated. Every kind fails
// the call: answering with no elements would read as "nothing matched".
type QueryErrorKind int

const (
	// QueryErrUnknownProperty is a property the metamodel does not define.
	QueryErrUnknownProperty QueryErrorKind = iota + 1
	// QueryErrMalformedConstraint is a constraint with no form, no operator or
	// no operand, or query text that does not parse.
	QueryErrMalformedConstraint
	// QueryErrUnorderedProperty is an ordered comparison (<, <=, >, >=) on a
	// property that is not ordered, or instanceOf on one that is not "@type".
	QueryErrUnorderedProperty
	// QueryErrUnparsableValue is an operand the operator cannot compare: a
	// non-number for an ordered comparison, an undeclared metaclass for instanceOf.
	QueryErrUnparsableValue
	// QueryErrUnknownScope is a scope naming an element the model does not have.
	QueryErrUnknownScope
	// QueryErrUnsupported is a construct the query language does not define or
	// this service does not evaluate, such as "or" in OSLC text.
	QueryErrUnsupported
	// QueryErrEvaluation is a query the plan refused while reading the model:
	// the QVTS record of the failure is attached as a QueryOutcome detail.
	QueryErrEvaluation
)

// QueryError is a query the service refuses to evaluate. It reports itself as
// INVALID_ARGUMENT, since every kind is a fault in the query, not in the model.
type QueryError struct {
	Kind    QueryErrorKind
	Message string
	// Err is the translator's or the evaluator's own fault, kept for its kind.
	Err error
}

func (e *QueryError) Error() string { return e.Message }

func (e *QueryError) Unwrap() error { return e.Err }

func queryErrorf(kind QueryErrorKind, format string, args ...any) *QueryError {
	return &QueryError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// QueryPropertyNames returns the queryable property names the evaluator derives
// for every element, sorted: what docs/reference/api.md's table documents. The
// metamodel's own properties are queryable besides, by their declared names.
func QueryPropertyNames() []string {
	return corequery.PropertyNames()
}

// Query evaluates a SysML v2 API & Services Query, or OSLC Query 3.0 text, over
// a parsed model: either is translated to a document-query plan and run by the
// evaluator over the model's elements, or the objects the model's session holds.
func (s *Service) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	if err := s.requireCapability(CapabilityQuery); err != nil {
		return nil, err
	}
	if req.OslcQuery != "" {
		if err := s.requireCapability(CapabilityOSLCQuery); err != nil {
			return nil, err
		}
	}
	if len(req.GetQuery().GetOrderBy()) > 0 {
		if err := s.requireCapability(CapabilityQueryOrderBy); err != nil {
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
	if req.AsView {
		if err := s.requireCapability(CapabilityQueryResultView); err != nil {
			return nil, err
		}
	}

	sc := cached.SymbolContext()
	defer sc.Lock()()
	held := s.objects(cached)
	defer held.lock(ctx)()
	qctx := held.queryContext()
	eval := &queryEval{sc: sc, cached: cached}
	qctx.Identity = eval.identity
	table := cached.queryTable(qctx)

	text := req.OslcQuery
	if text == "" {
		text = apiQueryText(req.Query)
	}
	spec := resultViewSpec(cached, req.ModelHash, queryexec.DefaultResultViewName, text)
	// refused is a translator refusal as its status, with the view of the path error it is.
	refused := func(err error) error {
		status := queryStatus(err)
		if outcome := querytranslate.OutcomeOf(err); req.AsView && outcome != nil {
			status = withResultView(status, spec.WriteOutcome(qctx, *outcome))
		}
		return status
	}
	var parsed querytranslate.Query
	var err error
	if req.OslcQuery != "" {
		parsed, err = querytranslate.ParseOSLC(req.OslcQuery, table)
	} else {
		parsed, err = apiQuery(req.Query)
	}
	if err != nil {
		return nil, refused(err)
	}
	plan, err := querytranslate.Translate(parsed, querytranslate.Context{Index: qctx.Index, Model: qctx.Model, Table: table}, "Query")
	if err != nil {
		return nil, refused(err)
	}
	bindings := queryexec.Bindings{}
	if !plan.Objects {
		candidates, err := eval.candidates(cached, parsed.Scope)
		if err != nil {
			return nil, queryStatus(err)
		}
		values := make([]queryexec.Value, len(candidates))
		for i, sym := range candidates {
			values[i] = queryexec.ElementValue(sym)
		}
		bindings[querytranslate.SourceParameter] = values
	}
	result, err := queryexec.Execute(plan.Program, qctx, bindings, queryexec.Options{})
	if err != nil {
		status := held.queryExecStatus(err)
		if req.AsView {
			status = withResultView(status, failedResultView(spec, qctx, err))
		}
		return nil, status
	}
	response := &pb.QueryResponse{Elements: eval.resultElements(qctx, result)}
	if req.AsView {
		response.ResultView, _ = spec.Write(qctx, result, nil)
	}
	return response, nil
}

// resultElements renders the plan's rows: an element by the identity this
// service reports and its metamodel type, an object by the element it is an
// instance of with the path it is held under, and the first value of each
// selected property as text — an empty string included, an expression element
// as written — omitting the properties the row has no value of and an element
// this service has no identity for.
func (e *queryEval) resultElements(qctx queryexec.Context, result *queryexec.RowSet) []*pb.QueryResultElement {
	rows := result.Rows()
	columns := result.Columns()
	out := make([]*pb.QueryResultElement, 0, len(rows))
	for _, row := range rows {
		value := row.Element()
		element := &pb.QueryResultElement{Properties: make(map[string]string, len(columns))}
		if decl := value.Declaration(); decl != nil {
			element.Id = e.identity(decl)
			element.Type = corequery.MetamodelTypeNameOf(decl)
		}
		if _, label, ok := value.Object(); ok {
			element.Path = label
		}
		for i, cell := range row.Cells() {
			if i >= len(columns) {
				break
			}
			values := cell.Values()
			if len(values) == 0 {
				continue
			}
			if text, ok := qctx.PropertyText(values[0]); ok {
				element.Properties[columns[i].Name()] = text
			}
		}
		out = append(out, element)
	}
	return out
}

// apiQuery reads the standard's Query message into the translator's form,
// refusing a constraint with no form, no operator or too few members.
func apiQuery(q *pb.Query) (querytranslate.Query, error) {
	out := querytranslate.Query{Scope: q.Scope}
	for _, name := range q.Select {
		out.Select = append(out.Select, querytranslate.Selection{Property: name})
	}
	for _, name := range q.OrderBy {
		out.OrderBy = append(out.OrderBy, querytranslate.OrderTerm{Property: name})
	}
	if q.Where != nil {
		where, err := apiConstraint(q.Where)
		if err != nil {
			return querytranslate.Query{}, err
		}
		out.Where = where
	}
	return out, nil
}

func apiConstraint(constraint *pb.Constraint) (querytranslate.Constraint, error) {
	switch form := constraint.GetConstraint().(type) {
	case *pb.Constraint_Primitive:
		return apiPrimitive(form.Primitive)
	case *pb.Constraint_Composite:
		return apiComposite(form.Composite)
	default:
		return nil, queryErrorf(QueryErrMalformedConstraint, "constraint has neither a primitive nor a composite form")
	}
}

func apiPrimitive(c *pb.PrimitiveConstraint) (querytranslate.Constraint, error) {
	if c == nil {
		return nil, queryErrorf(QueryErrMalformedConstraint, "PrimitiveConstraint is unset")
	}
	operator, ok := apiOperators[c.Operator]
	if !ok {
		return nil, queryErrorf(QueryErrMalformedConstraint, "PrimitiveConstraint on %q has no operator", c.Property)
	}
	if c.Property == QueryPropType && (operator == querytranslate.Equal || operator == querytranslate.In || operator == querytranslate.InstanceOf) {
		if len(c.Value) == 0 {
			return nil, queryErrorf(QueryErrMalformedConstraint, "PrimitiveConstraint on %q has no value to compare against", c.Property)
		}
		if instances, ok, err := querytranslate.ElementInstances(c.Value); err != nil {
			return nil, err
		} else if ok {
			if c.Inverse || operator == querytranslate.InstanceOf {
				return nil, queryErrorf(QueryErrUnsupported, "an instance term on %q selects the objects of its elements; it is not negated and takes =", c.Property)
			}
			return instances, nil
		}
		return querytranslate.TypeTest{Types: c.Value, Exact: operator != querytranslate.InstanceOf, Inverse: c.Inverse}, nil
	}
	return querytranslate.Comparison{Property: c.Property, Operator: operator, Values: c.Value, Inverse: c.Inverse}, nil
}

// apiOperators spells each operator of the standard as the translator does.
var apiOperators = map[pb.PrimitiveOperator]querytranslate.Operator{
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:         querytranslate.Equal,
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER:       querytranslate.Greater,
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:          querytranslate.Less,
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS_EQUAL:    querytranslate.LessEqual,
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER_EQUAL: querytranslate.GreaterEqual,
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_IN:            querytranslate.In,
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_INSTANCE_OF:   querytranslate.InstanceOf,
}

// operatorText is the standard's spelling of an operator, for diagnostics.
func operatorText(op pb.PrimitiveOperator) string {
	if operator, ok := apiOperators[op]; ok {
		return string(operator)
	}
	return "(no operator)"
}

// apiComposite reads a composite constraint: the standard combines at least
// two, so an empty list has no verdict and a one-member list is that member.
func apiComposite(c *pb.CompositeConstraint) (querytranslate.Constraint, error) {
	if c == nil {
		return nil, queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint is unset")
	}
	if len(c.Constraint) == 0 {
		return nil, queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint has no constraints to combine")
	}
	if len(c.Constraint) == 1 {
		return nil, queryErrorf(QueryErrMalformedConstraint,
			"CompositeConstraint combines at least two constraints, got one; write the constraint on its own")
	}
	if c.Operator == pb.CompositeOperator_COMPOSITE_OPERATOR_UNSPECIFIED {
		return nil, queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint has no operator")
	}
	terms := make([]querytranslate.Constraint, 0, len(c.Constraint))
	for _, member := range c.Constraint {
		term, err := apiConstraint(member)
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	switch c.Operator {
	case pb.CompositeOperator_COMPOSITE_OPERATOR_AND:
		return querytranslate.Conjunction{Terms: terms}, nil
	case pb.CompositeOperator_COMPOSITE_OPERATOR_OR:
		return querytranslate.Disjunction{Terms: terms}, nil
	default:
		return nil, queryErrorf(QueryErrMalformedConstraint, "CompositeConstraint operator %d is not one this service defines", c.Operator)
	}
}

// queryStatus reports a refused query as INVALID_ARGUMENT under its kind: the
// translator's faults map onto the kinds the service documents.
func queryStatus(err error) error {
	if err == nil {
		return nil
	}
	var qerr *QueryError
	if !errors.As(err, &qerr) {
		qerr = &QueryError{Kind: translatorKind(err), Message: err.Error(), Err: err}
	}
	status := connect.NewError(connect.CodeInvalidArgument, qerr)
	if outcome := querytranslate.OutcomeOf(err); outcome != nil {
		return withQueryOutcome(status, outcome)
	}
	return status
}

// translatorKind is the service's kind for a translator refusal.
func translatorKind(err error) QueryErrorKind {
	var terr *querytranslate.Error
	if !errors.As(err, &terr) {
		return QueryErrMalformedConstraint
	}
	switch terr.Kind {
	case querytranslate.ErrUnknownProperty:
		return QueryErrUnknownProperty
	case querytranslate.ErrUnorderedProperty:
		return QueryErrUnorderedProperty
	case querytranslate.ErrUnparsableValue:
		return QueryErrUnparsableValue
	case querytranslate.ErrUnknownScope:
		return QueryErrUnknownScope
	case querytranslate.ErrUnsupportedDisjunction, querytranslate.ErrUnsupportedLiteral,
		querytranslate.ErrUnsupportedInstanceTerm, querytranslate.ErrUnsupportedWildcard:
		return QueryErrUnsupported
	default:
		return QueryErrMalformedConstraint
	}
}

// queryExecStatus reports a plan the evaluator refused: a fault in the query
// as INVALID_ARGUMENT under its kind with the QVTS record attached, a model no
// conversion writes elementIds for as FAILED_PRECONDITION, exhaustion as the
// population reports it.
func (h *heldObjects) queryExecStatus(err error) error {
	if status := h.exhausted(err); status != nil {
		return status
	}
	var execErr *queryexec.Error
	if !errors.As(err, &execErr) {
		return documentStatus(err)
	}
	if execErr.Kind == queryexec.ErrorUnevaluableFeature && execErr.Property == corequery.PropertyElementID && execErr.Cause != nil {
		return elementIDStatus(execErr.Cause)
	}
	qerr := &QueryError{Kind: execKind(execErr), Message: execErr.Error(), Err: execErr}
	return withQueryOutcome(connect.NewError(connect.CodeInvalidArgument, qerr), execErr.Outcome)
}

// execKind is the service's kind for an evaluator refusal of a translated plan.
func execKind(err *queryexec.Error) QueryErrorKind {
	switch err.Kind {
	case queryexec.ErrorUnknownProperty, queryexec.ErrorMissingFeature, queryexec.ErrorUnservedProperty:
		return QueryErrUnknownProperty
	case queryexec.ErrorInvalidOperator, queryexec.ErrorInvalidOrder:
		return QueryErrUnorderedProperty
	case queryexec.ErrorInvalidArgument, queryexec.ErrorUnknownClassification:
		return QueryErrUnparsableValue
	default:
		return QueryErrEvaluation
	}
}

// elementIDStatus reports a query that read elementId of a model no conversion
// can write: what it would report does not exist, so the query fails with the
// conversion's refusal rather than answering without the property.
func elementIDStatus(err error) error {
	return statusErrorf(connect.CodeFailedPrecondition,
		"elementId is the id a conversion of the model writes, and converting this model is refused: %v", err)
}

// queryEval names the elements of one query over a model's symbol context, and
// enumerates the ones a scope considers.
type queryEval struct {
	sc     *symbolfacts.Context
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
			if sym := e.sc.Positional.Symbol(fqn); sym != nil {
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
	return e.sc.Identity(sym)
}

// elementIDReader reads the ids Convert writes for this model, from one
// conversion made on first use and shared by every query of the model.
func (m *CachedModel) elementIDReader() *export.ElementIDReader {
	m.elementIDsOnce.Do(func() {
		documents := make([]export.ElementIDDocument, 0, len(m.Documents))
		for _, doc := range m.Documents {
			if doc != nil {
				documents = append(documents, export.ElementIDDocument{File: doc.Source, Root: doc.Root, ParseDiags: doc.ParseDiags})
			}
		}
		m.elementIDs = export.NewElementIDReader(documents)
	})
	return m.elementIDs
}

// queryTable is the model's reflective property table, built once per model.
func (m *CachedModel) queryTable(qctx queryexec.Context) *querytranslate.Table {
	m.queryTableOnce.Do(func() {
		m.propertyTable = querytranslate.NewTable(qctx.Index, qctx.Model)
	})
	return m.propertyTable
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
