package export

// The behavioral members an action or state body declares: control nodes,
// statements, loops, conditionals, states, regions and transitions. Each one is
// mapped to a metaclass and to the properties its notation is rebuilt from, so a
// model that states behavior converts rather than being refused. Guards and
// conditions are expression graphs, as every expression-valued position is.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Metaclass names for the behavioral nodes the SysML metamodel has no
// counterpart for, typed in the OpenSysML namespace.
const (
	mInitialNode     = "InitialNode"
	mFinalNode       = "FinalNode"
	mActionExecution = "ActionExecutionNode"
	mIfBranch        = "IfBranch"
	mPseudostate     = "Pseudostate"
	mDeferMember     = "DeferMember"
)

// The OMG metaclasses of the behavioral nodes that have one.
const (
	mFork       = "ForkNode"
	mJoin       = "JoinNode"
	mMerge      = "MergeNode"
	mDecision   = "DecisionNode"
	mPerform    = "PerformActionUsage"
	mAssignment = "AssignmentActionUsage"
	mSend       = "SendActionUsage"
	mTerminate  = "TerminateActionUsage"
	mWhileLoop  = "WhileLoopActionUsage"
	mForLoop    = "ForLoopActionUsage"
	mIfAction   = "IfActionUsage"
	mSubaction  = "StateSubactionMembership"
	mSuccession = "SuccessionAsUsage"
	mTransition = "TransitionUsage"
	mStateUsage = "StateUsage"
)

// Property names for the parts of a behavioral node that the SysML vocabulary
// has no predicate for; see docs/reference/rdf-mapping.md § Behavior.
const (
	xGuard            = "guard"
	xExpression       = "expression"
	xIsElse           = "isElse"
	xWhileCondition   = "whileCondition"
	xUntilCondition   = "untilCondition"
	xLoopVariable     = "loopVariable"
	xCollection       = "collection"
	xBranchKind       = "branchKind"
	xSubactionKind    = "subactionKind"
	xPseudostateKind  = "pseudostateKind"
	xTransitionSyntax = "transitionSyntax"
	xTrigger          = "trigger"
	xTriggerKeyword   = "triggerKeyword"
	xEffectMember     = "effectMember"
	xBracedEffect     = "bracedEffect"
	xBodyMember       = "bodyMember"
	xDeferredEvent    = "deferredEvent"
	xAssignOperator   = "assignmentOperator"
	xPayload          = "payload"
	xReceiver         = "receiver"
	xIsVia            = "isVia"
	xTarget           = "target"
)

