package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// PassLevel is the dependency tier of a validation pass. Passes at a higher
// level are skipped when a lower level produced an error, avoiding cascade
// noise (e.g. no type-check on an unresolved reference).
type PassLevel int

const (
	LevelSyntax PassLevel = iota
	LevelNameResolution
	LevelType
	LevelConstraint
)

var passLevelNames = map[PassLevel]string{
	LevelSyntax:         "syntax",
	LevelNameResolution: "name-resolution",
	LevelType:           "type",
	LevelConstraint:     "constraint",
}

// String returns the lowercase name of the level, or "unknown".
func (l PassLevel) String() string {
	if name, ok := passLevelNames[l]; ok {
		return name
	}
	return "unknown"
}

// Pass is a single validation rule. Level reports its dependency tier; Run
// executes it over a whole document and returns any diagnostics found.
type Pass interface {
	Level() PassLevel
	Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic
}

// Context carries shared state made available to every pass in a run: the
// document name, the global symbol index, a lazily-created shared resolver,
// and the syntax diagnostics produced by the caller's parse (passes must not
// import the parser, so these enter here).
type Context struct {
	Name             string
	Kind             source.Kind
	Index            *symbols.Index
	ParseDiagnostics []Diagnostic
	// Options is what the caller asked for, fixed at construction: a pass reads
	// it, and nothing mutates it during a run.
	Options Options
	// Batch is the batch this document is analyzed in, nil when it is analyzed
	// alone; every context of a batch reads the one value and none writes it.
	Batch *Batch

	resolver *resolve.Resolver
	model    *semantics.Model
	w8dCache map[*symbols.Scope][]*symbols.Symbol
	w8cCache map[*symbols.Scope][]*symbols.Symbol
	// failures is where the tiers below the pass now running found blocking
	// faults, so an element-scoped pass can gate itself per element.
	failures []source.Span
}

// Batch is what a batch of analyses computes once before its documents are
// analyzed together; anything a pass would gather over every document belongs here.
type Batch struct {
	// Documents names the documents the batch analyzes, in the order asked for.
	Documents []string
}

// Options is the analysis configuration of one run. The zero value is what
// every existing caller gets: today's behavior, unchanged.
type Options struct {
	// Conformance is the strictness the notation is judged at.
	Conformance conformance.Mode
}

// NewContext builds a Context for a document, in the default mode.
func NewContext(name string, idx *symbols.Index, parseDiags []Diagnostic) *Context {
	return NewContextWithKind(name, source.KindOf(name), idx, parseDiags)
}

// NewContextWithKind builds a context with an explicit source language.
func NewContextWithKind(name string, kind source.Kind, idx *symbols.Index,
	parseDiags []Diagnostic) *Context {
	return NewContextWithOptions(name, kind, idx, parseDiags, Options{})
}

// NewContextWithOptions builds a context that carries explicit analysis options.
func NewContextWithOptions(name string, kind source.Kind, idx *symbols.Index,
	parseDiags []Diagnostic, opts Options) *Context {
	return &Context{Name: name, Kind: kind, Index: idx, ParseDiagnostics: parseDiags, Options: opts}
}

// setFailures records the blocking spans of the tiers below the pass about to
// run. Only the registry calls it, once per pass.
func (c *Context) setFailures(spans []source.Span) { c.failures = spans }

// DownstreamOfFailure reports whether a reference an element's meaning rests on
// — its type, its metaclass — carries a blocking fault from a lower tier, so a
// pass whose subject is that element has nothing sound to judge.
func (c *Context) DownstreamOfFailure(ref ast.Node) bool {
	if c == nil || ref == nil {
		return false
	}
	return c.downstreamSpan(ref.Span())
}

func (c *Context) downstreamSpan(span source.Span) bool {
	if c == nil {
		return false
	}
	for _, f := range c.failures {
		if f.Offset >= span.Offset && f.End() <= span.End() {
			return true
		}
	}
	return false
}

// Resolver returns the shared resolver for this context, creating it on first
// use so multiple passes reuse one memoized instance.
func (c *Context) Resolver() *resolve.Resolver {
	if c.resolver == nil {
		c.resolver = resolve.New(c.Index)
	}
	return c.resolver
}

// Model returns the shared semantic model (specialization graph, multiplicity,
// inherited members, evaluator) for this context, creating it on first use over
// the shared resolver so constraint passes reuse one memoized instance.
func (c *Context) Model() *semantics.Model {
	if c.model == nil {
		c.model = semantics.NewModel(c.Resolver())
		// Attach model to resolver for inheritance-aware member resolution
		c.Resolver().SetModel(c.model)
		c.model.SetArgumentTyper(NewArgumentTyper(c.Resolver(), c.model))
	}
	return c.model
}
