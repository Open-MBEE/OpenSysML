package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"sort"
)

// Model is the model-derived part of execution: the semantic model and resolver a
// run reads, and what is memoized from them — calc shapes, write targets,
// invocation targets, literal values, effective features, the behaviors a type
// binds — which the model fixes and no run changes. Every context built over one
// Model reuses it, so a run pays for none of it again; the run-derived state, what
// a run creates and a snapshot captures, is the Context's.
//
// The memo tables fill lazily into plain maps, as the resolver's and the semantic
// model's do, so a Model is for one goroutine at a time: one per analysis worker.
type Model struct {
	semantics *semantics.Model
	resolver  *resolve.Resolver

	features map[*symbols.Symbol][]EffectiveFeature

	// arrayFeatures memoizes the declarations of Collections::Array's features
	// by name; see arrayFeatureSymbols.
	arrayFeatures map[*symbols.Symbol]string

	// frameFeatures memoizes the declarations of the MeasurementReferences features
	// a coordinate frame, scale or transformation is read by; see frameFeatureSymbols.
	frameFeatures map[*symbols.Symbol]string

	// denotedFeatures memoizes, per type, the name of its feature each declared
	// feature symbol denotes on an object of that type: itself or a redefinition.
	denotedFeatures map[*symbols.Symbol]map[*symbols.Symbol]string

	// holders memoizes, per type, the features whose stated value lists each
	// named feature of the type.
	holders map[*symbols.Symbol]map[string][]string

	// returnedParams memoizes, per calc shape, the parameters its result passes on;
	// returnedStack is the shapes under analysis, returnedProvisional those awaiting
	// the root of their call cycle.
	returnedParams      map[*calcShape]*returnedAnalysis
	returnedStack       []*returnedAnalysis
	returnedProvisional []*returnedAnalysis

	// redefined memoizes, per feature of a type, the features it redefines
	// transitively; callers read the shared slice and never append to it.
	redefined map[featureOfType][]*symbols.Symbol

	// writeTargets memoizes the declaration an assignment's target names, per
	// scope the statement was written in: what a value written must conform to.
	writeTargets map[writeTargetKey]*writeTarget

	// calcShapes memoizes resolved calc invocation interfaces (parameters,
	// defaults, result expression) per calc symbol.
	calcShapes map[*symbols.Symbol]*calcShape

	// predicateShapes memoizes the invocation interfaces of constraints and
	// requirements applied as predicates.
	predicateShapes map[*symbols.Symbol]*calcShape

	// libraryPerformances memoizes, per model calc, the inherited library function a
	// call of it applies; nil for a calc that computes on its own.
	libraryPerformances map[*symbols.Symbol]*libraryPerformance

	// invocationTargets memoizes what each invocation expression denotes in the
	// scope it is evaluated in; the model does not change under one Model.
	invocationTargets map[invocationKey]*invocationTarget

	// integerLiterals and realLiterals memoize the value each numeric literal
	// node spells, so a literal in a recursion is parsed once per model.
	integerLiterals map[*ast.LiteralInteger]int64
	realLiterals    map[*ast.LiteralReal]float64

	// behaving memoizes runsBehaviors per type; the model is fixed for the Model's life.
	behaving map[*symbols.Symbol]bool
	// behavingFeatures memoizes behavingParts and redefGroups redefinitionGroups, per type.
	behavingFeatures map[*symbols.Symbol][]int
	redefGroups      map[*symbols.Symbol][][]string

	// toolExecutions memoizes toolExecutionOf per action; toolUnits the units tool
	// answers spell, per scope they are read in.
	toolExecutions map[*symbols.Symbol]*toolExecution
	toolUnits      map[toolUnitKey]semantics.Unit

	// objectConns memoizes the connections declared by each type an object is
	// of, which a behavior that object performs routes over.
	objectConns map[*symbols.Symbol][]lower.Connection

	// bindingIR memoizes binding connectors declared by each materialized
	// object type, including bindings inherited from its supertypes.
	bindingIR       map[*symbols.Symbol][]lower.Binding
	bindingFeatures map[*symbols.Symbol]map[string][]lower.Binding

	// classifierBehaviors memoizes the behaviors each type binds to its objects:
	// the machines it exhibits and the actions it performs.
	classifierBehaviors map[*symbols.Symbol][]classifierBehaviorDecl

	// sources holds the text of the files the model was read from, by name, so an
	// error about a declaration can say where it was written. A file no caller
	// registered is reported by name and byte offset instead.
	sources map[string]*source.SourceFile

	// scopes holds the scope trees the caller resolves references in; declared
	// maps each declaration node to the symbol they declare for it, built on first use.
	scopes   []*symbols.Scope
	declared map[ast.Node]*symbols.Symbol
}