// encodeBehavior emits the triples of a behavioral node, reporting whether the
// node was one. head writes the properties every member carries.
func (e *encoder) encodeBehavior(node ast.Node, head func(rdf.Term), subject rdf.Term, fqn, owner string, index int) (bool, error) {
	switch n := node.(type) {
	case *ast.InitialNode:
		head(rdf.OpenSysMLTerm(mInitialNode))
		// `first x` names the member the body starts at, or declares a label
		// for transitions to name when no member answers to it.
		if n.Name != "" {
			start := rdf.Term(rdf.String(n.Name))
			if decl, fqn, ok := e.linked(e.res.InitialSymbol(n)); ok {
				start = e.ids.subjectForNode(decl, fqn)
			}
			e.graph.Add(subject, e.sysml(pSourceFeature), start)
		}
		e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard)
		if qualifiedText(n.Successor) != "" {
			e.graph.Add(subject, e.sysml(pTargetFeature), e.edgeReference(n.Successor))
		} else if n.Guard != nil {
			return true, &UnsupportedError{
				What: fmt.Sprintf("the guarded initial node at %s", e.where(n)),
				Note: "it names no successor, so the branch its guard states cannot be written back",
			}
		}
		if n.HasBody {
			e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
		}
		return true, e.encode(n.Members, fqn, subject)

	case *ast.FinalNode:
		head(rdf.OpenSysMLTerm(mFinalNode))
		return true, nil

	case *ast.ForkNode:
		head(rdf.SysMLTerm(mFork))
		e.name(subject, n.Name)
		return true, nil

	case *ast.JoinNode:
		head(rdf.SysMLTerm(mJoin))
		e.name(subject, n.Name)
		return true, nil

	case *ast.MergeNode:
		head(rdf.SysMLTerm(mMerge))
		e.name(subject, n.Name)
		return true, nil

	case *ast.DecisionNode:
		head(rdf.SysMLTerm(mDecision))
		e.name(subject, n.Name)
		return true, nil

	case *ast.ActionExecutionNode:
		// `action [<name>] <ref>;` performs an action declared elsewhere;
		// `action <name> { <expr> }` performs the expression it states.
		head(rdf.OpenSysMLTerm(mActionExecution))
		e.name(subject, n.Name)
		switch {
		case n.Expression != nil:
			e.expression(subject, e.sysx(xExpression), xExpression, owner, n.Expression)
		case qualifiedText(n.ActionRef) != "":
			e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelReferences]),
				e.reference(n.ActionRef))
		default:
			return true, &UnsupportedError{
				What: fmt.Sprintf("the action node at %s", e.where(n)),
				Note: "it states neither an action to perform nor an expression to evaluate",
			}
		}
		return true, nil

	case *ast.PerformActionNode:
		head(rdf.SysMLTerm(mPerform))
		e.expression(subject, e.sysx(xExpression), xExpression, owner, n.ActionRef)
		return true, nil

	case *ast.AssignmentActionNode:
		head(rdf.SysMLTerm(mAssignment))
		e.writtenKeyword(subject, n, "", "assign")
		if operator := e.between(n.Target, n.Value); operator != "" && operator != ":=" {
			e.graph.Add(subject, e.sysx(xAssignOperator), rdf.String(operator))
		}
		e.expression(subject, e.sysx(xTarget), xTarget, owner, n.Target)
		e.expression(subject, e.sysml(pValue), pValue, owner, n.Value)
		return true, nil

	case *ast.SendStatement:
		head(rdf.SysMLTerm(mSend))
		e.expression(subject, e.sysx(xPayload), xPayload, owner, n.Message)
		e.expression(subject, e.sysx(xReceiver), xReceiver, owner, n.Target)
		if n.IsVia {
			e.graph.Add(subject, e.sysx(xIsVia), rdf.Bool(true))
		}
		return true, nil

	case *ast.TerminateStatement:
		head(rdf.SysMLTerm(mTerminate))
		e.expression(subject, e.sysx(xExpression), xExpression, owner, n.Target)
		return true, nil

	case *ast.SuccessionEdge:
		head(rdf.SysMLTerm(mSuccession))
		implied := impliedSource(n, n.Source)
		if implied {
			// `then b;` states no source end: the notation sequences from the
			// member written before it, which the form records.
			e.graph.Add(subject, e.sysx(xEndForm), rdf.String(formThen))
		}
		if err := e.edgeEnds(subject, n, owner,
			edgeEnd{name: n.Source, member: n.SourceMember, implied: implied, stands: e.preceding[n]},
			edgeEnd{name: n.Target, member: n.TargetMember, implied: n.TargetImplied, stands: e.introduced[n]}); err != nil {
			return true, err
		}
		if n.HasBody {
			e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
		}
		return true, e.encode(n.Members, fqn, subject)

	case *ast.ControlFlowEdge:
		// A guarded branch of a decision, or the `else` branch taken when no
		// guarded one is. Which keyword introduced it decides how it is written.
		head(rdf.SysMLTerm(mSuccession))
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(firstWord(e.text(n))))
		e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard)
		if n.IsElse {
			e.graph.Add(subject, e.sysx(xIsElse), rdf.Bool(true))
		}
		return true, e.edgeEnds(subject, n, owner,
			edgeEnd{name: n.Source, member: n.SourceMember, implied: impliedSource(n, n.Source), stands: e.preceding[n]},
			edgeEnd{name: n.Target, member: n.TargetMember})

	case *ast.WhileLoopActionNode:
		return true, e.encodeLoop(n, head, subject, fqn, owner)

	case *ast.IfActionNode:
		head(rdf.SysMLTerm(mIfAction))
		e.expression(subject, e.sysx(xCondition), xCondition, owner, n.Condition)
		branches := make([]ast.Node, 0, 2)
		for _, branch := range n.Branches() {
			branches = append(branches, branch)
		}
		return true, e.encode(branches, fqn, subject)

	case *ast.IfBranchNode:
		head(rdf.OpenSysMLTerm(mIfBranch))
		e.graph.Add(subject, e.sysx(xBranchKind), rdf.String(n.Kind.String()))
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBranch(n)))
		return true, e.encode(n.Body, fqn, subject)

	case *ast.StateNode:
		// A state of a state machine.
		head(rdf.SysMLTerm(mStateUsage))
		e.name(subject, n.Name)
		members := stateBody(n)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(len(members) > 0))
		return true, e.encode(members, fqn, subject)

	case *ast.SubstateMember:
		head(rdf.SysMLTerm(mStateUsage))
		e.name(subject, n.Name)
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(false))
		return true, nil

	case *ast.EntryMember:
		return true, e.encodeSubaction(n, n.Actions, "entry", head, subject, fqn)

	case *ast.DoMember:
		return true, e.encodeSubaction(n, n.Actions, "do", head, subject, fqn)

	case *ast.ExitMember:
		return true, e.encodeSubaction(n, n.Actions, "exit", head, subject, fqn)

	case *ast.DeferMember:
		head(rdf.OpenSysMLTerm(mDeferMember))
		for _, trigger := range n.Triggers {
			e.graph.Add(subject, e.sysx(xDeferredEvent), rdf.String(e.text(trigger)))
		}
		return true, nil

	case *ast.PseudostateNode:
		head(rdf.OpenSysMLTerm(mPseudostate))
		e.name(subject, n.Name)
		e.graph.Add(subject, e.sysx(xPseudostateKind), rdf.String(n.Kind.String()))
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(e.pseudostateKeyword(n)))
		return true, nil

	case *ast.TransitionMember:
		return true, e.encodeTransition(n, head, subject, fqn, owner)
	}
	return false, nil
}

// encodeLoop emits a loop of an action body. Which conditions it carries is what
// tells the three forms apart: a `while` states its condition before the body, a
// `loop` only in an `until` clause after it, and a `for` iterates a collection.
// A condition is read in the loop's own scope, whose body declares the actions
// it tests; the collection is evaluated before the loop is entered.
func (e *encoder) encodeLoop(n *ast.WhileLoopActionNode, head func(rdf.Term), subject rdf.Term, fqn, owner string) error {
	if n.Kind == ast.LoopFor {
		head(rdf.SysMLTerm(mForLoop))
		if n.Variable.Name == "" {
			return &UnsupportedError{
				What: fmt.Sprintf("the loop at %s", e.where(n)),
				Note: "its iteration variable has no name, so the variable the body binds cannot be written back",
			}
		}
		e.graph.Add(subject, e.sysx(xLoopVariable), rdf.String(n.Variable.Name))
		e.expression(subject, e.sysx(xCollection), xCollection, owner, n.Collection)
	} else {
		head(rdf.SysMLTerm(mWhileLoop))
		switch n.Kind {
		case ast.LoopWhile:
			e.expression(subject, e.sysx(xWhileCondition), xWhileCondition, fqn, n.Condition)
			e.expression(subject, e.sysx(xUntilCondition), xUntilCondition, fqn, n.Until)
		default:
			// A `loop` tests its condition after each iteration, which is what an
			// `until` clause states; without one it has no condition at all.
			e.expression(subject, e.sysx(xUntilCondition), xUntilCondition, fqn, n.Condition)
		}
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBody(n, n.Body)))
	return e.encode(n.Body, fqn, subject)
}

