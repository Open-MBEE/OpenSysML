package semantics

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// expressionParts are the elements an expression owns as the KerML metamodel
// reads it (KerML 8.3.4.8 Expression; 8.4.4.9 operator expressions): one `in`
// parameter per operand, each binding its operand through a FeatureValue, an
// extra `in` parameter typed by a type argument, and the `out` result
// parameter of an expression whose evaluation yields a value. The conversion
// writes exactly these (rdf_expr.go expressionOperandOwnership, typeArgument,
// resultParameter), so the reflective read synthesizes them in the same order.
type expressionParts struct {
	params    []*symbols.Symbol
	operands  []*symbols.Symbol // operands[i] is the value of params[i]; nil for a type argument
	result    *symbols.Symbol
	condition *symbols.Symbol   // a change wrapper's referent, the written condition
	body      *symbols.Symbol   // a `{ … }` reference's referent, the body Expression
	members   []*symbols.Symbol // a body's parameters and members, in the conversion's order
	chain     *symbols.Symbol   // the Feature a receiver-reached call names its function by
}

// resultBearingExpressions are the expression metaclasses whose evaluation
// yields a value the `out` parameter holds, as the conversion classifies them.
var resultBearingExpressions = map[string]bool{
	"FeatureReferenceExpression": true, "FeatureChainExpression": true,
	"OperatorExpression": true, "IndexExpression": true,
	"InvocationExpression": true, "ConstructorExpression": true,
	"CollectExpression": true, "SelectExpression": true,
	"TriggerInvocationExpression": true,
}

// isExpressionSymbol reports whether sym is an expression element the
// reflective read synthesized: a value or an operand.
func isExpressionSymbol(sym *symbols.Symbol) bool {
	if sym == nil || sym.Expr == nil {
		return false
	}
	switch sym.Expr.Role {
	case symbols.ExpressionValue, symbols.ExpressionArgument, symbols.ExpressionChangeWrapper, symbols.ExpressionChangeCondition,
		symbols.ExpressionBody, symbols.ExpressionBodyResult:
		return true
	}
	return false
}

// expressionElementMetaclassName is the metaclass of the expression element
// sym: a change wrapper is the FeatureReferenceExpression the conversion
// writes; every other element is classified by its node.
func expressionElementMetaclassName(sym *symbols.Symbol) string {
	if sym.Expr != nil {
		switch sym.Expr.Role {
		case symbols.ExpressionChangeWrapper:
			return "FeatureReferenceExpression"
		case symbols.ExpressionBody:
			return "Expression"
		}
	}
	return expressionMetaclassName(sym.Decl)
}

// isExpressionParameterSymbol reports whether sym is a parameter the
// reflective read synthesized for an expression: an `in` or the `out`.
func isExpressionParameterSymbol(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil &&
		(sym.Expr.Role == symbols.ExpressionParameter || sym.Expr.Role == symbols.ExpressionResult)
}

