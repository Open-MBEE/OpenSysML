package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// CodeNonstandardNotation marks notation OpenSysML accepts that no production of
// the pinned SysML v2 grammars admits.
const CodeNonstandardNotation = "nonstandard-notation"

// CodeKerMLNotation marks KerML notation used in a SysML file, where the SysML
// grammar has no production for it. Strict mode rejects it too: the pinned
// SysML grammar admits it nowhere.
const CodeKerMLNotation = "kerml-notation"

// CodeSysMLNotation marks SysML notation used in a KerML file, which the pinned
// KerML grammar admits nowhere.
const CodeSysMLNotation = "sysml-notation"

// CodeReservedKeywordName marks a recovered declared name spelled as a
// reserved keyword, which analysis rejects in every mode.
const CodeReservedKeywordName = "reserved-keyword-name"

// NonstandardNotationPass reports extension notation and recoverable grammar
// violations without gating later semantic analysis.
type NonstandardNotationPass struct{}

// Level reports the syntax level: the written notation is all it reads.
func (NonstandardNotationPass) Level() PassLevel { return LevelSyntax }

// Run walks the document for extension and language-specific notation, and
// under strict conformance escalates the parser's nonstandard-notation
// warnings — a /* */ comment where no member may start — to errors.
func (NonstandardNotationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic {
	if root == nil {
		return nil
	}
	// A document of no known kind — the REPL and CLI buffer — reads as SysML,
	// the notation its prompt takes; the REPL drops the finding for a snippet it
	// loaded from a .kerml file.
	w := &notationWalker{
		sysml:       ctx.Kind != source.KindKerML,
		severity:    notationSeverity(ctx.Options.Conformance),
		keywordName: keywordNameSpans(ctx.ParseDiagnostics),
		doc:         name,
		lookup:      ctx.Source,
		root:        ctx.Index.DocumentRoot(name),
		resolver:    ctx.Resolver(),
	}
	if w.lookup == nil && ctx.Batch != nil {
		w.lookup = ctx.Batch.Source
	}
	w.bodies = append(w.bodies, bodyFrame{members: root.Members, namespaceish: true})
	w.walk(root.Members)
	w.bodies = w.bodies[:0]
	// A comment where no member may start is the parser's own warning, which
	// strict conformance escalates; the escalated finding replaces the warning.
	if ctx.Options.Conformance.IsStrict() {
		for _, d := range ctx.ParseDiagnostics {
			if d.Severity == diag.SeverityWarning && d.Code == CodeNonstandardNotation {
				d.Severity = diag.SeverityError
				w.diags = append(w.diags, d)
			}
		}
	}
	// Notation errors describe the writing, not the recovered model's meaning.
	for i := range w.diags {
		w.diags[i].Notation = true
	}
	return w.diags
}

// notationSeverity maps the mode onto extension-notation severity.
func notationSeverity(mode diag.ConformanceMode) diag.Severity {
	if mode.IsStrict() {
		return diag.SeverityError
	}
	return diag.SeverityWarning
}

// notationWalker accumulates the diagnostics of one document.
type notationWalker struct {
	sysml bool
	// severity applies to mode-sensitive extension findings.
	severity diag.Severity
	diags    []diag.Diagnostic
	// inActionBody records that the body being walked admits ActionBodyItem
	// members (SysML.xtext:1367).
	inActionBody bool
	// inViewDefBody records that the body being walked is a ViewDefinitionBody
	// (SysML.xtext ViewDefinitionBodyItem), which admits no Expose.
	inViewDefBody bool
	// inViewBody records that the body being walked is a view definition's or
	// view usage's, neither of which admits a FramedConcernMember.
	inViewBody bool
	// inKerMLDeclaration counts the enclosing declarations already reported as
	// KerML notation; their members move with them, so are not reported again.
	inKerMLDeclaration int
	// keywordName holds the offsets where the parser recovered a keyword written as
	// a name, the only spans keywordAsName escalates.
	keywordName map[int]bool
	// doc names the document being walked; lookup reads its text when the run
	// carries a source lookup, which an indent-preserving fix needs.
	doc    string
	lookup source.Lookup
	// bodies stacks the member lists enclosing the member being walked, so a
	// fix can see the imports already written and where a new one belongs.
	bodies []bodyFrame
	// root and resolver read whether the name `StateMachines` still reaches the
	// library package, nil where the run carries no index.
	root     *symbols.Scope
	resolver *resolve.Resolver
}

// bodyFrame is one enclosing member list: the members it declares and whether
// it is the body of a namespace or package, where a fix places an import.
type bodyFrame struct {
	members      []ast.Node
	namespaceish bool
}

// keywordNameSpans collects where the parser recovered a keyword written as a name,
// which is the parser's own reading of the text rather than a re-derivation of it.
func keywordNameSpans(diags []diag.Diagnostic) map[int]bool {
	spans := map[int]bool{}
	for _, d := range diags {
		if d.Code == CodeReservedKeywordName {
			spans[d.Span.Offset] = true
		}
	}
	return spans
}

// hasParseError reports whether the parser errored on the document.
func hasParseError(diags []diag.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			return true
		}
	}
	return false
}

