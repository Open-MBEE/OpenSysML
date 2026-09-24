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

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// Metaclass names for the behavioral nodes the SysML metamodel has no
// counterpart for, typed in the OpenSysML namespace. InitialNode and FinalNode
// are an older graph's forms of `first`/`done`, read only.
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
	// The elements a TransitionUsage owns in the abstract syntax (SysML v2 1.0 § 8.3.18.9).
	mTransitionFeatureMembership = "TransitionFeatureMembership"
	mAcceptAction                = "AcceptActionUsage"
	mStateUsage                  = "StateUsage"
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
	xHasEffect        = "hasEffect"
	xBracedEffect     = "bracedEffect" // an older mapping's flat braced effect, refused
	xBodyMember       = "bodyMember"
	xDeferredEvent    = "deferredEvent"
	xAssignOperator   = "assignmentOperator"
	xPayload          = "payload"
	xReceiver         = "receiver"
	xIsVia            = "isVia"
	xTarget           = "target"
)

// libraryDone is the library feature a `done;` member names.
var libraryDone = ast.QualifiedNameOf("Actions", "Action", "done")

// libraryReference is the subject of a standard library element named from
// the global scope, or its name when no library is loaded.
func (e *encoder) libraryReference(name *ast.QualifiedName) rdf.Term {
	if decl, fqn, ok := e.linked(e.res.ResolveQualified(nil, name)); ok {
		return e.ids.subjectForNode(decl, fqn)
	}
	return rdf.String(qualifiedText(name))
}

