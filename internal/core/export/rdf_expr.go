package export

// Expression-valued positions — values, bounds, guards, conditions, filters —
// are mapped to a graph of expression nodes, each keeping its notation as
// sysx:sourceText. See docs/reference/rdf-mapping.md § Expressions.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
)

const literalStatesValue = "a literal expression states the value it evaluates to"

// Metaclasses of the expression nodes, as SysML v2 8.4 names them.
const (
	mExpression       = "Expression"
	mLiteralBoolean   = "LiteralBoolean"
	mLiteralInteger   = "LiteralInteger"
	mLiteralRational  = "LiteralRational"
	mLiteralString    = "LiteralString"
	mLiteralInfinity  = "LiteralInfinity"
	mNullExpression   = "NullExpression"
	mFeatureReference = "FeatureReferenceExpression"
	mFeatureChain     = "FeatureChainExpression"
	mOperator         = "OperatorExpression"
	mInvocation       = "InvocationExpression"
	mCollect          = "CollectExpression"
	mSelect           = "SelectExpression"
	mMetadataAccess   = "MetadataAccessExpression"
)

// Properties of an expression node in the SysML vocabulary.
const (
	pOperator          = "operator"
	pArgument          = "argument"
	pOperand           = "operand"
	pReferent          = "referent"
	pFunction          = "function"
	pReferencedElement = "referencedElement"
)

// Properties this mapping adds: argument order, which RDF does not carry, and
// parts the 202407 metamodel rendering has no property for.
const (
	xArgumentIndex    = "argumentIndex"
	xArgumentName     = "argumentName"
	xTypeArgument     = "typeArgument"
	xIsConstructor    = "isConstructor"
	xBodyParameter    = "bodyParameter"
	xResultExpression = "resultExpression"
)

// mBodyMember types a declaration an expression body makes between its
// parameters and its result, carried as its notation.
const mBodyMember = "BodyMember"

// Operator spellings whose notation is not the plain infix form.
const (
	opSequence = ","
	opIndex    = "[]"
	opAt       = "#"
	opIf       = "if"
)

// expression emits the graph of one expression-valued property. slot names the
// position, so each expression of an element has a distinct identity.
func (e *encoder) expression(subject, property rdf.Term, slot, owner string, node ast.Node) {
	if node == nil {
		return
	}
	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	target := rdf.ExpressionIRI(subject, slot)
	e.graph.Add(subject, property, target)
	e.expressionNode(target, owner, node)
}

// expressionNode emits one expression node and, recursively, its operands.
// Every node carries its notation, so the exact text always survives.
func (e *encoder) expressionNode(subject rdf.Term, owner string, node ast.Node) {
	e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.text(node)))
	// The id an API reader addresses the node by, as on an element: a node has no
	// qualified name, but its position in the model gives it a valid id.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	e.expressionStructure(subject, owner, node)
}

