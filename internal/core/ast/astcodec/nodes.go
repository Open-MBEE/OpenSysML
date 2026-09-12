// This file enumerates every concrete node type of package ast: its kind, its
// bulk allocation, and the order its fields take in a stream. A node type or
// field missing here fails TestRoundTripEveryNodeType.

package astcodec

import "github.com/Open-MBEE/OpenSysML/internal/core/ast"

// kind numbers the concrete node types; a stream stores nodes grouped by kind.
type kind int

const (
	kindAcceptActionUsage kind = iota
	kindAcceptEvent
	kindActionExecutionNode
	kindAlias
	kindAssignmentActionNode
	kindAssumeMember
	kindBodyExpr
	kindCallEvent
	kindCastExpr
	kindChangeEvent
	kindCollectExpr
	kindComment
	kindConnectorEnd
	kindConstraintMember
	kindConstructorExpr
	kindControlFlowEdge
	kindCrossFeatureMember
	kindDecisionNode
	kindDeferMember
	kindDefinition
	kindDependency
	kindDoMember
	kindDocumentation
	kindEntryMember
	kindErrorNode
	kindExitMember
	kindFeatureChainExpr
	kindFeatureReference
	kindFilterMember
	kindFinalNode
	kindFlowEnds
	kindForkNode
	kindIfActionNode
	kindIfBranchNode
	kindImport
	kindIndexExpr
	kindInitialNode
	kindInvocationExpr
	kindJoinNode
	kindLiteralBool
	kindLiteralInfinity
	kindLiteralInteger
	kindLiteralReal
	kindLiteralString
	kindMembership
	kindMergeNode
	kindMetadataAccessExpr
	kindMultiplicity
	kindMultiplicityDecl
	kindNamespace
	kindNullExpr
	kindObjectFlowEdge
	kindOperatorExpr
	kindPackage
	kindPerformActionNode
	kindPrefixMetadata
	kindPseudostateNode
	kindQualifiedName
	kindRelationship
	kindRelationshipMember
	kindRequireMember
	kindRootNamespace
	kindSelectExpr
	kindSendStatement
	kindSequenceExpr
	kindStateNode
	kindStateRegion
	kindSubjectMember
	kindSubstateMember
	kindSuccessionEdge
	kindTerminateStatement
	kindTextualRepresentation
	kindTimeEvent
	kindTransitionEdge
	kindTransitionMember
	kindUsage
	kindWhileLoopActionNode
	numKinds
)

// kindOf classifies a node, or reports false for a nil node or a nil pointer
// held in the interface; a type it does not know is numKinds.
func kindOf(node ast.Node) (kind, bool) {
	switch n := node.(type) {
	case *ast.AcceptActionUsage:
		return kindAcceptActionUsage, n != nil
	case *ast.AcceptEvent:
		return kindAcceptEvent, n != nil
	case *ast.ActionExecutionNode:
		return kindActionExecutionNode, n != nil
	case *ast.Alias:
		return kindAlias, n != nil
	case *ast.AssignmentActionNode:
		return kindAssignmentActionNode, n != nil
	case *ast.AssumeMember:
		return kindAssumeMember, n != nil
	case *ast.BodyExpr:
		return kindBodyExpr, n != nil
	case *ast.CallEvent:
		return kindCallEvent, n != nil
	case *ast.CastExpr:
		return kindCastExpr, n != nil
	case *ast.ChangeEvent:
		return kindChangeEvent, n != nil
	case *ast.CollectExpr:
		return kindCollectExpr, n != nil
	case *ast.Comment:
		return kindComment, n != nil
	case *ast.ConnectorEnd:
		return kindConnectorEnd, n != nil
	case *ast.ConstraintMember:
		return kindConstraintMember, n != nil
	case *ast.ConstructorExpr:
		return kindConstructorExpr, n != nil
	case *ast.ControlFlowEdge:
		return kindControlFlowEdge, n != nil
	case *ast.CrossFeatureMember:
		return kindCrossFeatureMember, n != nil
	case *ast.DecisionNode:
		return kindDecisionNode, n != nil
	case *ast.DeferMember:
		return kindDeferMember, n != nil
	case *ast.Definition:
		return kindDefinition, n != nil
	case *ast.Dependency:
		return kindDependency, n != nil
	case *ast.DoMember:
		return kindDoMember, n != nil
	case *ast.Documentation:
		return kindDocumentation, n != nil
	case *ast.EntryMember:
		return kindEntryMember, n != nil
	case *ast.ErrorNode:
		return kindErrorNode, n != nil
	case *ast.ExitMember:
		return kindExitMember, n != nil
	case *ast.FeatureChainExpr:
		return kindFeatureChainExpr, n != nil
	case *ast.FeatureReference:
		return kindFeatureReference, n != nil
	case *ast.FilterMember:
		return kindFilterMember, n != nil
	case *ast.FinalNode:
		return kindFinalNode, n != nil
	case *ast.FlowEnds:
		return kindFlowEnds, n != nil
	case *ast.ForkNode:
		return kindForkNode, n != nil
	case *ast.IfActionNode:
		return kindIfActionNode, n != nil
	case *ast.IfBranchNode:
		return kindIfBranchNode, n != nil
	case *ast.Import:
		return kindImport, n != nil
	case *ast.IndexExpr:
		return kindIndexExpr, n != nil
	case *ast.InitialNode:
		return kindInitialNode, n != nil
	case *ast.InvocationExpr:
		return kindInvocationExpr, n != nil
	case *ast.JoinNode:
		return kindJoinNode, n != nil
	case *ast.LiteralBool:
		return kindLiteralBool, n != nil
	case *ast.LiteralInfinity:
		return kindLiteralInfinity, n != nil
	case *ast.LiteralInteger:
		return kindLiteralInteger, n != nil
	case *ast.LiteralReal:
		return kindLiteralReal, n != nil
	case *ast.LiteralString:
		return kindLiteralString, n != nil
	case *ast.Membership:
		return kindMembership, n != nil
	case *ast.MergeNode:
		return kindMergeNode, n != nil
	case *ast.MetadataAccessExpr:
		return kindMetadataAccessExpr, n != nil
	case *ast.Multiplicity:
		return kindMultiplicity, n != nil
	case *ast.MultiplicityDecl:
		return kindMultiplicityDecl, n != nil
	case *ast.Namespace:
		return kindNamespace, n != nil
	case *ast.NullExpr:
		return kindNullExpr, n != nil
	case *ast.ObjectFlowEdge:
		return kindObjectFlowEdge, n != nil
	case *ast.OperatorExpr:
		return kindOperatorExpr, n != nil
	case *ast.Package:
		return kindPackage, n != nil
	case *ast.PerformActionNode:
		return kindPerformActionNode, n != nil
	case *ast.PrefixMetadata:
		return kindPrefixMetadata, n != nil
	case *ast.PseudostateNode:
		return kindPseudostateNode, n != nil
	case *ast.QualifiedName:
		return kindQualifiedName, n != nil
	case *ast.Relationship:
		return kindRelationship, n != nil
	case *ast.RelationshipMember:
		return kindRelationshipMember, n != nil
	case *ast.RequireMember:
		return kindRequireMember, n != nil
	case *ast.RootNamespace:
		return kindRootNamespace, n != nil
	case *ast.SelectExpr:
		return kindSelectExpr, n != nil
	case *ast.SendStatement:
		return kindSendStatement, n != nil
	case *ast.SequenceExpr:
		return kindSequenceExpr, n != nil
	case *ast.StateNode:
		return kindStateNode, n != nil
	case *ast.StateRegion:
		return kindStateRegion, n != nil
	case *ast.SubjectMember:
		return kindSubjectMember, n != nil
	case *ast.SubstateMember:
		return kindSubstateMember, n != nil
	case *ast.SuccessionEdge:
		return kindSuccessionEdge, n != nil
	case *ast.TerminateStatement:
		return kindTerminateStatement, n != nil
	case *ast.TextualRepresentation:
		return kindTextualRepresentation, n != nil
	case *ast.TimeEvent:
		return kindTimeEvent, n != nil
	case *ast.TransitionEdge:
		return kindTransitionEdge, n != nil
	case *ast.TransitionMember:
		return kindTransitionMember, n != nil
	case *ast.Usage:
		return kindUsage, n != nil
	case *ast.WhileLoopActionNode:
		return kindWhileLoopActionNode, n != nil
	}
	return numKinds, false
}