// encodeBehavior emits the triples of a behavioral node, reporting whether the
// node was one. head writes the properties every member carries.
func (e *encoder) encodeBehavior(node ast.Node, head func(rdf.Term), subject rdf.Term, fqn, owner string, index int) (bool, error) {
	switch n := node.(type) {
	case *ast.InitialNode:
		// `first x;` is a Membership of the member the body starts at (SysML.xtext
		// InitialNodeMember); `first x then y` the SuccessionAsUsage it sequences.
		if qualifiedText(n.Successor) != "" {
			head(rdf.SysMLTerm(mSuccession))
		} else {
			head(rdf.SysMLTerm(mMembership))
		}
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("first"))
		if n.Name() != "" {
			start := rdf.Term(rdf.String(n.Name()))
			if decl, fqn, ok := e.linked(e.res.InitialSymbol(n)); ok {
				start = e.ids.subjectForNode(decl, fqn)
			}
			if qualifiedText(n.Successor) != "" {
				e.graph.Add(subject, e.sysml(pSourceFeature), start)
			} else {
				e.graph.Add(subject, e.sysml(pMemberElement), start)
			}
		}
		if err := e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard); err != nil {
			return true, err
		}
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
		// `done;` is a Membership of the library's Actions::Action::done.
		head(rdf.SysMLTerm(mMembership))
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String("done"))
		e.graph.Add(subject, e.sysml(pMemberElement), e.libraryReference(libraryDone))
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
			return true, e.expression(subject, e.sysx(xExpression), xExpression, owner, n.Expression)
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
		return true, e.expression(subject, e.sysx(xExpression), xExpression, owner, n.ActionRef)

	case *ast.AssignmentActionNode:
		head(rdf.SysMLTerm(mAssignment))
		e.writtenKeyword(subject, n, "", "assign")
		if operator := e.between(n.Target, n.Value); operator != "" && operator != ":=" {
			e.graph.Add(subject, e.sysx(xAssignOperator), rdf.String(operator))
		}
		if err := e.expression(subject, e.sysx(xTarget), xTarget, owner, n.Target); err != nil {
			return true, err
		}
		return true, e.expression(subject, e.sysml(pValue), pValue, owner, n.Value)

	case *ast.SendStatement:
		head(rdf.SysMLTerm(mSend))
		if err := e.expression(subject, e.sysx(xPayload), xPayload, owner, n.Message); err != nil {
			return true, err
		}
		if err := e.expression(subject, e.sysx(xReceiver), xReceiver, owner, n.Target); err != nil {
			return true, err
		}
		if n.IsVia {
			e.graph.Add(subject, e.sysx(xIsVia), rdf.Bool(true))
		}
		return true, nil

	case *ast.TerminateStatement:
		head(rdf.SysMLTerm(mTerminate))
		return true, e.expression(subject, e.sysx(xExpression), xExpression, owner, n.Target)

	case *ast.SuccessionEdge:
		head(rdf.SysMLTerm(mSuccession))
		implied := impliedSource(n, n.Source)
		if implied {
			// `then b;` sequences from the member before it: an empty source
			// end and the target end (SysML.xtext TargetSuccession).
			e.graph.Add(subject, e.sysx(xEndForm), rdf.String(formThen))
			if err := e.connectorEnd(subject, connectorEndSpec{owner: owner, slot: "end0", index: 0, ends: 2, empty: true, noCollapse: true}); err != nil {
				return true, err
			}
			target := connectorEndSpec{owner: owner, slot: "end1", index: 1, ends: 2, noCollapse: true}
			if n.Target != nil {
				target.target = n.Target
			} else if n.TargetMember != nil {
				if fqn, ok := e.fqn[n.TargetMember]; ok {
					target.targetTerm = e.ids.subjectForNode(n.TargetMember, fqn)
				}
			}
			if err := e.connectorEnd(subject, target); err != nil {
				return true, err
			}
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
		if err := e.expression(subject, e.sysx(xGuard), xGuard, owner, n.Guard); err != nil {
			return true, err
		}
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
		if err := e.expressionAs(subject, e.sysx(xCondition), xCondition, owner, n.Condition, mParameterMembership); err != nil {
			return true, err
		}
		branches := make([]ast.Node, 0, 2)
		for _, branch := range n.Branches() {
			branches = append(branches, branch)
		}
		return true, e.encode(branches, fqn, subject)

	case *ast.IfBranchNode:
		head(rdf.SysMLTerm(usageMetaclass[ast.UsageAction]))
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
		if err := e.expression(subject, e.sysx(xCollection), xCollection, owner, n.Collection); err != nil {
			return err
		}
	} else {
		head(rdf.SysMLTerm(mWhileLoop))
		// A `loop` tests its condition after each iteration, which is what an
		// `until` clause states; without one it has no condition at all.
		while, until := ast.Node(nil), n.Condition
		if n.Kind == ast.LoopWhile {
			while, until = n.Condition, n.Until
		}
		if err := e.expression(subject, e.sysx(xWhileCondition), xWhileCondition, fqn, while); err != nil {
			return err
		}
		if err := e.expression(subject, e.sysx(xUntilCondition), xUntilCondition, fqn, until); err != nil {
			return err
		}
	}
	e.graph.Add(subject, e.sysx(xHasBody), rdf.Bool(e.bracedBody(n, n.Body)))
	return e.encode(n.Body, fqn, subject)
}

// encodeSubaction emits one of a state's entry/do/exit subactions. The kind is
// the membership's; the action it performs, if any, is its one member, a braced
// block being an anonymous action whose own body holds the statements.
func (e *encoder) encodeSubaction(n ast.Node, actions []ast.Node, kind string, head func(rdf.Term), subject rdf.Term, fqn string) error {
	head(rdf.SysMLTerm(mSubaction))
	e.graph.Add(subject, e.sysml(pKind), rdf.String(kind))
	e.graph.Add(subject, e.sysx(xSubactionKind), rdf.String(kind))
	e.markPerformed(actions)
	// `entry do { … }` states the subaction's own keyword and `do` as well, with
	// or without a space or a comment between them and the body.
	if written := strings.Fields(withoutComments(e.text(n))); len(written) > 1 && bareWord(written[1]) == "do" && kind != "do" {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(kind+" do"))
	}
	return e.encode(actions, fqn, subject)
}