// walk reports the extension notation in a member list and descends into the
// bodies its members carry.
func (w *notationWalker) walk(members []ast.Node) {
	var previous ast.Node
	for _, member := range members {
		unwrapped := kit.UnwrapMembership(member)
		w.targetSuccession(unwrapped, previous)
		if !isMemberAttachedSuccession(unwrapped) {
			previous = unwrapped
		}
		switch n := unwrapped.(type) {
		case *ast.Namespace:
			w.kermlNamespace(n)
			w.keywordAsName(n.Ident)
			w.walkPackageMembers(n.Members)
		case *ast.Package:
			w.keywordAsName(n.Ident)
			w.walkPackageMembers(n.Members)
		case *ast.Definition:
			reported := w.kermlDeclaration(n, n.Keyword)
			w.kermlRelationships(n.Relationships)
			w.sysmlDeclaration(n, n.Keyword)
			w.keywordAsName(n.Ident)
			w.walkDeclaration(n.Members, n)
			w.leaveKerMLDeclaration(reported)
		case *ast.Usage:
			reported := w.kermlDeclaration(n, n.Keyword)
			w.kermlRelationships(n.Relationships)
			if n.CrossFeature != nil {
				w.kermlRelationships(n.CrossFeature.Relationships)
			}
			w.sysmlDeclaration(n, n.Keyword)
			w.keywordAsName(n.Ident)
			w.framedConcern(n)
			w.walkDeclaration(n.Members, n)
			w.leaveKerMLDeclaration(reported)
		case *ast.Import:
			w.expose(n)
			w.walk(n.Body)
		// An alias and a named multiplicity are the remaining members that parse
		// with a keyword for a name; the rest do not parse at all, so no span reaches here.
		case *ast.Alias:
			w.keywordAsName(n.Ident)
			w.walk(n.Body)
		case *ast.MultiplicityDecl:
			reported := w.kermlDeclaration(n, "multiplicity")
			w.keywordAsName(n.Ident)
			w.walk(n.Members)
			w.leaveKerMLDeclaration(reported)
		case *ast.ConstraintMember:
			w.walk(n.Body)
		case *ast.AssumeMember:
			w.walk(n.Body)
		case *ast.RequireMember:
			w.walk(n.Body)
		case *ast.StateNode:
			w.stateNode(n)
		case *ast.PseudostateNode:
			w.pseudostate(n)
		case *ast.InitialNode:
			// `first a then b { … }` ends in the succession's UsageBody (SysML.xtext:1698).
			w.walkDeclaration(n.Members, n)
		case *ast.DecisionNode:
			w.walkActionBody(n.Members)
		case *ast.TransitionMember:
			w.transition(n)
			w.walkActionBody(n.Effect)
			w.walkActionBody(n.Members)
		case *ast.SuccessionEdge:
			// `then b { … }` ends in the succession's UsageBody (SysML.xtext:1698).
			w.walkDeclaration(n.Members, n)
		case *ast.EntryMember:
			w.walkActionBody(n.Actions)
		case *ast.DoMember:
			w.walkActionBody(n.Actions)
		case *ast.ExitMember:
			w.walkActionBody(n.Actions)
		case *ast.IfActionNode:
			for _, branch := range n.Branches() {
				w.walkActionBody(branch.Body)
			}
		case *ast.WhileLoopActionNode:
			w.walkActionBody(n.Body)
		default:
			w.walkActionBody(ast.NodeBodyMembers(n))
		}
	}
}