// encodeSubaction emits one of a state's entry/do/exit subactions. The kind is
// the membership's, so the same three notations — empty, a braced sequence, a
// single action — are told apart by the body rather than by the keyword.
func (e *encoder) encodeSubaction(n ast.Node, actions []ast.Node, kind string, head func(rdf.Term), subject rdf.Term, fqn string) error {
	head(rdf.SysMLTerm(mSubaction))
	e.graph.Add(subject, e.sysx(xSubactionKind), rdf.String(kind))
	// `entry do { … }` states the subaction's own keyword and `do` as well, with
	// or without a space or a comment between them and the body.
	if written := strings.Fields(withoutComments(e.text(n))); len(written) > 1 && bareWord(written[1]) == "do" && kind != "do" {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(kind+" do"))
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBody(n, actions)))
	return e.encode(actions, fqn, subject)
}

// encodeTransition emits a transition of a state machine: its ends as
// references, its trigger and guard as the text they were written as, and its
// effect and body as the members it owns, the effect's linked as such.
func (e *encoder) encodeTransition(n *ast.TransitionMember, head func(rdf.Term), subject rdf.Term, fqn, owner string) error {
	head(rdf.SysMLTerm(mTransition))
	e.name(subject, n.Name)
	e.graph.Add(subject, e.sysx(xTransitionSyntax), rdf.String(e.transitionSyntax(n)))
	e.transitionKeyword(subject, n)
	if qualifiedText(n.Source) != "" {
		e.graph.Add(subject, e.sysml(pSourceFeature), e.edgeReference(n.Source))
	}
	if qualifiedText(n.Target) == "" {
		return &UnsupportedError{
			What: fmt.Sprintf("the transition at %s", e.where(n)),
			Note: "it names no target state, so the edge it declares cannot be written back",
		}
	}
	e.graph.Add(subject, e.sysml(pTargetFeature), e.edgeReference(n.Target))
	if n.Trigger != nil {
		e.graph.Add(subject, e.sysx(xTrigger), rdf.String(e.text(n.Trigger)))
		e.graph.Add(subject, e.sysx(xTriggerKeyword), rdf.String(e.introducer(n, n.Trigger)))
	}
	if n.Via != nil {
		e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelVia]), e.reference(n.Via))
	}
	// The guard reads the parameters the trigger declares, in the transition's scope.
	e.expression(subject, e.sysx(xGuard), xGuard, fqn, n.Guard)
	if n.HasEffect {
		e.graph.Add(subject, e.sysx(xBracedEffect), rdf.Bool(len(n.Effect) == 0 || e.bracedBody(n, n.Effect)))
	}
	if err := e.transitionMemberLinks(n, subject, xEffectMember, n.Effect); err != nil {
		return err
	}
	if err := e.transitionMemberLinks(n, subject, xBodyMember, n.Members); err != nil {
		return err
	}
	if n.HasBody {
		e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(true))
	}
	// `then t` lies between the effect and the body, so neither tiles the
	// transition's lines on its own.
	if len(n.Effect) > 0 && len(n.Members) > 0 {
		return e.encodeInline(transitionMembers(n), fqn, subject)
	}
	return e.encode(transitionMembers(n), fqn, subject)
}

// transitionMemberLinks marks each member of a transition's effect or body as
// such, so a reader can tell the two apart.
func (e *encoder) transitionMemberLinks(n *ast.TransitionMember, subject rdf.Term, property string, members []ast.Node) error {
	for _, member := range e.kept(members) {
		node, _ := unwrapMember(member)
		if node == nil {
			continue
		}
		memberFQN, ok := e.fqn[node]
		if !ok {
			return &UnsupportedError{
				What: fmt.Sprintf("the transition at %s", e.where(n)),
				Note: "it owns a member the graph gives no identity, so its effect cannot be told from its body",
			}
		}
		e.graph.Add(subject, e.sysx(property), e.ids.subjectForNode(node, memberFQN))
	}
	return nil
}

// transitionMembers lists what a transition owns in written order: the actions
// of its `do` effect, then the members of its body.
func transitionMembers(n *ast.TransitionMember) []ast.Node {
	if len(n.Effect) == 0 {
		return n.Members
	}
	members := make([]ast.Node, 0, len(n.Effect)+len(n.Members))
	members = append(members, n.Effect...)
	return append(members, n.Members...)
}

// impliedSource reports whether the source name came from the member before the
// edge rather than the edge itself, which its span outside the edge's tells.
func impliedSource(edge ast.Node, source *ast.QualifiedName) bool {
	return source != nil && source.Span().Offset < edge.Span().Offset
}

// edgeEnd is one end of a succession: the member it names, or the member the
// notation reaches by position where it names none.
type edgeEnd struct {
	name   *ast.QualifiedName
	member ast.Node
	// implied marks an end the notation states no name for, whose name the
	// parser took from a member beside the edge; stands is that member.
	implied bool
	stands  ast.Node
}

// answersToFeature reports whether a member declares no name of its own and
// answers to its naming feature's, a name other members of the body may share.
func answersToFeature(member ast.Node) bool {
	u, ok := member.(*ast.Usage)
	return ok && ast.NamingFeature(u) != nil
}

// edgeEnds writes the ends of a succession: a name as a feature reference, an
// unnamed end as the member it binds by position, anything else refused.
func (e *encoder) edgeEnds(subject rdf.Term, node ast.Node, owner string, src, tgt edgeEnd) error {
	ends := []struct {
		end      edgeEnd
		feature  string
		member   string
		sequence string
	}{
		{src, pSourceFeature, xSourceMember, "sequences from"},
		{tgt, pTargetFeature, xTargetMember, "sequences to"},
	}
	for _, end := range ends {
		if qualifiedText(end.end.name) == "" {
			fqn, ok := e.fqn[end.end.member]
			if !ok {
				return &UnsupportedError{
					What: fmt.Sprintf("the succession at %s", e.where(node)),
					Note: fmt.Sprintf("it neither names nor reaches the member it %s, so the order it declares cannot be written back", end.sequence),
				}
			}
			e.graph.Add(subject, e.sysx(end.member), e.ids.subjectForNode(end.end.member, fqn))
			continue
		}
		// A name the parser took from an unnamed member is its naming feature's,
		// which another member may share: the end is that member itself.
		if end.end.implied && answersToFeature(end.end.stands) {
			if fqn, ok := e.fqn[end.end.stands]; ok {
				e.graph.Add(subject, e.sysml(end.feature), e.ids.subjectForNode(end.end.stands, fqn))
				continue
			}
		}
		term := e.edgeReference(end.end.name)
		e.graph.Add(subject, e.sysml(end.feature), term)
		// A name the parser took from the member before that links no element
		// still binds that member: the graph states it by position as well.
		if before, ok := e.preceding[node]; ok && end.end.implied && term.IsLiteral() {
			if fqn, ok := e.fqn[before]; ok {
				e.graph.Add(subject, e.sysx(end.member), e.ids.subjectForNode(before, fqn))
			}
		}
	}
	return nil
}