// markPerformed records the action usages among members as performed actions.
func (e *encoder) markPerformed(members []ast.Node) {
	for _, member := range members {
		if node, _ := unwrapMember(member); node != nil {
			if usage, ok := node.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
				e.performed[usage] = true
			}
		}
	}
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
		e.graph.Add(subject, e.sysml(pSource), e.edgeReference(n.Source))
	}
	if qualifiedText(n.Target) == "" {
		return &UnsupportedError{
			What: fmt.Sprintf("the transition at %s", e.where(n)),
			Note: "it names no target state, so the edge it declares cannot be written back",
		}
	}
	e.graph.Add(subject, e.sysml(pTarget), e.edgeReference(n.Target))
	if n.Trigger != nil {
		e.graph.Add(subject, e.sysx(xTrigger), rdf.String(e.text(n.Trigger)))
		e.graph.Add(subject, e.sysx(xTriggerKeyword), rdf.String(e.introducer(n, n.Trigger)))
	}
	if n.Via != nil {
		e.graph.Add(subject, e.sysml(relationshipProperty[ast.RelVia]), e.reference(n.Via))
	}
	// The guard reads the parameters the trigger declares, in the transition's scope.
	if err := e.expression(subject, e.sysx(xGuard), xGuard, fqn, n.Guard); err != nil {
		return err
	}
	if n.HasEffect {
		e.graph.Add(subject, e.sysx(xHasEffect), rdf.Bool(true))
	}
	e.markPerformed(n.Effect)
	for _, member := range e.kept(n.Effect) {
		if node, _ := unwrapMember(member); node != nil {
			e.effects[node] = true
		}
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
		head, err := d.initialNodeHead(el)
		return head, true, err

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
			// A `perform` member is a usage element, not a statement: the usage
			// head writes its keyword, name and performed-action typing.
			return "", false, nil
		}
		// A named perform types the action it performs: `perform action a : A`.
		if name := strings.Join(d.identWords(el), " "); name != "" {
			return "perform action " + name + " : " + action, true, nil
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
		// A declared `action a terminate;` is a usage head, not a statement; it keeps its
		// name through usageHead. A name alone declares too: no statement carries one.
		if d.declaresUsage(el) || len(d.identWords(el)) > 0 {
			return "", false, nil
		}
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

// membershipKeyword is the notation a Membership member is written with: the
// `alias`, `first` or `done` its graph states, or the one its member implies.
func (d *decoder) membershipKeyword(el *element) string {
	if el.membershipKeyword != "" {
		return el.membershipKeyword
	}
	if keyword, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
		return keyword
	}
	switch el.metaclass {
	case mSuccession:
		return ""
	case mAlias:
		return "alias"
	case mFinalNode:
		return "done"
	case mInitialNode:
		return "first"
	}
	subject := rdf.IRI(el.iri)
	if d.graph.HasProperty(subject, rdf.SysML+pMemberName) ||
		d.graph.HasProperty(subject, rdf.SysML+pMemberShortName) {
		return "alias"
	}
	if member, ok := d.graph.Object(subject, rdf.SysML+pMemberElement); ok {
		done := qualifiedText(libraryDone)
		if member.IsLiteral() && member.Value == done {
			return "done"
		}
		if target, err := d.referencedElement(member.Value); err == nil && target.qname == done {
			return "done"
		}
	}
	// An unnamed Membership in a behavior body is `first x`; elsewhere an alias.
	if el.owner != nil && (ontology.IsAncestorOrSelf(el.owner.metaclass, "Behavior") ||
		ontology.IsAncestorOrSelf(el.owner.metaclass, "Feature")) {
		return "first"
	}
	return "alias"
}

// initialNode reports whether el is a `first` member: an older graph's
// sysx:InitialNode, or the Membership or SuccessionAsUsage written with `first`.
func (d *decoder) initialNode(el *element) bool {
	switch el.metaclass {
	case mInitialNode:
		return true
	case mMembership:
		return d.membershipKeyword(el) == "first"
	case mSuccession:
		return d.membershipKeyword(el) == "first"
	}
	return false
}

