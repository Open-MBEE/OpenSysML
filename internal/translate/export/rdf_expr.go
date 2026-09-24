package export

// Expression-valued positions — values, bounds, guards, conditions, filters —
// are mapped to a graph of expression nodes, each keeping its notation as
// sysx:sourceText. See docs/reference/rdf-mapping.md § Expressions.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

const literalStatesValue = "a literal expression states the value it evaluates to"

// Metaclasses of the expression nodes, as SysML v2 8.4 names them.
const (
	mExpression          = "Expression"
	mLiteralBoolean      = "LiteralBoolean"
	mLiteralInteger      = "LiteralInteger"
	mLiteralRational     = "LiteralRational"
	mLiteralString       = "LiteralString"
	mLiteralInfinity     = "LiteralInfinity"
	mNullExpression      = "NullExpression"
	mFeatureReference    = "FeatureReferenceExpression"
	mFeatureChain        = "FeatureChainExpression"
	mOperator            = "OperatorExpression"
	mInvocation          = "InvocationExpression"
	mConstructor         = "ConstructorExpression"
	mIndex               = "IndexExpression"
	mCollect             = "CollectExpression"
	mSelect              = "SelectExpression"
	mMetadataAccess      = "MetadataAccessExpression"
	mReferenceSubsetting = "ReferenceSubsetting"
)

// Properties of an expression node in the SysML vocabulary.
const (
	pOperator          = "operator"
	pArgument          = "argument"
	pParameter         = "parameter"
	pInput             = "input"
	pOperand           = "operand"
	pReferent          = "referent"
	pFunction          = "function"
	pReferencedElement = "referencedElement"
	pResult            = "result"
)

// Properties this mapping adds: argument order, which RDF does not carry, and
// parts the 202407 metamodel rendering has no property for.
const (
	xArgumentIndex    = "argumentIndex"
	xArgumentName     = "argumentName"
	xTypeArgument     = "typeArgument"
	xBodyParameter    = "bodyParameter"
	xResultExpression = "resultExpression"
)

// Operator spellings whose notation is not the plain infix form; `[` is the
// spelling the SysML v2 interchange gives the bracket operator (sysmlv2 json).
const (
	opSequence = ","
	opIndex    = "["
	opIndexOld = "[]"
	opAt       = "#"
	opIf       = "if"
)

// expression emits the graph of one expression-valued property. slot names the
// position, so each expression of an element has a distinct identity; owner is
// the qualified name of the member whose declaration the expression is part of.
func (e *encoder) expression(subject, property rdf.Term, slot, owner string, node ast.Node) error {
	membership := mOwningMembership
	if property == e.sysml(pValue) {
		membership = mFeatureValue
	}
	return e.expressionAs(subject, property, slot, owner, node, membership)
}

// expressionAs is expression with the metaclass of the membership that owns
// the node: a ParameterMembership for a condition the pilot reads as a parameter.
func (e *encoder) expressionAs(subject, property rdf.Term, slot, owner string, node ast.Node, membership string) error {
	if node == nil {
		return nil
	}
	e.graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	target := e.ids.mintedNode(rdf.ExpressionIRI(subject, slot), subject, slot)
	e.graph.Add(subject, property, target)
	if err := e.expressionNode(target, owner, node); err != nil {
		return err
	}
	if strings.HasPrefix(subject.Value, rdf.Expression) {
		return nil
	}
	return e.expressionOwnership(target, subject, membership)
}

// expressionNode emits one expression node and, recursively, its operands.
// Every node carries its notation, so the exact text always survives.
func (e *encoder) expressionNode(subject rdf.Term, owner string, node ast.Node) error {
	e.graph.Add(subject, e.sysx(xSourceText), rdf.String(e.text(node)))
	// The id an API reader addresses the node by, as on an element: a node has no
	// qualified name, but its position in the model gives it a valid id.
	e.graph.Add(subject, e.sysml(pElementID), rdf.String(rdf.LocalName(subject.Value)))
	return e.expressionStructure(subject, owner, node)
}

// resultBearing lists the expression metaclasses the pilot gives an owned
// result parameter (KerML 1.0 § 8.3.4.7.3 Expression).
var resultBearing = map[string]bool{
	mFeatureReference: true, mFeatureChain: true, mOperator: true, mIndex: true,
	mInvocation: true, mConstructor: true, mCollect: true, mSelect: true,
}

// resultParameter emits the `out` Feature an expression's ReturnParameterMembership owns.
func (e *encoder) resultParameter(node rdf.Term) {
	result := e.ids.mintedNode(rdf.ExpressionIRI(node, "out"), node, "out")
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(result), result, rdf.OwningMembershipSuffix)
	e.typed(result, mFeature)
	e.graph.Add(result, e.sysml(pElementID), rdf.String(rdf.LocalName(result.Value)))
	e.graph.Add(result, e.sysml(pDirection), rdf.String("out"))
	e.emitMembershipCore(membership, result, node, mReturnParameterMembership, true)
	e.graph.Add(membership, e.sysml(pOwnedMemberFeature), result)
	e.graph.Add(membership, e.sysml(pOwnedMemberParameter), result)
	e.graph.Add(node, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(node, e.sysml(pOwnedMembership), membership)
	e.graph.Add(node, e.sysml(pOwnedFeatureMembership), membership)
	e.graph.Add(node, e.sysml(pOwnedFeature), result)
	e.graph.Add(node, e.sysml(pParameter), result)
	e.graph.Add(node, e.sysml(pResult), result)
}

// expressionStructure emits an expression's type and operands; an Expression
// element states its text as an element does, so it takes none here.
func (e *encoder) expressionStructure(subject rdf.Term, owner string, node ast.Node) error {
	e.typed(subject, expressionMetaclass(node))
	if err := e.expressionOperands(subject, owner, node); err != nil {
		return err
	}
	if resultBearing[expressionMetaclass(node)] {
		e.resultParameter(subject)
	}
	return nil
}

// expressionOperands emits the operands and stored properties of one expression node.
func (e *encoder) expressionOperands(subject rdf.Term, owner string, node ast.Node) error {
	switch n := node.(type) {
	case *ast.LiteralBool:
		e.graph.Add(subject, e.sysml(pValue), rdf.Bool(n.Value))

	case *ast.LiteralString:
		e.graph.Add(subject, e.sysml(pValue), rdf.String(source.StringValue(n.Value)))

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
		if err := e.arguments(subject, owner, n.Operands); err != nil {
			return err
		}
		if n.TypeRef != nil {
			e.typeArgument(subject, n.TypeRef, len(n.Operands))
		}

	case *ast.CastExpr:
		// `(as T[m])` is the classification operator with a type argument only,
		// its multiplicity written as bounds the way a usage's is.
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(ast.OpAs.String()))
		e.typeArgument(subject, n.TargetType, 0)
		return e.multiplicity(subject, owner, n.Multiplicity)

	case *ast.FeatureChainExpr:
		if err := e.arguments(subject, owner, []ast.Node{n.Operand}); err != nil {
			return err
		}
		if qualifiedNameHasChain(n.Member) {
			// A chained member is the chain feature the expression owns through
			// an OwningMembership (OwnedFeatureChainMember).
			chain := e.ids.mintedNode(rdf.ExpressionIRI(subject, "targetFeature"), subject, "targetFeature")
			e.typed(chain, mFeature)
			e.graph.Add(chain, e.sysml(pElementID), rdf.String(rdf.LocalName(chain.Value)))
			e.featureChainings(chain, e.qualifiedChainReferences(n.Member))
			membership := e.ids.minted(rdf.OwningMembershipIRIOf(chain), chain, rdf.OwningMembershipSuffix)
			e.emitMembershipCore(membership, chain, subject, mOwningMembership, true)
			e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
			e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
			e.graph.Add(subject, e.sysml(pTargetFeature), chain)
		} else {
			e.graph.Add(subject, e.sysml(pTargetFeature), e.reference(n.Member))
		}

	case *ast.IndexExpr:
		operator := opAt
		if n.Bracket {
			operator = opIndex
		}
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(operator))
		return e.arguments(subject, owner, []ast.Node{n.Operand, n.Index})

	case *ast.InvocationExpr:
		return e.invocation(subject, owner, n.Type, n.Operand, n.Args, n.NamedArgs)

	case *ast.ConstructorExpr:
		return e.invocation(subject, owner, n.Type, nil, n.Args, n.NamedArgs)

	case *ast.CollectExpr:
		return e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SelectExpr:
		return e.arguments(subject, owner, []ast.Node{n.Operand, n.Body})

	case *ast.SequenceExpr:
		e.graph.Add(subject, e.sysml(pOperator), rdf.String(opSequence))
		return e.arguments(subject, owner, n.Elements)

	case *ast.MetadataAccessExpr:
		e.graph.Add(subject, e.sysml(pReferencedElement), e.reference(n.Ref))

	case *ast.BodyExpr:
		return e.bodyExpression(subject, owner, n)
	}
	return nil
}