// expressionPartsOf are the parts of the expression element expr, synthesized
// once per model so every read answers the same identities.
func (m *Model) expressionPartsOf(expr *symbols.Symbol) *expressionParts {
	if m == nil || !isExpressionSymbol(expr) {
		return nil
	}
	defer m.own(expr).LeaveDoc()
	if parts, ok := m.expressionParts[expr]; ok {
		return parts
	}
	if m.expressionParts == nil {
		m.expressionParts = make(map[*symbols.Symbol]*expressionParts)
	}
	journal(m, m.expressionParts, expr, expr.Decl)
	parts := &expressionParts{}
	if expr.Expr.Role == symbols.ExpressionChangeWrapper {
		// The wrapper owns the condition it refers to and its result; the
		// condition's own parts hang off the condition (changeArgument).
		parts.condition = &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    expr.DocName,
			Decl:       expr.Decl.(*ast.ChangeEvent).Condition,
			DeclSpan:   expr.Decl.(*ast.ChangeEvent).Condition.Span(),
			Visibility: expr.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: expr, Role: symbols.ExpressionChangeCondition},
		}
		parts.result = m.expressionParameter(expr, symbols.ExpressionResult, 0)
		m.expressionParts[expr] = parts
		return parts
	}
	if expr.Expr.Role == symbols.ExpressionBody {
		// The body owns the parameters and members its scope declares, in the
		// conversion's order, then its result (bodyExpression).
		body := expr.Decl.(*ast.BodyExpr)
		scope := m.bodyScopeOf(expr)
		for _, decl := range ast.BodyDeclarations(body.Params, body.Members) {
			if member := bodyDeclarationSymbol(scope, body, decl); member != nil {
				parts.members = append(parts.members, member)
			}
		}
		if body.Result != nil {
			parts.result = &symbols.Symbol{
				Kind:       symbols.SymbolExpression,
				DocName:    expr.DocName,
				Decl:       body.Result,
				DeclSpan:   body.Result.Span(),
				Visibility: expr.Visibility,
				Expr:       &symbols.ExpressionElement{Owner: expr, Role: symbols.ExpressionBodyResult},
			}
		}
		m.expressionParts[expr] = parts
		return parts
	}
	if body, ok := expr.Decl.(*ast.BodyExpr); ok {
		// A `{ … }` is a FeatureReferenceExpression whose referent is the body
		// Expression it owns, beside its result (KerMLExpressions BodyExpression).
		parts.body = &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    expr.DocName,
			Decl:       body,
			DeclSpan:   body.Span(),
			Visibility: expr.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: expr, Role: symbols.ExpressionBody},
		}
		if m.bodySymbols == nil {
			m.bodySymbols = make(map[*ast.BodyExpr]*symbols.Symbol)
		}
		journal(m, m.bodySymbols, body, body)
		m.bodySymbols[body] = parts.body
		parts.result = m.expressionParameter(expr, symbols.ExpressionResult, 0)
		m.expressionParts[expr] = parts
		return parts
	}
	if event, ok := expr.Decl.(*ast.ChangeEvent); ok && event.Condition != nil {
		// A change trigger binds one `in` parameter to a wrapper whose
		// referent is the condition, as the conversion writes it.
		parameter := m.expressionParameter(expr, symbols.ExpressionParameter, 0)
		parts.params = append(parts.params, parameter)
		parts.operands = append(parts.operands, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    expr.DocName,
			Decl:       expr.Decl,
			DeclSpan:   expr.DeclSpan,
			Visibility: expr.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: parameter, Role: symbols.ExpressionChangeWrapper},
		})
		parts.result = m.expressionParameter(expr, symbols.ExpressionResult, 0)
		m.expressionParts[expr] = parts
		return parts
	}
	operands, typeArgument := ast.ExpressionOperands(expr.Decl)
	for _, operand := range operands {
		if operand.Index < 0 {
			// A receiver no function is named for is an operand the
			// conversion writes without a parameter binding it.
			continue
		}
		parameter := m.expressionParameter(expr, symbols.ExpressionParameter, operand.Index)
		parts.params = append(parts.params, parameter)
		parts.operands = append(parts.operands, &symbols.Symbol{
			Kind:       symbols.SymbolExpression,
			DocName:    expr.DocName,
			Decl:       operand.Node,
			DeclSpan:   operand.Node.Span(),
			Visibility: expr.Visibility,
			Expr:       &symbols.ExpressionElement{Owner: parameter, Role: symbols.ExpressionArgument, Index: operand.Index},
		})
	}
	if typeArgument >= 0 {
		parts.params = append(parts.params, m.expressionParameter(expr, symbols.ExpressionParameter, typeArgument))
		parts.operands = append(parts.operands, nil)
	}
	if resultBearingExpressions[expressionElementMetaclassName(expr)] {
		parts.result = m.expressionParameter(expr, symbols.ExpressionResult, 0)
	}
	if slot, _, _ := expressionChain(expr.Decl); slot != "" {
		parts.chain = m.expressionParameter(expr, symbols.ExpressionChainFeature, 0)
		parts.chain.Expr.Slot = slot
	}
	m.expressionParts[expr] = parts
	return parts
}