// alloc allocates count nodes of kind k in one block and appends them to out.
func alloc(k kind, count int, out []ast.Node) []ast.Node {
	switch k {
	case kindAcceptActionUsage:
		block := make([]ast.AcceptActionUsage, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindAcceptEvent:
		block := make([]ast.AcceptEvent, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindActionExecutionNode:
		block := make([]ast.ActionExecutionNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindAlias:
		block := make([]ast.Alias, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindAssignmentActionNode:
		block := make([]ast.AssignmentActionNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindAssumeMember:
		block := make([]ast.AssumeMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindBodyExpr:
		block := make([]ast.BodyExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindCallEvent:
		block := make([]ast.CallEvent, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindCastExpr:
		block := make([]ast.CastExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindChangeEvent:
		block := make([]ast.ChangeEvent, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindCollectExpr:
		block := make([]ast.CollectExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindComment:
		block := make([]ast.Comment, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindConnectorEnd:
		block := make([]ast.ConnectorEnd, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindConstraintMember:
		block := make([]ast.ConstraintMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindConstructorExpr:
		block := make([]ast.ConstructorExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindControlFlowEdge:
		block := make([]ast.ControlFlowEdge, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindCrossFeatureMember:
		block := make([]ast.CrossFeatureMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindDecisionNode:
		block := make([]ast.DecisionNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindDeferMember:
		block := make([]ast.DeferMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindDefinition:
		block := make([]ast.Definition, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindDependency:
		block := make([]ast.Dependency, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindDoMember:
		block := make([]ast.DoMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindDocumentation:
		block := make([]ast.Documentation, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindEntryMember:
		block := make([]ast.EntryMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindErrorNode:
		block := make([]ast.ErrorNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindExitMember:
		block := make([]ast.ExitMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindFeatureChainExpr:
		block := make([]ast.FeatureChainExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindFeatureReference:
		block := make([]ast.FeatureReference, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindFilterMember:
		block := make([]ast.FilterMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindFinalNode:
		block := make([]ast.FinalNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindFlowEnds:
		block := make([]ast.FlowEnds, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindForkNode:
		block := make([]ast.ForkNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindIfActionNode:
		block := make([]ast.IfActionNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindIfBranchNode:
		block := make([]ast.IfBranchNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindImport:
		block := make([]ast.Import, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindIndexExpr:
		block := make([]ast.IndexExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindInitialNode:
		block := make([]ast.InitialNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindInvocationExpr:
		block := make([]ast.InvocationExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindJoinNode:
		block := make([]ast.JoinNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindLiteralBool:
		block := make([]ast.LiteralBool, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindLiteralInfinity:
		block := make([]ast.LiteralInfinity, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindLiteralInteger:
		block := make([]ast.LiteralInteger, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindLiteralReal:
		block := make([]ast.LiteralReal, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindLiteralString:
		block := make([]ast.LiteralString, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindMembership:
		block := make([]ast.Membership, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindMergeNode:
		block := make([]ast.MergeNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindMetadataAccessExpr:
		block := make([]ast.MetadataAccessExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindMultiplicity:
		block := make([]ast.Multiplicity, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindMultiplicityDecl:
		block := make([]ast.MultiplicityDecl, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindNamespace:
		block := make([]ast.Namespace, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindNullExpr:
		block := make([]ast.NullExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindObjectFlowEdge:
		block := make([]ast.ObjectFlowEdge, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindOperatorExpr:
		block := make([]ast.OperatorExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindPackage:
		block := make([]ast.Package, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindPerformActionNode:
		block := make([]ast.PerformActionNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindPrefixMetadata:
		block := make([]ast.PrefixMetadata, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindPseudostateNode:
		block := make([]ast.PseudostateNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindQualifiedName:
		block := make([]ast.QualifiedName, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindRelationship:
		block := make([]ast.Relationship, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindRelationshipMember:
		block := make([]ast.RelationshipMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindRequireMember:
		block := make([]ast.RequireMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindRootNamespace:
		block := make([]ast.RootNamespace, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindSelectExpr:
		block := make([]ast.SelectExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindSendStatement:
		block := make([]ast.SendStatement, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindSequenceExpr:
		block := make([]ast.SequenceExpr, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindStateNode:
		block := make([]ast.StateNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindStateRegion:
		block := make([]ast.StateRegion, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindSubjectMember:
		block := make([]ast.SubjectMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindSubstateMember:
		block := make([]ast.SubstateMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindSuccessionEdge:
		block := make([]ast.SuccessionEdge, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindTerminateStatement:
		block := make([]ast.TerminateStatement, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindTextualRepresentation:
		block := make([]ast.TextualRepresentation, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindTimeEvent:
		block := make([]ast.TimeEvent, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindTransitionEdge:
		block := make([]ast.TransitionEdge, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindTransitionMember:
		block := make([]ast.TransitionMember, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindUsage:
		block := make([]ast.Usage, count)
		for i := range block {
			out = append(out, &block[i])
		}
	case kindWhileLoopActionNode:
		block := make([]ast.WhileLoopActionNode, count)
		for i := range block {
			out = append(out, &block[i])
		}
	}
	return out
}

// encodeFields writes every field of node after its kind has been recorded.
func (e *Encoder) encodeFields(node ast.Node) {
	switch n := node.(type) {
	case *ast.AcceptActionUsage:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.w.String(n.ParamName)
		e.node(n.ParamType)
	case *ast.AcceptEvent:
		e.base(&n.NodeBase)
		e.node(n.SignalType)
		e.node(n.Subsets)
		e.node(n.Payload)
	case *ast.ActionExecutionNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.node(n.ActionRef)
		e.node(n.Expression)
	case *ast.Alias:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Visibility))
		e.ident(n.Ident)
		e.node(n.For)
		e.nodes(n.Body)
		e.w.Bool(n.HasBody)
	case *ast.AssignmentActionNode:
		e.base(&n.NodeBase)
		e.node(n.Target)
		e.node(n.Value)
	case *ast.AssumeMember:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.node(n.Expression)
		e.node(n.Reference)
		e.nodes(n.Body)
		e.ident(n.Ident)
		e.span(n.DeclSpan)
		e.rels(n.Relationships)
		e.node(n.Multiplicity)
		e.node(n.Value)
		e.span(n.ValueOperatorSpan)
		e.w.Bool(n.ValueIsDefault)
		e.w.Bool(n.ValueIsInitial)
		e.w.Bool(n.HasBody)
	case *ast.BodyExpr:
		e.base(&n.NodeBase)
		e.params(n.Params)
		e.nodes(n.Members)
		e.node(n.Result)
	case *ast.CallEvent:
		e.base(&n.NodeBase)
		e.node(n.Operation)
		e.segments(n.Parameters)
	case *ast.CastExpr:
		e.base(&n.NodeBase)
		e.node(n.TargetType)
		e.node(n.Multiplicity)
	case *ast.ChangeEvent:
		e.base(&n.NodeBase)
		e.node(n.Condition)
	case *ast.CollectExpr:
		e.base(&n.NodeBase)
		e.node(n.Operand)
		e.node(n.Body)
	case *ast.Comment:
		e.base(&n.NodeBase)
		e.ident(n.Ident)
		e.qnames(n.About)
		e.w.String(n.Locale)
		e.span(n.BodySpan)
	case *ast.ConnectorEnd:
		e.base(&n.NodeBase)
		e.node(n.Target)
		e.node(n.Multiplicity)
		e.node(n.Reference)
		e.rels(n.Relationships)
	case *ast.ConstraintMember:
		e.base(&n.NodeBase)
		e.w.Bool(n.IsAssert)
		e.w.String(n.Keyword)
		e.w.Bool(n.IsNegated)
		e.node(n.Expression)
		e.w.String(n.Name)
		e.nodes(n.Body)
	case *ast.ConstructorExpr:
		e.base(&n.NodeBase)
		e.node(n.Type)
		e.nodes(n.Args)
		e.namedArgs(n.NamedArgs)
	case *ast.ControlFlowEdge:
		e.base(&n.NodeBase)
		e.node(n.Source)
		e.node(n.Target)
		e.node(n.Guard)
		e.w.Bool(n.IsElse)
		e.node(n.SourceMember)
		e.node(n.TargetMember)
		e.w.Bool(n.SourceImplied)
		e.w.Bool(n.TargetImplied)
	case *ast.CrossFeatureMember:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Direction))
		e.w.Bool(n.IsDerived)
		e.w.Bool(n.IsAbstract)
		e.w.Bool(n.IsVariation)
		e.w.Bool(n.IsComposite)
		e.w.Bool(n.IsPortion)
		e.w.Bool(n.IsVariable)
		e.w.Bool(n.IsConstant)
		e.w.Bool(n.IsReference)
		e.ident(n.Ident)
		e.node(n.Multiplicity)
		e.w.Bool(n.IsOrdered)
		e.w.Bool(n.IsNonunique)
		e.rels(n.Relationships)
	case *ast.DecisionNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.DeferMember:
		e.base(&n.NodeBase)
		e.nodes(n.Triggers)
	case *ast.Definition:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.w.Int(int64(n.Kind))
		e.w.String(n.Keyword)
		e.w.Bool(n.HasDefKeyword)
		e.w.Bool(n.IsAbstract)
		e.w.Bool(n.IsVariation)
		e.w.Bool(n.IsAll)
		e.w.Bool(n.IsConstant)
		e.w.Bool(n.IsEvent)
		e.w.Bool(n.IsIndividual)
		e.w.Bool(n.IsParallel)
		e.w.Int(int64(n.Visibility))
		e.ident(n.Ident)
		e.node(n.Multiplicity)
		e.rels(n.Relationships)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.Dependency:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.ident(n.Ident)
		e.qnames(n.Clients)
		e.qnames(n.Suppliers)
		e.nodes(n.Body)
		e.w.Bool(n.HasBody)
	case *ast.DoMember:
		e.base(&n.NodeBase)
		e.nodes(n.Actions)
	case *ast.Documentation:
		e.base(&n.NodeBase)
		e.ident(n.Ident)
		e.w.String(n.Locale)
		e.span(n.BodySpan)
	case *ast.EntryMember:
		e.base(&n.NodeBase)
		e.nodes(n.Actions)
	case *ast.ErrorNode:
		e.base(&n.NodeBase)
		e.w.String(n.Message)
	case *ast.ExitMember:
		e.base(&n.NodeBase)
		e.nodes(n.Actions)
	case *ast.FeatureChainExpr:
		e.base(&n.NodeBase)
		e.node(n.Operand)
		e.node(n.Member)
	case *ast.FeatureReference:
		e.base(&n.NodeBase)
		e.node(n.Name)
	case *ast.FilterMember:
		e.base(&n.NodeBase)
		e.node(n.Condition)
	case *ast.FinalNode:
		e.base(&n.NodeBase)
	case *ast.FlowEnds:
		e.base(&n.NodeBase)
		e.node(n.From)
		e.node(n.To)
		e.node(n.Payload)
		e.node(n.PayloadDecl)
		e.node(n.PayloadMultiplicity)
	case *ast.ForkNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.IfActionNode:
		e.base(&n.NodeBase)
		e.node(n.Condition)
		e.node(n.Then)
		e.node(n.Else)
	case *ast.IfBranchNode:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Kind))
		e.nodes(n.Body)
	case *ast.Import:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Visibility))
		e.w.Bool(n.IsAll)
		e.w.Int(int64(n.Kind))
		e.node(n.Imported)
		e.w.Bool(n.IsRecursive)
		e.node(n.FilterExpr)
		e.nodes(n.Body)
		e.w.Bool(n.HasBody)
		e.w.Bool(n.IsExpose)
	case *ast.IndexExpr:
		e.base(&n.NodeBase)
		e.node(n.Operand)
		e.node(n.Index)
		e.w.Bool(n.Bracket)
	case *ast.InitialNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
		e.node(n.Successor)
		e.node(n.Guard)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.InvocationExpr:
		e.base(&n.NodeBase)
		e.node(n.Operand)
		e.node(n.Type)
		e.nodes(n.Args)
		e.namedArgs(n.NamedArgs)
	case *ast.JoinNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.LiteralBool:
		e.base(&n.NodeBase)
		e.w.Bool(n.Value)
	case *ast.LiteralInfinity:
		e.base(&n.NodeBase)
	case *ast.LiteralInteger:
		e.base(&n.NodeBase)
		e.w.String(n.Value)
	case *ast.LiteralReal:
		e.base(&n.NodeBase)
		e.w.String(n.Value)
	case *ast.LiteralString:
		e.base(&n.NodeBase)
		e.w.String(n.Value)
	case *ast.Membership:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Visibility))
		e.w.Bool(n.IsTypeFeature)
		e.node(n.Member)
	case *ast.MergeNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.MetadataAccessExpr:
		e.base(&n.NodeBase)
		e.node(n.Ref)
	case *ast.Multiplicity:
		e.base(&n.NodeBase)
		e.node(n.Lower)
		e.node(n.Upper)
		e.w.Bool(n.IsRange)
	case *ast.MultiplicityDecl:
		e.base(&n.NodeBase)
		e.ident(n.Ident)
		e.node(n.Range)
		e.node(n.Subsets)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.Namespace:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.ident(n.Ident)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.NullExpr:
		e.base(&n.NodeBase)
	case *ast.ObjectFlowEdge:
		e.base(&n.NodeBase)
		e.node(n.Source)
		e.node(n.Target)
	case *ast.OperatorExpr:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Operator))
		e.nodes(n.Operands)
		e.node(n.TypeRef)
	case *ast.Package:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.ident(n.Ident)
		e.w.Bool(n.IsLibrary)
		e.w.Bool(n.IsStandard)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.PerformActionNode:
		e.base(&n.NodeBase)
		e.node(n.ActionRef)
	case *ast.PrefixMetadata:
		e.base(&n.NodeBase)
		e.ident(n.Ident)
		e.node(n.Type)
		e.nodes(n.Body)
		e.w.Bool(n.HasBody)
		e.qnames(n.About)
	case *ast.PseudostateNode:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Kind))
		e.w.String(n.Name)
		e.w.String(n.Keyword)
	case *ast.QualifiedName:
		e.base(&n.NodeBase)
		e.w.Bool(n.Global)
		e.segments(n.Parts)
	case *ast.Relationship:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Kind))
		e.node(n.Target)
		e.w.Bool(n.Conjugated)
		e.node(n.Multiplicity)
	case *ast.RelationshipMember:
		e.base(&n.NodeBase)
		e.ident(n.Ident)
		e.w.Int(int64(n.Kind))
		e.w.String(n.Keyword)
		e.w.String(n.PrefixKeyword)
		e.node(n.Source)
		e.node(n.Target)
		e.w.Bool(n.Conjugated)
		e.w.Int(int64(n.Visibility))
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.RequireMember:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.node(n.Expression)
		e.node(n.Reference)
		e.nodes(n.Body)
		e.ident(n.Ident)
		e.span(n.DeclSpan)
		e.rels(n.Relationships)
		e.node(n.Multiplicity)
		e.node(n.Value)
		e.span(n.ValueOperatorSpan)
		e.w.Bool(n.ValueIsDefault)
		e.w.Bool(n.ValueIsInitial)
		e.w.Bool(n.HasBody)
	case *ast.RootNamespace:
		e.base(&n.NodeBase)
		e.nodes(n.Members)
	case *ast.SelectExpr:
		e.base(&n.NodeBase)
		e.node(n.Operand)
		e.node(n.Body)
	case *ast.SendStatement:
		e.base(&n.NodeBase)
		e.node(n.Message)
		e.node(n.Target)
		e.w.Bool(n.IsVia)
		e.node(n.Receiver)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.SequenceExpr:
		e.base(&n.NodeBase)
		e.nodes(n.Elements)
	case *ast.StateNode:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.nodes(n.Entry)
		e.nodes(n.Do)
		e.nodes(n.Exit)
		e.nodes(n.Defer)
		e.nodes(n.Substates)
		e.regions(n.Regions)
	case *ast.StateRegion:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.nodes(n.States)
	case *ast.SubjectMember:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.ident(n.Ident)
		e.node(n.TypeRef)
		e.node(n.Multiplicity)
		e.rels(n.Relationships)
		e.nodes(n.Body)
		e.node(n.BindingExpr)
		e.span(n.ValueOperatorSpan)
		e.w.Bool(n.ValueIsDefault)
		e.w.Bool(n.ValueIsInitial)
		e.w.Bool(n.HasBody)
	case *ast.SubstateMember:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
	case *ast.SuccessionEdge:
		e.base(&n.NodeBase)
		e.node(n.Source)
		e.node(n.Target)
		e.node(n.SourceMember)
		e.node(n.TargetMember)
		e.w.Bool(n.SourceImplied)
		e.w.Bool(n.TargetImplied)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
	case *ast.TerminateStatement:
		e.base(&n.NodeBase)
		e.node(n.Target)
	case *ast.TextualRepresentation:
		e.base(&n.NodeBase)
		e.ident(n.Ident)
		e.w.String(n.Language)
		e.span(n.BodySpan)
	case *ast.TimeEvent:
		e.base(&n.NodeBase)
		e.node(n.Duration)
		e.w.Bool(n.Absolute)
	case *ast.TransitionEdge:
		e.base(&n.NodeBase)
		e.node(n.Source)
		e.node(n.Target)
		e.node(n.Trigger)
		e.node(n.Guard)
		e.nodes(n.Effect)
	case *ast.TransitionMember:
		e.base(&n.NodeBase)
		e.w.String(n.Name)
		e.span(n.NameSpan)
		e.node(n.Source)
		e.node(n.Target)
		e.node(n.Trigger)
		e.span(n.TriggerSpan)
		e.node(n.Guard)
		e.nodes(n.Effect)
		e.w.Bool(n.HasEffect)
		e.node(n.Via)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
		e.w.Bool(n.IsSuccession)
	case *ast.Usage:
		e.base(&n.NodeBase)
		e.prefixes(n.Prefixes)
		e.w.Int(int64(n.Kind))
		e.w.String(n.Keyword)
		e.w.String(n.PrefixKeyword)
		e.w.Bool(n.IsAbstract)
		e.w.Bool(n.IsVariation)
		e.w.Bool(n.IsVariant)
		e.w.Bool(n.IsReference)
		e.w.Bool(n.IsVariable)
		e.w.Bool(n.IsAll)
		e.w.Bool(n.IsEnd)
		e.w.Bool(n.IsChain)
		e.w.Bool(n.IsConstant)
		e.w.Bool(n.IsEvent)
		e.w.Bool(n.IsIndividual)
		e.w.Int(int64(n.Portion))
		e.w.Bool(n.IsAccept)
		e.w.Bool(n.IsBodyParameter)
		e.w.Bool(n.IsActionNode)
		e.w.Bool(n.IsResult)
		e.w.Bool(n.IsNegated)
		e.w.Bool(n.DeclaresRequirement)
		e.w.Int(int64(n.Visibility))
		e.w.Int(int64(n.Direction))
		e.w.Bool(n.IsComposite)
		e.w.Bool(n.IsPortion)
		e.w.Bool(n.IsParallel)
		e.w.Bool(n.IsDerived)
		e.w.Bool(n.IsOrdered)
		e.w.Bool(n.IsNonunique)
		e.ident(n.Ident)
		e.rels(n.Relationships)
		e.node(n.Multiplicity)
		e.node(n.CrossFeature)
		e.node(n.Value)
		e.span(n.ValueOperatorSpan)
		e.w.Bool(n.ValueIsDefault)
		e.w.Bool(n.ValueIsInitial)
		e.nodes(n.Members)
		e.w.Bool(n.HasBody)
		e.ends(n.ConnectorEnds)
		e.node(n.FlowEnds)
	case *ast.WhileLoopActionNode:
		e.base(&n.NodeBase)
		e.w.Int(int64(n.Kind))
		e.node(n.Condition)
		e.nodes(n.Body)
		e.node(n.Until)
		e.ident(n.Variable)
		e.rels(n.VariableRelationships)
		e.node(n.Collection)
	}
}

// decodeFields reads the fields of node, in the order encodeFields wrote them.
func (d *Decoder) decodeFields(node ast.Node) {
	switch n := node.(type) {
	case *ast.AcceptActionUsage:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.ParamName = d.r.String()
		n.ParamType = typed[*ast.QualifiedName](d)
	case *ast.AcceptEvent:
		d.base(&n.NodeBase)
		n.SignalType = typed[*ast.QualifiedName](d)
		n.Subsets = typed[*ast.QualifiedName](d)
		n.Payload = typed[*ast.Usage](d)
	case *ast.ActionExecutionNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.ActionRef = typed[*ast.QualifiedName](d)
		n.Expression = d.node()
	case *ast.Alias:
		d.base(&n.NodeBase)
		n.Visibility = ast.Visibility(d.r.Int())
		n.Ident = d.ident()
		n.For = typed[*ast.QualifiedName](d)
		n.Body = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.AssignmentActionNode:
		d.base(&n.NodeBase)
		n.Target = d.node()
		n.Value = d.node()
	case *ast.AssumeMember:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Expression = d.node()
		n.Reference = typed[*ast.QualifiedName](d)
		n.Body = d.nodes()
		n.Ident = d.ident()
		n.DeclSpan = d.span()
		n.Relationships = d.rels()
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.Value = d.node()
		n.ValueOperatorSpan = d.span()
		n.ValueIsDefault = d.r.Bool()
		n.ValueIsInitial = d.r.Bool()
		n.HasBody = d.r.Bool()
	case *ast.BodyExpr:
		d.base(&n.NodeBase)
		n.Params = d.params()
		n.Members = d.nodes()
		n.Result = d.node()
	case *ast.CallEvent:
		d.base(&n.NodeBase)
		n.Operation = typed[*ast.QualifiedName](d)
		n.Parameters = d.segments()
	case *ast.CastExpr:
		d.base(&n.NodeBase)
		n.TargetType = typed[*ast.QualifiedName](d)
		n.Multiplicity = typed[*ast.Multiplicity](d)
	case *ast.ChangeEvent:
		d.base(&n.NodeBase)
		n.Condition = d.node()
	case *ast.CollectExpr:
		d.base(&n.NodeBase)
		n.Operand = d.node()
		n.Body = d.node()
	case *ast.Comment:
		d.base(&n.NodeBase)
		n.Ident = d.ident()
		n.About = d.qnames()
		n.Locale = d.r.String()
		n.BodySpan = d.span()
	case *ast.ConnectorEnd:
		d.base(&n.NodeBase)
		n.Target = d.node()
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.Reference = d.node()
		n.Relationships = d.rels()
	case *ast.ConstraintMember:
		d.base(&n.NodeBase)
		n.IsAssert = d.r.Bool()
		n.Keyword = d.r.String()
		n.IsNegated = d.r.Bool()
		n.Expression = d.node()
		n.Name = d.r.String()
		n.Body = d.nodes()
	case *ast.ConstructorExpr:
		d.base(&n.NodeBase)
		n.Type = typed[*ast.QualifiedName](d)
		n.Args = d.nodes()
		n.NamedArgs = d.namedArgs()
	case *ast.ControlFlowEdge:
		d.base(&n.NodeBase)
		n.Source = typed[*ast.QualifiedName](d)
		n.Target = typed[*ast.QualifiedName](d)
		n.Guard = d.node()
		n.IsElse = d.r.Bool()
		n.SourceMember = d.node()
		n.TargetMember = d.node()
		n.SourceImplied = d.r.Bool()
		n.TargetImplied = d.r.Bool()
	case *ast.CrossFeatureMember:
		d.base(&n.NodeBase)
		n.Direction = ast.FeatureDirection(d.r.Int())
		n.IsDerived = d.r.Bool()
		n.IsAbstract = d.r.Bool()
		n.IsVariation = d.r.Bool()
		n.IsComposite = d.r.Bool()
		n.IsPortion = d.r.Bool()
		n.IsVariable = d.r.Bool()
		n.IsConstant = d.r.Bool()
		n.IsReference = d.r.Bool()
		n.Ident = d.ident()
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.IsOrdered = d.r.Bool()
		n.IsNonunique = d.r.Bool()
		n.Relationships = d.rels()
	case *ast.DecisionNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.DeferMember:
		d.base(&n.NodeBase)
		n.Triggers = d.nodes()
	case *ast.Definition:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Kind = ast.DefinitionKind(d.r.Int())
		n.Keyword = d.r.String()
		n.HasDefKeyword = d.r.Bool()
		n.IsAbstract = d.r.Bool()
		n.IsVariation = d.r.Bool()
		n.IsAll = d.r.Bool()
		n.IsConstant = d.r.Bool()
		n.IsEvent = d.r.Bool()
		n.IsIndividual = d.r.Bool()
		n.IsParallel = d.r.Bool()
		n.Visibility = ast.Visibility(d.r.Int())
		n.Ident = d.ident()
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.Relationships = d.rels()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.Dependency:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Ident = d.ident()
		n.Clients = d.qnames()
		n.Suppliers = d.qnames()
		n.Body = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.DoMember:
		d.base(&n.NodeBase)
		n.Actions = d.nodes()
	case *ast.Documentation:
		d.base(&n.NodeBase)
		n.Ident = d.ident()
		n.Locale = d.r.String()
		n.BodySpan = d.span()
	case *ast.EntryMember:
		d.base(&n.NodeBase)
		n.Actions = d.nodes()
	case *ast.ErrorNode:
		d.base(&n.NodeBase)
		n.Message = d.r.String()
	case *ast.ExitMember:
		d.base(&n.NodeBase)
		n.Actions = d.nodes()
	case *ast.FeatureChainExpr:
		d.base(&n.NodeBase)
		n.Operand = d.node()
		n.Member = typed[*ast.QualifiedName](d)
	case *ast.FeatureReference:
		d.base(&n.NodeBase)
		n.Name = typed[*ast.QualifiedName](d)
	case *ast.FilterMember:
		d.base(&n.NodeBase)
		n.Condition = d.node()
	case *ast.FinalNode:
		d.base(&n.NodeBase)
	case *ast.FlowEnds:
		d.base(&n.NodeBase)
		n.From = d.node()
		n.To = d.node()
		n.Payload = d.node()
		n.PayloadDecl = typed[*ast.Usage](d)
		n.PayloadMultiplicity = typed[*ast.Multiplicity](d)
	case *ast.ForkNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.IfActionNode:
		d.base(&n.NodeBase)
		n.Condition = d.node()
		n.Then = typed[*ast.IfBranchNode](d)
		n.Else = typed[*ast.IfBranchNode](d)
	case *ast.IfBranchNode:
		d.base(&n.NodeBase)
		n.Kind = ast.IfBranchKind(d.r.Int())
		n.Body = d.nodes()
	case *ast.Import:
		d.base(&n.NodeBase)
		n.Visibility = ast.Visibility(d.r.Int())
		n.IsAll = d.r.Bool()
		n.Kind = ast.ImportKind(d.r.Int())
		n.Imported = typed[*ast.QualifiedName](d)
		n.IsRecursive = d.r.Bool()
		n.FilterExpr = d.node()
		n.Body = d.nodes()
		n.HasBody = d.r.Bool()
		n.IsExpose = d.r.Bool()
	case *ast.IndexExpr:
		d.base(&n.NodeBase)
		n.Operand = d.node()
		n.Index = d.node()
		n.Bracket = d.r.Bool()
	case *ast.InitialNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
		n.Successor = typed[*ast.QualifiedName](d)
		n.Guard = d.node()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.InvocationExpr:
		d.base(&n.NodeBase)
		n.Operand = d.node()
		n.Type = typed[*ast.QualifiedName](d)
		n.Args = d.nodes()
		n.NamedArgs = d.namedArgs()
	case *ast.JoinNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.LiteralBool:
		d.base(&n.NodeBase)
		n.Value = d.r.Bool()
	case *ast.LiteralInfinity:
		d.base(&n.NodeBase)
	case *ast.LiteralInteger:
		d.base(&n.NodeBase)
		n.Value = d.r.String()
	case *ast.LiteralReal:
		d.base(&n.NodeBase)
		n.Value = d.r.String()
	case *ast.LiteralString:
		d.base(&n.NodeBase)
		n.Value = d.r.String()
	case *ast.Membership:
		d.base(&n.NodeBase)
		n.Visibility = ast.Visibility(d.r.Int())
		n.IsTypeFeature = d.r.Bool()
		n.Member = d.node()
	case *ast.MergeNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.MetadataAccessExpr:
		d.base(&n.NodeBase)
		n.Ref = typed[*ast.QualifiedName](d)
	case *ast.Multiplicity:
		d.base(&n.NodeBase)
		n.Lower = d.node()
		n.Upper = d.node()
		n.IsRange = d.r.Bool()
	case *ast.MultiplicityDecl:
		d.base(&n.NodeBase)
		n.Ident = d.ident()
		n.Range = typed[*ast.Multiplicity](d)
		n.Subsets = typed[*ast.QualifiedName](d)
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.Namespace:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Ident = d.ident()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.NullExpr:
		d.base(&n.NodeBase)
	case *ast.ObjectFlowEdge:
		d.base(&n.NodeBase)
		n.Source = typed[*ast.QualifiedName](d)
		n.Target = typed[*ast.QualifiedName](d)
	case *ast.OperatorExpr:
		d.base(&n.NodeBase)
		n.Operator = ast.OperatorKind(d.r.Int())
		n.Operands = d.nodes()
		n.TypeRef = typed[*ast.QualifiedName](d)
	case *ast.Package:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Ident = d.ident()
		n.IsLibrary = d.r.Bool()
		n.IsStandard = d.r.Bool()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.PerformActionNode:
		d.base(&n.NodeBase)
		n.ActionRef = d.node()
	case *ast.PrefixMetadata:
		d.base(&n.NodeBase)
		n.Ident = d.ident()
		n.Type = typed[*ast.QualifiedName](d)
		n.Body = d.nodes()
		n.HasBody = d.r.Bool()
		n.About = d.qnames()
	case *ast.PseudostateNode:
		d.base(&n.NodeBase)
		n.Kind = ast.PseudostateKind(d.r.Int())
		n.Name = d.r.String()
		n.Keyword = d.r.String()
	case *ast.QualifiedName:
		d.base(&n.NodeBase)
		n.Global = d.r.Bool()
		d.parts(n)
	case *ast.Relationship:
		d.base(&n.NodeBase)
		n.Kind = ast.RelationshipKind(d.r.Int())
		n.Target = d.node()
		n.Conjugated = d.r.Bool()
		n.Multiplicity = typed[*ast.Multiplicity](d)
	case *ast.RelationshipMember:
		d.base(&n.NodeBase)
		n.Ident = d.ident()
		n.Kind = ast.RelationshipKind(d.r.Int())
		n.Keyword = d.r.String()
		n.PrefixKeyword = d.r.String()
		n.Source = d.node()
		n.Target = d.node()
		n.Conjugated = d.r.Bool()
		n.Visibility = ast.Visibility(d.r.Int())
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.RequireMember:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Expression = d.node()
		n.Reference = typed[*ast.QualifiedName](d)
		n.Body = d.nodes()
		n.Ident = d.ident()
		n.DeclSpan = d.span()
		n.Relationships = d.rels()
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.Value = d.node()
		n.ValueOperatorSpan = d.span()
		n.ValueIsDefault = d.r.Bool()
		n.ValueIsInitial = d.r.Bool()
		n.HasBody = d.r.Bool()
	case *ast.RootNamespace:
		d.base(&n.NodeBase)
		n.Members = d.nodes()
	case *ast.SelectExpr:
		d.base(&n.NodeBase)
		n.Operand = d.node()
		n.Body = d.node()
	case *ast.SendStatement:
		d.base(&n.NodeBase)
		n.Message = d.node()
		n.Target = d.node()
		n.IsVia = d.r.Bool()
		n.Receiver = d.node()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.SequenceExpr:
		d.base(&n.NodeBase)
		n.Elements = d.nodes()
	case *ast.StateNode:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.Entry = d.nodes()
		n.Do = d.nodes()
		n.Exit = d.nodes()
		n.Defer = d.nodes()
		n.Substates = d.nodes()
		n.Regions = d.regions()
	case *ast.StateRegion:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.States = d.nodes()
	case *ast.SubjectMember:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Ident = d.ident()
		n.TypeRef = typed[*ast.QualifiedName](d)
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.Relationships = d.rels()
		n.Body = d.nodes()
		n.BindingExpr = d.node()
		n.ValueOperatorSpan = d.span()
		n.ValueIsDefault = d.r.Bool()
		n.ValueIsInitial = d.r.Bool()
		n.HasBody = d.r.Bool()
	case *ast.SubstateMember:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
	case *ast.SuccessionEdge:
		d.base(&n.NodeBase)
		n.Source = typed[*ast.QualifiedName](d)
		n.Target = typed[*ast.QualifiedName](d)
		n.SourceMember = d.node()
		n.TargetMember = d.node()
		n.SourceImplied = d.r.Bool()
		n.TargetImplied = d.r.Bool()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
	case *ast.TerminateStatement:
		d.base(&n.NodeBase)
		n.Target = d.node()
	case *ast.TextualRepresentation:
		d.base(&n.NodeBase)
		n.Ident = d.ident()
		n.Language = d.r.String()
		n.BodySpan = d.span()
	case *ast.TimeEvent:
		d.base(&n.NodeBase)
		n.Duration = d.node()
		n.Absolute = d.r.Bool()
	case *ast.TransitionEdge:
		d.base(&n.NodeBase)
		n.Source = typed[*ast.QualifiedName](d)
		n.Target = typed[*ast.QualifiedName](d)
		n.Trigger = typed[ast.TriggerEvent](d)
		n.Guard = d.node()
		n.Effect = d.nodes()
	case *ast.TransitionMember:
		d.base(&n.NodeBase)
		n.Name = d.r.String()
		n.NameSpan = d.span()
		n.Source = typed[*ast.QualifiedName](d)
		n.Target = typed[*ast.QualifiedName](d)
		n.Trigger = d.node()
		n.TriggerSpan = d.span()
		n.Guard = d.node()
		n.Effect = d.nodes()
		n.HasEffect = d.r.Bool()
		n.Via = typed[*ast.QualifiedName](d)
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
		n.IsSuccession = d.r.Bool()
	case *ast.Usage:
		d.base(&n.NodeBase)
		n.Prefixes = d.prefixes()
		n.Kind = ast.UsageKind(d.r.Int())
		n.Keyword = d.r.String()
		n.PrefixKeyword = d.r.String()
		n.IsAbstract = d.r.Bool()
		n.IsVariation = d.r.Bool()
		n.IsVariant = d.r.Bool()
		n.IsReference = d.r.Bool()
		n.IsVariable = d.r.Bool()
		n.IsAll = d.r.Bool()
		n.IsEnd = d.r.Bool()
		n.IsChain = d.r.Bool()
		n.IsConstant = d.r.Bool()
		n.IsEvent = d.r.Bool()
		n.IsIndividual = d.r.Bool()
		n.Portion = ast.PortionKind(d.r.Int())
		n.IsAccept = d.r.Bool()
		n.IsBodyParameter = d.r.Bool()
		n.IsActionNode = d.r.Bool()
		n.IsResult = d.r.Bool()
		n.IsNegated = d.r.Bool()
		n.DeclaresRequirement = d.r.Bool()
		n.Visibility = ast.Visibility(d.r.Int())
		n.Direction = ast.FeatureDirection(d.r.Int())
		n.IsComposite = d.r.Bool()
		n.IsPortion = d.r.Bool()
		n.IsParallel = d.r.Bool()
		n.IsDerived = d.r.Bool()
		n.IsOrdered = d.r.Bool()
		n.IsNonunique = d.r.Bool()
		n.Ident = d.ident()
		n.Relationships = d.rels()
		n.Multiplicity = typed[*ast.Multiplicity](d)
		n.CrossFeature = typed[*ast.CrossFeatureMember](d)
		n.Value = d.node()
		n.ValueOperatorSpan = d.span()
		n.ValueIsDefault = d.r.Bool()
		n.ValueIsInitial = d.r.Bool()
		n.Members = d.nodes()
		n.HasBody = d.r.Bool()
		n.ConnectorEnds = d.ends()
		n.FlowEnds = typed[*ast.FlowEnds](d)
	case *ast.WhileLoopActionNode:
		d.base(&n.NodeBase)
		n.Kind = ast.LoopKind(d.r.Int())
		n.Condition = d.node()
		n.Body = d.nodes()
		n.Until = d.node()
		n.Variable = d.ident()
		n.VariableRelationships = d.rels()
		n.Collection = d.node()
	}
}