// bodyExpression emits `{ in x; ... result }` as the pilot does (KerMLExpressions
// BodyExpression): a FeatureReferenceExpression whose referent is the body
// Expression it owns through a FeatureMembership. The body declares its
// parameters and members through memberships and its result through a
// ResultExpressionMembership; sysx:hasBody marks it even when it declares nothing.
func (e *encoder) bodyExpression(subject rdf.Term, owner string, n *ast.BodyExpr) error {
	body := e.ids.mintedNode(rdf.ExpressionIRI(subject, "body"), subject, "body")
	e.typed(body, mExpression)
	e.graph.Add(body, e.sysml(pElementID), rdf.String(rdf.LocalName(body.Value)))
	e.graph.Add(subject, e.sysml(pReferent), body)
	e.bodyMembership(subject, body, mFeatureMembership)
	e.graph.Add(body, e.sysx(xHasBody), rdf.Bool(true))
	// The parts keep the ids they had off the reference node, so an id form
	// does not move with the body node between them.
	if err := e.bodyDeclarations(body, subject, owner, n.Params, n.Members); err != nil {
		return err
	}
	if n.Result != nil {
		result := e.ids.mintedNode(rdf.ExpressionIRI(subject, "result"), subject, "result")
		e.graph.Add(body, e.sysx(xResultExpression), result)
		if err := e.expressionNode(result, owner, n.Result); err != nil {
			return err
		}
		membership := e.bodyMembership(body, result, mResultExpressionMembership)
		e.graph.Add(membership, e.sysml(pOwnedResultExpression), result)
	}
	return nil
}

