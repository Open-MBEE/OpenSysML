package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
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

// Run walks the document for extension and language-specific notation.
func (NonstandardNotationPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if root == nil {
		return nil
	}
	// A document of no known kind — the REPL and CLI buffer — reads as SysML,
	// the notation its prompt takes; the REPL drops the finding for a snippet it
	// loaded from a .kerml file.
	w := &notationWalker{
		sysml:       ctx.Kind != source.KindKerML,
		severity:    notationSeverity(ctx.Options.Conformance),
		parsedClean: !hasParseError(ctx.ParseDiagnostics),
		keywordName: keywordNameSpans(ctx.ParseDiagnostics),
	}
	w.walk(root.Members)
	// Notation errors describe the writing, not the recovered model's meaning.
	for i := range w.diags {
		w.diags[i].Notation = true
	}
	return w.diags
}

// notationSeverity maps the mode onto extension-notation severity.
func notationSeverity(mode conformance.Mode) Severity {
	if mode.IsStrict() {
		return SeverityError
	}
	return SeverityWarning
}

// notationWalker accumulates the diagnostics of one document.
type notationWalker struct {
	sysml bool
	// severity applies to mode-sensitive extension findings.
	severity          Severity
	diags             []Diagnostic
	inRequirementBody bool
	// inActionBody records that the body being walked admits ActionBodyItem
	// members (SysML.xtext:1367).
	inActionBody bool
	// inViewDefBody records that the body being walked is a ViewDefinitionBody
	// (SysML.xtext ViewDefinitionBodyItem), which admits no Expose.
	inViewDefBody bool
	// parsedClean records that the document parsed without an error, which a
	// finding needs when recovery can shape the tree it reads (see initialNode).
	parsedClean bool
	// keywordName holds the offsets where the parser recovered a keyword written as
	// a name, the only spans keywordAsName escalates.
	keywordName map[int]bool
}

// keywordNameSpans collects where the parser recovered a keyword written as a name,
// which is the parser's own reading of the text rather than a re-derivation of it.
func keywordNameSpans(diags []Diagnostic) map[int]bool {
	spans := map[int]bool{}
	for _, d := range diags {
		if d.Code == CodeReservedKeywordName {
			spans[d.Span.Offset] = true
		}
	}
	return spans
}

// hasParseError reports whether the parser errored on the document.
func hasParseError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// walk reports the extension notation in a member list and descends into the
// bodies its members carry.
func (w *notationWalker) walk(members []ast.Node) {
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.Namespace:
			w.kermlNamespace(n)
			w.keywordAsName(n.Ident)
			w.walk(n.Members)
		case *ast.Package:
			w.keywordAsName(n.Ident)
			w.walk(n.Members)
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
			w.requirementConstraint(n)
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
			w.extension(keywordSpan(n, "defer"), "`defer <event>;`",
				"no notation states a deferred event")
		case *ast.InitialNode:
			w.initialNode(n)
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
	requirement, action, viewDef := w.inRequirementBody, w.inActionBody, w.inViewDefBody
	w.inRequirementBody = isRequirementBodyDeclaration(declaration)
	w.inActionBody = admitsActionBodyItems(declaration)
	w.inViewDefBody = isViewDefinition(declaration)
	w.walk(members)
	w.inRequirementBody, w.inActionBody, w.inViewDefBody = requirement, action, viewDef
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

func isRequirementBodyDeclaration(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Definition:
		switch n.Kind {
		case ast.DefRequirement:
			return true
		case ast.DefConcern, ast.DefViewpoint:
			return true
		}
	case *ast.Usage:
		switch n.Kind {
		case ast.UsageRequirement, ast.UsageSatisfy:
			return true
		case ast.UsageConcern, ast.UsageViewpoint, ast.UsageFramedConcern, ast.UsageObjective:
			return true
		}
	}
	return false
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

// initialNode reports a one-ended `first <node>;` outside an action body:
// InitialNodeMember is reachable from ActionBodyItem alone (SysML.xtext:1376),
// never from DefinitionBodyItem (:516).
func (w *notationWalker) initialNode(n *ast.InitialNode) {
	// A recovered `first <source> then <target>` reads as a one-ended node, so
	// only a document that parsed cleanly is judged here.
	if !w.inActionBody && n.Successor == nil && w.parsedClean {
		w.extension(keywordSpan(n, "first"), "a one-ended `first <node>;` outside an action body",
			"only an action body admits it; elsewhere a succession names both ends, `first <source> then <target>`")
	}
}

// requirementConstraint reports an `assume`/`require` member outside a requirement
// body, where RequirementConstraintMember alone admits it (SysML.xtext:2039).
func (w *notationWalker) requirementConstraint(n *ast.Usage) {
	if w.inRequirementBody || n.Kind != ast.UsageConstraint {
		return
	}
	keyword := n.PrefixKeyword
	if keyword != "assume" && keyword != "require" {
		keyword = n.Keyword
	}
	if keyword != "assume" && keyword != "require" {
		return
	}
	w.extension(keywordSpan(n, keyword), fmt.Sprintf("`%s` outside a requirement body", keyword),
		"only a requirement, concern, viewpoint or objective body admits it")
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
	w.extension(keywordSpan(n, n.Keyword), fmt.Sprintf("`%s <name>;`", n.Keyword),
		"the grammars define no pseudostate notation")
}

// kermlNamespace reports a `namespace` declaration in a SysML file, whose root
// production admits package members only.
func (w *notationWalker) kermlNamespace(n *ast.Namespace) {
	if !w.sysml {
		return
	}
	w.diags = append(w.diags, Diagnostic{
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
		if rel == nil || rel.Kind != ast.RelFeaturedBy {
			continue
		}
		w.diags = append(w.diags, Diagnostic{
			Severity: w.severity,
			Span:     rel.Span(),
			Message: "`featured by` is KerML notation: the SysML v2 grammar has no featuring clause, " +
				"so move the declaration to a .kerml file",
			Code:   CodeKerMLNotation,
			Source: "syntax",
		})
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
	w.diags = append(w.diags, Diagnostic{
		Severity: SeverityError,
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
	w.diags = append(w.diags, Diagnostic{
		Severity: SeverityError,
		Span:     id.NameSpan,
		Message: fmt.Sprintf("%q is a reserved keyword, not a name the ID terminal admits; "+
			"write '%s' to use it as a name", id.Name, id.Name),
		Code:   CodeReservedKeywordName,
		Source: "syntax",
	})
}

// extension reports one construct as an OpenSysML extension.
func (w *notationWalker) extension(span source.Span, construct, standard string) {
	w.diags = append(w.diags, Diagnostic{
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