// expressionStructure emits an expression's type and operands; an Expression
// element states its text as an element does, so it takes none here.
func (e *encoder) expressionStructure(subject rdf.Term, owner string, node ast.Node) {
	e.typed(subject, expressionMetaclass(node))
	switch n := node.(type) {
	case *ast.LiteralBool:
		e.graph.Add(subject, e.sysml(pValue), rdf.Bool(n.Value))

	case *ast.LiteralString:
		e.graph.Add(subject, e.sysml(pValue), rdf.String(lexer.StringValue(n.Value)))

	case *ast.LiteralInteger:
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, rdf.XSD+"integer"))

	case *ast.LiteralReal:
		e.graph.Add(subject, e.sysml(pValue), rdf.TypedLiteral(n.Value, realDatatype(n.Value)))

	case *ast.QualifiedName:
		// A position whose notation is a bare name holds the feature it names.
		e.graph.Add(subject, e.sysml(pReferent), e.reference(n))

	case *ast.FeatureReference:
		e.graph.Add(subject, e.sysml(pReferent), e.reference(n.Name))

	case *ast.OperatorExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(n.Operator.String()))
		e.arguments(subject, owner, n.Operands)
		if n.TypeRef != nil {
			e.graph.Add(subject, e.sysx(xTypeArgument), e.reference(n.TypeRef))
		}

	case *ast.CastExpr:
		// `(as T[m])` is the classification operator with a type argument only,
		// its multiplicity written as bounds the way a usage's is.
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(ast.OpAs.String()))
		e.graph.Add(subject, e.sysx(xTypeArgument), e.reference(n.TargetType))
		e.multiplicity(subject, owner, n.Multiplicity)

	case *ast.FeatureChainExpr:
		e.arguments(subject, owner, []ast.Node{n.Operand})
		e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(n.Member))

	case *ast.IndexExpr:
		operator := opAt
		if n.Bracket {
			operator = opIndex
		}
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(operator))
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Index})

	case *ast.InvocationExpr:
		e.invocation(subject, owner, n.Type, n.Operand, n.Args, n.NamedArgs)

	case *ast.ConstructorExpr:
		// The 202407 rendering declares no ConstructorExpression, so `new` is a flag.
		e.graph.Add(subject, e.sysx(xIsConstructor), rdf.Bool(true))
		e.invocation(subject, owner, n.Type, nil, n.Args, n.NamedArgs)

	case *ast.CollectExpr:
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SelectExpr:
		e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SequenceExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(opSequence))
		e.arguments(subject, owner, n.Elements)

	case *ast.MetadataAccessExpr:
		e.graph.Add(subject, e.sysml(pReferencedElement), e.reference(n.Ref))

	case *ast.BodyExpr:
		// A body declares its own parameters and members, then a result expression;
		// sysx:hasBody marks it as one even when it declares nothing. The
		// parameters' annotations are read outside the body, the result inside.
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
		e.bodyDeclarations(subject, owner, n.Params, n.Members)
		if n.Result != nil {
			result := rdf.ExpressionIRI(subject, "result")
			e.graph.Add(subject, e.sysx(xResultExpression), result)
			e.expressionNode(result, owner, n.Result)
		}
	}
}

// realDatatype is the datatype whose lexical space holds a REAL_VALUE token:
// xsd:decimal, or xsd:double when the token has an exponent.
func realDatatype(value string) string {
	if strings.ContainsAny(value, "eE") {
		return rdf.XSD + "double"
	}
	return rdf.XSD + "decimal"
}

func (e *encoder) typed(subject rdf.Term, metaclass string) {
	e.graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
}

// expressionMetaclass is the metaclass an expression node is typed with. A shape
// this mapping does not decompose still states it is an expression.
func expressionMetaclass(node ast.Node) string {
	switch node.(type) {
	case *ast.LiteralBool:
		return mLiteralBoolean
	case *ast.LiteralString:
		return mLiteralString
	case *ast.LiteralInteger:
		return mLiteralInteger
	case *ast.LiteralReal:
		return mLiteralRational
	case *ast.LiteralInfinity:
		return mLiteralInfinity
	case *ast.NullExpr:
		return mNullExpression
	case *ast.QualifiedName, *ast.FeatureReference:
		return mFeatureReference
	case *ast.OperatorExpr, *ast.CastExpr, *ast.IndexExpr, *ast.SequenceExpr:
		return mOperator
	case *ast.FeatureChainExpr:
		return mFeatureChain
	case *ast.InvocationExpr, *ast.ConstructorExpr:
		return mInvocation
	case *ast.CollectExpr:
		return mCollect
	case *ast.SelectExpr:
		return mSelect
	case *ast.MetadataAccessExpr:
		return mMetadataAccess
	}
	return mExpression
}

// bodyDeclarations emits what an expression body declares ahead of its result,
// parameters and members alike, indexed in the one order they were written.
func (e *encoder) bodyDeclarations(subject rdf.Term, owner string, params []ast.BodyParam, members []ast.Node) {
	type declaration struct {
		offset int
		param  *ast.BodyParam
		member ast.Node
	}
	declarations := make([]declaration, 0, len(params)+len(members))
	for i := range params {
		declarations = append(declarations, declaration{offset: params[i].Span.Offset, param: &params[i]})
	}
	for _, member := range members {
		declarations = append(declarations, declaration{offset: member.Span().Offset, member: member})
	}
	sort.SliceStable(declarations, func(i, j int) bool {
		return declarations[i].offset < declarations[j].offset
	})
	for i, decl := range declarations {
		if decl.param != nil {
			e.bodyParameter(subject, owner, i, *decl.param)
		} else {
			e.bodyMember(subject, i, decl.member)
		}
	}
}