func (e *encoder) name(subject rdf.Term, name string) {
	if name != "" {
		e.graph.Add(subject, e.sysml(pDeclaredName), rdf.String(name))
	}
}

// writtenKeyword records the keyword a node was written with when it is a
// synonym of the canonical one, so the notation comes back as the author spelled
// it. canonical may be empty, for a statement whose keyword is optional.
func (e *encoder) writtenKeyword(subject rdf.Term, node ast.Node, canonical string, synonyms ...string) {
	word := firstWord(e.text(node))
	if word == canonical {
		return
	}
	for _, synonym := range synonyms {
		if word == synonym {
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(word))
			return
		}
	}
}

// pseudostateKeyword gives the notation of a pseudostate's kind; a shallow
// history may be written with `history` alone.
func (e *encoder) pseudostateKeyword(n *ast.PseudostateNode) string {
	if n.Kind == ast.PseudostateShallowHistory && firstWord(e.text(n)) == "history" {
		return "history"
	}
	return n.Kind.String()
}

// transitionSyntax names the spelling a transition was written in: `first`
// marking the source, `source` stating it without that marker, or the `accept`
// of a transition that states only its trigger.
func (e *encoder) transitionSyntax(n *ast.TransitionMember) string {
	if n.Source == nil {
		return "accept"
	}
	// The source is the node the AST places right after an optional `first`.
	if head := words(e.before(n, n.Source)); len(head) > 0 && head[len(head)-1] == "first" {
		return "first"
	}
	return "source"
}

// transitionKeyword records `succession` on a guarded succession, which the
// parser reads as a transition; the keyword is the one written before the source.
func (e *encoder) transitionKeyword(subject rdf.Term, n *ast.TransitionMember) {
	if n.Source == nil {
		return
	}
	for _, word := range words(e.before(n, n.Source)) {
		switch word {
		case "succession":
			e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(word))
			return
		case "transition":
			return
		}
	}
}

// bracedBody reports whether a body was written with braces rather than as the
// single action or body parameter the notation also allows.
func (e *encoder) bracedBody(node ast.Node, body []ast.Node) bool {
	if len(body) == 0 {
		return strings.Contains(e.text(node), "{")
	}
	first, _ := unwrapMember(body[0])
	if first == nil {
		return false
	}
	return strings.HasSuffix(e.before(node, first), "{")
}

// bracedBranch reports whether a branch of a conditional was written with
// braces rather than as the `else if` or action parameter the notation allows.
func (e *encoder) bracedBranch(n *ast.IfBranchNode) bool {
	text := strings.TrimSpace(strings.TrimPrefix(e.text(n), "else"))
	return strings.HasPrefix(text, "{")
}

// before returns the text between the start of node and the start of inner.
func (e *encoder) before(node, inner ast.Node) string {
	start, end := node.Span().Offset, inner.Span().Offset
	if end <= start {
		return ""
	}
	return strings.TrimSpace(e.file.Text(source.Span{Offset: start, Len: end - start}))
}

// between returns the text between two nodes, which is the operator or keyword
// written there. Parentheses opening the second node belong to it, not here.
func (e *encoder) between(from, to ast.Node) string {
	if from == nil || to == nil {
		return ""
	}
	start, end := from.Span().End(), to.Span().Offset
	if end <= start {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(e.file.Text(source.Span{Offset: start, Len: end - start}), "( \t\r\n"))
}

// introducer returns the keyword written immediately before inner, which is
// what the clause inner belongs to was introduced with.
func (e *encoder) introducer(node, inner ast.Node) string {
	fields := strings.Fields(e.before(node, inner))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func firstWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return bareWord(fields[0])
}

// bareWord cuts a word at the punctuation it can run into, which the notation
// allows without a space between them.
func bareWord(field string) string {
	if cut := strings.IndexAny(field, ";{"); cut >= 0 {
		return field[:cut]
	}
	return field
}

// bareAcceptNode reports whether an accept node was written without the `action`
// keyword, which the notation makes optional and the parser records regardless.
func bareAcceptNode(n *ast.Usage, text string) bool {
	if n.Kind != ast.UsageAction || n.Keyword != "action" {
		return false
	}
	for _, word := range strings.Fields(text) {
		switch word {
		case "action":
			return false
		case "accept":
			return true
		}
	}
	return false
}