// walkDeclaration walks the body of a declaration under the body kind that
// declaration opens.
func (w *notationWalker) walkDeclaration(members []ast.Node, declaration ast.Node) {
	action, viewDef, view := w.inActionBody, w.inViewDefBody, w.inViewBody
	w.inActionBody = admitsActionBodyItems(declaration)
	w.inViewDefBody = isViewDefinition(declaration)
	w.inViewBody = w.inViewDefBody || isViewUsage(declaration)
	w.bodies = append(w.bodies, bodyFrame{members: members})
	w.walk(members)
	w.bodies = w.bodies[:len(w.bodies)-1]
	w.inActionBody, w.inViewDefBody, w.inViewBody = action, viewDef, view
}

// walkPackageMembers walks a namespace or package body, the member lists a fix
// may add an import to.
func (w *notationWalker) walkPackageMembers(members []ast.Node) {
	w.bodies = append(w.bodies, bodyFrame{members: members, namespaceish: true})
	w.walk(members)
	w.bodies = w.bodies[:len(w.bodies)-1]
}

// walkActionBody walks the body of an action node, which is an ActionBody
// wherever the node itself is written.
func (w *notationWalker) walkActionBody(members []ast.Node) {
	if len(members) == 0 {
		return
	}
	action, viewDef, view := w.inActionBody, w.inViewDefBody, w.inViewBody
	w.inActionBody, w.inViewDefBody, w.inViewBody = true, false, false
	w.walk(members)
	w.inActionBody, w.inViewDefBody, w.inViewBody = action, viewDef, view
}

func isViewDefinition(node ast.Node) bool {
	def, ok := node.(*ast.Definition)
	return ok && def.Kind == ast.DefView
}

func isViewUsage(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	return ok && usage.Kind == ast.UsageView
}

// admitsActionBodyItems reports whether the body a declaration opens is an
// ActionBody (SysML.xtext:1361) or one that includes ActionBodyItem: a
// CalculationBody (:1947) or a CaseBody (:2183).
func admitsActionBodyItems(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Definition:
		switch n.Kind {
		case ast.DefAction, ast.DefCalc, ast.DefConstraint, ast.DefBehavior, ast.DefPredicate:
			return true
		case ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	case *ast.Usage:
		switch n.Kind {
		case ast.UsageAction, ast.UsageStep, ast.UsageCalc, ast.UsageExpr, ast.UsageConstraint:
			return true
		case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		case ast.UsageTransition, ast.UsagePredicate, ast.UsageBehavior:
			return true
		}
	}
	return false
}

// targetSuccession reports a one-name `then <target>;`, `if <guard> then
// <target>;` or `else <target>;` whose preceding member the grammar admits no
// target succession after. ActionBodyItem (SysML.xtext:1367) hangs
// TargetSuccessionMember off an ActionNodeMember, a BehaviorUsageMember or an
// InitialNodeMember alone, so after a `succession`, a comment or a structural
// usage the pilot has no production for it and its source is unstated.
func (w *notationWalker) targetSuccession(n, previous ast.Node) {
	keyword := targetSuccessionKeyword(n)
	if keyword == "" || !w.sysml || previous == nil || admitsTargetSuccession(previous) {
		return
	}
	notation := "`" + keyword + " <target>;`"
	if keyword == "if" {
		notation = "`if <guard> then <target>;`"
	}
	w.extension(keywordSpan(n, keyword), notation+" after a member that is not an action node",
		"a target succession sequences from the member written right before it, so only an action node, "+
			"a behavior usage, a one-ended `first <node>;` or another target succession may precede it")
}

// isMemberAttachedSuccession reports the edge a member-attached `then` (`then
// action a;`) desugars to, which is no member the author wrote between the two
// it sequences.
func isMemberAttachedSuccession(n ast.Node) bool {
	edge, ok := n.(*ast.SuccessionEdge)
	return ok && edge.SourceImplied && edge.TargetImplied
}