// bodyParameter emits one parameter of an expression body as a node of its
// own, so its type, value and bounds are structure, not text.
func (e *encoder) bodyParameter(subject rdf.Term, owner string, index int, param ast.BodyParam) {
	node := rdf.ExpressionIRI(subject, fmt.Sprintf("in%d", index))
	e.graph.Add(subject, e.sysx(xBodyParameter), node)
	e.graph.Add(node, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(keywordMetaclass["ref"]))
	e.graph.Add(node, e.sysml(pElementID), rdf.String(rdf.LocalName(node.Value)))
	e.graph.Add(node, e.sysx(xMemberIndex), rdf.Int(index))
	e.graph.Add(node, e.sysml(pDirection), rdf.String(directionKeyword(ast.DirIn)))
	e.name(node, param.Name)
	e.flags(node, []boolProperty{{"isReference", param.IsReference}})
	if param.Type != nil {
		e.graph.Add(node, e.sysml(relationshipProperty[ast.RelTyping]), e.reference(param.Type))
	}
	e.relationships(node, owner, param.Relationships)
	e.multiplicity(node, owner, param.Multiplicity)
	e.expression(node, e.sysml(pValue), pValue, owner, param.Value)
	e.bodyDeclarations(node, owner, nil, param.Members)
}

// bodyMember carries one declaration of an expression body, or of one of its
// parameters. Documentation is structure; any other declaration is its notation.
func (e *encoder) bodyMember(subject rdf.Term, index int, member ast.Node) {
	node := rdf.ExpressionIRI(subject, fmt.Sprintf("m%d", index))
	e.graph.Add(subject, e.sysx(xBodyMember), node)
	e.graph.Add(node, e.sysml(pElementID), rdf.String(rdf.LocalName(node.Value)))
	e.graph.Add(node, e.sysx(xMemberIndex), rdf.Int(index))
	e.graph.Add(node, e.sysx(xSourceText), rdf.String(e.text(member)))
	if doc, ok := member.(*ast.Documentation); ok {
		e.typed(node, mDocumentation)
		e.documentation(node, doc)
		return
	}
	e.graph.Add(node, rdf.IRI(rdf.RDFType), rdf.OpenSysMLTerm(mBodyMember))
}

