package passes

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
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
	}
	if ctx.Batch != nil {
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
			w.kermlRelationships(n.Relationships)
			w.sysmlDeclaration(n, n.Keyword)
			w.keywordAsName(n.Ident)
			w.walkDeclaration(n.Members, n)
		case *ast.Usage:
			w.kermlRelationships(n.Relationships)
			if n.CrossFeature != nil {
				w.kermlRelationships(n.CrossFeature.Relationships)
			}
			w.sysmlDeclaration(n, n.Keyword)
			w.keywordAsName(n.Ident)
			w.walkDeclaration(n.Members, n)
		case *ast.Import:
			w.expose(n)
			w.walk(n.Body)
		// An alias and a named multiplicity are the remaining members that parse
		// with a keyword for a name; the rest do not parse at all, so no span reaches here.
		case *ast.Alias:
			w.keywordAsName(n.Ident)
			w.walk(n.Body)
		case *ast.MultiplicityDecl:
			w.keywordAsName(n.Ident)
			w.walk(n.Members)
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
		case *ast.DeferMember:
			w.deferredMember(n)
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
	action, viewDef := w.inActionBody, w.inViewDefBody
	w.inActionBody = admitsActionBodyItems(declaration)
	w.inViewDefBody = isViewDefinition(declaration)
	w.bodies = append(w.bodies, bodyFrame{members: members})
	w.walk(members)
	w.bodies = w.bodies[:len(w.bodies)-1]
	w.inActionBody, w.inViewDefBody = action, viewDef
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
	action, viewDef := w.inActionBody, w.inViewDefBody
	w.inActionBody, w.inViewDefBody = true, false
	w.walk(members)
	w.inActionBody, w.inViewDefBody = action, viewDef
}

func isViewDefinition(node ast.Node) bool {
	def, ok := node.(*ast.Definition)
	return ok && def.Kind == ast.DefView
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
	w.walk(n.Defer)
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
	written := fmt.Sprintf("`%s %s;`", n.Keyword, n.Name)
	replacement := fmt.Sprintf("#%s state %s;", annotation, n.Name)
	needsImport := true
	if n.Name == annotation {
		// A member named like the annotation would shadow it, so the
		// metadata is spelled qualified and no import is needed.
		replacement = fmt.Sprintf("#StateMachines::%s state %s;", annotation, n.Name)
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

// deferredMember reports `defer <event>[, <event>]*;`, which the StateMachines
// library writes as one `#deferred ref : <event>;` per trigger.
func (w *notationWalker) deferredMember(n *ast.DeferMember) {
	refs := make([]string, 0, len(n.Triggers))
	spelled := make([]string, 0, len(n.Triggers))
	needsImport := false
	for _, trigger := range n.Triggers {
		name := deferredRefTarget(trigger)
		if name == "" {
			continue
		}
		spelled = append(spelled, triggerText(trigger))
		// A signal named `deferred` resolves ahead of the metadata, so the
		// annotation is spelled qualified for it and needs no import.
		if name == "deferred" {
			refs = append(refs, "`#StateMachines::deferred ref : deferred;`")
		} else {
			needsImport = true
			refs = append(refs, fmt.Sprintf("`#deferred ref : %s;`", name))
		}
	}
	if len(refs) == 0 {
		w.extension(keywordSpan(n, "defer"), "`defer <event>;`",
			"no notation states a deferred event")
		return
	}
	importNote := ""
	if needsImport {
		importNote = " (with `private import StateMachines::*;`)"
	}
	w.extensionFix(keywordSpan(n, "defer"), fmt.Sprintf(
		"`defer %s;` is an OpenSysML extension; write %s%s",
		strings.Join(spelled, ", "), strings.Join(refs, " and "), importNote),
		"DeferredMetadata", needsImport, diag.Replace(n.Span(), w.deferredRefLines(n, refs)))
}

// deferredRefLines spells the `#deferred ref` members a `defer` member rewrites
// as, one per trigger, each on its own line indented like the member when the
// source text can be read, else on one line.
func (w *notationWalker) deferredRefLines(n *ast.DeferMember, refs []string) string {
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		lines = append(lines, strings.Trim(ref, "`"))
	}
	if w.lookup != nil {
		return strings.Join(lines, "\n"+w.indentOf(n.Span().Offset))
	}
	return strings.Join(lines, " ")
}

// indentOf returns the leading whitespace of the line offset opens, or "" when
// the member sits mid-line.
func (w *notationWalker) indentOf(offset int) string {
	prefix := w.lookup(w.doc, source.Span{Offset: 0, Len: offset})
	line := prefix[strings.LastIndex(prefix, "\n")+1:]
	if strings.TrimLeft(line, " \t") != "" {
		return ""
	}
	return line
}

// deferredRefTarget names the occurrence a deferred trigger defers: the signal
// a bare name accepts, or the operation a call event invokes (its arguments are
// dropped, matching what `#deferred ref` carries).
func deferredRefTarget(trigger ast.Node) string {
	switch t := trigger.(type) {
	case *ast.QualifiedName:
		return qualifiedNameText(t)
	case *ast.CallEvent:
		return qualifiedNameText(t.Operation)
	}
	return ""
}

// triggerText spells a deferred trigger as it was written, for the message.
func triggerText(trigger ast.Node) string {
	switch t := trigger.(type) {
	case *ast.QualifiedName:
		return qualifiedNameText(t)
	case *ast.CallEvent:
		params := make([]string, 0, len(t.Parameters))
		for _, p := range t.Parameters {
			params = append(params, p.Text)
		}
		return fmt.Sprintf("%s(%s)", qualifiedNameText(t.Operation), strings.Join(params, ", "))
	}
	return ""
}

// qualifiedNameText spells a qualified name as `A::B::c`.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, p := range qn.Parts {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "::")
}

// extensionFix reports one construct as an OpenSysML extension and attaches the
// fix rewriting it: the member's own replacement plus the library import when
// needsImport and no enclosing body already makes the metadata visible.
func (w *notationWalker) extensionFix(span source.Span, message, metadata string, needsImport bool, replace diag.Edit) {
	edits := []diag.Edit{replace}
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
	if !w.sysml {
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
	if !w.sysml {
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
		Span:     keywordSpan(n, keyword),
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

// keywordSpan spans the notation that opens a node, so the diagnostic points at
// the word rather than the whole declaration.
func keywordSpan(n ast.Node, keyword string) source.Span {
	sp := n.Span()
	if sp.Len > len(keyword) {
		sp.Len = len(keyword)
	}
	return sp
}
