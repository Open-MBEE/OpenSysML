package naming

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TriggerSegment is the segment under which a transition's trigger action owns
// its payload parameter.
const TriggerSegment = "@trigger"

// UnwrapMember returns the declaration inside a membership wrapper together
// with the visibility the wrapper declared.
func UnwrapMember(member ast.Node) (ast.Node, ast.Visibility) {
	switch n := member.(type) {
	case *ast.Membership:
		if n.Member == nil {
			return nil, n.Visibility
		}
		return n.Member, n.Visibility
	case *ast.Import:
		return n, n.Visibility
	case *ast.Alias:
		return n, n.Visibility
	case *ast.RelationshipMember:
		return n, n.Visibility
	}
	return member, ast.VisibilityDefault
}

// ShorthandRelationship reports a `bind` shorthand, which declares no name.
func ShorthandRelationship(n *ast.Usage) bool {
	return n.Kind == ast.UsageBinding && n.Keyword == "bind"
}

// BodyMembers returns the members written in a usage's body: a payload declared
// inline by `of name : Type` is parsed as a member but belongs to the head.
func BodyMembers(n *ast.Usage) []ast.Node {
	if n.FlowEnds == nil || n.FlowEnds.PayloadDecl == nil {
		return n.Members
	}
	members := make([]ast.Node, 0, len(n.Members))
	for _, m := range n.Members {
		if m != ast.Node(n.FlowEnds.PayloadDecl) {
			members = append(members, m)
		}
	}
	return members
}

// DeclaredPrefixes returns the `#M` prefix annotations a declaration carries.
func DeclaredPrefixes(node ast.Node) []*ast.PrefixMetadata {
	switch n := node.(type) {
	case *ast.Package:
		return n.Prefixes
	case *ast.Namespace:
		return n.Prefixes
	case *ast.Dependency:
		return n.Prefixes
	case *ast.Definition:
		return n.Prefixes
	case *ast.Usage:
		return n.Prefixes
	case *ast.SubjectMember:
		return n.Prefixes
	case *ast.AssumeMember:
		return n.Prefixes
	case *ast.RequireMember:
		return n.Prefixes
	}
	return nil
}

// DeclaredNameAndMembers returns the name a declaration introduces and the
// members it owns, for the node kinds that have either.
func DeclaredNameAndMembers(node ast.Node) (string, []ast.Node) {
	if name, members, ok := BehaviorNameAndMembers(node); ok {
		return name, members
	}
	switch n := node.(type) {
	case *ast.Package:
		return n.Ident.Name, n.Members
	case *ast.Namespace:
		return n.Ident.Name, n.Members
	case *ast.Definition:
		return n.Ident.Name, n.Members
	case *ast.Usage:
		if ShorthandRelationship(n) {
			return "", BodyMembers(n)
		}
		// A named send node is the statement it is written as, whose body
		// declares the node's own members.
		if send, ok := NodeStatement(n).(*ast.SendStatement); ok {
			return n.Ident.Name, send.Members
		}
		return n.Ident.Name, BodyMembers(n)
	case *ast.Import:
		return "", n.Body
	case *ast.Alias:
		return n.Ident.Name, n.Body
	case *ast.Dependency:
		return n.Ident.Name, n.Body
	case *ast.RelationshipMember:
		return n.Ident.Name, n.Members
	case *ast.MultiplicityDecl:
		return n.Ident.Name, n.Members
	case *ast.ConstraintMember:
		return n.Name, n.Body
	case *ast.AssumeMember:
		return n.Ident.Name, n.Body
	case *ast.RequireMember:
		return n.Ident.Name, n.Body
	case *ast.SubjectMember:
		return n.Ident.Name, n.Body
	case *ast.PrefixMetadata:
		return n.Ident.Name, n.Body
	case *ast.Comment:
		return n.Ident.Name, nil
	case *ast.Documentation:
		return n.Ident.Name, nil
	case *ast.TextualRepresentation:
		return n.Ident.Name, nil
	}
	return "", nil
}

// NodeStatement is the statement an action node is written as, which the node
// is (`action s send x to r;` is one SendActionUsage named s, SysML.xtext
// SendNode; `action a assign x := y;` one AssignmentActionUsage,
// AssignmentNode), or nil for any other usage: a node chaining further
// statements with `then`, or one whose declaration says more than its
// visibility and name (`#M action a : A assign …`), which the statement's
// notation cannot write back and so keeps the action it is declared as.
func NodeStatement(n *ast.Usage) ast.Node {
	if n.Kind != ast.UsageAction || !n.IsActionNode || len(n.Members) != 1 || !PlainNodeHead(n) {
		return nil
	}
	statement := n.Members[0]
	if m, ok := statement.(*ast.Membership); ok {
		statement = m.Member
	}
	switch s := statement.(type) {
	case *ast.SendStatement:
		return s
	case *ast.AssignmentActionNode, *ast.IfActionNode, *ast.WhileLoopActionNode:
		return s
	}
	return nil
}

// PlainNodeHead reports whether an action node declares nothing but its visibility
// and name: no prefix, modifier, specialization, multiplicity or value.
func PlainNodeHead(n *ast.Usage) bool {
	return len(n.Prefixes) == 0 && n.PrefixKeyword == "" && len(n.Relationships) == 0 &&
		n.Multiplicity == nil && n.CrossFeature == nil && n.Value == nil &&
		!n.IsAbstract && !n.IsVariation && !n.IsVariant && !n.IsReference && !n.IsVariable &&
		!n.IsIndividual && n.Portion == ast.PortionNone && !n.IsPortion && n.Direction == ast.DirNone &&
		!n.IsEnd && !n.IsConstant && !n.IsDerived && !n.IsOrdered && !n.IsNonunique && !n.IsParallel
}