// targetSuccessionKeyword is the keyword that opens a one-name target succession,
// "" for any other member: an edge naming both ends, or the edge a
// member-attached `then` desugars to, states its own source.
func targetSuccessionKeyword(n ast.Node) string {
	switch edge := n.(type) {
	case *ast.SuccessionEdge:
		if edge.TargetImplied || !(edge.SourceImplied || edge.SourceMember != nil) {
			return ""
		}
		return "then"
	case *ast.ControlFlowEdge:
		if edge.TargetImplied || !(edge.SourceImplied || edge.SourceMember != nil) {
			return ""
		}
		if edge.IsElse {
			return "else"
		}
		return "if"
	}
	return ""
}

// admitsTargetSuccession reports whether the grammar admits a target succession
// right after the member: an action node, a behavior usage, a one-ended
// `first <node>;` or a target succession continuing the same chain.
func admitsTargetSuccession(previous ast.Node) bool {
	switch n := previous.(type) {
	case *ast.InitialNode:
		return n.Successor == nil
	case *ast.SuccessionEdge, *ast.ControlFlowEdge:
		return targetSuccessionKeyword(n) != ""
	case *ast.TransitionMember:
		// A sourceless transition is a TargetTransitionUsage or an entry
		// transition, which chains like a target succession.
		return n.Source == nil
	case *ast.Usage:
		return isBehaviorUsage(n.Kind)
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode,
		*ast.AssignmentActionNode, *ast.PerformActionNode, *ast.WhileLoopActionNode, *ast.IfActionNode,
		*ast.SendStatement, *ast.TerminateStatement, *ast.AcceptActionUsage,
		*ast.StateNode, *ast.SubstateMember, *ast.EntryMember, *ast.DoMember, *ast.ExitMember, *ast.PseudostateNode:
		return true
	}
	return false
}

// isBehaviorUsage reports whether a usage of the kind is a BehaviorUsageElement
// (SysML.xtext:679), the usages an action body sequences by position.
func isBehaviorUsage(kind ast.UsageKind) bool {
	switch kind {
	case ast.UsageAction, ast.UsageCalc, ast.UsageState, ast.UsageConstraint, ast.UsageRequirement,
		ast.UsageConcern, ast.UsageFramedConcern, ast.UsageViewpoint, ast.UsageSatisfy, ast.UsageObjective,
		ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
		ast.UsageStep, ast.UsageExpr, ast.UsageBehavior, ast.UsagePredicate, ast.UsageBool:
		return true
	}
	return false
}

// expose reports an `expose` in a view def body: Expose is a ViewBodyItem alone
// (SysML.xtext), and every other body rejects it in the parser.
func (w *notationWalker) expose(n *ast.Import) {
	if !w.inViewDefBody || !n.IsExpose {
		return
	}
	w.extension(keywordSpan(n, "expose"), "`expose` in a view def body",
		"only a view usage body admits Expose; a view def body states what it renders and filters")
}

// framedConcern reports a `frame` in a view body: FramedConcernMember belongs to
// requirement, concern and viewpoint bodies only (SysML v2 §8.3.20, §8.3.26).
func (w *notationWalker) framedConcern(n *ast.Usage) {
	if !w.inViewBody || n.Kind != ast.UsageFramedConcern {
		return
	}
	w.extension(w.declarationKeywordSpan(n, "frame"), "`frame` in a view body",
		"only a requirement, concern or viewpoint body frames a concern; a view is checked against the concerns the viewpoints it satisfies frame")
}

// transition reports a `transition` in an action body: only `succession … if …`
// is standard there (SysML.xtext GuardedSuccession); TransitionUsageMember is a StateBodyItem.
func (w *notationWalker) transition(n *ast.TransitionMember) {
	if !w.inActionBody || n.IsSuccession {
		return
	}
	w.extension(keywordSpan(n, "transition"), "`transition` in an action body",
		"only a state body admits TransitionUsageMember; an action body writes a guarded `succession … if … then …`")
}