// stateBody gathers the members of a state, which the AST holds in one bucket
// per kind, back into the order they were written in.
func stateBody(n *ast.StateNode) []ast.Node {
	members := make([]ast.Node, 0,
		len(n.Entry)+len(n.Do)+len(n.Exit)+len(n.Defer)+len(n.Substates)+len(n.Regions))
	for _, bucket := range [][]ast.Node{n.Entry, n.Do, n.Exit, n.Defer, n.Substates} {
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

// behaviorNameAndMembers returns the name a behavioral node declares and the
// members it owns, for the nodes declaredNameAndMembers does not cover.
func behaviorNameAndMembers(node ast.Node) (string, []ast.Node, bool) {
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
		return n.Name, stateBody(n), true
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
		return n.Name, transitionMembers(n), true
	}
	return "", nil, false
}

// behaviorHead builds the declaration text of a behavioral element whose
// notation is a head and a terminator, reporting whether the metaclass was one.
func (d *decoder) behaviorHead(el *element) (string, bool, error) {
	switch el.metaclass {
	case mInitialNode:
		words := []string{"first"}
		// The start is a member of this body or a label, so it is written by
		// its own name: `first` takes no qualified name.
		if starts := d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pSourceFeature); len(starts) > 0 {
			start, target, err := d.memberName(starts[0])
			if err != nil {
				return "", true, err
			}
			if target != nil {
				d.wanted.starts[el.qname] = target.qname
			}
			words = append(words, start)
		}
		if guard, ok := d.stringOf(el, rdf.OpenSysML+xGuard); ok {
			words = append(words, "if", guard)
		}
		successor, err := d.referenceText(el, rdf.SysML+pTargetFeature)
		if err != nil {
			return "", true, err
		}
		if successor != "" {
			words = append(words, "then", successor)
		}
		return strings.Join(words, " "), true, nil

	case mFinalNode:
		return "done", true, nil

	case mFork, mJoin, mMerge, mDecision:
		words := []string{controlNodeKeyword[el.metaclass]}
		return strings.Join(append(words, d.identWords(el)...), " "), true, nil

	case mActionExecution:
		words := []string{"action"}
		words = append(words, d.identWords(el)...)
		reference, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelReferences])
		if err != nil {
			return "", true, err
		}
		switch expression, ok := d.stringOf(el, rdf.OpenSysML+xExpression); {
		case ok:
			words = append(words, "{ "+expression+" }")
		case reference != "":
			words = append(words, reference)
		default:
			return "", true, d.missing(el, "sysx:"+xExpression, "an action node performs an action or evaluates an expression")
		}
		return strings.Join(words, " "), true, nil

	case mPerform:
		action, ok := d.stringOf(el, rdf.OpenSysML+xExpression)
		if !ok {
			return "", true, d.missing(el, "sysx:"+xExpression, "a perform statement names the action it performs")
		}
		return "perform " + action, true, nil

	case mAssignment:
		target, hasTarget := d.stringOf(el, rdf.OpenSysML+xTarget)
		value, hasValue := d.stringOf(el, rdf.SysML+pValue)
		if !hasTarget || !hasValue {
			return "", true, d.missing(el, "sysx:"+xTarget+" and sysml:"+pValue, "an assignment states what it assigns to what")
		}
		var words []string
		if keyword, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
			words = append(words, keyword)
		}
		operator := ":="
		if written, ok := d.stringOf(el, rdf.OpenSysML+xAssignOperator); ok {
			operator = written
		}
		words = append(words, target, operator, value)
		return strings.Join(words, " "), true, nil

	case mSend:
		payload, hasPayload := d.stringOf(el, rdf.OpenSysML+xPayload)
		receiver, hasReceiver := d.stringOf(el, rdf.OpenSysML+xReceiver)
		if !hasPayload || !hasReceiver {
			return "", true, d.missing(el, "sysx:"+xPayload+" and sysx:"+xReceiver, "a send states what it sends and where")
		}
		keyword := "to"
		if d.boolOf(el, rdf.OpenSysML+xIsVia) {
			keyword = "via"
		}
		return strings.Join([]string{"send", payload, keyword, receiver}, " "), true, nil

	case mTerminate:
		words := []string{"terminate"}
		if target, ok := d.stringOf(el, rdf.OpenSysML+xExpression); ok {
			words = append(words, target)
		}
		return strings.Join(words, " "), true, nil

	case mDeferMember:
		events := d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xDeferredEvent)
		if len(events) == 0 {
			return "", true, d.missing(el, "sysx:"+xDeferredEvent, "a defer member names the events it defers")
		}
		names := make([]string, 0, len(events))
		for _, event := range events {
			names = append(names, event.Value)
		}
		return "defer " + strings.Join(names, ", "), true, nil

	case mPseudostate:
		kind, ok := d.stringOf(el, rdf.OpenSysML+xPseudostateKind)
		if !ok {
			return "", true, d.missing(el, "sysx:"+xPseudostateKind, "a pseudostate states which kind it is")
		}
		if !pseudostateKinds[kind] {
			return "", true, &UnsupportedError{
				What: fmt.Sprintf("the pseudostate <%s>", el.iri),
				Note: fmt.Sprintf("it states sysx:%s %q, and no pseudostate of that kind can be written in notation", xPseudostateKind, kind),
			}
		}
		words := []string{d.keywordOr(el, kind)}
		return strings.Join(append(words, d.identWords(el)...), " "), true, nil
	}
	return "", false, nil
}

// pseudostateKinds are the sysx:pseudostateKind values the encoder writes.
var pseudostateKinds = func() map[string]bool {
	kinds := make(map[string]bool)
	for k := ast.PseudostateChoice; k <= ast.PseudostateDeepHistory; k++ {
		kinds[k.String()] = true
	}
	return kinds
}()

// controlNodeKeyword gives the notation of each control node metaclass.
var controlNodeKeyword = map[string]string{
	mFork:     "fork",
	mJoin:     "join",
	mMerge:    "merge",
	mDecision: "decide",
}

// successionHead writes a succession back using standard end notation.
func (d *decoder) successionHead(el *element) (string, error) {
	target, err := d.referenceText(el, rdf.SysML+pTargetFeature)
	if err != nil {
		return "", err
	}
	guard, hasGuard := d.stringOf(el, rdf.OpenSysML+xGuard)
	if target == "" {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: "it does not name both of the members it sequences, so the order it declares cannot be written back",
		}
	}
	_, positionalSource := d.graph.Object(rdf.IRI(el.iri), rdf.OpenSysML+xSourceMember)
	switch keyword := d.keywordOr(el, "then"); {
	case keyword == "else" || d.boolOf(el, rdf.OpenSysML+xIsElse):
		return "else " + target, nil
	case keyword == "if":
		if !hasGuard {
			return "", d.missing(el, "sysx:"+xGuard, "a branch written with `if` states the condition it is taken under")
		}
		return "if " + guard + " then " + target, nil
	}
	if form, _ := d.stringOf(el, rdf.OpenSysML+xEndForm); form == formThen || positionalSource {
		// The source end is the member written before, which this form leaves
		// unwritten.
		return "then " + target, nil
	}
	// Only the forms that name the source read it, so no spelling is chosen
	// for one the notation leaves unwritten.
	source, err := d.referenceText(el, rdf.SysML+pSourceFeature)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: "it does not name a source end and is not a positional `then` succession",
		}
	}
	words := []string{"succession", "first", source, "then", target}
	if hasGuard {
		words = []string{"succession", "first", source, "if", guard, "then", target}
	}
	return strings.Join(words, " "), nil
}