// startOf is the member a `first` names: a Membership's member, a
// succession's source feature.
func (d *decoder) startOf(el *element) (rdf.Term, bool) {
	if start, ok := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pMemberElement); ok {
		return start, true
	}
	return d.graph.Object(rdf.IRI(el.iri), rdf.SysML+pSourceFeature)
}

// initialNodeHead writes `first x [if g then y]`. The start is a member of
// this body or a label, written by its own name: `first` takes no qualified name.
func (d *decoder) initialNodeHead(el *element) (string, error) {
	words := []string{"first"}
	if start, ok := d.startOf(el); ok {
		name, target, err := d.memberName(start)
		if err != nil {
			return "", err
		}
		if target != nil {
			d.wanted.starts[el.qname] = target.qname
		}
		words = append(words, name)
	}
	if guard, ok := d.stringOf(el, rdf.OpenSysML+xGuard); ok {
		words = append(words, "if", guard)
	}
	successor, err := d.referenceText(el, rdf.SysML+pTargetFeature)
	if err != nil {
		return "", err
	}
	if successor != "" {
		words = append(words, "then", successor)
	}
	return strings.Join(words, " "), nil
}

// successionHead writes a succession back using standard end notation.
func (d *decoder) successionHead(el *element) (string, error) {
	if d.initialNode(el) {
		return d.initialNodeHead(el)
	}
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

// positionalSuccession reports a succession written as `then`, whose unnamed
// source end is the member before it, whatever end features it owns.
func (d *decoder) positionalSuccession(el *element) bool {
	form, _ := d.stringOf(el, rdf.OpenSysML+xEndForm)
	subject := rdf.IRI(el.iri)
	return form == formThen || d.graph.HasProperty(subject, rdf.OpenSysML+xSourceMember) ||
		d.graph.HasProperty(subject, rdf.OpenSysML+xTargetMember)
}

// positionalSuccessions resolves the successions of one body stating no source
// name: each sequences from the member before it, folded in as `then action b;`.
func (d *decoder) positionalSuccessions(children []*element) ([]*element, error) {
	src := &keptSources{kept: make([]*element, 0, len(children))}
	for _, child := range children {
		keep, err := d.successionChild(src, child)
		if err != nil {
			return nil, err
		}
		if keep {
			src.kept = append(src.kept, child)
		}
	}
	return src.kept, nil
}

// keptSources are the members a body keeps, read by position for `then` forms.
type keptSources struct {
	kept []*element
}

// last is the member written most recently.
func (k *keptSources) last() *element {
	if len(k.kept) == 0 {
		return nil
	}
	return k.kept[len(k.kept)-1]
}

// sourceBefore is the member a `then` after the last skip members of kept
// sequences from: like the parser, it passes over edges and non-features.
func (k *keptSources) sourceBefore(skip int) *element {
	for i := len(k.kept) - 1 - skip; i >= 0; i-- {
		if isSuccessionSource(k.kept[i]) {
			return k.kept[i]
		}
	}
	return nil
}

// sourceBeforeMember is the source a `then` targeting member sequences from.
func (k *keptSources) sourceBeforeMember(member *element) *element {
	for i, keptMember := range k.kept {
		if keptMember != member {
			continue
		}
		for i--; i >= 0; i-- {
			if isSuccessionSource(k.kept[i]) {
				return k.kept[i]
			}
		}
		return nil
	}
	return nil
}

// successionChild reads one member; keep reports whether it stays in the body.
func (d *decoder) successionChild(src *keptSources, child *element) (bool, error) {
	if child.metaclass != mSuccession {
		return true, nil
	}
	// A source the graph states by position has no name to write, so the
	// form that leaves it unwritten is the only one for it.
	form, _ := d.stringOf(child, rdf.OpenSysML+xEndForm)
	_, positionalSource := d.graph.Object(rdf.IRI(child.iri), rdf.OpenSysML+xSourceMember)
	_, positionalTarget := d.graph.Object(rdf.IRI(child.iri), rdf.OpenSysML+xTargetMember)
	if form != formThen && !positionalSource && !positionalTarget {
		return true, nil
	}
	target := src.last()
	if positionalTarget {
		if term, ok := d.graph.Object(rdf.IRI(child.iri), rdf.OpenSysML+xTargetMember); ok {
			target = d.byIRI[term.Value]
		}
	}
	if d.sequencesTo(child, target) && (positionalTarget || d.keywordOr(child, "then") == "then") {
		// The target is the member written just before, which this form
		// introduces: `then` is written ahead of that member's declaration.
		from := src.sourceBefore(1)
		if positionalTarget {
			from = src.sourceBeforeMember(target)
		}
		if err := d.attachable(child, from); err != nil {
			return false, err
		}
		target.prefix = "then "
		d.folded[child] = target
		return false, nil
	}
	// The target is a member elsewhere in the body, so the succession
	// is written where it stands, sequencing from the member before it.
	if positionalTarget {
		return false, d.positionalError(child, "to", "before it")
	}
	if err := d.impliedSource(child, src.sourceBefore(0)); err != nil {
		return false, err
	}
	return true, nil
}

// ifBranch reports whether el is a branch of `if`: an older graph's
// sysx:IfBranch, or the ActionUsage parameter written with its branch kind.
func (d *decoder) ifBranch(el *element) bool {
	if el.metaclass == mIfBranch {
		return true
	}
	return el.metaclass == usageMetaclass[ast.UsageAction] &&
		d.graph.HasProperty(rdf.IRI(el.iri), rdf.OpenSysML+xBranchKind)
}

// isSuccessionSource is ast.IsSuccessionSource read off a metaclass: a feature that
// is not an edge. A subaction membership stands for the action it owns.
func isSuccessionSource(el *element) bool {
	if el.membershipKeyword == "first" {
		return true
	}
	if kind, ok := metaclassUsage[el.metaclass]; ok {
		return !kind.IsEdge()
	}
	switch el.metaclass {
	case mInitialNode, mFinalNode, mActionExecution, mFork, mJoin, mMerge, mDecision,
		mPerform, mAssignment, mSend, mTerminate, mWhileLoop, mForLoop, mIfAction,
		mSubaction, mStateUsage:
		return true
	case mMembership:
		return el.membershipKeyword != "alias"
	case mAlias, mFilter, mMultiplicity, mMultiplicityClass, mMultiplicityRange, mDeferMember:
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
	if d.initialNode(el) {
		if term, ok := d.startOf(el); ok {
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
	if to == nil || d.initialNode(to) {
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
	if !d.initialNode(member) && end.Equal(rdf.IRI(member.iri)) {
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
	annotations := d.identityAnnotations(el)
	switch el.metaclass {
	case mWhileLoop, mForLoop:
		if len(annotations) > 0 {
			return true, d.behaviorUnannotatable(el, "loop")
		}
		text, err := d.loopText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + d.nl)
		return true, nil

	case mIfAction:
		if len(annotations) > 0 {
			return true, d.behaviorUnannotatable(el, "if action")
		}
		text, err := d.conditionalText(el, depth)
		if err != nil {
			return true, err
		}
		b.WriteString(indent + text + d.nl)
		return true, nil

	case mSubaction:
		if len(annotations) > 0 {
			return true, d.behaviorUnannotatable(el, "subaction membership")
		}
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
		if _, structural, err := d.transitionObject(el, pTarget); err != nil {
			return false, err
		} else if !structural {
			return false, nil
		}
		text, body, err := d.transitionText(el, annotations, depth)
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

// behaviorUnannotatable refuses an element whose identity the notation has no
// place to state: its form declares neither a name nor a body of its own.
func (d *decoder) behaviorUnannotatable(el *element, form string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the %s <%s>", form, el.iri),
		Note: "its id or project reference is not the one its position implies, and its notation has no place for an identity annotation",
	}
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
		if !d.ifBranch(child) {
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
// was written: empty, or the one action it performs — a braced block is an
// anonymous action that writes its own braces.
func (d *decoder) subactionText(el *element, depth int) (string, error) {
	kind, ok := d.stringOf(el, rdf.OpenSysML+xSubactionKind)
	normative, hasNormative := d.stringOf(el, rdf.SysML+pKind)
	switch {
	case ok && hasNormative && kind != normative:
		return "", &UnsupportedError{
			What: fmt.Sprintf("the state subaction <%s>", el.iri),
			Note: fmt.Sprintf("its kind is %q, but sysx:subactionKind says %q, and the two statements cannot both hold", normative, kind),
		}
	case !ok && hasNormative:
		kind = normative
	case !ok:
		return "", d.missing(el, "sysml:"+pKind+" or sysx:"+xSubactionKind, "a state subaction states whether it runs on entry, throughout or on exit")
	}
	keyword := d.keywordOr(el, kind)
	// A braced block is one anonymous action; a graph that wrote it as the
	// statements it holds cannot be read back as that action.
	if d.boolOf(el, rdf.OpenSysML+xHasBody) {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the %s subaction %s", kind, el.iri),
			Note: "its braced `" + kind + " { … }` block is written as its statements, not as the anonymous action the braces declare",
		}
	}
	// `entry;` is an empty ActionUsage in the pilot's graph (SysML.xtext
	// EmptyActionUsage); this mapping writes no element for it.
	if len(el.children) == 0 || len(el.children) == 1 && d.emptyActionUsage(el.children[0]) {
		return keyword + ";", nil
	}
	body, err := d.membersText(el.children, false, depth)
	if err != nil {
		return "", err
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
// is returned separately, its identity annotations ahead of its members.
func (d *decoder) transitionText(el *element, annotations []string, depth int) (string, string, error) {
	target, err := d.transitionReferenceText(el, pTarget, pTargetFeature)
	if err != nil {
		return "", "", err
	}
	syntax := "first"
	if written, ok := d.stringOf(el, rdf.OpenSysML+xTransitionSyntax); ok {
		syntax = written
	}
	words, err := d.transitionHead(el, syntax)
	if err != nil {
		return "", "", err
	}
	if trigger, ok := d.stringOf(el, rdf.OpenSysML+xTrigger); ok {
		triggerWords, err := d.triggerWords(el, trigger)
		if err != nil {
			return "", "", err
		}
		words = append(words, triggerWords...)
	}
	if guard, ok := d.stringOf(el, rdf.OpenSysML+xGuard); ok {
		words = append(words, "if", guard)
	}
	effect, body, hasEffect, hasBody, err := d.transitionMembers(el)
	if err != nil {
		return "", "", err
	}
	if hasEffect || len(effect) > 0 {
		text, err := d.effectText(effect, depth)
		if err != nil {
			return "", "", err
		}
		words = append(words, "do", text)
	}
	words = append(words, "then", target)
	bodyText, err := d.transitionBody(body, hasBody, annotations, depth)
	if err != nil {
		return "", "", err
	}
	return strings.Join(words, " "), bodyText, nil
}

// effectText writes a transition's `do` effect members.
func (d *decoder) effectText(effect []*element, depth int) (string, error) {
	text, err := d.membersText(effect, false, depth)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(text, ";"), nil
}

// transitionBody writes a transition's body, empty when it has none and no
// annotations its braces would carry.
func (d *decoder) transitionBody(body []*element, hasBody bool, annotations []string, depth int) (string, error) {
	if len(body) == 0 && !hasBody && len(annotations) == 0 {
		return "", nil
	}
	return d.membersText(body, true, depth, annotations...)
}

// transitionHead writes the words before a transition's trigger: nothing for the
// `accept` syntax of a state body's trigger alone, else keyword, name and source.
func (d *decoder) transitionHead(el *element, syntax string) ([]string, error) {
	if syntax == "accept" {
		return nil, nil
	}
	source, err := d.transitionReferenceText(el, pSource, pSourceFeature)
	if err != nil {
		return nil, err
	}
	if source == "" {
		return nil, d.missing(el, "sysml:"+pSourceFeature, "a transition written with `transition` names the state it leaves")
	}
	keyword := "transition"
	if written, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
		keyword = written
	}
	ident := d.identWords(el)
	var words []string
	if visibility := d.visibility(el); visibility != "" {
		words = append(words, visibility)
	}
	words = append(words, keyword)
	words = append(words, ident...)
	// The grammar admits a bare source only on a nameless `transition`.
	if syntax == "first" || len(ident) > 0 || keyword == "succession" {
		words = append(words, "first")
	}
	return append(words, source), nil
}

// triggerWords writes a transition's trigger and the port it arrives via.
func (d *decoder) triggerWords(el *element, trigger string) ([]string, error) {
	keyword := "accept"
	if written, ok := d.stringOf(el, rdf.OpenSysML+xTriggerKeyword); ok {
		keyword = written
	}
	words := []string{keyword, trigger}
	via, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelVia])
	if err != nil {
		return nil, err
	}
	if via != "" {
		words = append(words, "via", via)
	}
	return words, nil
}

// transitionMembers partitions a transition's members into effect and body:
// members are linked as effect or body; unlinked members are from a mapping
// that owned the effect alone, with sysx:hasBody its braces.
func (d *decoder) transitionMembers(el *element) (effect, body []*element, hasEffect, hasBody bool, err error) {
	inEffect := d.linked(el, xEffectMember)
	inBody := d.linked(el, xBodyMember)
	children := d.bodyChildren(el)
	collapsed := len(inEffect) > 0 || len(inBody) > 0
	// A TransitionFeatureMembership of kind `effect` (SysML v2 1.0 § 8.3.18.8) owns an
	// effect action; the collapsed links, when written too, must agree with it.
	for _, child := range children {
		if !d.effectMembership(child) {
			continue
		}
		if collapsed && !inEffect[child.iri] {
			return nil, nil, false, false, transitionLinkError(el, child.iri, "is owned by an effect membership but is not linked as an effect")
		}
		inEffect[child.iri] = true
	}
	if !collapsed && len(inEffect) > 0 {
		for _, child := range children {
			if !inEffect[child.iri] {
				inBody[child.iri] = true
			}
		}
	}
	legacy := len(children) > 0 && len(inEffect) == 0 && len(inBody) == 0
	hasEffect = d.boolOf(el, rdf.OpenSysML+xHasEffect) || (!collapsed && len(inEffect) > 0)
	hasBody = d.boolOf(el, rdf.OpenSysML+xHasBody)
	// A braced effect is one anonymous action; a graph that wrote it as the
	// statements it holds cannot be read back as that action.
	if d.boolOf(el, rdf.OpenSysML+xBracedEffect) || (legacy && hasBody) {
		return nil, nil, false, false, &UnsupportedError{
			What: fmt.Sprintf("the transition %s", el.iri),
			Note: "its braced `do { … }` effect is written as its statements, not as the anonymous action the braces declare",
		}
	}
	if legacy {
		hasEffect, hasBody = true, false
	}
	for _, child := range children {
		switch {
		case legacy, inEffect[child.iri] && !inBody[child.iri]:
			effect = append(effect, child)
		case inBody[child.iri] && !inEffect[child.iri]:
			body = append(body, child)
		case inBody[child.iri]:
			return nil, nil, false, false, transitionLinkError(el, child.iri, "is linked as both effect and body")
		default:
			return nil, nil, false, false, transitionLinkError(el, child.iri, "is linked as neither effect nor body")
		}
		delete(inEffect, child.iri)
		delete(inBody, child.iri)
	}
	for iri := range inEffect {
		return nil, nil, false, false, transitionLinkError(el, iri, "is linked as an effect but is no member")
	}
	for iri := range inBody {
		return nil, nil, false, false, transitionLinkError(el, iri, "is linked as a body member but is no member")
	}
	return effect, body, hasEffect, hasBody, nil
}

// transitionObject reads a standard transition endpoint, then its legacy spelling.
func (d *decoder) transitionObject(el *element, property string) (rdf.Term, bool, error) {
	standard, hasStandard := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+property)
	var legacy string
	switch property {
	case pSource:
		legacy = pSourceFeature
	case pTarget:
		legacy = pTargetFeature
	default:
		return rdf.Term{}, false, nil
	}
	legacyTerm, hasLegacy := d.graph.Object(rdf.IRI(el.iri), rdf.SysML+legacy)
	if hasStandard && hasLegacy && standard != legacyTerm {
		return rdf.Term{}, false, &UnsupportedError{
			What: fmt.Sprintf("the transition <%s>", el.iri),
			Note: fmt.Sprintf("its sysml:%s and sysml:%s endpoints disagree", property, legacy),
		}
	}
	if hasStandard {
		return standard, true, nil
	}
	return legacyTerm, hasLegacy, nil
}

// transitionReferenceText renders a standard endpoint, falling back to legacy RDF.
func (d *decoder) transitionReferenceText(el *element, property, legacy string) (string, error) {
	_, standard, err := d.transitionObject(el, property)
	if err != nil {
		return "", err
	}
	if standard {
		return d.referenceText(el, rdf.SysML+property)
	}
	return d.referenceText(el, rdf.SysML+legacy)
}

// transitionLinkError refuses a transition whose effect and body links do not
// partition its members.
func transitionLinkError(el *element, member, fault string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the transition %s", el.iri),
		Note: fmt.Sprintf("%s %s, so its actions cannot be placed", member, fault),
	}
}

// effectMembership reports whether el is owned by a TransitionFeatureMembership
// of kind `effect`.
func (d *decoder) effectMembership(el *element) bool {
	m, owned := d.owningMembership[el.iri]
	if !owned || d.metaclass(rdf.IRI(m.iri)) != mTransitionFeatureMembership {
		return false
	}
	kind, _ := d.graph.Lexical(rdf.IRI(m.iri), rdf.SysML+pKind)
	return kind == "effect"
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
	children, err := d.bodyMembers(el)
	if err != nil {
		return "", err
	}
	return d.membersText(children, d.boolOf(el, rdf.OpenSysML+xHasBody), depth)
}

// membersText writes members one per line, in braces when braced, any lead
// lines ahead of them; an unbraced member continues the line the head is on,
// at its depth.
func (d *decoder) membersText(members []*element, braced bool, depth int, lead ...string) (string, error) {
	memberDepth := depth
	if braced {
		memberDepth = depth + 1
	}
	var b strings.Builder
	for _, line := range lead {
		b.WriteString(strings.Repeat("    ", memberDepth) + line + d.nl)
	}
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

// emptyActionUsage reports whether el is the ActionUsage an `entry;`, `do;` or
// `exit;` declares: nameless, without body, relationships or references.
func (d *decoder) emptyActionUsage(el *element) bool {
	if el.metaclass != usageMetaclass[ast.UsageAction] || len(d.identWords(el)) > 0 || len(el.children) > 0 {
		return false
	}
	subject := rdf.IRI(el.iri)
	for _, rel := range d.graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
		if !d.graph.BoolValue(rel, rdf.SysML+pIsImplied) && !impliedRelationshipMetaclasses[d.metaclass(rel)] {
			return false
		}
	}
	for _, property := range []string{pReferences, relationshipProperty[ast.RelTyping]} {
		if d.graph.HasProperty(subject, rdf.SysML+property) {
			return false
		}
	}
	return !d.graph.HasProperty(subject, rdf.OpenSysML+xExpression) && !d.boolOf(el, rdf.OpenSysML+xHasBody)
}
