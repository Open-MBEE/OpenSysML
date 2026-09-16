package passes

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DefaultRegistry returns the default pass registry: syntax, name resolution,
// state transitions, type checking, and semantic constraints.
func DefaultRegistry() *Registry {
	reg := NewRegistry()
	reg.Register(SyntaxPass{})
	reg.Register(GrammarViolationPass{})
	reg.Register(NonstandardNotationPass{})
	reg.Register(NameResolutionPass{})
	reg.Register(StateTransitionPass{})
	reg.Register(ActionEndpointPass{})
	reg.Register(TypeCheckPass{})
	reg.Register(TransitionGuardPass{})
	reg.Register(TriggerArgumentPass{})
	reg.Register(SendActionPass{})
	reg.Register(KerMLSubsettingMetaclassPass{})
	reg.Register(RedefinitionDirectionPass{})
	reg.Register(ElementFilterPass{})
	reg.Register(ConstraintPass{})
	reg.Register(VariantOwnerPass{})
	reg.Register(DocumentQueryPass{})
	reg.Register(DocumentPlanPass{})
	reg.Register(TypeRelationshipsPass{})
	reg.Register(W11EConjugatedSpecializationPass{})
	reg.Register(ImplicitBasePass{})
	reg.Register(MultiplicityBoundsPass{})
	reg.Register(ReferenceSubsettingPass{})
	reg.Register(TopLevelImportPass{})
	reg.Register(AssociationEndTypesPass{})
	reg.Register(VariableFeaturePass{})
	reg.Register(EndFeaturePass{})
	reg.Register(AssignmentReferentPass{})
	reg.Register(ResultExpressionPass{})
	reg.Register(FeatureReferencePass{})
	reg.Register(MetadataTypePass{})
	reg.Register(MetadataAnnotationPass{})
	reg.Register(AnnotationOwnershipPass{})
	reg.Register(W8DOccurrenceTypingPass{})
	reg.Register(W8DConnectorFeaturingPass{})
	reg.Register(W8DFlowEndPass{})
	reg.Register(W8DVariabilityPass{})
	reg.Register(W10BRelatedElementsPass{})
	reg.Register(W10BIndividualTypingPass{})
	reg.Register(W10BEndKindPass{})
	reg.Register(W10BStructuralPass{})
	reg.Register(W10BPortionOwnerPass{})
	reg.Register(W8DVerificationPass{})
	reg.Register(W8DViewRenderingPass{})
	reg.Register(W8DMetadataUsagePass{})
	reg.Register(IdentityMetadataPass{})
	reg.Register(DiagramLayoutPass{})
	reg.Register(RedefinitionConformancePass{})
	reg.Register(W9CShortNameDistinguishabilityPass{})
	reg.Register(W9CUserStandardLibraryPass{})
	reg.Register(W9CInheritedNameConflictPass{})
	reg.Register(W9CBoundFeatureTypesPass{})
	reg.Register(W11AUsageTypingPass{})
	reg.Register(W11AKerMLSpecializationPass{})
	reg.Register(ControlNodeSuccessionPass{})
	reg.Register(OOSEMMethodPass{})
	reg.Register(MOSAPass{})
	return reg
}

// Analyze runs the default validation passes over a single parsed document and
// returns diagnostics sorted by span offset, then source, then message.
// parseDiags are the parser's diagnostics, already adapted to passes.Diagnostic
// by the caller.
func Analyze(name string, root *ast.RootNamespace, parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic {
	return AnalyzeWithKind(name, source.KindOf(name), root, parseDiags, idx)
}

// AnalyzeWithKind validates a document whose language is not encoded in name,
// in the default conformance mode.
func AnalyzeWithKind(name string, kind source.Kind, root *ast.RootNamespace,
	parseDiags []Diagnostic, idx *symbols.Index) []Diagnostic {
	return AnalyzeWithOptions(name, kind, root, parseDiags, idx, Options{})
}

// dropEscalatedWarnings removes a parser warning a pass reports again as an
// error of the same code at the same span, so a finding is reported once.
func dropEscalatedWarnings(diags []Diagnostic) []Diagnostic {
	type finding struct {
		code   string
		offset int
	}
	escalated := map[finding]bool{}
	for _, d := range diags {
		if d.Severity == SeverityError {
			escalated[finding{d.Code, d.Span.Offset}] = true
		}
	}
	out := make([]Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.Severity == SeverityWarning && escalated[finding{d.Code, d.Span.Offset}] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// AnalyzeWithOptions validates a document under explicit analysis options.
func AnalyzeWithOptions(name string, kind source.Kind, root *ast.RootNamespace,
	parseDiags []Diagnostic, idx *symbols.Index, opts Options) []Diagnostic {
	return analyze(NewContextWithOptions(name, kind, idx, parseDiags, opts), root)
}

// PrepareBatch links what resolving every workspace document would write into
// its scope tree, so AnalyzeInBatch contexts only read the index. Call it alone,
// first. Every document, not only the batch's: the workspace-wide gathers read
// them all. The linker resolves as a context does, model attached.
func PrepareBatch(idx *symbols.Index, batch *Batch) {
	if idx == nil || batch == nil {
		return
	}
	linker := resolve.New(idx)
	attachModel(linker).SetSourceText(batch.Source)
	for _, name := range idx.WorkspaceDocuments() {
		linker.LinkMetadataBodies(name)
	}
}

// AnalyzeInBatch validates one document of a prepared batch in a context of its
// own; the result does not depend on which documents share the batch.
func AnalyzeInBatch(name string, kind source.Kind, root *ast.RootNamespace,
	parseDiags []Diagnostic, idx *symbols.Index, opts Options, batch *Batch) []Diagnostic {
	ctx := NewContextWithOptions(name, kind, idx, parseDiags, opts)
	ctx.Batch = batch
	if batch != nil {
		ctx.gathers = batch.Gathers
	}
	return analyze(ctx, root)
}

// AnalyzeShared validates a document over a resolver and model kept across
// analyses: what the run memoizes is owned by the document (see
// Resolver.InDocument), to be dropped when it or what it read changes.
func AnalyzeShared(name string, kind source.Kind, root *ast.RootNamespace,
	parseDiags []Diagnostic, opts Options, resolver *resolve.Resolver, model *semantics.Model,
	gathers *Gathers) []Diagnostic {
	ctx := NewContextWithOptions(name, kind, resolver.Index(), parseDiags, opts)
	ctx.Share(resolver, model, gathers)
	var diags []Diagnostic
	resolver.InDocument(name, func() { diags = analyze(ctx, root) })
	return diags
}

func analyze(ctx *Context, root *ast.RootNamespace) []Diagnostic {
	diags := dropEscalatedWarnings(DefaultRegistry().Run(ctx, ctx.Name, root))
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Span.Offset != b.Span.Offset {
			return a.Span.Offset < b.Span.Offset
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.Message < b.Message
	})
	return diags
}
