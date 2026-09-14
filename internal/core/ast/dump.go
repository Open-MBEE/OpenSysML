package ast

import (
	"fmt"
	"strings"
)

// Dump renders a node tree as an indented S-expression for golden tests.
func Dump(n Node) string {
	var b strings.Builder
	dumpNode(&b, n, 0)
	return b.String()
}

func indent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

// endString renders the end of a flow — the feature the payload leaves or
// reaches — as the notation names it. An end is a feature chain as often as it
// is a single name (`generateTorque.engineTorque`), so both are spelled out: a
// dump that showed only the node type would not lock which feature the flow
// connects.
func endString(end Node) string {
	switch n := end.(type) {
	case nil:
		return "nil"
	case *QualifiedName:
		return qnString(n)
	case *FeatureReference:
		return qnString(n.Name)
	case *FeatureChainExpr:
		return endString(n.Operand) + "." + qnString(n.Member)
	default:
		return fmt.Sprintf("%T", end)
	}
}

// qnString renders a QualifiedName as `A::B::C` (with `$::` prefix if global).
func qnString(qn *QualifiedName) string {
	if qn == nil {
		return ""
	}
	s := ""
	for i, p := range qn.Parts {
		switch {
		case i == 0:
		case p.Chained:
			// A chained segment was written with '.', which reaches through a
			// feature rather than into a namespace.
			s += "."
		default:
			s += "::"
		}
		s += p.Text
	}
	if qn.Global {
		return "$::" + s
	}
	return s
}

// successionEnd renders one end of a succession: the name it references, or
// `@<kind>` for an end the notation binds by position, where the member beside
// the `then` declares no name (see SuccessionEdge.SourceMember).
func successionEnd(qn *QualifiedName, member Node) string {
	if qn != nil && len(qn.Parts) > 0 {
		return qnString(qn)
	}
	if member == nil {
		return ""
	}
	return "@" + positionalEndKind(member)
}

// positionalEndKind names the kind of node a positional succession end refers
// to, which is what identifies it in the absence of a name.
func positionalEndKind(member Node) string {
	switch m := member.(type) {
	case *WhileLoopActionNode:
		return m.Kind.String()
	case *IfActionNode:
		return "if"
	case *SendStatement:
		return "send"
	case *AssignmentActionNode:
		return "assign"
	case *TerminateStatement:
		return "terminate"
	case *FinalNode:
		return "done"
	case *ForkNode:
		return "fork"
	case *JoinNode:
		return "join"
	case *MergeNode:
		return "merge"
	case *DecisionNode:
		return "decide"
	case *ActionExecutionNode:
		return "action"
	case *InitialNode:
		return "start"
	case *EntryMember:
		return "entry"
	case *DoMember:
		return "do"
	case *ExitMember:
		return "exit"
	case *SubstateMember:
		return "state"
	case *TransitionMember:
		return "transition"
	case *PseudostateNode:
		return m.Kind.String()
	case *Usage:
		return m.Kind.String()
	case *Definition:
		return m.Kind.String()
	default:
		return fmt.Sprintf("%T", member)
	}
}

// dumpNode writes `(Type attrs children...)`. It writes the open line with
// header and any leaf attributes, then children each on their own line at
// depth+1, closing all open parens on the final line.
func dumpNode(b *strings.Builder, n Node, depth int) {
	indent(b, depth)
	switch {
	case dumpExpression(b, n, depth):
	case dumpNamespaceMember(b, n, depth):
	case dumpDeclaration(b, n, depth):
	case dumpBehavior(b, n, depth):
	default:
		fmt.Fprintf(b, `(%T)`, n)
	}
}