// stateNode descends into the members of a state.
func (w *notationWalker) stateNode(n *ast.StateNode) {
	w.walkActionBody(n.Entry)
	w.walkActionBody(n.Do)
	w.walkActionBody(n.Exit)
	w.walk(n.Substates)
	for _, region := range n.Regions {
		w.walk([]ast.Node{region})
	}
}

// pseudostate reports the pseudostates that are ours. `fork` and `join` are not
// among them: both are action node literals a state body admits.
func (w *notationWalker) pseudostate(n *ast.PseudostateNode) {
	switch n.Kind {
	case ast.PseudostateFork, ast.PseudostateJoin:
		return
	}
	annotation, ok := pseudostateAnnotations[n.Kind]
	if !ok {
		return
	}
	name := source.NameText(n.Name)
	written := fmt.Sprintf("`%s %s;`", n.Keyword, name)
	replacement := fmt.Sprintf("#%s state %s;", annotation, name)
	needsImport := true
	switch {
	case w.shadowsStateMachines():
		// A `StateMachines` member hides the library package, so the
		// metadata is spelled from the root and no import is needed.
		replacement = fmt.Sprintf("#$::StateMachines::%s state %s;", annotation, name)
		needsImport = false
	case n.Name == annotation || w.shadowsAnnotation(annotation):
		// A member named like the annotation would shadow it, so the
		// metadata is spelled qualified and no import is needed.
		replacement = fmt.Sprintf("#StateMachines::%s state %s;", annotation, name)
		needsImport = false
	}
	importNote := ""
	if needsImport {
		importNote = " (with `private import StateMachines::*;`)"
	}
	w.extensionFix(keywordSpan(n, n.Keyword), fmt.Sprintf(
		"%s is an OpenSysML extension; write `%s`%s",
		written, replacement, importNote), annotation+"Metadata", needsImport,
		diag.Replace(n.Span(), replacement))
}

// pseudostateAnnotations names the StateMachines metadata definition a
// keyword-spelled pseudostate rewrites as.
var pseudostateAnnotations = map[ast.PseudostateKind]string{
	ast.PseudostateChoice:         "choice",
	ast.PseudostateJunction:       "junction",
	ast.PseudostateShallowHistory: "shallowHistory",
	ast.PseudostateDeepHistory:    "deepHistory",
}

// shadowsAnnotation reports whether a member of an enclosing body declares
// keyword as its name, which the written annotation would resolve to first.
func (w *notationWalker) shadowsAnnotation(keyword string) bool {
	for _, body := range w.bodies {
		for _, member := range body.members {
			if memberDeclaredName(kit.UnwrapMembership(member)) == keyword {
				return true
			}
		}
	}
	return false
}

// shadowsStateMachines reports whether a `StateMachines` member hides the
// library package the annotation spellings name: one declared in an enclosing
// body, or one the name resolves to that is not it.
func (w *notationWalker) shadowsStateMachines() bool {
	if w.shadowsAnnotation("StateMachines") {
		return true
	}
	if w.root == nil || w.resolver == nil {
		return false
	}
	sym, ok := w.resolver.ReadQualified(w.root, ast.QualifiedNameOf("StateMachines")).Symbol()
	return ok && sym != nil && symbols.FQNOf(sym) != "StateMachines"
}

// memberDeclaredName is the name a member declares, "" when it declares none.
func memberDeclaredName(member ast.Node) string {
	switch n := member.(type) {
	case *ast.Namespace:
		return n.Ident.Name
	case *ast.Package:
		return n.Ident.Name
	case *ast.Alias:
		return n.Ident.Name
	case *ast.PrefixMetadata:
		return n.Ident.Name
	case *ast.MultiplicityDecl:
		return n.Ident.Name
	case *ast.SubjectMember:
		return n.Ident.Name
	case *ast.CrossFeatureMember:
		return n.Ident.Name
	case *ast.AssumeMember:
		return n.Ident.Name
	case *ast.RequireMember:
		return n.Ident.Name
	case *ast.Definition:
		return n.Ident.Name
	case *ast.Usage:
		name, _ := ast.EffectiveName(n)
		return name
	case *ast.PseudostateNode:
		return n.Name
	case *ast.InitialNode:
		return n.Name()
	case *ast.SubstateMember:
		return n.Name
	case *ast.TransitionMember:
		return n.Name
	}
	return ""
}