// arguments emits the operands of an expression, each carrying the position it
// was written in: RDF states no order between the objects of one property.
func (e *encoder) arguments(subject rdf.Term, owner string, args []ast.Node) {
	for i, arg := range args {
		if arg == nil {
			continue
		}
		child := rdf.ExpressionIRI(subject, fmt.Sprintf("a%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		e.expressionNode(child, owner, arg)
		e.graph.Add(child, e.sysx(xArgumentIndex), rdf.Int(i))
	}
}

// invocation emits the parts an invocation and a constructor share: the function
// invoked, the receiver of a `->` form, and the arguments.
func (e *encoder) invocation(subject rdf.Term, owner string, function *ast.QualifiedName, operand ast.Node, args []ast.Node, named []ast.NamedArg) {
	if function != nil {
		e.graph.Add(subject, e.sysml(pFunction), e.reference(function))
	}
	if operand != nil {
		receiver := rdf.ExpressionIRI(subject, "operand")
		e.graph.Add(subject, e.sysml(pOperand), receiver)
		e.expressionNode(receiver, owner, operand)
	}
	e.arguments(subject, owner, args)
	for i, arg := range named {
		if arg.Value == nil {
			continue
		}
		child := rdf.ExpressionIRI(subject, fmt.Sprintf("n%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		e.expressionNode(child, owner, arg.Value)
		e.graph.Add(child, e.sysx(xArgumentIndex), rdf.Int(i))
		e.graph.Add(child, e.sysx(xArgumentName), rdf.String(qualifiedText(arg.Name)))
	}
}

// expressionMetaclasses are the metaclasses of an expression node, which is a
// part of an element's declaration rather than an element the printer writes.
var expressionMetaclasses = map[string]bool{
	mExpression: true, mLiteralBoolean: true, mLiteralInteger: true,
	mLiteralRational: true, mLiteralString: true, mLiteralInfinity: true,
	mNullExpression: true, mFeatureReference: true, mFeatureChain: true,
	mOperator: true, mInvocation: true, mCollect: true, mSelect: true,
	mMetadataAccess: true,
}

// isExpressionNode reports whether a subject is an expression node rather than an
// element: unowned, and in the expression namespace or an unnamed expression class.
func (d *decoder) isExpressionNode(subject rdf.Term) bool {
	if !subject.IsIRI() {
		return false
	}
	if _, owned := d.owningMembership[subject.Value]; owned || d.graph.HasProperty(subject, rdf.SysML+pOwningMembership) {
		return false
	}
	return strings.HasPrefix(subject.Value, rdf.Expression) ||
		expressionMetaclasses[d.metaclass(subject)] && !d.graph.HasProperty(subject, rdf.SysML+pQualifiedName)
}

// resolveExpressions renders every element's expression-valued properties as
// notation, so the printer reads one text per property.
func (d *decoder) resolveExpressions() error {
	parents := map[string][]rdf.Term{}
	for _, triple := range d.graph.Triples() {
		if !d.isExpressionNode(triple.Object) {
			continue
		}
		parents[triple.Object.Value] = append(parents[triple.Object.Value], triple.Subject)
		el, ok := d.byIRI[triple.Subject.Value]
		if !ok || d.isResultExpression(el) {
			// The subject is an expression node; its parts are written with it.
			continue
		}
		if triple.Predicate.Value == rdf.OpenSysML+xRelatedFeature {
			// A connector end is refused as one before its feature is written.
			if _, err := d.endNameText(triple.Object, el); err != nil {
				return err
			}
		}
		text, err := d.expressionOperand(triple.Object, el, positionBinding(strings.TrimPrefix(triple.Predicate.Value, rdf.SysML)))
		if err != nil {
			return err
		}
		if el.expressions == nil {
			el.expressions = map[string]string{}
		}
		el.expressions[triple.Predicate.Value] = text
	}
	return d.noteSegments(parents)
}

// noteSegments records in wanted the element every feature chain reaches, kept
// notation included, so chooseNames checks the segment reads as it there.
func (d *decoder) noteSegments(parents map[string][]rdf.Term) error {
	for _, node := range d.graph.Subjects() {
		if d.metaclass(node) != mFeatureChain {
			continue
		}
		object, ok := d.graph.Object(node, rdf.SysML+pTargetFeature)
		if !ok || !object.IsIRI() {
			continue
		}
		owners := d.expressionOwners(node, parents)
		if len(owners) == 0 {
			continue
		}
		target, name, err := d.namedMember(object)
		if err != nil {
			return err
		}
		operand := d.operandElement(node)
		for _, in := range owners {
			d.wanted.segments[segmentKey{member: in.qname, operand: operand, name: name, target: target.qname}] = true
		}
	}
	return nil
}

// realValueText spells a number as a REAL_VALUE token (`3` becomes `3.0`);
// a signed or non-finite form has no such token.
func realValueText(lexical string) (string, bool) {
	if lexer.IsRealValue(lexical) {
		return lexical, true
	}
	mantissa, exponent := lexical, ""
	if i := strings.IndexAny(lexical, "eE"); i >= 0 {
		mantissa, exponent = lexical[:i], lexical[i:]
	}
	switch {
	case lexer.IsDecimalValue(mantissa):
		mantissa += ".0"
	case lexer.IsDecimalValue(strings.TrimSuffix(mantissa, ".")):
		mantissa += "0"
	default:
		return "", false
	}
	text := mantissa + exponent
	return text, lexer.IsRealValue(text)
}

// segmentName renders the segment a chain written in `in` reaches: a literal as
// written, an IRI as its element's own name, qualified as chooseNames chose
// where that name alone reads as another element after the operand.
func (d *decoder) segmentName(chain, term rdf.Term, in *element) (string, error) {
	if term.IsLiteral() {
		return qualifiedNameText(term.Value), nil
	}
	target, name, err := d.namedMember(term)
	if err != nil {
		return "", err
	}
	if d.names != nil {
		key := segmentKey{member: in.qname, operand: d.operandElement(chain), name: name, target: target.qname}
		if spelling, ok := d.names.segments[key]; ok {
			return qualifiedNameText(spelling), nil
		}
	}
	return nameText(name), nil
}

// operandElement is the qualified name of the element a chain's operand links
// to: a feature reference's referent or an inner chain's target, the name as
// written where the graph keeps only that, else "".
func (d *decoder) operandElement(chain rdf.Term) string {
	operands := d.graph.Objects(chain, rdf.SysML+pArgument)
	if len(operands) != 1 {
		return ""
	}
	var property string
	switch d.metaclass(operands[0]) {
	case mFeatureReference:
		property = pReferent
	case mFeatureChain:
		property = pTargetFeature
	default:
		return ""
	}
	object, ok := d.graph.Object(operands[0], rdf.SysML+property)
	if !ok {
		return ""
	}
	if object.IsLiteral() {
		return object.Value
	}
	if !object.IsIRI() {
		return ""
	}
	el, err := d.referencedElement(object.Value)
	if err != nil {
		return ""
	}
	return el.qname
}

// expressionOwners are the elements whose declarations an expression node is
// part of: every element reached up the parents, a node shared by several included.
func (d *decoder) expressionOwners(node rdf.Term, parents map[string][]rdf.Term) []*element {
	var owners []*element
	seen := map[string]bool{}
	pending := []rdf.Term{node}
	for len(pending) > 0 {
		node, pending = pending[0], pending[1:]
		if seen[node.Value] {
			continue
		}
		seen[node.Value] = true
		if el, ok := d.byIRI[node.Value]; ok {
			owners = append(owners, el)
			continue
		}
		pending = append(pending, parents[node.Value]...)
	}
	return owners
}

// expressionNodeText writes an expression node back as notation: the notation it
// kept, or notation rebuilt from its structure when it kept none it can use.
func (d *decoder) expressionNodeText(node rdf.Term, in *element) (string, error) {
	if text, ok := d.expressionText(node); ok {
		return text, nil
	}
	form, err := d.expressionForm(node, in)
	return form.text, err
}

// expressionOperand writes an expression node where the notation must bind at
// least as tightly as floor, enclosed in parentheses where its form binds less.
func (d *decoder) expressionOperand(node rdf.Term, in *element, floor int) (string, error) {
	if floor == bindConditional {
		return d.expressionNodeText(node, in)
	}
	form, err := d.operandForm(node, in)
	if err != nil {
		return "", err
	}
	return form.at(floor), nil
}

// operandForm is an expression node as notation, kept or rebuilt, with how
// tightly that notation binds.
func (d *decoder) operandForm(node rdf.Term, in *element) (operand, error) {
	if text, ok := d.expressionText(node); ok {
		return operand{text: text, binding: notationBinding(text)}, nil
	}
	return d.expressionForm(node, in)
}

// expressionForm rebuilds an expression node from its structure, with how
// tightly the notation binds; the notation it kept is not consulted here.
func (d *decoder) expressionForm(node rdf.Term, in *element) (operand, error) {
	primary := func(text string, err error) (operand, error) {
		return operand{text: text, binding: bindPrimary}, err
	}
	metaclass := d.metaclass(node)
	unsupported := func(note string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: note,
		}
	}
	switch metaclass {
	case mLiteralBoolean:
		if !d.graph.HasProperty(node, rdf.SysML+pValue) {
			return primary("", unsupported(literalStatesValue))
		}
		return primary(strconv.FormatBool(d.graph.BoolValue(node, rdf.SysML+pValue)), nil)
	case mLiteralInteger:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return primary("", unsupported(literalStatesValue))
		}
		if !lexer.IsDecimalValue(value) {
			return primary("", unsupported(fmt.Sprintf("the notation spells an integer literal as digits alone, not %q; a sign is an OperatorExpression applied to it", value)))
		}
		return primary(value, nil)
	case mLiteralRational:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return primary("", unsupported(literalStatesValue))
		}
		text, ok := realValueText(value)
		if !ok {
			return primary("", unsupported(fmt.Sprintf("the notation spells a rational literal as an unsigned finite number, not %q; a sign is an OperatorExpression applied to it", value)))
		}
		return primary(text, nil)
	case mLiteralString:
		value, ok := d.graph.Lexical(node, rdf.SysML+pValue)
		if !ok {
			return primary("", unsupported(literalStatesValue))
		}
		return primary(lexer.StringText(value), nil)
	case mLiteralInfinity:
		return primary("*", nil)
	case mNullExpression:
		return primary("null", nil)
	case mFeatureReference:
		return primary(d.expressionReference(node, rdf.SysML+pReferent, in,
			"a feature reference names the feature it reads"))
	case mMetadataAccess:
		name, err := d.expressionReference(node, rdf.SysML+pReferencedElement, in,
			"a metadata access names the element it reads the metadata of")
		if err != nil {
			return primary("", err)
		}
		return primary(name+".metadata", nil)
	case mFeatureChain:
		operands, err := d.expressionArguments(node, in, bindPrimary)
		if err != nil {
			return primary("", err)
		}
		object, ok := d.graph.Object(node, rdf.SysML+pTargetFeature)
		if !ok {
			return primary("", unsupported("a feature chain names the feature it reaches"))
		}
		member, err := d.segmentName(node, object, in)
		if err != nil {
			return primary("", err)
		}
		if len(operands) != 1 {
			return primary("", unsupported("a feature chain applies to exactly one operand"))
		}
		return primary(operands[0]+"."+member, nil)
	case mCollect, mSelect:
		operands, err := d.expressionArguments(node, in, bindPrimary)
		if err != nil {
			return primary("", err)
		}
		if len(operands) != 2 {
			return primary("", unsupported("a collect or select expression applies a body to one operand"))
		}
		separator := "."
		if metaclass == mSelect {
			separator = ".?"
		}
		return primary(operands[0]+separator+operands[1], nil)
	case mOperator:
		return d.operatorForm(node, in)
	case mInvocation:
		return primary(d.invocationText(node, in))
	case mExpression:
		if d.graph.BoolValue(node, rdf.OpenSysML+xHasBody) ||
			d.graph.HasProperty(node, rdf.OpenSysML+xResultExpression) ||
			d.graph.HasProperty(node, rdf.OpenSysML+xBodyParameter) ||
			d.graph.HasProperty(node, rdf.OpenSysML+xBodyMember) {
			return primary(d.expressionBodyText(node, in))
		}
	}
	return primary("", unsupported("this expression states no notation and no structure to write one from; "+rdfLimitationsNote))
}

// expressionBodyText rebuilds an expression body: its declarations and its
// result, in the order the graph records.
func (d *decoder) expressionBodyText(node rdf.Term, in *element) (string, error) {
	parts, err := d.bodyDeclarationsText(node, in)
	if err != nil {
		return "", err
	}
	if result, ok := d.graph.Object(node, rdf.OpenSysML+xResultExpression); ok {
		text, err := d.expressionNodeText(result, in)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return "{}", nil
	}
	return "{ " + strings.Join(parts, " ") + " }", nil
}

// bodyParameterText rebuilds one `in` parameter of an expression body. A
// parameter a graph states as a bare name literal is that name alone.
func (d *decoder) bodyParameterText(param rdf.Term, in *element) (string, error) {
	if !param.IsIRI() {
		return "in " + nameText(param.Value) + ";", nil
	}
	el := d.expressionElement(param, in)
	what := fmt.Sprintf("the body parameter <%s>", param.Value)
	// A body parameter is written `in name`, so the node must be a Feature whose
	// direction, when stated, is in; any other shape would be rewritten, not kept.
	if metaclass := d.metaclass(param); !ontology.IsAncestorOrSelf(metaclass, "Feature") {
		return "", &UnsupportedError{
			What: what,
			Note: fmt.Sprintf("a parameter of an expression body is a Feature, and this one is %s", typeDescription(metaclass)),
		}
	}
	if direction, ok := d.stringOf(el, rdf.SysML+pDirection); ok && direction != directionKeyword(ast.DirIn) {
		return "", &UnsupportedError{
			What: what,
			Note: fmt.Sprintf("a parameter of an expression body is written `in`, and its sysml:direction is %q", direction),
		}
	}
	name, ok := d.stringOf(el, rdf.SysML+pDeclaredName)
	if !ok {
		return "", &UnsupportedError{
			What: what,
			Note: "a parameter of an expression body is named, and this one states no sysml:declaredName",
		}
	}
	// The bounds and the value are expressions, resolved here since the
	// parameter is not an element of the model.
	for _, property := range []string{pLowerBound, pUpperBound, pValue} {
		object, ok := d.graph.Object(param, rdf.SysML+property)
		if !ok {
			continue
		}
		text, err := d.expressionOperand(object, in, positionBinding(property))
		if err != nil {
			return "", err
		}
		el.expressions[rdf.SysML+property] = text
	}
	words := []string{"in"}
	if d.boolOf(el, rdf.SysML+"isReference") {
		words = append(words, "ref")
	}
	words = append(words, nameText(name))
	relationships, err := d.relationshipWords(el, d.multiplicityText(el))
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	head := strings.Join(words, " ")
	if value, ok := d.stringOf(el, rdf.SysML+pValue); ok {
		head += " = " + value
	}
	members, err := d.bodyDeclarationsText(param, in)
	if err != nil {
		return "", err
	}
	if len(members) == 0 {
		return head + ";", nil
	}
	return head + " { " + strings.Join(members, " ") + " }", nil
}

// expressionElement stands for an expression node written inside element in:
// the node's own properties, read where in's references are.
func (d *decoder) expressionElement(node rdf.Term, in *element) *element {
	return &element{iri: node.Value, qname: in.qname, scope: in.scope, expressions: map[string]string{}}
}

func typeDescription(metaclass string) string {
	if metaclass == "" {
		return "of no rdf:type"
	}
	return "a " + curie(rdf.SysML+metaclass)
}

// bodyDeclarationsText writes what an expression body declares, parameters and
// members merged by the one sysx:memberIndex they were written in.
func (d *decoder) bodyDeclarationsText(node rdf.Term, in *element) ([]string, error) {
	type declaration struct {
		term  rdf.Term
		param bool
	}
	var declarations []declaration
	for _, param := range d.graph.Objects(node, rdf.OpenSysML+xBodyParameter) {
		declarations = append(declarations, declaration{term: param, param: true})
	}
	for _, member := range d.graph.Objects(node, rdf.OpenSysML+xBodyMember) {
		declarations = append(declarations, declaration{term: member})
	}
	sort.SliceStable(declarations, func(i, j int) bool {
		return intOf(d.graph, declarations[i].term, rdf.OpenSysML+xMemberIndex) < intOf(d.graph, declarations[j].term, rdf.OpenSysML+xMemberIndex)
	})
	var out []string
	for _, decl := range declarations {
		var (
			text string
			err  error
		)
		if decl.param {
			text, err = d.bodyParameterText(decl.term, in)
		} else {
			text, err = d.bodyMemberText(decl.term, in)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, text)
	}
	return out, nil
}

// bodyMemberText writes one declaration of an expression body: documentation
// from its structure, anything else from its notation, or reports it by name.
func (d *decoder) bodyMemberText(member rdf.Term, in *element) (string, error) {
	if d.metaclass(member) == mDocumentation {
		return d.documentationHead(d.expressionElement(member, in)), nil
	}
	text, ok := d.graph.Lexical(member, rdf.OpenSysML+xSourceText)
	if !ok || text == "" {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the body member <%s>", member.Value),
			Note: "a declaration inside an expression body is carried as its notation in sysx:sourceText, and this one has none; " + rdfLimitationsNote,
		}
	}
	return strings.TrimSpace(text), nil
}

