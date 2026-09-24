package runtime

import (
	"errors"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ErrNoArgumentTyper reports a call reached on a semantic model with no argument
// typing installed: the run would select among overloads by arity alone, weaker
// than the checker did, so it selects nothing.
var ErrNoArgumentTyper = errors.New("no argument typing installed on the semantic model")

// selectCall is the declaration e calls in scope as the checker selects it, from the
// semantic model's typing of the arguments; ErrNoArgumentTyper when none is installed.
func (m *Model) selectCall(scope *symbols.Scope, e *ast.InvocationExpr, performs semantics.Performs) (*semantics.InvocationSelection, error) {
	if !m.semantics.HasArgumentTyper() {
		return nil, ErrNoArgumentTyper
	}
	return m.semantics.SelectCall(scope, e, performs), nil
}

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

	// librarySymbols memoizes the library declaration each qualified name denotes;
	// see librarySymbol.
	librarySymbols map[string]*symbols.Symbol

	// verificationCases memoizes the verification cases declared under a scope;
	// see verificationCasesIn.
	verificationCases map[*symbols.Scope][]*symbols.Symbol

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

	// census memoizes the object usages the model's namespaces declare; see modelUsages.
	census *usageCensus

	// behaving memoizes runsBehaviors per type; the model is fixed for the Model's life.
	behaving map[*symbols.Symbol]bool
	// behavingFeatures memoizes behavingParts and redefGroups redefinitionGroups, per type.
	behavingFeatures map[*symbols.Symbol][]int
	redefGroups      map[*symbols.Symbol][][]string
	// subsetters memoizes, per type, the features subsetting each named feature of it
	// under any of its redefinition names; callers read the shared slice.
	subsetters map[*symbols.Symbol]map[string][]EffectiveFeature
	// subsetted memoizes subsettedNames per feature of a type; callers read the shared slice.
	subsetted map[featureOfType][]string

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

	// triggerTypes memoizes the definition an accept's type reference denotes in the
	// scope it is written in, and signalMatches whether a signal conforms to one; a
	// machine judges every message in flight against every trigger it holds each step.
	triggerTypes  map[triggerTypeKey]*symbols.Symbol
	signalMatches map[signalMatchKey]bool

	// sources holds the text of the files the model was read from, by name, so an
	// error about a declaration can say where it was written. A file no caller
	// registered is reported by name and byte offset instead.
	sources map[string]*source.SourceFile

	// scopes holds the scope trees the caller resolves references in; declared
	// maps each declaration node to the symbol they declare for it, built on first use.
	scopes   []*symbols.Scope
	declared map[ast.Node]*symbols.Symbol

	// parse reads the notation text a run receives as text: a witness file's input
	// values and the units a tool answers in; installed by SetExpressionParser.
	parse ExpressionParser
}

// ExpressionParser parses text, read from origin, as exactly one expression; false for
// anything else. The caller that builds a Model supplies it; the runtime parses nothing itself.
type ExpressionParser func(origin, text string) (ast.Node, bool)

// ErrNoExpressionParser is the typed error a run returns on reaching notation text
// to read with no ExpressionParser installed on its Model.
var ErrNoExpressionParser = errors.New("no expression parser installed on the runtime model")

// SetExpressionParser installs the parser the Model reads witness input values and tool
// units with; units the previous parser read are forgotten, so every lookup goes through it.
func (m *Model) SetExpressionParser(parse ExpressionParser) {
	m.parse = parse
	clear(m.toolUnits)
}

// parseOneExpression reads text as exactly one expression with the installed parser; ok is
// false for text that is not one, err ErrNoExpressionParser when no parser is installed.
func (m *Model) parseOneExpression(origin, text string) (expr ast.Node, ok bool, err error) {
	if m.parse == nil {
		return nil, false, ErrNoExpressionParser
	}
	expr, ok = m.parse(origin, text)
	return expr, ok, nil
}

// NewModel builds the model-derived part of execution over a semantic model and
// the resolver it resolves names with; either may be nil for a context that
// evaluates literals alone. The caller installs the checker's argument typing on
// sem (semantics.Model.SetArgumentTyper) before any call is selected; a run that
// selects a call without one fails with ErrNoArgumentTyper. Contexts are built
// over it with NewContext.
func NewModel(sem *semantics.Model, resolver *resolve.Resolver) *Model {
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
		librarySymbols:      make(map[string]*symbols.Symbol),
		verificationCases:   make(map[*symbols.Scope][]*symbols.Symbol),
		libraryPerformances: make(map[*symbols.Symbol]*libraryPerformance),
		invocationTargets:   make(map[invocationKey]*invocationTarget),
		integerLiterals:     make(map[*ast.LiteralInteger]int64),
		realLiterals:        make(map[*ast.LiteralReal]float64),
		behaving:            make(map[*symbols.Symbol]bool),
		behavingFeatures:    make(map[*symbols.Symbol][]int),
		redefGroups:         make(map[*symbols.Symbol][][]string),
		subsetters:          make(map[*symbols.Symbol]map[string][]EffectiveFeature),
		subsetted:           make(map[featureOfType][]string),
		toolExecutions:      make(map[*symbols.Symbol]*toolExecution),
		toolUnits:           make(map[toolUnitKey]semantics.Unit),
		objectConns:         make(map[*symbols.Symbol][]lower.Connection),
		bindingIR:           make(map[*symbols.Symbol][]lower.Binding),
		bindingFeatures:     make(map[*symbols.Symbol]map[string][]lower.Binding),
		classifierBehaviors: make(map[*symbols.Symbol][]classifierBehaviorDecl),
		triggerTypes:        make(map[triggerTypeKey]*symbols.Symbol),
		signalMatches:       make(map[signalMatchKey]bool),
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
	m.census = nil
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