// qualifiedNameText spells a qualified name as `A::B::c`, quoting each
// segment that cannot be written as a basic name.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, p := range qn.Parts {
		parts = append(parts, p.Text)
	}
	return source.QualifiedNameOf(parts)
}

// extensionFix reports one construct as an OpenSysML extension and attaches the
// fix rewriting it: the member's own replacement plus the library import when
// needsImport and no enclosing body already makes the metadata visible.
func (w *notationWalker) extensionFix(span source.Span, message, metadata string, needsImport bool, edits ...diag.Edit) {
	if needsImport {
		if edit, ok := w.stateMachinesImport(metadata); ok {
			edits = append(edits, edit)
		}
	}
	w.diags = append(w.diags, diag.Diagnostic{
		Severity: w.severity,
		Span:     span,
		Message:  message,
		Code:     CodeNonstandardNotation,
		Source:   "syntax",
		Fixes: []diag.Fix{{
			Title:     "Rewrite as standard `StateMachines` metadata notation",
			Edits:     edits,
			Preferred: true,
		}},
	})
}

// stateMachinesImport is the edit that inserts `private import StateMachines::*;`
// as the first member of the innermost enclosing namespace body, nil when an
// import already in scope covers the needed metadata definition.
func (w *notationWalker) stateMachinesImport(metadata string) (diag.Edit, bool) {
	for _, body := range w.bodies {
		for _, member := range body.members {
			imp, ok := kit.UnwrapMembership(member).(*ast.Import)
			if !ok || imp.IsExpose {
				continue
			}
			target := qualifiedNameText(imp.Imported)
			if target == "StateMachines" && (imp.Kind == ast.ImportNamespace || imp.IsRecursive) ||
				target == "StateMachines::"+metadata {
				return diag.Edit{}, false
			}
		}
	}
	for i := len(w.bodies) - 1; i >= 0; i-- {
		body := w.bodies[i]
		if !body.namespaceish || len(body.members) == 0 {
			continue
		}
		return diag.InsertLine(body.members[0].Span().Offset,
			"private import StateMachines::*;"), true
	}
	return diag.Edit{}, false
}

// kermlNamespace reports a `namespace` declaration in a SysML file, whose root
// production admits package members only.
func (w *notationWalker) kermlNamespace(n *ast.Namespace) {
	if !w.sysml || w.inKerMLDeclaration > 0 {
		return
	}
	w.diags = append(w.diags, diag.Diagnostic{
		Severity: w.severity,
		Span:     keywordSpan(n, "namespace"),
		Message: "`namespace` is KerML notation: the SysML v2 grammar has no namespace declaration, " +
			"so write `package` here or move the declaration to a .kerml file",
		Code:   CodeKerMLNotation,
		Source: "syntax",
	})
}

// kermlRelationships reports a `featured by` clause in a SysML file: the
// featuring relationship is KerML.xtext:569 only, absent from SysML.xtext.
func (w *notationWalker) kermlRelationships(rels []*ast.Relationship) {
	if !w.sysml || w.inKerMLDeclaration > 0 {
		return
	}
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if rel.Kind == ast.RelFeaturedBy {
			w.diags = append(w.diags, diag.Diagnostic{
				Severity: w.severity,
				Span:     rel.Span(),
				Message: "`featured by` is KerML notation: the SysML v2 grammar has no featuring clause, " +
					"so move the declaration to a .kerml file",
				Code:   CodeKerMLNotation,
				Source: "syntax",
			})
			continue
		}
		if clause, ok := kermlRelationshipClauses[rel.Kind]; ok {
			w.diags = append(w.diags, diag.Diagnostic{
				Severity: w.severity,
				Span:     rel.Span(),
				Message: fmt.Sprintf("`%s` is KerML notation: the SysML v2 grammar has no %s clause, "+
					"so move the declaration to a .kerml file", clause.spelling, clause.name),
				Code:   CodeKerMLNotation,
				Source: "syntax",
			})
		}
	}
}