func (m *Model) expressionParameter(expr *symbols.Symbol, role symbols.ExpressionRole, index int) *symbols.Symbol {
	return &symbols.Symbol{
		Kind:       symbols.SymbolKerMLFeature,
		DocName:    expr.DocName,
		Decl:       expr.Decl,
		DeclSpan:   expr.DeclSpan,
		Visibility: expr.Visibility,
		Expr:       &symbols.ExpressionElement{Owner: expr, Role: role, Index: index},
	}
}

// ExpressionParameters are the `in` parameters of the expression element expr,
// one per operand and one per type argument, in the conversion's order.
func (m *Model) ExpressionParameters(expr *symbols.Symbol) []*symbols.Symbol {
	if parts := m.expressionPartsOf(expr); parts != nil {
		return parts.params
	}
	return nil
}

// ExpressionArguments are the operands of the expression element expr, as
// expression elements; each is the value of the parameter at its position.
func (m *Model) ExpressionArguments(expr *symbols.Symbol) []*symbols.Symbol {
	parts := m.expressionPartsOf(expr)
	if parts == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, operand := range parts.operands {
		if operand != nil {
			out = append(out, operand)
		}
	}
	return out
}

// ExpressionResult is the `out` parameter of the expression element expr, nil
// for an expression whose evaluation yields none (a literal, a reference).
func (m *Model) ExpressionResult(expr *symbols.Symbol) *symbols.Symbol {
	if parts := m.expressionPartsOf(expr); parts != nil && parts.result != nil &&
		parts.result.Expr.Role == symbols.ExpressionResult {
		// A body's result expression is a ResultExpressionMember the body
		// owns, not the result parameter the conversion omits for it.
		return parts.result
	}
	return nil
}

// expressionParameterValue is the operand the synthesized `in` parameter
// binds, nil for a type argument's parameter or the `out`.
func (m *Model) expressionParameterValue(param *symbols.Symbol) *symbols.Symbol {
	if m == nil || param == nil || param.Expr == nil || param.Expr.Role != symbols.ExpressionParameter {
		return nil
	}
	parts := m.expressionPartsOf(param.Expr.Owner)
	if parts == nil {
		return nil
	}
	for i, p := range parts.params {
		if p == param {
			return parts.operands[i]
		}
	}
	return nil
}

// expressionOwnedElements are expressionOwnedParts with the chain Feature expr
// owns through an OwningMembership (so an owned element, not an owned feature),
// where the conversion lists it: a function's chain leads, a target's follows
// its operand.
func (m *Model) expressionOwnedElements(expr *symbols.Symbol) []*symbols.Symbol {
	parts := m.expressionOwnedParts(expr)
	chain := m.ExpressionChainFeature(expr)
	if chain == nil {
		return parts
	}
	if _, _, leads := expressionChain(expr.Decl); leads {
		return append([]*symbols.Symbol{chain}, parts...)
	}
	out := append([]*symbols.Symbol{}, parts...)
	at := len(out)
	if result := m.ExpressionResult(expr); result != nil && at > 0 && out[at-1] == result {
		at--
	}
	return append(out[:at], append([]*symbols.Symbol{chain}, out[at:]...)...)
}

// expressionOwnedParts are the elements expr owns, parameters first then the
// result, as the conversion lists them.
func (m *Model) expressionOwnedParts(expr *symbols.Symbol) []*symbols.Symbol {
	parts := m.expressionPartsOf(expr)
	if parts == nil {
		return nil
	}
	out := append([]*symbols.Symbol{}, parts.params...)
	if parts.condition != nil {
		// A change wrapper owns its condition through a FeatureMembership,
		// ahead of its result (changeArgument).
		out = append(out, parts.condition)
	}
	if parts.body != nil {
		// A `{ … }` reference owns its body ahead of its result (bodyExpression).
		out = append(out, parts.body)
	}
	// A body owns its parameters and members ahead of its result.
	out = append(out, parts.members...)
	if parts.result != nil {
		out = append(out, parts.result)
	}
	return out
}