// positionalSuccessions resolves the successions of one body stating no source
// name: each sequences from the member before it, folded in as `then action b;`.
func (d *decoder) positionalSuccessions(children []*element) ([]*element, error) {
	kept := make([]*element, 0, len(children))
	last := func() *element {
		if len(kept) == 0 {
			return nil
		}
		return kept[len(kept)-1]
	}
	// sourceBefore is the member a `then` after the last skip members of kept
	// sequences from: like the parser, it passes over edges and non-features.
	sourceBefore := func(skip int) *element {
		for i := len(kept) - 1 - skip; i >= 0; i-- {
			if isSuccessionSource(kept[i]) {
				return kept[i]
			}
		}
		return nil
	}
	for _, child := range children {
		if child.metaclass != mSuccession {
			kept = append(kept, child)
			continue
		}
		// A source the graph states by position has no name to write, so the
		// form that leaves it unwritten is the only one for it.
		form, _ := d.stringOf(child, rdf.OpenSysML+xEndForm)
		_, positionalSource := d.graph.Object(rdf.IRI(child.iri), rdf.OpenSysML+xSourceMember)
		_, positionalTarget := d.graph.Object(rdf.IRI(child.iri), rdf.OpenSysML+xTargetMember)
		if form != formThen && !positionalSource && !positionalTarget {
			kept = append(kept, child)
			continue
		}
		if d.sequencesTo(child, last()) && (positionalTarget || d.keywordOr(child, "then") == "then") {
			// The target is the member written just before, which this form
			// introduces: `then` is written ahead of that member's declaration.
			if err := d.attachable(child, sourceBefore(1)); err != nil {
				return nil, err
			}
			last().prefix = "then "
			d.folded[child] = last()
			continue
		}
		// The target is a member elsewhere in the body, so the succession
		// is written where it stands, sequencing from the member before it.
		if positionalTarget {
			return nil, d.positionalError(child, "to", "before it")
		}
		if err := d.impliedSource(child, sourceBefore(0)); err != nil {
			return nil, err
		}
		kept = append(kept, child)
	}
	return kept, nil
}

// isSuccessionSource is ast.IsSuccessionSource read off a metaclass: a feature that
// is not an edge. A subaction membership stands for the action it owns.
func isSuccessionSource(el *element) bool {
	if kind, ok := metaclassUsage[el.metaclass]; ok {
		return !kind.IsEdge()
	}
	switch el.metaclass {
	case mSubaction:
		return true
	case mAlias, mFilter, mMultiplicity, mDeferMember:
		return false
	}
	if _, declared := ontology.LookupClass(el.metaclass); declared {
		return ontology.IsAncestorOrSelf(el.metaclass, "Feature")
	}
	return true
}

// answersTo returns the end a `then` sequencing from member el records: the name
// the parser gives el — `first x` names x, an unnamed usage its naming feature.
func (d *decoder) answersTo(el *element) (rdf.Term, bool) {
	if el == nil {
		return rdf.Term{}, false
	}
	subject := rdf.IRI(el.iri)
	if el.metaclass == mInitialNode {
		if term, ok := d.graph.Object(subject, rdf.SysML+pSourceFeature); ok {
			return term, true
		}
		return subject, true
	}
	if _, named := d.stringOf(el, rdf.SysML+pDeclaredName); named {
		return subject, true
	}
	if naming, ok := d.namingFeature(el); ok {
		return naming, true
	}
	return subject, true
}

// sequencesTo reports whether the target end el states is the member to: by
// position that member itself, by name the one the parser gives it.
func (d *decoder) sequencesTo(el, to *element) bool {
	if to == nil || to.metaclass == mInitialNode {
		return false
	}
	if term, positional := d.graph.Object(rdf.IRI(el.iri), rdf.OpenSysML+xTargetMember); positional {
		return term.Value == to.iri
	}
	target, ok := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pTargetFeature)
	if !ok {
		return false
	}
	return d.namesMember(target, to)
}

// namesMember reports whether an end names member: the member itself, or the
// naming feature an unnamed one answers to.
func (d *decoder) namesMember(end rdf.Term, member *element) bool {
	if member.metaclass != mInitialNode && end.Equal(rdf.IRI(member.iri)) {
		return true
	}
	answers, ok := d.answersTo(member)
	return ok && end.Equal(answers)
}

// sourceEnd returns the end a succession sequences from, by position or by the
// name it states, and whether it states either.
func (d *decoder) sourceEnd(el *element) (rdf.Term, bool) {
	if term, positional := d.graph.Object(rdf.IRI(el.iri), rdf.OpenSysML+xSourceMember); positional {
		return term, true
	}
	return d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pSourceFeature)
}

// sequencesFrom reports whether the source end el states is the one a `then`
// written after from records.
func (d *decoder) sequencesFrom(el, from *element) bool {
	if from == nil {
		return false
	}
	if term, positional := d.graph.Object(rdf.IRI(el.iri), rdf.OpenSysML+xSourceMember); positional {
		return term.Value == from.iri
	}
	source, ok := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pSourceFeature)
	if !ok {
		return false
	}
	return d.namesMember(source, from)
}

// impliedSource checks a `then <target>`, whose source end is the member before
// it: the graph has to agree, or the order it states would not be written back.
func (d *decoder) impliedSource(el, from *element) error {
	if _, states := d.sourceEnd(el); !states {
		return d.missing(el, "sysml:"+pSourceFeature, "a succession written as `then` sequences from the member before it")
	}
	if !d.sequencesFrom(el, from) {
		return d.positionalError(el, "from", "before it")
	}
	return nil
}