// bodyMembership owns one part of an expression body through a membership of
// the given metaclass, returning the membership.
func (e *encoder) bodyMembership(owner, member rdf.Term, metaclass string) rdf.Term {
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(member), member, rdf.OwningMembershipSuffix)
	e.emitMembershipCore(membership, member, owner, metaclass, true)
	e.graph.Add(owner, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(owner, e.sysml(pOwnedMembership), membership)
	if metaclass == mFeatureMembership {
		e.graph.Add(owner, e.sysml(pOwnedFeatureMembership), membership)
		e.graph.Add(owner, e.sysml(pOwnedFeature), member)
		e.graph.Add(membership, e.sysml(pOwnedMemberFeature), member)
	}
	return membership
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
	switch n := node.(type) {
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
	case *ast.QualifiedName, *ast.FeatureReference, *ast.BodyExpr:
		return mFeatureReference
	case *ast.OperatorExpr, *ast.CastExpr, *ast.SequenceExpr:
		return mOperator
	case *ast.IndexExpr:
		// `x[i]` is the `[` operator; only `x#(i)` is an IndexExpression.
		if n.Bracket {
			return mOperator
		}
		return mIndex
	case *ast.FeatureChainExpr:
		return mFeatureChain
	case *ast.InvocationExpr:
		return mInvocation
	case *ast.ConstructorExpr:
		return mConstructor
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
// The body owns them; their ids are minted off base.
func (e *encoder) bodyDeclarations(subject, base rdf.Term, owner string, params []ast.BodyParam, members []ast.Node) error {
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
		var err error
		if decl.param != nil {
			err = e.bodyParameter(subject, base, owner, i, *decl.param)
		} else {
			err = e.bodyMember(subject, base, owner, i, decl.member)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// bodyParameter emits one parameter of an expression body as a node of its
// own, so its type, value and bounds are structure, not text.
func (e *encoder) bodyParameter(subject, base rdf.Term, owner string, index int, param ast.BodyParam) error {
	node := e.ids.mintedNode(rdf.ExpressionIRI(base, fmt.Sprintf("in%d", index)), base, fmt.Sprintf("in%d", index))
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
	e.bodyMembership(subject, node, mFeatureMembership)
	if err := e.multiplicity(node, owner, param.Multiplicity); err != nil {
		return err
	}
	if err := e.expression(node, e.sysml(pValue), pValue, owner, param.Value); err != nil {
		return err
	}
	return e.bodyDeclarations(node, node, owner, nil, param.Members)
}

// bodyMember emits one declaration of an expression body as the member it is:
// positioned by the body, with no qualified name or namespace of its own.
func (e *encoder) bodyMember(subject, base rdf.Term, owner string, index int, member ast.Node) error {
	node, visibility := unwrapMember(member)
	if node == nil {
		return &UnsupportedError{
			What: fmt.Sprintf("the empty member at %s", e.where(member)),
			Note: "a membership in an expression body declares the member it holds",
		}
	}
	local := e.ids.mintedNode(rdf.ExpressionIRI(base, fmt.Sprintf("m%d", index)), base, fmt.Sprintf("m%d", index))
	e.graph.Add(subject, e.sysx(xBodyMember), local)
	if err := e.encodeMember(memberHead{node: node, visibility: visibility, index: index, inline: true, local: local}, owner); err != nil {
		return err
	}
	metaclass := mOwningMembership
	switch {
	case ast.IsExpression(node):
		metaclass = mResultExpressionMembership
	case ontology.IsAncestorOrSelf(e.metaclassOf(local), mFeature):
		metaclass = mFeatureMembership
	}
	membership := e.bodyMembership(subject, local, metaclass)
	if metaclass == mResultExpressionMembership {
		e.graph.Add(membership, e.sysml(pOwnedResultExpression), local)
	}
	return nil
}

// arguments emits the operands of an expression, each carrying the position it
// was written in: RDF states no order between the objects of one property.
func (e *encoder) arguments(subject rdf.Term, owner string, args []ast.Node) error {
	return e.argumentsFrom(subject, owner, args, 0)
}

// argumentsFrom emits args as the operands numbered from first onwards.
func (e *encoder) argumentsFrom(subject rdf.Term, owner string, args []ast.Node, first int) error {
	for i, arg := range args {
		if arg == nil {
			continue
		}
		child := e.ids.mintedNode(rdf.ExpressionIRI(subject, fmt.Sprintf("a%d", i)), subject, fmt.Sprintf("a%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		if err := e.expressionNode(child, owner, arg); err != nil {
			return err
		}
		if err := e.expressionOperandOwnership(subject, child, first+i); err != nil {
			return err
		}
	}
	return nil
}

// typeArgument states the type a classification operator tests or casts to:
// collapsed as sysx:typeArgument and, as the pilot writes it (SysML.xtext
// TypeReferenceMember), as an in parameter typed by that type with no value.
func (e *encoder) typeArgument(subject rdf.Term, typeRef *ast.QualifiedName, index int) {
	target := e.reference(typeRef)
	e.graph.Add(subject, e.sysx(xTypeArgument), target)
	if !target.IsIRI() {
		return
	}
	parameter := e.ids.mintedNode(rdf.ExpressionIRI(subject, fmt.Sprintf("in%d", index)), subject, fmt.Sprintf("in%d", index))
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(parameter), parameter, rdf.OwningMembershipSuffix)
	e.typed(parameter, mFeature)
	e.graph.Add(parameter, e.sysml(pElementID), rdf.String(rdf.LocalName(parameter.Value)))
	e.graph.Add(parameter, e.sysml(pDirection), rdf.String("in"))
	e.emitMembershipCore(membership, parameter, subject, mParameterMembership, true)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
	e.graph.Add(subject, e.sysml(pOwnedFeatureMembership), membership)
	e.graph.Add(subject, e.sysml(pOwnedFeature), parameter)
	e.graph.Add(subject, e.sysml(pParameter), parameter)
	e.graph.Add(subject, e.sysml(pInput), parameter)
	e.graph.Add(membership, e.sysml(pOwnedMemberFeature), parameter)
	e.graph.Add(membership, e.sysml(pOwnedMemberParameter), parameter)
	spec, _ := e.relationshipSpec(parameter, ast.RelTyping, 1, 0)
	e.emitRelationship(parameter, target, spec)
}

// invocation emits the parts an invocation and a constructor share: the function
// invoked, the receiver of a `->` form, and the arguments. The receiver is the
// first argument (KerML 8.2.5.8.4: `x->f(a)` is `f(x, a)`), kept apart as
// sysml:operand so the arrow spelling survives.
func (e *encoder) invocation(subject rdf.Term, owner string, function *ast.QualifiedName, operand ast.Node, args []ast.Node, named []ast.NamedArg) error {
	if function != nil {
		e.graph.Add(subject, e.sysml(pFunction), e.calleeReference(function))
		e.calleeMembership(subject, function)
	}
	first := 0
	if operand != nil {
		receiver := e.ids.mintedNode(rdf.ExpressionIRI(subject, "operand"), subject, "operand")
		e.graph.Add(subject, e.sysml(pOperand), receiver)
		if err := e.expressionNode(receiver, owner, operand); err != nil {
			return err
		}
		if function != nil {
			if err := e.expressionOperandOwnership(subject, receiver, 0); err != nil {
				return err
			}
			first = 1
		}
	}
	if err := e.argumentsFrom(subject, owner, args, first); err != nil {
		return err
	}
	first += len(args)
	for i, arg := range named {
		if arg.Value == nil {
			continue
		}
		child := e.ids.mintedNode(rdf.ExpressionIRI(subject, fmt.Sprintf("n%d", i)), subject, fmt.Sprintf("n%d", i))
		e.graph.Add(subject, e.sysml(pArgument), child)
		if err := e.expressionNode(child, owner, arg.Value); err != nil {
			return err
		}
		if err := e.expressionOperandOwnership(subject, child, first+i); err != nil {
			return err
		}
		e.graph.Add(child, e.sysx(xArgumentName), rdf.String(qualifiedText(arg.Name)))
	}
	return nil
}

// calleeReference is the collapsed function: the element it resolves to, else
// its spelling, dots kept at the chained joints so it matches the owned chain.
func (e *encoder) calleeReference(function *ast.QualifiedName) rdf.Term {
	target := e.reference(function)
	if target.IsIRI() || !qualifiedNameHasChain(function) {
		return target
	}
	return rdf.String(chainedText(function))
}

// chainedText spells a name as written: `::` between qualifying segments and
// `.` before each chained one.
func chainedText(name *ast.QualifiedName) string {
	var out strings.Builder
	if name.Global {
		out.WriteString("$::")
	}
	for i, part := range name.Parts {
		switch {
		case i == 0:
		case part.Chained:
			out.WriteString(".")
		default:
			out.WriteString("::")
		}
		out.WriteString(part.Text)
	}
	return out.String()
}

// calleeMembership emits the relationship an invocation owns to what it
// invokes (KerML 1.0 § 8.3.4.8.7 InstantiationExpression): a Membership whose member is
// the named function (its name, where it resolves to nothing), or an
// OwningMembership owning the chain feature a dotted callee reaches.
func (e *encoder) calleeMembership(subject rdf.Term, function *ast.QualifiedName) {
	if qualifiedNameHasChain(function) {
		chain := e.ids.mintedNode(rdf.ExpressionIRI(subject, "function"), subject, "function")
		e.typed(chain, mFeature)
		e.graph.Add(chain, e.sysml(pElementID), rdf.String(rdf.LocalName(chain.Value)))
		e.featureChainings(chain, e.qualifiedChainReferences(function))
		membership := e.ids.minted(rdf.OwningMembershipIRIOf(chain), chain, rdf.OwningMembershipSuffix)
		e.emitMembershipCore(membership, chain, subject, mOwningMembership, true)
		e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
		e.graph.Add(subject, e.sysml(pOwnedMembership), membership)
		return
	}
	target := e.reference(function)
	membership := e.ids.mintedNode(rdf.ExpressionIRI(subject, "function"), subject, "function")
	e.typed(membership, mMembership)
	e.graph.Add(membership, e.sysml(pElementID), rdf.String(rdf.LocalName(membership.Value)))
	e.graph.Add(membership, e.sysml(pMemberElement), target)
	e.graph.Add(membership, e.sysml(pOwner), subject)
	e.graph.Add(membership, e.sysml(pOwningRelatedElement), subject)
	e.graph.Add(subject, e.sysml(pOwnedRelationship), membership)
}

// expressionOwnership owns an expression root through the position that holds it.
func (e *encoder) expressionOwnership(node, owner rdf.Term, metaclass string) error {
	if isRelationship(e.metaclassOf(owner)) {
		e.relationshipOwnership(node, owner, e.metaclassOf(owner), e.metaclassOf(node))
		return nil
	}
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(node), node, rdf.OwningMembershipSuffix)
	e.emitMembershipCore(membership, node, owner, metaclass, true)
	e.graph.Add(owner, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(owner, e.sysml(pOwnedMembership), membership)
	if metaclass == mParameterMembership {
		e.graph.Add(owner, e.sysml(pOwnedFeatureMembership), membership)
		e.graph.Add(membership, e.sysml(pOwnedMemberParameter), node)
	}
	if metaclass == mFeatureValue {
		e.graph.Add(owner, e.sysml(pValue), node)
		e.graph.Add(membership, e.sysml(pFeatureWithValue), owner)
		e.graph.Add(membership, e.sysml(pValue), node)
		if e.graph.BoolValue(owner, rdf.SysML+pIsDefault) {
			e.graph.Add(membership, e.sysml(pIsDefault), rdf.Bool(true))
		}
		if e.graph.BoolValue(owner, rdf.SysML+pIsInitial) {
			e.graph.Add(membership, e.sysml(pIsInitial), rdf.Bool(true))
		}
	}
	return nil
}

// expressionOperandOwnership models an operand as an input parameter and value.
func (e *encoder) expressionOperandOwnership(node, operand rdf.Term, index int) error {
	parameter := e.ids.mintedNode(rdf.ExpressionIRI(node, fmt.Sprintf("in%d", index)), node, fmt.Sprintf("in%d", index))
	membership := e.ids.minted(rdf.OwningMembershipIRIOf(parameter), parameter, rdf.OwningMembershipSuffix)
	valueMembership := e.ids.minted(rdf.OwningMembershipIRIOf(operand), operand, rdf.OwningMembershipSuffix)
	e.typed(parameter, mFeature)
	e.graph.Add(parameter, e.sysml(pElementID), rdf.String(rdf.LocalName(parameter.Value)))
	e.graph.Add(parameter, e.sysml(pDirection), rdf.String("in"))
	e.emitMembershipCore(membership, parameter, node, mParameterMembership, true)
	e.graph.Add(node, e.sysml(pOwnedRelationship), membership)
	e.graph.Add(node, e.sysml(pOwnedMembership), membership)
	e.graph.Add(node, e.sysml(pOwnedFeatureMembership), membership)
	e.graph.Add(node, e.sysml(pOwnedFeature), parameter)
	e.graph.Add(node, e.sysml(pParameter), parameter)
	e.graph.Add(node, e.sysml(pInput), parameter)
	e.graph.Add(node, e.sysml(pArgument), operand)
	e.graph.Add(membership, e.sysml(pOwnedMemberFeature), parameter)
	e.graph.Add(membership, e.sysml(pOwnedMemberParameter), parameter)
	e.graph.Add(parameter, e.sysml(pOwnedRelationship), valueMembership)
	e.graph.Add(parameter, e.sysml(pOwnedMembership), valueMembership)
	e.emitMembershipCore(valueMembership, operand, parameter, mFeatureValue, true)
	e.graph.Add(valueMembership, e.sysml(pFeatureWithValue), parameter)
	e.graph.Add(valueMembership, e.sysml(pValue), operand)
	e.graph.Add(operand, e.sysml(pOwner), parameter)
	e.graph.Add(operand, e.sysml(pOwningRelationship), valueMembership)
	e.graph.Add(operand, e.sysml(pOwningMembership), valueMembership)
	return nil
}

// expressionMetaclasses are the metaclasses of an expression node, which is a
// part of an element's declaration rather than an element the printer writes.
var expressionMetaclasses = map[string]bool{
	mExpression: true, mLiteralBoolean: true, mLiteralInteger: true,
	mLiteralRational: true, mLiteralString: true, mLiteralInfinity: true,
	mNullExpression: true, mFeatureReference: true, mFeatureChain: true,
	mOperator: true, mIndex: true, mInvocation: true, mConstructor: true,
	mCollect: true, mSelect: true, mMetadataAccess: true,
	// A MultiplicityRange is a part of the feature's declaration, and a
	// Membership relates an expression to its referent: both are minted in the
	// expression namespace under the element they belong to.
	mMultiplicityRange: true, mMembership: true,
}

// isExpressionRoot reports whether the encoder mints an expr: node of this
// metaclass directly under a declared element: an expression class, the cross
// feature a connector writes at its end, an interface end's PortUsage, or its
// reference subsetting.
func isExpressionRoot(metaclass string) bool {
	return expressionMetaclasses[metaclass] || metaclass == mReferenceSubsetting ||
		metaclass == crossFeatureMetaclass(true) ||
		metaclass == crossFeatureMetaclass(false) || metaclass == "PortUsage"
}

// isExpressionNode reports whether a subject is part of a declaration (an expression
// node or a body declaration): unowned, in the expression namespace or an unnamed expression class.
func (d *decoder) isExpressionNode(subject rdf.Term) bool {
	if !subject.IsIRI() {
		return false
	}
	if known, ok := d.expressionNodes[subject.Value]; ok {
		return known
	}
	if membership, owned := d.owningMembership[subject.Value]; owned && !d.nodeMembership[membership.iri] {
		d.expressionNodes[subject.Value] = false
		return false
	}
	known := strings.HasPrefix(subject.Value, rdf.Expression) ||
		d.nodeMemberships[subject.Value].member == subject.Value ||
		expressionMetaclasses[d.metaclass(subject)] && !d.graph.HasProperty(subject, rdf.SysML+pQualifiedName) &&
			!d.declaredMembership(subject) ||
		d.nodeRelationship(subject)
	d.expressionNodes[subject.Value] = known
	return known
}

// declaredMembership reports whether a Membership is owned by a declared
// element rather than by an expression: a transition's source member, say.
func (d *decoder) declaredMembership(subject rdf.Term) bool {
	if d.metaclass(subject) != mMembership || strings.HasPrefix(subject.Value, rdf.Expression) {
		return false
	}
	owner := firstIRI(d.graph, subject, pOwningRelatedElement, pMembershipOwningNamespace, pOwner)
	if owner.Value == "" {
		return false
	}
	_, declared := d.byIRI[owner.Value]
	return declared && !d.isExpressionNode(owner)
}

// nodeRelationship reports whether a subject is a relationship element a node
// owns directly, such as the FeatureTyping of a feature an expression body declares.
func (d *decoder) nodeRelationship(subject rdf.Term) bool {
	if metaclass := d.metaclass(subject); !impliedRelationshipMetaclasses[metaclass] && metaclass != mFeatureChaining {
		return false
	}
	owner := firstIRI(d.graph, subject, pOwningRelatedElement, pOwner)
	return owner.IsIRI() && owner.Value != subject.Value && d.isExpressionNode(owner)
}

// ownershipPredicates are structural edges that do not form expression text.
var ownershipPredicates = func() map[string]bool {
	properties := []string{
		pOwner, pOwningRelationship, pOwningMembership, pOwnedRelationship,
		pOwnedMembership, pOwnedMember, pMemberElement, pOwnedMemberElement,
		pOwnedRelatedElement, pOwningRelatedElement, pMembershipOwningNamespace,
		pOwnedMemberFeature, pOwnedMemberParameter, pOwningType, pOwnedFeature,
		pOwnedFeatureMembership, pFeatureWithValue,
		pOwnedReferenceSubsetting, pOwnedSubsetting, pOwnedSpecialization,
		"ownedTyping", "ownedSubclassification", "ownedRedefinition",
		pConnectorEnd, pOwnedEndFeature,
		// The range a head's collapsed bounds are owned by is a structural
		// edge, not an expression-valued property to render.
		pMultiplicity, pConjugatedPortDefinition,
	}
	set := make(map[string]bool, len(properties))
	for _, property := range properties {
		set[rdf.SysML+property] = true
	}
	return set
}()

// resolveExpressions renders every element's expression-valued properties as
// notation, so the printer reads one text per property.
func (d *decoder) resolveExpressions() error {
	parents := map[string][]rdf.Term{}
	valueTargets := map[string]string{}
	directValues := map[string]rdf.Term{}
	for _, triple := range d.graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+pValue {
			if _, ok := d.byIRI[triple.Subject.Value]; ok {
				directValues[triple.Subject.Value] = triple.Object
			}
		}
	}
	for _, triple := range d.graph.Triples() {
		if err := d.resolveExpression(triple, parents, valueTargets, directValues); err != nil {
			return err
		}
	}
	return d.noteSegments(parents)
}

// recordValueTarget notes the expression a feature value resolves to for owner;
// a usage and a FeatureValue stating different expressions is refused.
func (d *decoder) recordValueTarget(valueTargets map[string]string, owner, value rdf.Term) error {
	key := owner.Value + "\x00" + rdf.SysML + pValue
	if prior, exists := valueTargets[key]; exists && prior != value.Value {
		return &UnsupportedError{
			What: fmt.Sprintf("the feature value of <%s>", owner.Value),
			Note: "its usage and FeatureValue state different expressions",
		}
	}
	valueTargets[key] = value.Value
	return nil
}

// valueOwner is the feature a FeatureValue triple's value belongs to; skip
// reports a non-resolving FeatureValue or one its direct value already records.
func (d *decoder) valueOwner(triple rdf.Triple, valueTargets map[string]string, directValues map[string]rdf.Term) (owner rdf.Term, skip bool, err error) {
	if d.metaclass(triple.Subject) != mFeatureValue ||
		triple.Predicate.Value != rdf.SysML+pValue {
		return rdf.Term{}, false, nil
	}
	feature, ownerOK := d.graph.Object(triple.Subject, rdf.SysML+pFeatureWithValue)
	if !ownerOK || d.featureValues[feature.Value] != triple.Subject {
		return rdf.Term{}, true, nil
	}
	if direct, hasDirect := directValues[feature.Value]; hasDirect {
		if err := d.recordValueTarget(valueTargets, feature, triple.Object); err != nil {
			return rdf.Term{}, false, err
		}
		if direct == triple.Object {
			return rdf.Term{}, true, nil
		}
	}
	return feature, false, nil
}

// resolveExpression renders one triple's expression-valued property as notation
// on the element it belongs to.
func (d *decoder) resolveExpression(triple rdf.Triple, parents map[string][]rdf.Term, valueTargets map[string]string, directValues map[string]rdf.Term) error {
	if ownershipPredicates[triple.Predicate.Value] || d.nodeMembership[triple.Object.Value] {
		return nil
	}
	if el, ok := d.byIRI[triple.Subject.Value]; ok && expressionMetaclasses[el.metaclass] {
		// An expression element's predicates are its structure, which its own
		// head renders whole; the collapsed head properties still resolve.
		switch triple.Predicate.Value {
		case rdf.SysML + pLowerBound, rdf.SysML + pUpperBound, rdf.SysML + pValue, rdf.SysML + pMultiplicity:
		default:
			return nil
		}
	}
	if triple.Predicate.Value == rdf.SysML+pReferences &&
		d.graph.BoolValue(triple.Subject, rdf.SysML+pIsEnd) {
		return nil
	}
	if !d.isExpressionNode(triple.Object) {
		return nil
	}
	if d.chainFeatureTerm(triple.Object) {
		// A chain feature is written as its `a.b.c` reference wherever the
		// property that names it is written, not as an expression.
		parents[triple.Object.Value] = append(parents[triple.Object.Value], triple.Subject)
		return nil
	}
	featureValueOwner, skip, err := d.valueOwner(triple, valueTargets, directValues)
	if err != nil || skip {
		return err
	}
	parents[triple.Object.Value] = append(parents[triple.Object.Value], triple.Subject)
	el, ok := d.byIRI[triple.Subject.Value]
	if !ok {
		if featureValueOwner.Value != "" {
			el, ok = d.byIRI[featureValueOwner.Value]
			if !ok {
				return nil
			}
		} else {
			return nil
		}
	}
	if d.isResultExpression(el) {
		// The subject is an expression node; its parts are written with it.
		return nil
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
	if triple.Predicate.Value == rdf.SysML+pValue {
		owner := rdf.IRI(el.iri)
		if featureValueOwner.Value != "" {
			owner = featureValueOwner
		}
		if err := d.recordValueTarget(valueTargets, owner, triple.Object); err != nil {
			return err
		}
	}
	el.expressions[triple.Predicate.Value] = text
	return nil
}

// noteSegments records in wanted the element every feature chain reaches, kept
// notation included, so chooseNames checks the segment reads as it there.
func (d *decoder) noteSegments(parents map[string][]rdf.Term) error {
	for _, node := range d.graph.Subjects() {
		if err := d.noteSegment(node, parents); err != nil {
			return err
		}
	}
	return nil
}

// noteSegment records every feature chain segment node reaches in wanted.
func (d *decoder) noteSegment(node rdf.Term, parents map[string][]rdf.Term) error {
	segments := d.chainSegments(node)
	if d.metaclass(node) != mFeatureChain && len(segments) == 0 {
		return nil
	}
	owners := d.segmentOwners(node, parents, segments)
	if len(owners) == 0 {
		return nil
	}
	object, ok := d.graph.Object(node, rdf.SysML+pTargetFeature)
	if ok && object.IsIRI() {
		target, name, err := d.namedMember(object)
		if err != nil {
			return err
		}
		d.recordSegment(d.operandElement(node), name, target.qname, owners)
		return nil
	}
	operand := ""
	for _, segment := range segments {
		if segment.IsLiteral() {
			operand = ""
			continue
		}
		target, name, err := d.namedMember(segment)
		if err != nil {
			return err
		}
		d.recordSegment(operand, name, target.qname, owners)
		operand = target.qname
	}
	return nil
}

// segmentOwners is the elements a chain is written in: its expression owners,
// or the connector owning a structural end chain's end.
func (d *decoder) segmentOwners(node rdf.Term, parents map[string][]rdf.Term, segments []rdf.Term) []*element {
	owners := d.expressionOwners(node, parents)
	if len(owners) > 0 || len(segments) == 0 {
		return owners
	}
	if end, ok := d.graph.Object(node, rdf.SysML+pOwner); ok {
		if subject, ok := d.graph.Object(end, rdf.SysML+pOwner); ok {
			if owner, ok := d.byIRI[subject.Value]; ok {
				owners = []*element{owner}
			}
		}
	}
	return owners
}

// recordSegment notes one chain segment in each owning element.
func (d *decoder) recordSegment(operand, name, target string, owners []*element) {
	for _, in := range owners {
		d.wanted.segments[segmentKey{member: in.qname, operand: operand, name: name, target: target}] = true
	}
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
		if term.Datatype == rdf.OpenSysML+dtExpression {
			return term.Value, nil
		}
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
	if body, ok, err := d.expressionBody(node); err != nil {
		return primary("", err)
	} else if ok {
		return primary(d.expressionBodyText(body, in))
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
		return primary(source.StringText(value), nil)
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
	case mOperator, mIndex:
		return d.operatorForm(node, in)
	case mInvocation, mConstructor:
		return primary(d.invocationText(node, in))
	}
	return primary("", unsupported("this expression states no notation and no structure to write one from; "+rdfLimitationsNote))
}

// expressionBody is the body Expression a node writes as `{ ... }`: the node
// itself when it declares one, or the body a FeatureReferenceExpression owns
// through a FeatureMembership (KerMLExpressions BodyExpression).
func (d *decoder) expressionBody(node rdf.Term) (rdf.Term, bool, error) {
	if d.declaresBody(node) {
		return node, true, nil
	}
	if d.metaclass(node) != mFeatureReference {
		return rdf.Term{}, false, nil
	}
	for _, membership := range d.graph.Objects(node, rdf.SysML+pOwnedRelationship) {
		if d.metaclass(membership) != mFeatureMembership {
			continue
		}
		member, ok, err := d.agreedObject(membership, "the body membership", "member",
			pMemberElement, pOwnedMemberElement, pOwnedMemberFeature, pOwnedRelatedElement)
		if err != nil {
			return rdf.Term{}, false, err
		}
		if !ok || !d.declaresBody(member) {
			continue
		}
		if referent, ok := d.graph.Object(node, rdf.SysML+pReferent); ok && referent != member {
			return rdf.Term{}, false, &UnsupportedError{
				What: fmt.Sprintf("the expression <%s>", node.Value),
				Note: fmt.Sprintf("its referent is <%s>, but the body it owns is <%s>, and the two statements cannot both hold", referent.Value, member.Value),
			}
		}
		return member, true, nil
	}
	return rdf.Term{}, false, nil
}

// declaresBody reports whether an Expression node is a body: it states one, or
// owns declarations or a result through memberships.
func (d *decoder) declaresBody(node rdf.Term) bool {
	if d.metaclass(node) != mExpression {
		return false
	}
	if d.graph.BoolValue(node, rdf.OpenSysML+xHasBody) ||
		d.graph.HasProperty(node, rdf.OpenSysML+xResultExpression) ||
		d.graph.HasProperty(node, rdf.OpenSysML+xBodyParameter) ||
		d.graph.HasProperty(node, rdf.OpenSysML+xBodyMember) {
		return true
	}
	for _, membership := range d.graph.Objects(node, rdf.SysML+pOwnedRelationship) {
		if bodyMembershipMetaclasses[d.metaclass(membership)] {
			return true
		}
	}
	return false
}

// bodyMembershipMetaclasses are the memberships an expression body declares through.
var bodyMembershipMetaclasses = map[string]bool{
	mFeatureMembership: true, mParameterMembership: true, mOwningMembership: true, mResultExpressionMembership: true,
}

// bodyDeclaration is one thing an expression body declares, in written order.
type bodyDeclaration struct {
	term  rdf.Term
	param bool
}

// bodyParts reads what a body declares and the result it computes, from the
// memberships it owns and from the collapsed sysx: links; stated both ways,
// the two must agree.
func (d *decoder) bodyParts(node rdf.Term) ([]bodyDeclaration, rdf.Term, bool, error) {
	var collapsed []bodyDeclaration
	for _, param := range d.graph.Objects(node, rdf.OpenSysML+xBodyParameter) {
		collapsed = append(collapsed, bodyDeclaration{term: param, param: true})
	}
	for _, member := range d.graph.Objects(node, rdf.OpenSysML+xBodyMember) {
		collapsed = append(collapsed, bodyDeclaration{term: member})
	}
	collapsedResult, hasCollapsedResult := d.graph.Object(node, rdf.OpenSysML+xResultExpression)

	var owned []bodyDeclaration
	var ownedResult rdf.Term
	for _, membership := range d.graph.Objects(node, rdf.SysML+pOwnedRelationship) {
		metaclass := d.metaclass(membership)
		if !bodyMembershipMetaclasses[metaclass] {
			continue
		}
		member, ok, err := d.agreedObject(membership, "the body membership", "member",
			pMemberElement, pOwnedMemberElement, pOwnedMemberFeature, pOwnedMemberParameter, pOwnedResultExpression, pOwnedRelatedElement)
		if err != nil {
			return nil, rdf.Term{}, false, err
		}
		if !ok {
			return nil, rdf.Term{}, false, &UnsupportedError{
				What: fmt.Sprintf("the body membership <%s>", membership.Value),
				Note: "it names no member",
			}
		}
		if metaclass == mResultExpressionMembership {
			if ownedResult.Value != "" {
				return nil, rdf.Term{}, false, &UnsupportedError{
					What: fmt.Sprintf("the expression body <%s>", node.Value),
					Note: "it owns two result expressions, and a body computes one",
				}
			}
			ownedResult = member
			continue
		}
		// A multiplicity or bound a declaration owns is its structure, not a member.
		if expressionMetaclasses[d.metaclass(member)] {
			continue
		}
		direction, _ := d.graph.Lexical(member, rdf.SysML+pDirection)
		param := metaclass == mParameterMembership ||
			direction == directionKeyword(ast.DirIn) && ontology.IsAncestorOrSelf(d.metaclass(member), mFeature)
		owned = append(owned, bodyDeclaration{term: member, param: param})
	}

	declarations := owned
	if len(collapsed) > 0 {
		if len(owned) > 0 && !sameBodyDeclarations(collapsed, owned) {
			return nil, rdf.Term{}, false, &UnsupportedError{
				What: fmt.Sprintf("the expression body <%s>", node.Value),
				Note: "its memberships and its sysx:bodyParameter/sysx:bodyMember links declare different members, and the two statements cannot both hold",
			}
		}
		declarations = collapsed
	}
	sort.SliceStable(declarations, func(i, j int) bool {
		return intOf(d.graph, declarations[i].term, rdf.OpenSysML+xMemberIndex) < intOf(d.graph, declarations[j].term, rdf.OpenSysML+xMemberIndex)
	})
	result, hasResult := ownedResult, ownedResult.Value != ""
	if hasCollapsedResult {
		if hasResult && result != collapsedResult {
			return nil, rdf.Term{}, false, &UnsupportedError{
				What: fmt.Sprintf("the expression body <%s>", node.Value),
				Note: fmt.Sprintf("its result expression is <%s>, but the ResultExpressionMembership it owns names <%s>, and the two statements cannot both hold", collapsedResult.Value, result.Value),
			}
		}
		result, hasResult = collapsedResult, true
	}
	return declarations, result, hasResult, nil
}

// sameBodyDeclarations reports whether the collapsed links and the memberships
// declare the same members: as many, and every node the links name is owned.
// A name literal among the links is a legacy parameter with no node to match.
func sameBodyDeclarations(collapsed, owned []bodyDeclaration) bool {
	if len(collapsed) != len(owned) {
		return false
	}
	seen := map[string]bool{}
	for _, decl := range owned {
		seen[decl.term.Value] = true
	}
	for _, decl := range collapsed {
		if decl.term.IsIRI() && !seen[decl.term.Value] {
			return false
		}
	}
	return true
}

// expressionBodyText rebuilds an expression body: its declarations and its
// result, in the order the graph records.
func (d *decoder) expressionBodyText(node rdf.Term, in *element) (string, error) {
	parts, err := d.bodyDeclarationsText(node, in)
	if err != nil {
		return "", err
	}
	_, result, hasResult, err := d.bodyParts(node)
	if err != nil {
		return "", err
	}
	if hasResult {
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
	if metaclass := d.metaclass(param); !ontology.IsAncestorOrSelf(metaclass, mFeature) {
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
	declarations, _, _, err := d.bodyParts(node)
	if err != nil {
		return nil, err
	}
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

// bodyMemberText writes one declaration of an expression body from its structure,
// as a member of a namespace is written: its head, then the body it declares.
func (d *decoder) bodyMemberText(member rdf.Term, in *element) (string, error) {
	el, err := d.localElement(member, in)
	if err != nil {
		return "", err
	}
	head, err := d.declarationHead(el)
	if err != nil {
		return "", err
	}
	if el.declaredID {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the body member <%s>", member.Value),
			Note: "it declares an id of its own, and an annotation is not mapped inside an expression body; " + rdfLimitationsNote,
		}
	}
	if annotationMetaclasses[el.metaclass] {
		return head, nil
	}
	members, err := d.bodyDeclarationsText(member, in)
	if err != nil {
		return "", err
	}
	switch {
	case len(members) > 0:
		return head + " { " + strings.Join(members, " ") + " }", nil
	case d.boolOf(el, rdf.OpenSysML+xHasBody):
		return head + " { }", nil
	}
	return head + ";", nil
}

// localElement reads a declaration inside an expression body: shaped as the element
// it declares, with no qualified name or membership, its references reading where in's do.
func (d *decoder) localElement(node rdf.Term, in *element) (*element, error) {
	el := d.expressionElement(node, in)
	el.local, el.owner = true, in
	el.metaclass = d.metaclass(node)
	if el.metaclass == "" {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the body member <%s>", node.Value),
			Note: "it has no rdf:type, so there is no way to tell what to write",
		}
	}
	el.memberIndex = intOf(d.graph, node, rdf.OpenSysML+xMemberIndex)
	el.elementID, _ = d.stringOf(el, rdf.SysML+pElementID)
	el.declaredID = d.boolOf(el, rdf.OpenSysML+xDeclaredID)
	for _, predicate := range d.graph.Predicates(node) {
		switch predicate {
		case rdf.OpenSysML + xBodyMember, rdf.OpenSysML + xBodyParameter:
			// The declarations of its own body are written after its head.
			continue
		}
		if ownershipPredicates[predicate] {
			continue
		}
		for _, object := range d.graph.Objects(node, predicate) {
			if !d.isExpressionNode(object) {
				continue
			}
			text, err := d.expressionOperand(object, in, positionBinding(strings.TrimPrefix(predicate, rdf.SysML)))
			if err != nil {
				return nil, err
			}
			el.expressions[predicate] = text
		}
	}
	return el, nil
}

// operatorForm rebuilds an operator expression. Each operand is enclosed in
// parentheses only where its own form binds too loosely for its position.
func (d *decoder) operatorForm(node rdf.Term, in *element) (operand, error) {
	operator, ok := d.graph.Lexical(node, rdf.SysML+pOperator)
	if !ok && d.metaclass(node) == mIndex {
		// An IndexExpression is the `#` operator applied (KerML 1.0 § 8.3.4.8.6).
		operator, ok = opAt, true
	}
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
	case (operator == opIndex || operator == opIndexOld) && len(args) == 2:
		return primary(args[0].at(bindPrimary) + "[" + indexText(args[1]) + "]")
	case operator == opAt && len(args) == 2:
		return primary(args[0].at(bindPrimary) + "#(" + indexText(args[1]) + ")")
	case hasType && len(args) == 1 && isInfix:
		return operand{text: args[0].at(infix) + " " + operator + " " + typeArgument, binding: infix}, nil
	case hasType && len(args) == 0 && operator == "@":
		// `@T` alone tests the implicit self, as a filter condition writes it.
		return operand{text: "@" + typeArgument, binding: bindUnary}, nil
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
	terms, normative, err := d.expressionOperandTerms(node)
	if err != nil {
		return "", err
	}
	receiver, hasReceiver := d.graph.Object(node, rdf.SysML+pOperand)
	// Older graphs flag `new` on an InvocationExpression instead of the metaclass.
	constructor := d.metaclass(node) == mConstructor || d.graph.BoolValue(node, rdf.OpenSysML+xIsConstructor)
	function, hasFunction, err := d.calleeText(node, in)
	if err != nil {
		return "", err
	}
	if hasReceiver && hasFunction {
		// The receiver of `x->f(a)` is the first parameter; the operand triple
		// only keeps the arrow spelling. Older graphs left it out of the
		// parameters altogether, so its absence is accepted, only a misplacement refused.
		if len(terms) > 0 && terms[0].term == receiver && terms[0].name == "" {
			terms = terms[1:]
		} else if normative && operandTermIndex(terms, receiver) >= 0 {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the expression <%s>", node.Value),
				Note: fmt.Sprintf("its operand <%s> is not its first parameter, and the two statements cannot both hold", receiver.Value),
			}
		}
	}
	operands, err := d.renderOperandTerms(terms, in)
	if err != nil {
		return "", err
	}
	arguments := "(" + joinOperands(operands, bindConditional) + ")"
	if !hasFunction {
		// `chain(args)`: the invoked type is the feature chain the operand reaches.
		if !hasReceiver || d.metaclass(receiver) != mFeatureChain || constructor {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the expression <%s>", node.Value),
				Note: "an invocation names the function it invokes, by name or as the feature chain it applies to",
			}
		}
		text, err := d.expressionOperand(receiver, in, bindPrimary)
		if err != nil {
			return "", err
		}
		return text + arguments, nil
	}
	call := function + arguments
	if constructor {
		call = "new " + call
	}
	if hasReceiver {
		text, err := d.expressionOperand(receiver, in, bindPrimary)
		if err != nil {
			return "", err
		}
		return text + "->" + call, nil
	}
	return call, nil
}

// calleeText names what an invocation invokes, from the collapsed `function`
// and from the callee relationship it owns: a Membership to the function or an
// OwningMembership owning the chain feature. Both present, they must agree.
func (d *decoder) calleeText(node rdf.Term, in *element) (string, bool, error) {
	collapsed, hasCollapsed := d.graph.Object(node, rdf.SysML+pFunction)
	var member rdf.Term
	var chain rdf.Term
	for _, relationship := range d.graph.Objects(node, rdf.SysML+pOwnedRelationship) {
		switch d.metaclass(relationship) {
		case mMembership:
			if m, ok := d.graph.Object(relationship, rdf.SysML+pMemberElement); ok {
				member = m
			}
		case mOwningMembership:
			if m, ok := d.graph.Object(relationship, rdf.SysML+pMemberElement); ok && d.chainFeatureTerm(m) {
				chain = m
			}
		}
	}
	switch {
	case chain.Value != "":
		segments := d.chainSegments(chain)
		var parts, spelled []string
		for _, segment := range segments {
			name, err := d.referenceName(segment, in)
			if err != nil {
				return "", false, err
			}
			parts = append(parts, name)
			if segment.IsLiteral() {
				name = segment.Value
			}
			spelled = append(spelled, name)
		}
		text := strings.Join(parts, ".")
		if hasCollapsed {
			name, err := d.referenceName(collapsed, in)
			if err != nil {
				return "", false, err
			}
			// The function is the chain's last feature when it resolves, else
			// the whole chain's spelling; anything else names a second callee.
			var agree bool
			if collapsed.IsIRI() {
				agree = len(segments) > 0 && segments[len(segments)-1] == collapsed
			} else {
				agree = collapsed.Value == strings.Join(spelled, ".")
			}
			if !agree {
				return "", false, &UnsupportedError{
					What: fmt.Sprintf("the expression <%s>", node.Value),
					Note: fmt.Sprintf("its function is %s, but the chain feature it owns reaches %s, and the two statements cannot both hold", name, text),
				}
			}
		}
		return text, true, nil
	case member.Value != "" && hasCollapsed && member != collapsed:
		return "", false, &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: fmt.Sprintf("its function is <%s>, but the membership it owns names <%s>, and the two statements cannot both hold", collapsed.Value, member.Value),
		}
	case hasCollapsed:
		name, err := d.referenceName(collapsed, in)
		return name, true, err
	case member.Value != "":
		name, err := d.referenceName(member, in)
		return name, true, err
	}
	return "", false, nil
}

// expressionOperands rebuilds operands from parameter memberships, with legacy
// argument triples retained for graphs written before the structural mapping.
func (d *decoder) expressionOperands(node rdf.Term, in *element) ([]operand, error) {
	terms, _, err := d.expressionOperandTerms(node)
	if err != nil {
		return nil, err
	}
	return d.renderOperandTerms(terms, in)
}

// expressionOperandTerms identifies the operands in order, from the parameter
// memberships where there are any, checked against the legacy arguments.
func (d *decoder) expressionOperandTerms(node rdf.Term) ([]expressionOperandTerm, bool, error) {
	standard, hasStandard, err := d.standardExpressionOperandTerms(node)
	if err != nil {
		return nil, false, err
	}
	legacy, err := d.legacyOperandTerms(node)
	if err != nil {
		return nil, false, err
	}
	if !hasStandard {
		return legacy, false, nil
	}
	if len(legacy) > 0 && !sameOperandTerms(standard, legacy) {
		return nil, true, &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: "its parameter memberships and legacy arguments state different operands",
		}
	}
	return standard, true, nil
}

// expressionOperandTerm identifies an operand and its optional named binding.
type expressionOperandTerm struct {
	term rdf.Term
	name string
}

// standardExpressionOperandTerms reads operand identities through parameter memberships.
func (d *decoder) standardExpressionOperandTerms(node rdf.Term) ([]expressionOperandTerm, bool, error) {
	var memberships []rdf.Term
	for _, membership := range d.graph.Objects(node, rdf.SysML+pOwnedFeatureMembership) {
		if d.metaclass(membership) == mParameterMembership {
			memberships = append(memberships, membership)
		}
	}
	if len(memberships) == 0 {
		return nil, false, nil
	}
	out := make([]expressionOperandTerm, 0, len(memberships))
	for _, membership := range memberships {
		parameter, ok, err := d.agreedObject(membership, "the parameter membership", "parameter",
			pOwnedMemberParameter, pMemberElement, pOwnedMemberElement, pOwnedMemberFeature, pOwnedRelatedElement)
		if err != nil {
			return nil, true, err
		}
		if !ok {
			return nil, true, &UnsupportedError{
				What: fmt.Sprintf("the parameter membership <%s>", membership.Value),
				Note: "it has no parameter feature",
			}
		}
		direction, _ := d.graph.Lexical(parameter, rdf.SysML+pDirection)
		if strings.EqualFold(direction, "return") || strings.EqualFold(direction, "out") {
			continue
		}
		valueMembership, ok := d.featureValueMembership(parameter)
		if !ok && d.typeArgumentParameter(parameter) {
			// A typed, valueless parameter is a classification operator's type
			// argument, read by expressionTypeArgument.
			continue
		}
		if !ok {
			return nil, true, &UnsupportedError{
				What: fmt.Sprintf("the expression <%s>", node.Value),
				Note: fmt.Sprintf("its parameter <%s> has no FeatureValue", parameter.Value),
			}
		}
		value, ok := d.graph.Object(valueMembership, rdf.SysML+pValue)
		if !ok {
			return nil, true, &UnsupportedError{
				What: fmt.Sprintf("the FeatureValue <%s>", valueMembership.Value),
				Note: "it has no sysml:value operand",
			}
		}
		name, _ := d.graph.Lexical(value, rdf.OpenSysML+xArgumentName)
		out = append(out, expressionOperandTerm{term: value, name: name})
	}
	return out, true, nil
}

// featureValueMembership returns the indexed FeatureValue for a feature.
func (d *decoder) featureValueMembership(parameter rdf.Term) (rdf.Term, bool) {
	membership, ok := d.featureValues[parameter.Value]
	return membership, ok
}

// legacyOperandTerms reads ordered operands from the pre-membership representation.
func (d *decoder) legacyOperandTerms(node rdf.Term) ([]expressionOperandTerm, error) {
	type argument struct {
		index int
		term  expressionOperandTerm
	}
	objects := d.graph.Objects(node, rdf.SysML+pArgument)
	args := make([]argument, 0, len(objects))
	for i, object := range objects {
		arg := argument{index: i, term: expressionOperandTerm{term: object}}
		if written, ok := d.graph.Lexical(object, rdf.OpenSysML+xArgumentIndex); ok {
			if parsed, err := strconv.Atoi(written); err == nil {
				arg.index = parsed
			}
		}
		if name, ok := d.graph.Lexical(object, rdf.OpenSysML+xArgumentName); ok {
			arg.term.name = name
		}
		args = append(args, arg)
	}
	sort.SliceStable(args, func(i, j int) bool { return args[i].index < args[j].index })
	out := make([]expressionOperandTerm, 0, len(args))
	for _, arg := range args {
		out = append(out, arg.term)
	}
	return out, nil
}

// renderOperandTerms renders operand identities in the requested expression scope.
func (d *decoder) renderOperandTerms(terms []expressionOperandTerm, in *element) ([]operand, error) {
	out := make([]operand, 0, len(terms))
	for _, term := range terms {
		form, err := d.operandForm(term.term, in)
		if err != nil {
			return nil, err
		}
		if term.name != "" {
			form = operand{text: qualifiedNameText(term.name) + " = " + form.text, binding: bindPrimary}
		}
		out = append(out, form)
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

// sameOperandTerms compares operand identity and named-argument spelling.
func sameOperandTerms(left, right []expressionOperandTerm) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].term != right[i].term || left[i].name != right[i].name {
			return false
		}
	}
	return true
}