// expressionParameterParts are the parameters expr owns, `in` ones first then
// the result: what its `parameter` and `directedFeature` list.
func (m *Model) expressionParameterParts(expr *symbols.Symbol) []*symbols.Symbol {
	parts := m.expressionPartsOf(expr)
	if parts == nil {
		return nil
	}
	out := append([]*symbols.Symbol{}, parts.params...)
	if result := m.ExpressionResult(expr); result != nil {
		out = append(out, result)
	}
	return out
}

func expressionResultList(m *Model, expr *symbols.Symbol) []*symbols.Symbol {
	if result := m.ExpressionResult(expr); result != nil {
		return []*symbols.Symbol{result}
	}
	return nil
}

// expressionMemberships are the memberships an expression part owns, as the
// conversion writes them: an expression's ParameterMemberships in operand
// order then its ReturnParameterMembership; an `in` parameter's FeatureValue.
func (m *Model) expressionMemberships(sym *symbols.Symbol) []*symbols.Symbol {
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.memberships[sym]; ok {
		return cached
	}
	journal(m, m.memberships, sym, sym.Decl)
	if sym.Expr != nil && sym.Expr.Role == symbols.ExpressionChainFeature {
		out := m.chainFeatureMemberships(sym)
		m.memberships[sym] = out
		return out
	}
	var out []*symbols.Symbol
	owning := func(member *symbols.Symbol) {
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   member.DeclSpan,
			Visibility: sym.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitMembership, Member: member,
			},
		})
	}
	reference := func(slot string, name *ast.QualifiedName) {
		target := m.referencedSymbol(name)
		if target == nil {
			return
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   sym.DeclSpan,
			Visibility: sym.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: len(out), Form: symbols.ImplicitReferenceMembership, Member: target, Slot: slot,
			},
		})
	}
	if sym.Expr != nil && sym.Expr.Role == symbols.ExpressionChangeWrapper {
		// The wrapper owns its condition through a FeatureMembership, then its result.
		parts := m.expressionPartsOf(sym)
		owning(parts.condition)
		owning(parts.result)
	} else if sym.Expr != nil && sym.Expr.Role == symbols.ExpressionBody {
		// The body owns its parameters and members, then its result.
		parts := m.expressionPartsOf(sym)
		for _, member := range parts.members {
			owning(member)
		}
		if parts.result != nil {
			owning(parts.result)
		}
	} else if parts := m.expressionPartsOf(sym); parts != nil && parts.body != nil {
		// A `{ … }` reference owns its body through a FeatureMembership, then its result.
		owning(parts.body)
		owning(parts.result)
	} else if isExpressionSymbol(sym) {
		// The conversion's order: a reference's referent and an invocation's
		// function lead; a chain's target follows its operand; the result ends.
		slot, name, leads := expressionReference(sym.Decl)
		chain := m.ExpressionChainFeature(sym)
		_, _, chainLeads := expressionChain(sym.Decl)
		if leads {
			reference(slot, name)
		}
		if chain != nil && chainLeads {
			owning(chain)
		}
		for _, parameter := range m.ExpressionParameters(sym) {
			owning(parameter)
		}
		if !leads && name != nil {
			reference(slot, name)
		}
		if chain != nil && !chainLeads {
			owning(chain)
		}
		if result := m.ExpressionResult(sym); result != nil {
			owning(result)
		}
	} else if value := m.expressionParameterValue(sym); value != nil {
		owning(value)
	}
	m.memberships[sym] = out
	return out
}

// expressionReference is the element an expression names by a plain Membership,
// at the slot the conversion writes it: a reference's referent, an invocation's
// function (leading its parameters), a chain's target (following its operand).
// A dotted name owns a chain feature instead and is not listed here.
func expressionReference(node ast.Node) (slot string, name *ast.QualifiedName, leads bool) {
	switch n := node.(type) {
	case *ast.QualifiedName:
		return "referent", n, true
	case *ast.FeatureReference:
		return "referent", n.Name, true
	case *ast.InvocationExpr:
		if n.Type != nil && !ast.QualifiedNameHasChain(n.Type) {
			return "function", n.Type, true
		}
	case *ast.ConstructorExpr:
		if n.Type != nil && !ast.QualifiedNameHasChain(n.Type) {
			return "function", n.Type, true
		}
	case *ast.FeatureChainExpr:
		if n.Member != nil && !ast.QualifiedNameHasChain(n.Member) {
			return "targetFeature", n.Member, false
		}
	case *ast.MetadataAccessExpr:
		// The conversion files a metadata access's referenced element at `function`.
		return "function", n.Ref, true
	}
	return "", nil, false
}