// kermlRelationshipClauses names the FeatureRelationshipPart and
// TypeRelationshipPart clauses KerML.xtext admits and SysML.xtext does not, so
// a SysML file carrying one is KerML notation.
var kermlRelationshipClauses = map[ast.RelationshipKind]struct {
	spelling string
	name     string
}{
	ast.RelDisjoint:    {"disjoint from", "disjoining"},
	ast.RelUnions:      {"unions", "unioning"},
	ast.RelIntersects:  {"intersects", "intersecting"},
	ast.RelDifferences: {"differences", "differencing"},
	ast.RelChains:      {"chains", "chaining"},
	ast.RelInverseOf:   {"inverse of", "inverting"},
}

// kermlDeclarationAlternatives names the SysML v2 declaration that takes the
// place of a KerML-only one; a keyword outside the map (`interaction`,
// `multiplicity`) has no SysML spelling and can only move to a .kerml file.
var kermlDeclarationAlternatives = map[string]string{
	"assoc":         "`connection def`",
	"assoc struct":  "`connection def`",
	"behavior":      "`action def`",
	"bool":          "`constraint`",
	"class":         "`occurrence def`",
	"classifier":    "a `… def` form such as `part def`",
	"connector":     "`connection` or `connect` (or `binding` for a binding connector)",
	"datatype":      "`attribute def`",
	"expr":          "`calc`",
	"feature":       "a usage keyword such as `attribute`, `part` or `ref`",
	"function":      "`calc def`",
	"inv":           "`constraint` or `assert constraint`",
	"metaclass":     "`metadata def`",
	"predicate":     "`constraint def`",
	"step":          "`action`",
	"struct":        "`item def` or `part def`",
	"subclassifier": "`specializes` on the definition itself",
}

// isKerMLOnlyDeclarationKeyword reports whether a declaration keyword is a
// literal of the pinned KerML grammar that the pinned SysML grammar does not
// spell, read from the per-language keyword sets of source. A compound keyword
// (`assoc struct`) is classified by its leading word.
func isKerMLOnlyDeclarationKeyword(keyword string) bool {
	word, _, _ := strings.Cut(keyword, " ")
	return source.IsKeywordIn(word, source.KindKerML) && !source.IsKeywordIn(word, source.KindSysML)
}

// kermlDeclaration reports a declaration whose keyword is KerML notation in a
// SysML file — `connector`, `class`, `feature`, `inv`, … — and reports whether
// it did, so the walk can withhold the findings its members would repeat. The
// declaration stays parsed for editor and analysis consumers; no fix rewrites
// it, since a keyword swap changes the declared kind.
func (w *notationWalker) kermlDeclaration(n ast.Node, keyword string) bool {
	if !w.sysml || w.inKerMLDeclaration > 0 || keyword == "" || !isKerMLOnlyDeclarationKeyword(keyword) {
		return false
	}
	remedy := "move the declaration to a .kerml file"
	if alternative, ok := kermlDeclarationAlternatives[keyword]; ok {
		remedy = "write " + alternative + " here or " + remedy
	}
	w.diags = append(w.diags, diag.Diagnostic{
		Severity: w.severity,
		Span:     w.declarationKeywordSpan(n, keyword),
		Message: fmt.Sprintf("`%s` is KerML notation: the SysML v2 grammar has no %s declaration, so %s",
			keyword, keyword, remedy),
		Code:   CodeKerMLNotation,
		Source: "syntax",
	})
	w.inKerMLDeclaration++
	return true
}

// leaveKerMLDeclaration closes the scope kermlDeclaration opened when it reported.
func (w *notationWalker) leaveKerMLDeclaration(reported bool) {
	if reported {
		w.inKerMLDeclaration--
	}
}

// kermlDeclarationKeywords are the definition and usage keywords the pinned
// KerML grammar spells; a kind keyword outside the set is SysML-only.
var kermlDeclarationKeywords = map[string]bool{
	"assoc": true, "assoc struct": true, "behavior": true, "binding": true, "bool": true,
	"class": true, "classifier": true, "connector": true, "datatype": true,
	"dependency": true, "expr": true, "feature": true, "flow": true,
	"function": true, "interaction": true, "inv": true, "metaclass": true,
	"metadata": true, "multiplicity": true, "predicate": true, "step": true,
	"struct": true, "succession": true, "type": true,
}