// operatorForm rebuilds an operator expression. Each operand is enclosed in
// parentheses only where its own form binds too loosely for its position.
func (d *decoder) operatorForm(node rdf.Term, in *element) (operand, error) {
	operator, ok := d.graph.Lexical(node, rdf.SysML+pOperator)
	if !ok {
		return operand{}, &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: "an operator expression states the operator it applies",
		}
	}
	args, err := d.expressionOperands(node, in)
	if err != nil {
		return operand{}, err
	}
	typeArgument, hasType, err := d.expressionTypeArgument(node, in)
	if err != nil {
		return operand{}, err
	}
	primary := func(text string) (operand, error) {
		return operand{text: text, binding: bindPrimary}, nil
	}
	infix, isInfix := infixBinding[operator]
	switch {
	case operator == opSequence:
		elements := joinOperands(args, bindConditional)
		return operand{text: "(" + elements + ")", binding: bindPrimary, elements: elements}, nil
	case operator == opIf && len(args) == 3:
		// The condition is read below the conditional form; either branch may be one.
		text := "if " + args[0].at(bindNullCoalesce) + " ? " + args[1].at(bindConditional) + " else " + args[2].at(bindConditional)
		return operand{text: text, binding: bindConditional}, nil
	case operator == opIndex && len(args) == 2:
		return primary(args[0].at(bindPrimary) + "[" + indexText(args[1]) + "]")
	case operator == opAt && len(args) == 2:
		return primary(args[0].at(bindPrimary) + "#(" + indexText(args[1]) + ")")
	case hasType && len(args) == 1 && isInfix:
		return operand{text: args[0].at(infix) + " " + operator + " " + typeArgument, binding: infix}, nil
	case hasType && len(args) == 0:
		multiplicity, err := d.expressionMultiplicityText(node, in)
		if err != nil {
			return operand{}, err
		}
		return primary("(" + operator + " " + typeArgument + multiplicity + ")")
	case len(args) == 1 && prefixOperators[operator]:
		return operand{text: operator + " " + args[0].at(bindUnary), binding: bindUnary}, nil
	case len(args) == 2 && isInfix:
		// Operands group to the left, so an equal binding on the right is enclosed;
		// exponentiation groups to the right.
		left, right := infix, infix+1
		if infix == bindExponent {
			left, right = infix+1, infix
		}
		return operand{text: args[0].at(left) + " " + operator + " " + args[1].at(right), binding: infix}, nil
	}
	return operand{}, &UnsupportedError{
		What: fmt.Sprintf("the expression <%s>", node.Value),
		Note: fmt.Sprintf("the operator %q is written with %d operand(s), which has no notation", operator, len(args)),
	}
}