// referencedSymbol is what the resolver bound name's last segment to, nil
// while it is unresolved.
func (m *Model) referencedSymbol(name *ast.QualifiedName) *symbols.Symbol {
	if name == nil || len(name.Parts) == 0 {
		return nil
	}
	sym, ok := m.resolver.PartSymbol(name, len(name.Parts)-1)
	if !ok {
		return nil
	}
	return sym
}

// expressionReferencedElements is the element the expression's reference
// membership at the slot named feature holds, read from its owned memberships.
func (m *Model) expressionReferencedElements(expr *symbols.Symbol, feature string) []*symbols.Symbol {
	if expr.Expr != nil && expr.Expr.Role == symbols.ExpressionChangeWrapper && feature == "referent" {
		return []*symbols.Symbol{m.expressionPartsOf(expr).condition}
	}
	if parts := m.expressionPartsOf(expr); parts != nil {
		if feature == "referent" && parts.body != nil {
			return []*symbols.Symbol{parts.body}
		}
		// Expression::function is the Function typing the expression (KerML
		// 8.3.4.7.3, 8.3.4.8.8): for a receiver-reached call the Function types
		// of its chain's final link; the chain Feature itself is the instantiated type.
		if feature == "function" && parts.chain != nil {
			links := m.ChainingFeatures(parts.chain)
			if n := len(links); n > 0 {
				if m.reflectiveMetaclassConforms(links[n-1], "Function") {
					return []*symbols.Symbol{links[n-1]}
				}
				return append([]*symbols.Symbol{}, m.FunctionTypes(links[n-1])...)
			}
			return []*symbols.Symbol{}
		}
	}
	for _, membership := range m.OwnedMemberships(expr) {
		if rel := membership.Implicit; rel != nil && rel.Form == symbols.ImplicitReferenceMembership && rel.Slot == feature {
			return []*symbols.Symbol{rel.Member}
		}
	}
	return []*symbols.Symbol{}
}

// referenceMembershipElements is ReflectiveElements for a plain Membership an
// expression owns: it relates the expression to the element it names and owns
// nothing (KerML 8.3.2.5 Membership; ownedMemberElement is OwningMembership's).
func referenceMembershipElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	rel := sym.Implicit
	switch feature {
	case "source", "membershipOwningNamespace", "owningRelatedElement", "owner":
		return []*symbols.Symbol{rel.Owner}, true
	case "target", "memberElement":
		return []*symbols.Symbol{rel.Member}, true
	case "relatedElement":
		return []*symbols.Symbol{rel.Owner, rel.Member}, true
	case "ownedRelatedElement", "ownedRelationship", "ownedElement", "ownedAnnotation", "owningRelationship",
		"owningMembership", "owningNamespace", "documentation", "textualRepresentation", "ownedImport", "ownedMembership":
		return []*symbols.Symbol{}, true
	}
	return nil, false
}

// referenceMembershipValue is reflectiveFeatureValue for a plain Membership an
// expression owns: its member's names, and the public visibility the
// conversion leaves unstated (KerML 8.3.2.5 Membership::visibility default).
func (m *Model) referenceMembershipValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch feature {
	case "memberName":
		return stringOrEmpty(SimpleSymbolName(sym.Implicit.Member)), true
	case "memberShortName":
		return stringOrEmpty(m.EffectiveShortNameOf(sym.Implicit.Member)), true
	case "visibility":
		return m.visibilityValue(ast.VisibilityPublic), true
	}
	return symbols.FilterValue{}, false
}