// operandTermIndex finds the operand bound to term, or -1.
func operandTermIndex(terms []expressionOperandTerm, term rdf.Term) int {
	for i := range terms {
		if terms[i].term == term {
			return i
		}
	}
	return -1
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

// typeArgumentParameter reports whether an in parameter is a classification
// operator's type argument: typed, and with no value bound to it.
func (d *decoder) typeArgumentParameter(parameter rdf.Term) bool {
	if _, valued := d.featureValueMembership(parameter); valued {
		return false
	}
	_, ok := d.typeArgumentOf(parameter)
	return ok
}

// typeArgumentOf is the type a type-argument parameter is typed by.
func (d *decoder) typeArgumentOf(parameter rdf.Term) (rdf.Term, bool) {
	for _, typing := range d.graph.Objects(parameter, rdf.SysML+"ownedTyping") {
		if d.metaclass(typing) == mFeatureTyping {
			return d.graph.Object(typing, rdf.SysML+"type")
		}
	}
	return d.graph.Object(parameter, rdf.SysML+"type")
}

// expressionTypeArgument names the type a classification operator applies:
// the collapsed sysx:typeArgument, the typed in parameter that states it, or
// both when they agree.
func (d *decoder) expressionTypeArgument(node rdf.Term, in *element) (string, bool, error) {
	collapsed, hasCollapsed := d.graph.Object(node, rdf.OpenSysML+xTypeArgument)
	var typed []rdf.Term
	for _, membership := range d.graph.Objects(node, rdf.SysML+pOwnedFeatureMembership) {
		if d.metaclass(membership) != mParameterMembership {
			continue
		}
		parameter, ok := d.graph.Object(membership, rdf.SysML+pOwnedMemberParameter)
		if !ok {
			parameter, ok = d.graph.Object(membership, rdf.SysML+pMemberElement)
		}
		if ok && d.typeArgumentParameter(parameter) {
			target, _ := d.typeArgumentOf(parameter)
			typed = append(typed, target)
		}
	}
	if len(typed) > 1 {
		return "", false, &UnsupportedError{
			What: fmt.Sprintf("the expression <%s>", node.Value),
			Note: fmt.Sprintf("it has %d type-argument parameters, and a classification operator takes one type", len(typed)),
		}
	}
	if !hasCollapsed && len(typed) == 0 {
		return "", false, nil
	}
	object := collapsed
	if !hasCollapsed {
		object = typed[0]
	}
	name, err := d.referenceName(object, in)
	if err != nil {
		return "", false, err
	}
	if hasCollapsed && len(typed) == 1 {
		stated, err := d.referenceName(typed[0], in)
		if err != nil {
			return "", false, err
		}
		if stated != name {
			return "", false, &UnsupportedError{
				What: fmt.Sprintf("the expression <%s>", node.Value),
				Note: fmt.Sprintf("its sysx:typeArgument names %s and its typed parameter names %s, so it is not clear which type to write", name, stated),
			}
		}
	}
	return name, true, nil
}