// indexText writes an index: the brackets enclose a sequence, so a
// multi-dimensional one is its bare elements.
func indexText(index operand) string {
	if index.elements != "" {
		return index.elements
	}
	return index.text
}

func (d *decoder) invocationText(node rdf.Term, in *element) (string, error) {
	function, err := d.expressionReference(node, rdf.SysML+pFunction, in,
		"an invocation names the function it invokes")
	if err != nil {
		return "", err
	}
	args, err := d.expressionArguments(node, in, bindConditional)
	if err != nil {
		return "", err
	}
	call := function + "(" + strings.Join(args, ", ") + ")"
	if d.graph.BoolValue(node, rdf.OpenSysML+xIsConstructor) {
		call = "new " + call
	}
	if receiver, ok := d.graph.Object(node, rdf.SysML+pOperand); ok {
		text, err := d.expressionOperand(receiver, in, bindPrimary)
		if err != nil {
			return "", err
		}
		return text + "->" + call, nil
	}
	return call, nil
}

// expressionOperands rebuilds the operands in the order sysx:argumentIndex records.
func (d *decoder) expressionOperands(node rdf.Term, in *element) ([]operand, error) {
	type argument struct {
		index int
		form  operand
	}
	objects := d.graph.Objects(node, rdf.SysML+pArgument)
	args := make([]argument, 0, len(objects))
	for i, object := range objects {
		form, err := d.operandForm(object, in)
		if err != nil {
			return nil, err
		}
		arg := argument{index: i, form: form}
		if written, ok := d.graph.Lexical(object, rdf.OpenSysML+xArgumentIndex); ok {
			if parsed, err := strconv.Atoi(written); err == nil {
				arg.index = parsed
			}
		}
		if name, ok := d.graph.Lexical(object, rdf.OpenSysML+xArgumentName); ok {
			// A named argument is delimited by the invocation it is written in.
			arg.form = operand{text: qualifiedNameText(name) + " = " + form.text, binding: bindPrimary}
		}
		args = append(args, arg)
	}
	sort.SliceStable(args, func(i, j int) bool { return args[i].index < args[j].index })
	out := make([]operand, 0, len(args))
	for _, arg := range args {
		out = append(out, arg.form)
	}
	return out, nil
}