// bodyScopeOf is the scope a body Expression's declarations live in: the child
// scope the symbol builder gave the body, found under the declaration whose
// expression holds it; nil for a body declaring nothing.
func (m *Model) bodyScopeOf(body *symbols.Symbol) *symbols.Scope {
	node, _ := body.Decl.(*ast.BodyExpr)
	root := body
	for root != nil && root.Expr != nil {
		root = root.Expr.Owner
	}
	if node == nil || root == nil {
		return nil
	}
	for _, scope := range []*symbols.Scope{root.Scope, root.OwnerScope} {
		if found := findBodyScope(scope, node); found != nil {
			return found
		}
	}
	return nil
}

// findBodyScope is the child of scope, or of a body-local child of it, whose
// node is body.
func findBodyScope(scope *symbols.Scope, body *ast.BodyExpr) *symbols.Scope {
	if scope == nil {
		return nil
	}
	for _, child := range scope.Children() {
		if child.Node() == body {
			return child
		}
		if child.BodyLocal() {
			if found := findBodyScope(child, body); found != nil {
				return found
			}
		}
	}
	return nil
}

// bodyDeclarationSymbol is the symbol scope declares for one declaration of
// body: a parameter's shares the body as its node and carries the parameter's
// span; a member's is declared at the member.
func bodyDeclarationSymbol(scope *symbols.Scope, body *ast.BodyExpr, decl ast.BodyDeclaration) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	var found *symbols.Symbol
	scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member == nil {
			return true
		}
		switch {
		case decl.Param != nil && member.Decl == body && member.DeclSpan == decl.Param.Span:
			found = member
		case decl.Param == nil && decl.Decl != nil && member.Decl == decl.Decl:
			found = member
		}
		return found == nil
	})
	return found
}

// isBodyParameterSymbol reports whether sym is a parameter an expression body
// declares (`{ in x; … }`): the builder declares it against the body node.
func isBodyParameterSymbol(sym *symbols.Symbol) bool {
	if sym == nil || sym.Expr != nil || sym.OwnerScope == nil || !sym.OwnerScope.BodyLocal() {
		return false
	}
	_, ok := sym.Decl.(*ast.BodyExpr)
	return ok && sym.OwnerScope.Node() == sym.Decl
}

// bodyDeclaredBy is the body Expression declaring sym, for a symbol a body's
// scope holds (its parameters and members); nil otherwise. The body's symbol
// is synthesized under the expression holding the body, so the declaration
// whose value reaches it is searched when none has been reflected yet.
func (m *Model) bodyDeclaredBy(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.Expr != nil || sym.OwnerScope == nil || !sym.OwnerScope.BodyLocal() {
		return nil
	}
	body, ok := sym.OwnerScope.Node().(*ast.BodyExpr)
	if !ok {
		return nil
	}
	if found, ok := m.bodySymbols[body]; ok {
		return found
	}
	root := sym.OwnerScope
	for root != nil && root.Owner() == nil {
		root = root.Parent()
	}
	if root == nil {
		return nil
	}
	// The body sits in the value of the scope's owner or of one of its
	// members, whichever spans it.
	span := body.Span()
	reflect := func(decl *symbols.Symbol) bool {
		if value := m.ValueExpression(decl); value != nil &&
			value.DeclSpan.Offset <= span.Offset && span.Offset+span.Len <= value.DeclSpan.Offset+value.DeclSpan.Len {
			m.reflectBodiesUnder(value)
		}
		_, found := m.bodySymbols[body]
		return !found
	}
	if reflect(root.Owner()) {
		root.ForEachMember(reflect)
	}
	if _, found := m.bodySymbols[body]; !found {
		root.ForEachAnonymousMember(reflect)
	}
	return m.bodySymbols[body]
}

// reflectBodiesUnder reflects every body expression under expr, so each is
// recorded against its node.
func (m *Model) reflectBodiesUnder(expr *symbols.Symbol) {
	if expr == nil {
		return
	}
	parts := m.expressionPartsOf(expr)
	if parts == nil {
		return
	}
	if parts.condition != nil {
		m.reflectBodiesUnder(parts.condition)
	}
	if parts.body != nil {
		m.reflectBodiesUnder(parts.body)
	}
	for _, member := range parts.members {
		if value := m.ValueExpression(member); value != nil {
			m.reflectBodiesUnder(value)
		}
	}
	if parts.result != nil && parts.result.Expr.Role == symbols.ExpressionBodyResult {
		m.reflectBodiesUnder(parts.result)
	}
	for _, operand := range parts.operands {
		m.reflectBodiesUnder(operand)
	}
}