// dumpExpression dumps a literal or expression node, reporting whether n was one.
func dumpExpression(b *strings.Builder, n Node, depth int) bool {
	switch v := n.(type) {
	case *LiteralInteger:
		fmt.Fprintf(b, `(LiteralInteger value=%q)`, v.Value)
		return true
	case *LiteralReal:
		fmt.Fprintf(b, `(LiteralReal value=%q)`, v.Value)
		return true
	case *LiteralString:
		fmt.Fprintf(b, `(LiteralString value=%q)`, v.Value)
		return true
	case *LiteralBool:
		fmt.Fprintf(b, `(LiteralBool value=%t)`, v.Value)
		return true
	case *LiteralInfinity:
		b.WriteString(`(LiteralInfinity)`)
		return true
	case *NullExpr:
		b.WriteString(`(NullExpr)`)
		return true
	case *FeatureReference:
		fmt.Fprintf(b, `(FeatureReference name=%q)`, qnString(v.Name))
		return true
	case *MetadataAccessExpr:
		fmt.Fprintf(b, `(MetadataAccessExpr ref=%q)`, qnString(v.Ref))
		return true
	case *OperatorExpr:
		fmt.Fprintf(b, `(OperatorExpr operator=%q`, v.Operator.String())
		writeChildren(b, depth, operandsWithTypeRef(v))
		return true
	case *FeatureChainExpr:
		fmt.Fprintf(b, `(FeatureChainExpr member=%q`, qnString(v.Member))
		writeChildren(b, depth, []Node{v.Operand})
		return true
	case *IndexExpr:
		b.WriteString(`(IndexExpr`)
		if v.Bracket {
			b.WriteString(` bracket=true`)
		}
		writeChildren(b, depth, []Node{v.Operand, v.Index})
		return true
	case *CollectExpr:
		b.WriteString(`(CollectExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Body})
		return true
	case *SelectExpr:
		b.WriteString(`(SelectExpr`)
		writeChildren(b, depth, []Node{v.Operand, v.Body})
		return true
	case *InvocationExpr:
		fmt.Fprintf(b, `(InvocationExpr type=%q`, qnString(v.Type))
		writeArgChildren(b, depth, invocationChildren(v), v.NamedArgs)
		return true
	case *ConstructorExpr:
		fmt.Fprintf(b, `(ConstructorExpr type=%q`, qnString(v.Type))
		writeArgChildren(b, depth, v.Args, v.NamedArgs)
		return true
	case *SequenceExpr:
		b.WriteString(`(SequenceExpr`)
		writeChildren(b, depth, v.Elements)
		return true
	case *BodyExpr:
		b.WriteString(`(BodyExpr`)
		if len(v.Params) > 0 {
			b.WriteString(` params=[`)
			for i, p := range v.Params {
				if i > 0 {
					b.WriteString(`,`)
				}
				b.WriteString(p.Name)
				if p.Type != nil {
					b.WriteString(`:`)
					b.WriteString(qnString(p.Type))
				}
				if p.Value != nil {
					b.WriteString(`=...`)
				}
			}
			b.WriteString(`]`)
		}
		var kids []Node
		// Add param values and the specializations they declare to children
		for _, p := range v.Params {
			if p.Value != nil {
				kids = append(kids, p.Value)
			}
			for _, r := range p.Relationships {
				kids = append(kids, r)
			}
		}
		kids = append(kids, v.Members...)
		if v.Result != nil {
			kids = append(kids, v.Result)
		}
		writeChildren(b, depth, kids)
		return true
	case *ErrorNode:
		fmt.Fprintf(b, `(ErrorNode message=%q)`, v.Message)
		return true
	default:
		return false
	}
}

// dumpNamespaceMember dumps a namespace, import or annotation node, reporting
// whether n was one.
func dumpNamespaceMember(b *strings.Builder, n Node, depth int) bool {
	switch v := n.(type) {
	case *RootNamespace:
		b.WriteString(`(RootNamespace`)
		writeChildren(b, depth, v.Members)
		return true
	case *Membership:
		fmt.Fprintf(b, `(Membership visibility=%q`, visibilityString(v.Visibility))
		if v.IsTypeFeature {
			b.WriteString(` typeFeature=true`)
		}
		writeChildren(b, depth, []Node{v.Member})
		return true
	case *Package:
		fmt.Fprintf(b, `(Package name=%q library=%t standard=%t`, identName(v.Ident), v.IsLibrary, v.IsStandard)
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Members))
		return true
	case *Namespace:
		fmt.Fprintf(b, `(Namespace name=%q`, identName(v.Ident))
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Members))
		return true
	case *Import:
		label := "Import"
		if v.IsExpose {
			label = "Expose"
		}
		fmt.Fprintf(b, `(%s visibility=%q all=%t kind=%s recursive=%t imported=%q filtered=%t`,
			label, visibilityString(v.Visibility), v.IsAll, importKindString(v.Kind), v.IsRecursive, qnString(v.Imported), v.FilterExpr != nil)
		kids := v.Body
		if v.FilterExpr != nil {
			kids = append([]Node{v.FilterExpr}, kids...)
		}
		writeChildren(b, depth, kids)
		return true
	case *Alias:
		fmt.Fprintf(b, `(Alias visibility=%q name=%q for=%q`,
			visibilityString(v.Visibility), identName(v.Ident), qnString(v.For))
		writeChildren(b, depth, v.Body)
		return true
	case *MultiplicityDecl:
		fmt.Fprintf(b, `(MultiplicityDecl name=%q subsets=%q`, identName(v.Ident), qnString(v.Subsets))
		kids := make([]Node, 0, len(v.Members)+1)
		if v.Range != nil {
			kids = append(kids, v.Range)
		}
		writeChildren(b, depth, append(kids, v.Members...))
		return true
	case *Dependency:
		fmt.Fprintf(b, `(Dependency clients=%q suppliers=%q`, qnList(v.Clients), qnList(v.Suppliers))
		writeChildren(b, depth, prefixesAnd(v.Prefixes, v.Body))
		return true
	case *RelationshipMember:
		fmt.Fprintf(b, `(RelationshipMember visibility=%q kind=%q name=%q keyword=%q source=%q target=%q conjugated=%t`,
			visibilityString(v.Visibility), v.Kind.String(), identName(v.Ident), v.Keyword, endString(v.Source), endString(v.Target), v.Conjugated)
		writeChildren(b, depth, v.Members)
		return true
	case *Comment:
		fmt.Fprintf(b, `(Comment about=%q locale=%q)`, qnList(v.About), v.Locale)
		return true
	case *Documentation:
		fmt.Fprintf(b, `(Documentation locale=%q)`, v.Locale)
		return true
	case *TextualRepresentation:
		fmt.Fprintf(b, `(TextualRepresentation language=%q`, v.Language)
		if name := identName(v.Ident); name != "" {
			fmt.Fprintf(b, ` name=%q`, name)
		}
		b.WriteString(`)`)
		return true
	case *PrefixMetadata:
		fmt.Fprintf(b, `(PrefixMetadata type=%q`, qnString(v.Type))
		if name := identName(v.Ident); name != "" {
			fmt.Fprintf(b, ` name=%q`, name)
		}
		if len(v.About) > 0 {
			fmt.Fprintf(b, ` about=%q`, qnList(v.About))
		}
		if v.HasBody && len(v.Body) == 0 {
			b.WriteString(` emptyBody=true`)
		}
		writeChildren(b, depth, v.Body)
		return true
	case *FilterMember:
		b.WriteString(`(FilterMember`)
		writeChildren(b, depth, []Node{v.Condition})
		return true
	default:
		return false
	}
}

// dumpDeclaration dumps a definition, usage or relationship node, reporting
// whether n was one.
func dumpDeclaration(b *strings.Builder, n Node, depth int) bool {
	switch v := n.(type) {
	case *Definition:
		fmt.Fprintf(b, `(Definition kind=%q abstract=%t variation=%t name=%q`,
			v.Kind.String(), v.IsAbstract, v.IsVariation, identName(v.Ident))
		// The modifier says something only where the kind does not: an
		// `individual part def` is a part definition of an individual.
		if v.IsIndividual && v.Kind != DefIndividual {
			b.WriteString(` individual=true`)
		}
		if v.IsParallel {
			b.WriteString(` parallel=true`)
		}
		if v.IsAll {
			b.WriteString(` all=true`)
		}
		writeChildren(b, depth, defusageChildren(v.Prefixes, v.Relationships, v.Multiplicity, nil, v.Members))
		return true
	case *Usage:
		fmt.Fprintf(b, `(Usage kind=%q name=%q ref=%t direction=%q composite=%t derived=%t ordered=%t nonunique=%t`,
			v.Kind.String(), identName(v.Ident), v.IsReference, v.Direction.String(),
			v.IsComposite, v.IsDerived, v.IsOrdered, v.IsNonunique)
		if v.IsVariation {
			b.WriteString(` variation=true`)
		}
		if v.IsVariant {
			b.WriteString(` variant=true`)
		}
		if v.IsVariable {
			b.WriteString(` variable=true`)
		}
		// The keyword a usage was written with, when it says more than the kind
		// does: `exhibit state modes` is a state usage the enclosing part
		// exhibits, `perform action a` an action it performs, and a dump that
		// showed only the kind would read the same as a plain declaration. A
		// portion keyword is printed by the portion itself.
		if kw := v.Keyword; kw != "" && kw != v.Kind.String() && kw != v.Portion.Keyword() {
			fmt.Fprintf(b, ` keyword=%q`, kw)
		}
		// The prefix says what the declaration is for: an asserted constraint
		// reads differently from a declared one.
		if v.PrefixKeyword != "" {
			fmt.Fprintf(b, ` prefix=%q`, v.PrefixKeyword)
		}
		if v.IsEnd {
			b.WriteString(` end=true`)
		}
		if v.IsChain {
			b.WriteString(` chain=true`)
		}
		if v.IsIndividual {
			b.WriteString(` individual=true`)
		}
		if v.IsParallel {
			b.WriteString(` parallel=true`)
		}
		if v.IsAll {
			b.WriteString(` all=true`)
		}
		if kw := v.Portion.Keyword(); kw != "" {
			fmt.Fprintf(b, ` %s=true`, kw)
		}
		if v.IsNegated {
			b.WriteString(` negated=true`)
		}
		if v.IsActionNode {
			b.WriteString(` node=true`)
		}
		writeValueOperator(b, v.ValueIsDefault, v.ValueIsInitial)
		writeChildren(b, depth, usageChildren(v))
		return true
	case *FlowEnds:
		fmt.Fprintf(b, `(FlowEnds from=%q to=%q payload=%q declared=%t`,
			endString(v.From), endString(v.To), endString(v.Payload), v.PayloadDecl != nil)
		var kids []Node
		if v.PayloadMultiplicity != nil {
			kids = append(kids, v.PayloadMultiplicity)
		}
		writeChildren(b, depth, kids)
		return true
	case *SendStatement:
		// The `to`/`via` distinction decides how the message is routed, so a
		// golden that did not show it would not lock the parse.
		fmt.Fprintf(b, `(SendStatement via=%t`, v.IsVia)
		var kids []Node
		if v.Message != nil {
			kids = append(kids, v.Message)
		}
		if v.Target != nil {
			kids = append(kids, v.Target)
		}
		if v.Receiver != nil {
			kids = append(kids, v.Receiver)
		}
		kids = append(kids, v.Members...)
		writeChildren(b, depth, kids)
		return true
	case *CrossFeatureMember:
		fmt.Fprintf(b, `(CrossFeatureMember name=%q`, identName(v.Ident))
		if v.Direction != DirNone {
			fmt.Fprintf(b, ` direction=%q`, v.Direction.String())
		}
		for _, flag := range []struct {
			name string
			set  bool
		}{
			{"derived", v.IsDerived}, {"abstract", v.IsAbstract}, {"variation", v.IsVariation},
			{"composite", v.IsComposite}, {"portion", v.IsPortion}, {"variable", v.IsVariable},
			{"constant", v.IsConstant}, {"ref", v.IsReference},
			{"ordered", v.IsOrdered}, {"nonunique", v.IsNonunique},
		} {
			if flag.set {
				fmt.Fprintf(b, ` %s=true`, flag.name)
			}
		}
		var kids []Node
		if v.Multiplicity != nil {
			kids = append(kids, v.Multiplicity)
		}
		for _, r := range v.Relationships {
			kids = append(kids, r)
		}
		writeChildren(b, depth, kids)
		return true
	case *ConnectorEnd:
		targetStr := ""
		if qn, ok := v.Target.(*QualifiedName); ok {
			targetStr = qnString(qn)
		} else {
			targetStr = fmt.Sprintf("%T", v.Target) // fallback: show type
		}
		fmt.Fprintf(b, `(ConnectorEnd target=%q`, targetStr)
		var kids []Node
		if v.Multiplicity != nil {
			kids = append(kids, v.Multiplicity)
		}
		if v.Target != nil {
			kids = append(kids, v.Target)
		}
		for _, r := range v.Relationships {
			kids = append(kids, r)
		}
		if v.Reference != nil {
			kids = append(kids, v.Reference)
		}
		writeChildren(b, depth, kids)
		return true
	case *Relationship:
		targetStr := "nil"
		if v.Target != nil {
			if qn, ok := v.Target.(*QualifiedName); ok {
				targetStr = qnString(qn)
			} else {
				targetStr = "(expr)"
			}
		}
		fmt.Fprintf(b, `(Relationship kind=%q target=%s`, v.Kind.String(), targetStr)
		if v.Conjugated {
			b.WriteString(` conjugated=true`)
		}
		var kids []Node
		if v.Target != nil {
			kids = append(kids, v.Target)
		}
		if v.Multiplicity != nil {
			kids = append(kids, v.Multiplicity)
		}
		writeChildren(b, depth, kids)
		return true
	case *Multiplicity:
		fmt.Fprintf(b, `(Multiplicity range=%t`, v.IsRange)
		var kids []Node
		if v.Lower != nil {
			kids = append(kids, v.Lower)
		}
		if v.Upper != nil {
			kids = append(kids, v.Upper)
		}
		writeChildren(b, depth, kids)
		return true
	case *SubjectMember:
		fmt.Fprintf(b, `(SubjectMember name=%q type=%q`, identName(v.Ident), qnString(v.TypeRef))
		writeValueOperator(b, v.ValueIsDefault, v.ValueIsInitial)
		kids := make([]Node, 0)
		for _, pm := range v.Prefixes {
			kids = append(kids, pm)
		}
		if v.Multiplicity != nil {
			kids = append(kids, v.Multiplicity)
		}
		for _, r := range v.Relationships {
			kids = append(kids, r)
		}
		kids = append(kids, v.Body...)
		writeChildren(b, depth, kids)
		return true
	case *RequireMember:
		if v.Reference != nil {
			// Reference form: require Q::r [mult] { body }
			fmt.Fprintf(b, `(RequireMember name=%q`, qnString(v.Reference))
			writeChildren(b, depth, referenceChildren(v.Multiplicity, v.Body))
		} else if v.Expression == nil {
			// Constraint form: require constraint [decl] (; | { expr })
			b.WriteString(`(RequireMember`)
			if name := identName(v.Ident); name != "" {
				fmt.Fprintf(b, ` constraint=%q`, name)
			}
			writeValueOperator(b, v.ValueIsDefault, v.ValueIsInitial)
			writeChildren(b, depth, prefixesAnd(v.Prefixes,
				ownedConstraintChildren(v.Relationships, v.Multiplicity, v.Value, v.Body)))
		} else {
			// Expression form: require expr;
			b.WriteString(`(RequireMember`)
			writeChildren(b, depth, []Node{v.Expression})
		}
		return true
	case *AssumeMember:
		if v.Reference != nil {
			fmt.Fprintf(b, `(AssumeMember name=%q`, qnString(v.Reference))
			writeChildren(b, depth, referenceChildren(v.Multiplicity, v.Body))
			return true
		}
		b.WriteString(`(AssumeMember`)
		if v.Expression == nil {
			if name := identName(v.Ident); name != "" {
				fmt.Fprintf(b, ` constraint=%q`, name)
			}
			writeValueOperator(b, v.ValueIsDefault, v.ValueIsInitial)
			writeChildren(b, depth, prefixesAnd(v.Prefixes,
				ownedConstraintChildren(v.Relationships, v.Multiplicity, v.Value, v.Body)))
		} else {
			writeChildren(b, depth, []Node{v.Expression})
		}
		return true
	case *ConstraintMember:
		fmt.Fprintf(b, `(ConstraintMember assert=%t negated=%t`, v.IsAssert, v.IsNegated)
		if v.Keyword != "" {
			fmt.Fprintf(b, ` keyword=%q`, v.Keyword)
		}
		if v.Expression == nil {
			// Nested-constraint form: assert constraint [name] { expr }
			if v.Name != "" {
				fmt.Fprintf(b, ` name=%q`, v.Name)
			}
			writeChildren(b, depth, v.Body)
		} else {
			writeChildren(b, depth, []Node{v.Expression})
		}
		return true
	default:
		return false
	}
}

// dumpBehavior dumps a behavioral node — a transition, event or control node —
// reporting whether n was one.
func dumpBehavior(b *strings.Builder, n Node, depth int) bool {
	switch v := n.(type) {
	case *TransitionMember:
		fmt.Fprintf(b, `(TransitionMember source=%q target=%q`, qnString(v.Source), qnString(v.Target))
		if v.Name != "" {
			fmt.Fprintf(b, ` name=%q`, v.Name)
		}
		if v.Via != nil {
			fmt.Fprintf(b, ` via=%q`, qnString(v.Via))
		}
		// Braces holding nothing leave no child to show them by.
		if v.HasEffect && len(v.Effect) == 0 {
			b.WriteString(` emptyEffect=true`)
		}
		if v.HasBody && len(v.Members) == 0 {
			b.WriteString(` emptyBody=true`)
		}
		kids := make([]Node, 0, len(v.Effect)+2)
		// A trigger written as a bare name — `accept 'Ground Station Ping'` —
		// is the signal it names, which reads better beside the ends than as a
		// nameless child.
		if qn, ok := v.Trigger.(*QualifiedName); ok {
			fmt.Fprintf(b, ` trigger=%q`, qnString(qn))
		} else if v.Trigger != nil {
			kids = append(kids, v.Trigger)
		}
		if v.Guard != nil {
			kids = append(kids, v.Guard)
		}
		kids = append(kids, v.Effect...)
		kids = append(kids, v.Members...)
		writeChildren(b, depth, kids)
		return true
	case *TimeEvent:
		// `at` and `after` differ only in this flag, so print it.
		fmt.Fprintf(b, `(TimeEvent absolute=%t`, v.Absolute)
		writeChildren(b, depth, []Node{v.Duration})
		return true
	case *ChangeEvent:
		b.WriteString(`(ChangeEvent`)
		writeChildren(b, depth, []Node{v.Condition})
		return true
	case *CallEvent:
		names := make([]string, len(v.Parameters))
		for i, p := range v.Parameters {
			names[i] = p.Text
		}
		fmt.Fprintf(b, `(CallEvent operation=%q parameters=[%s])`,
			qnString(v.Operation), strings.Join(names, " "))
		return true
	case *WhileLoopActionNode:
		// kind and variable distinguish the three loop forms, which the node
		// shape alone does not.
		fmt.Fprintf(b, `(WhileLoopActionNode kind=%q variable=%q`, v.Kind.String(), v.Variable.Name)
		kids := []Node{}
		for _, rel := range v.VariableRelationships {
			kids = append(kids, rel)
		}
		if v.Condition != nil {
			kids = append(kids, v.Condition)
		}
		if v.Until != nil {
			kids = append(kids, v.Until)
		}
		if v.Collection != nil {
			kids = append(kids, v.Collection)
		}
		kids = append(kids, v.Body...)
		writeChildren(b, depth, kids)
		return true
	case *IfActionNode:
		b.WriteString(`(IfActionNode`)
		kids := []Node{}
		if v.Condition != nil {
			kids = append(kids, v.Condition)
		}
		for _, branch := range v.Branches() {
			kids = append(kids, branch)
		}
		writeChildren(b, depth, kids)
		return true
	case *IfBranchNode:
		fmt.Fprintf(b, `(IfBranchNode kind=%q`, v.Kind.String())
		writeChildren(b, depth, v.Body)
		return true
	case *PseudostateNode:
		fmt.Fprintf(b, `(PseudostateNode kind=%q name=%q)`, v.Kind.String(), v.Name)
		return true
	case *DeferMember:
		b.WriteString(`(DeferMember`)
		writeChildren(b, depth, v.Triggers)
		return true
	case *EntryMember:
		b.WriteString(`(EntryMember`)
		writeChildren(b, depth, v.Actions)
		return true
	case *DoMember:
		b.WriteString(`(DoMember`)
		writeChildren(b, depth, v.Actions)
		return true
	case *ExitMember:
		b.WriteString(`(ExitMember`)
		writeChildren(b, depth, v.Actions)
		return true
	case *SubstateMember:
		fmt.Fprintf(b, `(SubstateMember name=%q)`, v.Name)
		return true
	case *SuccessionEdge:
		// The ends are what a `then` says, whether the author wrote the edge
		// form or the parser desugared a member-attached keyword into it.
		fmt.Fprintf(b, `(SuccessionEdge source=%q target=%q`,
			successionEnd(v.Source, v.SourceMember), successionEnd(v.Target, v.TargetMember))
		if len(v.Members) > 0 {
			writeChildren(b, depth, v.Members)
			return true
		}
		b.WriteString(`)`)
		return true
	case *ControlFlowEdge:
		// The branches of one decision differ only in their guard and in which
		// one is the default, so print both alongside the ends.
		fmt.Fprintf(b, `(ControlFlowEdge source=%q target=%q else=%t`,
			successionEnd(v.Source, v.SourceMember), successionEnd(v.Target, v.TargetMember), v.IsElse)
		if v.Guard != nil {
			writeChildren(b, depth, []Node{v.Guard})
			return true
		}
		b.WriteString(`)`)
		return true
	case *InitialNode:
		fmt.Fprintf(b, `(InitialNode name=%q successor=%q`, v.Name(), qnString(v.Successor))
		kids := []Node{}
		if v.Guard != nil {
			kids = append(kids, v.Guard)
		}
		kids = append(kids, v.Members...)
		writeChildren(b, depth, kids)
		return true
	case *FinalNode:
		fmt.Fprint(b, `(FinalNode)`)
		return true
	case *ForkNode:
		fmt.Fprintf(b, `(ForkNode name=%q`, v.Name)
		writeChildren(b, depth, v.Members)
		return true
	case *JoinNode:
		fmt.Fprintf(b, `(JoinNode name=%q`, v.Name)
		writeChildren(b, depth, v.Members)
		return true
	case *MergeNode:
		fmt.Fprintf(b, `(MergeNode name=%q`, v.Name)
		writeChildren(b, depth, v.Members)
		return true
	case *DecisionNode:
		fmt.Fprintf(b, `(DecisionNode name=%q`, v.Name)
		writeChildren(b, depth, v.Members)
		return true
	case *TerminateStatement:
		b.WriteString(`(TerminateStatement`)
		if v.Target != nil {
			writeChildren(b, depth, []Node{v.Target})
			return true
		}
		b.WriteString(`)`)
		return true
	default:
		return false
	}
}

// writeChildren appends children lines under a header that was written
// WITHOUT its closing paren; it closes with `)` after the last child (or
// immediately if there are none).
func writeChildren(b *strings.Builder, depth int, kids []Node) {
	if len(kids) == 0 {
		b.WriteString(")")
		return
	}
	for _, k := range kids {
		b.WriteString("\n")
		dumpNode(b, k, depth+1)
	}
	b.WriteString(")")
}

// writeArgChildren is writeChildren for an argument list, each named argument
// appearing as `(NamedArg name="n" <value>)` after the positional ones.
func writeArgChildren(b *strings.Builder, depth int, kids []Node, named []NamedArg) {
	if len(named) == 0 {
		writeChildren(b, depth, kids)
		return
	}
	for _, k := range kids {
		b.WriteString("\n")
		dumpNode(b, k, depth+1)
	}
	for _, na := range named {
		b.WriteString("\n")
		indent(b, depth+1)
		fmt.Fprintf(b, `(NamedArg name=%q`, qnString(na.Name))
		writeChildren(b, depth+1, []Node{na.Value})
	}
	b.WriteString(")")
}

func operandsWithTypeRef(v *OperatorExpr) []Node {
	kids := append([]Node{}, v.Operands...)
	if v.TypeRef != nil {
		kids = append(kids, &FeatureReference{Name: v.TypeRef})
	}
	return kids
}

func invocationChildren(v *InvocationExpr) []Node {
	kids := []Node{}
	if v.Operand != nil {
		kids = append(kids, v.Operand)
	}
	kids = append(kids, v.Args...)
	return kids
}

func visibilityString(v Visibility) string {
	switch v {
	case VisibilityPublic:
		return "public"
	case VisibilityPrivate:
		return "private"
	case VisibilityProtected:
		return "protected"
	default:
		return "default"
	}
}

func importKindString(k ImportKind) string {
	if k == ImportNamespace {
		return "namespace"
	}
	return "membership"
}

// referenceChildren orders the children of a constraint reference: its
// specialization multiplicity, then its body.
func referenceChildren(mult *Multiplicity, body []Node) []Node {
	if mult == nil {
		return body
	}
	return append([]Node{mult}, body...)
}

func identName(id Identification) string {
	if id.ShortName != "" && id.Name != "" {
		return "<" + id.ShortName + "> " + id.Name
	}
	if id.ShortName != "" {
		return "<" + id.ShortName + ">"
	}
	return id.Name
}

func qnList(qns []*QualifiedName) string {
	parts := make([]string, len(qns))
	for i, qn := range qns {
		parts[i] = qnString(qn)
	}
	return strings.Join(parts, ", ")
}

// prefixesAnd renders prefix-metadata nodes (as children) followed by members.
func prefixesAnd(prefixes []*PrefixMetadata, members []Node) []Node {
	kids := make([]Node, 0, len(prefixes)+len(members))
	for _, pm := range prefixes {
		kids = append(kids, pm)
	}
	kids = append(kids, members...)
	return kids
}

// defusageChildren flattens the ordered child set for a Definition/Usage
// dump: prefixes, relationships, optional multiplicity, optional value,
// then members. nil multiplicity/value are omitted.
func defusageChildren(prefixes []*PrefixMetadata, rels []*Relationship, mult *Multiplicity, value Node, members []Node) []Node {
	kids := make([]Node, 0, len(prefixes)+len(rels)+2+len(members))
	for _, pm := range prefixes {
		kids = append(kids, pm)
	}
	for _, r := range rels {
		kids = append(kids, r)
	}
	if mult != nil {
		kids = append(kids, mult)
	}
	if value != nil {
		kids = append(kids, value)
	}
	kids = append(kids, members...)
	return kids
}

// ownedConstraintChildren are the children of the constraint an assume/require
// member declares, in written order.
// writeValueOperator emits the `default` and `:=` of a value part.
func writeValueOperator(b *strings.Builder, isDefault, isInitial bool) {
	if isDefault {
		b.WriteString(` default=true`)
	}
	if isInitial {
		b.WriteString(` initial=true`)
	}
}

func ownedConstraintChildren(rels []*Relationship, mult *Multiplicity, value Node, body []Node) []Node {
	kids := make([]Node, 0, len(rels)+len(body)+2)
	for _, r := range rels {
		kids = append(kids, r)
	}
	if mult != nil {
		kids = append(kids, mult)
	}
	if value != nil {
		kids = append(kids, value)
	}
	return append(kids, body...)
}

// usageChildren is defusageChildren for a Usage, additionally emitting the
// optional ConnectorEnd and FlowEnds nodes (after value, before members).
func usageChildren(v *Usage) []Node {
	kids := make([]Node, 0)
	for _, pm := range v.Prefixes {
		kids = append(kids, pm)
	}
	for _, r := range v.Relationships {
		kids = append(kids, r)
	}
	if v.Multiplicity != nil {
		kids = append(kids, v.Multiplicity)
	}
	if v.CrossFeature != nil {
		kids = append(kids, v.CrossFeature)
	}
	if v.Value != nil {
		kids = append(kids, v.Value)
	}
	for _, ce := range v.ConnectorEnds {
		kids = append(kids, ce)
	}
	if v.FlowEnds != nil {
		kids = append(kids, v.FlowEnds)
	}
	kids = append(kids, v.Members...)
	return kids
}