// sysmlDeclaration rejects a declaration keyword no KerML production spells
// while retaining the parsed declaration for editor and analysis consumers.
func (w *notationWalker) sysmlDeclaration(n ast.Node, keyword string) {
	if w.sysml || keyword == "" || kermlDeclarationKeywords[keyword] {
		return
	}
	w.diags = append(w.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     w.declarationKeywordSpan(n, keyword),
		Message: fmt.Sprintf("`%s` is SysML notation: the KerML grammar has no such declaration keyword, "+
			"so move the declaration to a .sysml file", keyword),
		Code:   CodeSysMLNotation,
		Source: "syntax",
	})
}

// keywordAsName rejects a recovered declared name the ID terminal excludes.
// The parser keeps the declaration available to editors and later analysis.
func (w *notationWalker) keywordAsName(id ast.Identification) {
	if id.Name == "" || id.NameSpan.Len != len(id.Name) {
		return
	}
	if !w.keywordName[id.NameSpan.Offset] {
		return
	}
	w.diags = append(w.diags, diag.Diagnostic{
		Severity: diag.SeverityError,
		Span:     id.NameSpan,
		Message: fmt.Sprintf("%q is a reserved keyword, not a name the ID terminal admits; "+
			"write '%s' to use it as a name", id.Name, id.Name),
		Code:   CodeReservedKeywordName,
		Source: "syntax",
	})
}

// extension reports one construct as an OpenSysML extension.
func (w *notationWalker) extension(span source.Span, construct, standard string) {
	w.diags = append(w.diags, diag.Diagnostic{
		Severity: w.severity,
		Span:     span,
		Message: fmt.Sprintf("%s is an OpenSysML extension with no SysML v2 production: %s",
			construct, standard),
		Code:   CodeNonstandardNotation,
		Source: "syntax",
	})
}

// declarationKeywordSpan spans the kind keyword of a declaration: its first
// keyword tokens spelling the (possibly compound) keyword outside any prefix
// metadata, past modifiers (`abstract`, `in`), comments and prefix names that
// repeat it (`#'class' class C;`, `#M::class class C;`). Without the source
// text the span falls back to the word after the prefixes the declaration
// opens with.
func (w *notationWalker) declarationKeywordSpan(n ast.Node, keyword string) source.Span {
	sp := n.Span()
	prefixes, _, _ := ast.DeclaredMetadata(n)
	if w.lookup != nil {
		words := strings.Fields(keyword)
		lx := lexer.New(source.New(w.doc, []byte(w.lookup(w.doc, sp))))
		var span source.Span
		matched := 0
		for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
			if tok.IsTrivia() {
				continue
			}
			at := source.Span{Offset: sp.Offset + tok.Span.Offset, Len: tok.Span.Len}
			if tok.Kind != lexer.Keyword || tok.KeywordID != words[matched] || insidePrefixMetadata(prefixes, at) {
				matched = 0
				continue
			}
			if matched == 0 {
				span = at
			}
			if matched++; matched == len(words) {
				span.Len = at.End() - span.Offset
				return span
			}
		}
	}
	if len(prefixes) > 0 && prefixes[0].Span().Offset == sp.Offset {
		if end := prefixes[len(prefixes)-1].Span().End(); end < sp.End() {
			sp.Len = sp.End() - end
			sp.Offset = end
		}
	}
	return keywordSpan(&ast.NodeBase{NodeSpan: sp}, keyword)
}

func insidePrefixMetadata(prefixes []*ast.PrefixMetadata, word source.Span) bool {
	for _, pm := range prefixes {
		if pm.Span().Contains(word) {
			return true
		}
	}
	return false
}

// keywordSpan spans the notation that opens a node, so the diagnostic points at
// the word rather than the whole declaration.
func keywordSpan(n ast.Node, keyword string) source.Span {
	sp := n.Span()
	if sp.Len > len(keyword) {
		sp.Len = len(keyword)
	}
	return sp
}