// expressionChain is the chain of names an expression instantiates or targets,
// at the slot the conversion owns the chain Feature at: a call reached through
// its receiver (`q.p.f(1)`, KerML 8.2.5.8.3 InstantiatedTypeMember) and a dotted
// function or target name (calleeMembership, FeatureChainExpr). A function's
// chain leads the parameters; a target's follows its operand.
func expressionChain(node ast.Node) (slot string, links []*ast.QualifiedName, leads bool) {
	dotted := func(name *ast.QualifiedName) []*ast.QualifiedName {
		var out []*ast.QualifiedName
		start := 0
		for i := 1; i <= len(name.Parts); i++ {
			if i == len(name.Parts) || name.Parts[i].Chained {
				out = append(out, &ast.QualifiedName{Parts: name.Parts[start:i]})
				start = i
			}
		}
		return out
	}
	switch n := node.(type) {
	case *ast.InvocationExpr:
		if n.Type == nil {
			if links := ast.ReceiverChain(n.Operand); links != nil {
				return "function", links, true
			}
		} else if ast.QualifiedNameHasChain(n.Type) {
			return "function", dotted(n.Type), true
		}
	case *ast.ConstructorExpr:
		if n.Type != nil && ast.QualifiedNameHasChain(n.Type) {
			return "function", dotted(n.Type), true
		}
	case *ast.FeatureChainExpr:
		if n.Member != nil && ast.QualifiedNameHasChain(n.Member) {
			return "targetFeature", dotted(n.Member), false
		}
	}
	return "", nil, false
}

// ExpressionChainFeature is the Feature the expression element expr owns for the
// dotted name of its function or target feature; nil where the name is plain.
func (m *Model) ExpressionChainFeature(expr *symbols.Symbol) *symbols.Symbol {
	if parts := m.expressionPartsOf(expr); parts != nil {
		return parts.chain
	}
	return nil
}

// chainFeatureMemberships are the FeatureChainings a chain Feature owns, one per
// segment of the dotted name, each to the feature the segment resolves to
// (qualifiedChainReferences); a segment resolving to nothing owns none.
func (m *Model) chainFeatureMemberships(chain *symbols.Symbol) []*symbols.Symbol {
	_, links, _ := expressionChain(chain.Expr.Owner.Decl)
	var out []*symbols.Symbol
	for _, link := range links {
		target := m.referencedSymbol(link)
		if target == nil {
			continue
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    chain.DocName,
			DeclSpan:   chain.DeclSpan,
			Visibility: chain.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: chain, Ordinal: len(out), Form: symbols.ImplicitFeatureChaining,
				Member: target, Slot: fmt.Sprintf("fc%d", len(out)),
			},
		})
	}
	return out
}

// ChainingFeatures are the features a chain Feature chains, in order.
func (m *Model) ChainingFeatures(chain *symbols.Symbol) []*symbols.Symbol {
	links := []*symbols.Symbol{}
	for _, chaining := range m.OwnedRelationships(chain) {
		if chaining.Implicit != nil && chaining.Implicit.Form == symbols.ImplicitFeatureChaining {
			links = append(links, chaining.Implicit.Member)
		}
	}
	return links
}

// featureChainingElements is ReflectiveElements for a reflected FeatureChaining.
func featureChainingElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	rel := sym.Implicit
	switch feature {
	case "source", "featureChained", "owningRelatedElement", "owner":
		return []*symbols.Symbol{rel.Owner}, true
	case "target", "chainingFeature":
		return []*symbols.Symbol{rel.Member}, true
	case "relatedElement":
		return []*symbols.Symbol{rel.Owner, rel.Member}, true
	case "ownedRelatedElement", "ownedRelationship", "ownedElement", "ownedAnnotation", "owningRelationship",
		"owningMembership", "owningNamespace", "documentation", "textualRepresentation":
		return []*symbols.Symbol{}, true
	}
	return nil, false
}