// BehaviorNameAndMembers returns the name a behavioral node declares and the
// members it owns, for the nodes DeclaredNameAndMembers does not cover itself.
func BehaviorNameAndMembers(node ast.Node) (string, []ast.Node, bool) {
	switch n := node.(type) {
	case *ast.InitialNode:
		// Its name references the starting member, so it declares none.
		return "", n.Members, true
	case *ast.SuccessionEdge:
		return "", n.Members, true
	case *ast.FinalNode:
		return "", nil, true
	case *ast.ForkNode:
		return n.Name, nil, true
	case *ast.JoinNode:
		return n.Name, nil, true
	case *ast.MergeNode:
		return n.Name, nil, true
	case *ast.DecisionNode:
		return n.Name, nil, true
	case *ast.ActionExecutionNode:
		return n.Name, nil, true
	case *ast.WhileLoopActionNode:
		return "", n.Body, true
	case *ast.IfActionNode:
		branches := make([]ast.Node, 0, 2)
		for _, branch := range n.Branches() {
			branches = append(branches, branch)
		}
		return "", branches, true
	case *ast.IfBranchNode:
		return "", n.Body, true
	case *ast.StateNode:
		return n.Name, StateBody(n), true
	case *ast.SubstateMember:
		return n.Name, nil, true
	case *ast.PseudostateNode:
		return n.Name, nil, true
	case *ast.EntryMember:
		return "", n.Actions, true
	case *ast.DoMember:
		return "", n.Actions, true
	case *ast.ExitMember:
		return "", n.Actions, true
	case *ast.TransitionMember:
		return n.Name, TransitionMembers(n), true
	}
	return "", nil, false
}

// StateBody lists what a state owns in written order: its entry, do and exit
// actions, its substates and its regions.
func StateBody(n *ast.StateNode) []ast.Node {
	members := make([]ast.Node, 0,
		len(n.Entry)+len(n.Do)+len(n.Exit)+len(n.Substates)+len(n.Regions))
	for _, bucket := range [][]ast.Node{n.Entry, n.Do, n.Exit, n.Substates} {
		members = append(members, bucket...)
	}
	for _, region := range n.Regions {
		members = append(members, region)
	}
	sort.SliceStable(members, func(i, j int) bool {
		return members[i].Span().Offset < members[j].Span().Offset
	})
	return members
}

// TransitionMembers lists what a transition owns in written order: the actions
// of its `do` effect, then the members of its body.
func TransitionMembers(n *ast.TransitionMember) []ast.Node {
	if len(n.Effect) == 0 {
		return n.Members
	}
	members := make([]ast.Node, 0, len(n.Effect)+len(n.Members))
	members = append(members, n.Effect...)
	return append(members, n.Members...)
}

// MemberName is a member's declared name, else the one its naming feature lends
// it (KerML Feature::effectiveName): `render tree;` is tree, as the symbol table has it.
func MemberName(node ast.Node) string {
	if name, _ := DeclaredNameAndMembers(node); name != "" {
		return name
	}
	if rel := ast.DeclNamingFeature(node); rel != nil {
		name, _ := ast.TargetName(rel.Target)
		return name
	}
	return ""
}

// ParamName is the name a parameter declaration introduces.
func ParamName(node ast.Node) string {
	name, _ := DeclaredNameAndMembers(node)
	return name
}

// WithoutComments replaces every comment with a space, told apart by the lexer
// the parser reads them with, so each shape scanned over text excludes them.
func WithoutComments(text string) string {
	var kept strings.Builder
	lx := lexer.New(source.New("head.sysml", []byte(text)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		switch tok.Kind {
		case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			kept.WriteByte(' ')
		default:
			kept.WriteString(text[tok.Span.Offset:tok.Span.End()])
		}
	}
	return kept.String()
}

// Before returns the text of file written between the start of node and the
// start of inner, trimmed.
func Before(file *source.SourceFile, node, inner ast.Node) string {
	start, end := node.Span().Offset, inner.Span().Offset
	if end <= start {
		return ""
	}
	return strings.TrimSpace(file.Text(source.Span{Offset: start, Len: end - start}))
}

// Introducer returns the keyword written immediately before inner, which is
// what the clause inner belongs to was introduced with.
func Introducer(file *source.SourceFile, node, inner ast.Node) string {
	fields := strings.Fields(WithoutComments(Before(file, node, inner)))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// TriggerPayload is the payload parameter a transition's trigger declares, nil
// for a trigger with no AcceptActionUsage form (a call event, `when <name>`).
func TriggerPayload(file *source.SourceFile, n *ast.TransitionMember) *ast.Usage {
	if n.Trigger == nil {
		return nil
	}
	valued := func(event ast.Node, span source.Span) *ast.Usage {
		payload := &ast.Usage{Kind: ast.UsageAttribute, IsReference: true, Direction: ast.DirOut, IsAccept: true, Value: event}
		payload.NodeSpan = span
		return payload
	}
	switch Introducer(file, n, n.Trigger) {
	case "accept":
		switch t := n.Trigger.(type) {
		case *ast.Usage:
			if t.IsAccept {
				return t
			}
		case *ast.TimeEvent, *ast.ChangeEvent:
			return valued(t, t.Span())
		}
	case "when":
		switch n.Trigger.(type) {
		case *ast.FeatureReference, *ast.QualifiedName, *ast.ErrorNode:
			return nil
		}
		change := &ast.ChangeEvent{Condition: n.Trigger}
		change.NodeSpan = n.TriggerSpan
		return valued(change, n.TriggerSpan)
	}
	return nil
}