// attachable checks a succession folded into the member it introduces: a plain
// `then` only, sequencing from the member written before that one.
func (d *decoder) attachable(el, from *element) error {
	if keyword := d.keywordOr(el, "then"); keyword != "then" {
		return &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: fmt.Sprintf("it reaches a member that states no name, which only `then` is written beside, not `%s`", keyword),
		}
	}
	if _, hasGuard := d.stringOf(el, rdf.OpenSysML+xGuard); hasGuard {
		return &UnsupportedError{
			What: fmt.Sprintf("the succession <%s>", el.iri),
			Note: "it states a guard and reaches a member that states no name, a form that carries no guard",
		}
	}
	if _, states := d.sourceEnd(el); states && !d.sequencesFrom(el, from) {
		return d.positionalError(el, "from", "before the member it introduces")
	}
	return nil
}

// positionalError reports a succession whose ends the graph states in an order
// the notation it is written in cannot express.
func (d *decoder) positionalError(el *element, end, where string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the succession <%s>", el.iri),
		Note: fmt.Sprintf("it sequences %s the member written %s, which is another member of this body", end, where),
	}
}

// printBehavior writes the behavioral elements whose notation is not a head
// followed by a body: the loops and conditionals whose conditions are written
// around the body, a state's subactions, and a transition's clauses.
// indent is what the declaration is written after, including the `then` of a
// succession folded into it.
func (d *decoder) printBehavior(b *strings.Builder, el *element, indent string, depth int) (bool, error) {
	switch el.metaclass {
	case mWhileLoop, mForLoop:
		text, err := d.loopText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + d.nl)
		return true, nil

	case mIfAction:
		text, err := d.conditionalText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + d.nl)
		return true, nil

	case mSubaction:
		text, err := d.subactionText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + d.nl)
		return true, nil

	case mIfBranch:
		return true, &UnsupportedError{
			What: fmt.Sprintf("the branch <%s>", el.iri),
			Note: "a branch is written inside the conditional that owns it, and no conditional owns this one",
		}

	case mTransition:
		// A transition carrying its ends as references is the one a state body
		// declares; a transition usage whose head was kept verbatim never
		// reaches here, since print() writes its source text.
		if _, structural := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pTargetFeature); !structural {
			return false, nil
		}
		text, body, err := d.transitionText(el, depth)
		if err != nil {
			return true, err
		}
		if body == "" {
			b.WriteString(indent + text + ";" + d.nl)
		} else {
			b.WriteString(indent + text + " " + body + d.nl)
		}
		return true, nil
	}
	return false, nil
}

// loopText writes a loop and the conditions around its body.
func (d *decoder) loopText(el *element, depth int) (string, error) {
	var head string
	switch el.metaclass {
	case mForLoop:
		variable, hasVariable := d.stringOf(el, rdf.OpenSysML+xLoopVariable)
		collection, hasCollection := d.stringOf(el, rdf.OpenSysML+xCollection)
		if !hasVariable || !hasCollection {
			return "", d.missing(el, "sysx:"+xLoopVariable+" and sysx:"+xCollection, "a for loop binds a variable over a collection")
		}
		head = "for " + nameText(variable) + " in " + collection
	default:
		if condition, ok := d.stringOf(el, rdf.OpenSysML+xWhileCondition); ok {
			head = "while " + condition
		} else {
			head = "loop"
		}
	}
	body, err := d.bodyText(el, depth)
	if err != nil {
		return "", err
	}
	text := head + " " + body
	// An `until` clause states the condition tested after each iteration, and
	// terminates the loop; so does a body written without braces.
	if until, ok := d.stringOf(el, rdf.OpenSysML+xUntilCondition); ok {
		return text + " until " + until + ";", nil
	}
	if !d.boolOf(el, rdf.OpenSysML+xHasBody) && !strings.HasSuffix(text, ";") && el.metaclass != mForLoop {
		return text + ";", nil
	}
	return text, nil
}

// conditionalText writes an if action: its condition, the branch taken when the
// condition holds, and the one taken when it does not.
func (d *decoder) conditionalText(el *element, depth int) (string, error) {
	condition, ok := d.stringOf(el, rdf.OpenSysML+xCondition)
	if !ok {
		return "", d.missing(el, "sysx:"+xCondition, "an if action states the condition it branches on")
	}
	var then, otherwise *element
	for _, child := range el.children {
		if child.metaclass != mIfBranch {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the member <%s> of the if action <%s>", child.iri, el.iri),
				Note: "an if action owns its branches, and this member is not one",
			}
		}
		kind, _ := d.stringOf(child, rdf.OpenSysML+xBranchKind)
		if kind == ast.IfBranchElse.String() {
			otherwise = child
		} else {
			then = child
		}
	}
	if then == nil {
		return "", d.missing(el, "sysx:"+xBranchKind, "an if action states the branch taken when its condition holds")
	}
	thenText, err := d.bodyText(then, depth)
	if err != nil {
		return "", err
	}
	text := "if " + condition + " " + thenText
	if otherwise == nil {
		return text, nil
	}
	elseText, err := d.bodyText(otherwise, depth)
	if err != nil {
		return "", err
	}
	return text + " else " + elseText, nil
}

// subactionText writes one of a state's entry/do/exit subactions in the shape it
// was written: empty, a braced sequence, or a single action.
func (d *decoder) subactionText(el *element, depth int) (string, error) {
	kind, ok := d.stringOf(el, rdf.OpenSysML+xSubactionKind)
	if !ok {
		return "", d.missing(el, "sysx:"+xSubactionKind, "a state subaction states whether it runs on entry, throughout or on exit")
	}
	keyword := d.keywordOr(el, kind)
	braced := d.boolOf(el, rdf.OpenSysML+xHasBody)
	if len(el.children) == 0 && !braced {
		return keyword + ";", nil
	}
	body, err := d.bodyText(el, depth)
	if err != nil {
		return "", err
	}
	if braced {
		return keyword + " " + body, nil
	}
	// A performed action states the subaction's keyword itself
	// (`entry warmUp;`), so writing the keyword again would declare it twice.
	if len(el.children) == 1 {
		if written, ok := d.stringOf(el.children[0], rdf.OpenSysML+xDeclaredKeyword); ok && written == kind {
			return body, nil
		}
	}
	return keyword + " " + body, nil
}