// expressionArguments writes the operands in order, each where it must bind at
// least as tightly as floor.
func (d *decoder) expressionArguments(node rdf.Term, in *element, floor int) ([]string, error) {
	args, err := d.expressionOperands(node, in)
	if err != nil {
		return nil, err
	}
	return splitOperands(args, floor), nil
}

// splitOperands writes each operand where it must bind at least as tightly as floor.
func splitOperands(args []operand, floor int) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, arg.at(floor))
	}
	return out
}

// joinOperands writes the operands comma-separated, each at floor.
func joinOperands(args []operand, floor int) string {
	return strings.Join(splitOperands(args, floor), ", ")
}

// expressionReference names the element an expression property points at.
func (d *decoder) expressionReference(node rdf.Term, property string, in *element, why string) (string, error) {
	object, ok := d.graph.Object(node, property)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: why,
		}
	}
	return d.referenceName(object, in)
}

// expressionMultiplicityText writes the `[lower..upper]` a cast states, its
// bounds being expressions of their own.
func (d *decoder) expressionMultiplicityText(node rdf.Term, in *element) (string, error) {
	el := d.expressionElement(node, in)
	for _, property := range []string{pLowerBound, pUpperBound} {
		object, ok := d.graph.Object(node, rdf.SysML+property)
		if !ok {
			continue
		}
		text, err := d.expressionOperand(object, in, positionBinding(property))
		if err != nil {
			return "", err
		}
		el.expressions[rdf.SysML+property] = text
	}
	return d.multiplicityText(el), nil
}

// expressionTypeArgument names the type a classification operator applies.
func (d *decoder) expressionTypeArgument(node rdf.Term, in *element) (string, bool, error) {
	object, ok := d.graph.Object(node, rdf.OpenSysML+xTypeArgument)
	if !ok {
		return "", false, nil
	}
	name, err := d.referenceName(object, in)
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}