// NewModel builds the model-derived part of execution over a semantic model and
// the resolver it resolves names with; either may be nil for a context that
// evaluates literals alone. Contexts are built over it with NewContext.
func NewModel(sem *semantics.Model, resolver *resolve.Resolver) *Model {
	if sem != nil {
		// Calls the model selects on its own (document queries, signal payloads) then
		// pick the overload the checker's argument typing picks.
		sem.SetArgumentTyper(passes.NewArgumentTyper(resolver, sem))
	}
	return &Model{
		semantics:           sem,
		resolver:            resolver,
		features:            make(map[*symbols.Symbol][]EffectiveFeature),
		denotedFeatures:     make(map[*symbols.Symbol]map[*symbols.Symbol]string),
		holders:             make(map[*symbols.Symbol]map[string][]string),
		returnedParams:      make(map[*calcShape]*returnedAnalysis),
		redefined:           make(map[featureOfType][]*symbols.Symbol),
		writeTargets:        make(map[writeTargetKey]*writeTarget),
		calcShapes:          make(map[*symbols.Symbol]*calcShape),
		predicateShapes:     make(map[*symbols.Symbol]*calcShape),
		libraryPerformances: make(map[*symbols.Symbol]*libraryPerformance),
		invocationTargets:   make(map[invocationKey]*invocationTarget),
		integerLiterals:     make(map[*ast.LiteralInteger]int64),
		realLiterals:        make(map[*ast.LiteralReal]float64),
		behaving:            make(map[*symbols.Symbol]bool),
		behavingFeatures:    make(map[*symbols.Symbol][]int),
		redefGroups:         make(map[*symbols.Symbol][][]string),
		toolExecutions:      make(map[*symbols.Symbol]*toolExecution),
		toolUnits:           make(map[toolUnitKey]semantics.Unit),
		objectConns:         make(map[*symbols.Symbol][]lower.Connection),
		bindingIR:           make(map[*symbols.Symbol][]lower.Binding),
		bindingFeatures:     make(map[*symbols.Symbol]map[string][]lower.Binding),
		classifierBehaviors: make(map[*symbols.Symbol][]classifierBehaviorDecl),
		sources:             make(map[string]*source.SourceFile),
	}
}

// Semantics returns the semantic model the Model is derived from.
func (m *Model) Semantics() *semantics.Model {
	return m.semantics
}

// Resolver returns the name resolver the Model resolves references with.
func (m *Model) Resolver() *resolve.Resolver {
	return m.resolver
}

// RegisterSource gives the Model the text of a file it was read from, so an
// error about a declaration in it reports a line and column.
func (m *Model) RegisterSource(sf *source.SourceFile) {
	if sf == nil {
		return
	}
	m.sources[sf.Name()] = sf
}

// Text answers a span in a document from the registered files, falling back to
// the notation lookup the semantic model reads documentation from.
func (m *Model) Text() source.Lookup {
	return source.TextOf(m.sources, m.semantics.SourceText())
}

// Sources returns the registered files of the model, in name order.
func (m *Model) Sources() []*source.SourceFile {
	files := make([]*source.SourceFile, 0, len(m.sources))
	for _, sf := range m.sources {
		files = append(files, sf)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	return files
}

// RegisterScope gives the Model a scope tree the caller resolves references in,
// so a declaration carried over by Adopt is rebound to the symbol that tree
// declares for it rather than to the index's own.
func (m *Model) RegisterScope(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	m.scopes = append(m.scopes, scope)
	m.declared = nil
}

// declaredSymbol is the symbol a registered scope tree declares for the
// declaration sym stands for, or sym itself when none does (a library declaration,
// or a Model resolving in the index's tree alone).
func (m *Model) declaredSymbol(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.Decl == nil || len(m.scopes) == 0 {
		return sym
	}
	if m.declared == nil {
		m.declared = make(map[ast.Node]*symbols.Symbol)
		for _, scope := range m.scopes {
			collectDeclared(scope, m.declared)
		}
	}
	if local, ok := m.declared[sym.Decl]; ok {
		return local
	}
	return sym
}