// transitionText writes a transition of a state machine, in the spelling it was
// written in, with its trigger, guard and effect; the body it ends in, if any,
// is returned separately.
func (d *decoder) transitionText(el *element, depth int) (string, string, error) {
	target, err := d.referenceText(el, rdf.SysML+pTargetFeature)
	if err != nil {
		return "", "", err
	}
	syntax := "first"
	if written, ok := d.stringOf(el, rdf.OpenSysML+xTransitionSyntax); ok {
		syntax = written
	}
	var words []string
	switch syntax {
	case "accept":
		// The transition of a state body that states only its trigger.
	default:
		source, err := d.referenceText(el, rdf.SysML+pSourceFeature)
		if err != nil {
			return "", "", err
		}
		if source == "" {
			return "", "", d.missing(el, "sysml:"+pSourceFeature, "a transition written with `transition` names the state it leaves")
		}
		keyword := "transition"
		if written, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
			keyword = written
		}
		ident := d.identWords(el)
		if visibility := d.visibility(el); visibility != "" {
			words = append(words, visibility)
		}
		words = append(words, keyword)
		words = append(words, ident...)
		// The grammar admits a bare source only on a nameless `transition`.
		if syntax == "first" || len(ident) > 0 || keyword == "succession" {
			words = append(words, "first")
		}
		words = append(words, source)
	}
	if trigger, ok := d.stringOf(el, rdf.OpenSysML+xTrigger); ok {
		keyword := "accept"
		if written, ok := d.stringOf(el, rdf.OpenSysML+xTriggerKeyword); ok {
			keyword = written
		}
		words = append(words, keyword, trigger)
		via, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelVia])
		if err != nil {
			return "", "", err
		}
		if via != "" {
			words = append(words, "via", via)
		}
	}
	if guard, ok := d.stringOf(el, rdf.OpenSysML+xGuard); ok {
		words = append(words, "if", guard)
	}
	// Members are linked as effect or body; unlinked members are from a mapping
	// that owned the effect alone, with sysx:hasBody its braces.
	inEffect := d.linked(el, xEffectMember)
	inBody := d.linked(el, xBodyMember)
	legacy := len(el.children) > 0 && len(inEffect) == 0 && len(inBody) == 0
	braced := d.boolOf(el, rdf.OpenSysML+xBracedEffect)
	hasEffect := d.graph.HasProperty(rdf.IRI(el.iri), rdf.OpenSysML+xBracedEffect)
	hasBody := d.boolOf(el, rdf.OpenSysML+xHasBody)
	if legacy {
		braced, hasEffect, hasBody = hasBody, true, false
	}
	var effect, body []*element
	for _, child := range el.children {
		switch {
		case legacy, inEffect[child.iri] && !inBody[child.iri]:
			effect = append(effect, child)
		case inBody[child.iri] && !inEffect[child.iri]:
			body = append(body, child)
		case inBody[child.iri]:
			return "", "", transitionLinkError(el, child.iri, "is linked as both effect and body")
		default:
			return "", "", transitionLinkError(el, child.iri, "is linked as neither effect nor body")
		}
		delete(inEffect, child.iri)
		delete(inBody, child.iri)
	}
	for iri := range inEffect {
		return "", "", transitionLinkError(el, iri, "is linked as an effect but is no member")
	}
	for iri := range inBody {
		return "", "", transitionLinkError(el, iri, "is linked as a body member but is no member")
	}
	if hasEffect {
		text, err := d.membersText(effect, braced, depth)
		if err != nil {
			return "", "", err
		}
		words = append(words, "do", strings.TrimSuffix(text, ";"))
	}
	words = append(words, "then", target)
	var bodyText string
	if len(body) > 0 || hasBody {
		if bodyText, err = d.membersText(body, true, depth); err != nil {
			return "", "", err
		}
	}
	return strings.Join(words, " "), bodyText, nil
}

// transitionLinkError refuses a transition whose effect and body links do not
// partition its members.
func transitionLinkError(el *element, member, fault string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the transition %s", el.iri),
		Note: fmt.Sprintf("%s %s, so its actions cannot be placed", member, fault),
	}
}

// linked is the set of members the element links through the property.
func (d *decoder) linked(el *element, property string) map[string]bool {
	set := map[string]bool{}
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+property) {
		set[term.Value] = true
	}
	return set
}

// bodyText writes the members of an element: braced when the notation was, and
// as the members alone when it stated them without braces.
func (d *decoder) bodyText(el *element, depth int) (string, error) {
	return d.membersText(el.children, d.boolOf(el, rdf.OpenSysML+xHasBody), depth)
}

// membersText writes members one per line, in braces when braced; an unbraced
// member continues the line the head is on, at its depth.
func (d *decoder) membersText(members []*element, braced bool, depth int) (string, error) {
	memberDepth := depth
	if braced {
		memberDepth = depth + 1
	}
	var b strings.Builder
	for _, child := range members {
		if err := d.print(&b, child, memberDepth); err != nil {
			return "", err
		}
	}
	if braced {
		return "{" + d.nl + b.String() + strings.Repeat("    ", depth) + "}", nil
	}
	return strings.TrimSpace(b.String()), nil
}

// referenceMemberKeyword reports whether a keyword introduces a member that
// names an existing feature rather than declaring one — `perform doIt;`,
// `exhibit modes;` and a state's `entry warmUp;` — so the reference is its
// notation.
func referenceMemberKeyword(keyword string) bool {
	switch keyword {
	case "perform", "exhibit", "entry", "do", "exit":
		return true
	}
	return false
}
