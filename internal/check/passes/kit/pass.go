package kit

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
	Run(ctx *Context, name string, root *ast.RootNamespace) []diag.Diagnostic
}

// Context carries shared state made available to every pass in a run: the
// document name, the global symbol index, a lazily-created shared resolver,
// and the syntax diagnostics produced by the caller's parse (passes must not
// import the parser, so these enter here).
type Context struct {
	Name             string
	Kind             source.Kind
	Index            *symbols.Index
	ParseDiagnostics []diag.Diagnostic
	// Options is what the caller asked for, fixed at construction: a pass reads
	// it, and nothing mutates it during a run.
	Options Options
	// Batch is the batch this document is analyzed in, nil when it is analyzed
	// alone; every context of a batch reads the one value and none writes it.
	Batch *Batch

	resolver    *resolve.Resolver
	model       *semantics.Model
	gathers     *Gathers
	symbolCache map[*symbols.Scope][]*symbols.Symbol
	memberCache map[*symbols.Scope][]*symbols.Symbol
	newModel    func(*resolve.Resolver) *semantics.Model
	// failures is where the tiers below the pass now running found blocking
	// faults, so an element-scoped pass can gate itself per element.
	failures []source.Span
}

// Batch is what a batch of analyses computes once before its documents are
// analyzed together; anything a pass would gather over every document belongs here.
type Batch struct {
	// Documents names the documents the batch analyzes, in the order asked for.
	Documents []string
	// Gathers is what the workspace-wide audits gather, once for the batch, on
	// first use by any of its contexts; nil leaves each context to gather alone.
	Gathers *Gathers
	// Source reads the documents' notation, which comment and documentation
	// bodies come from; nil leaves every body unreadable, as an editor never is.
	Source source.Lookup
}

// Options is the analysis configuration of one run. The zero value is what
// every existing caller gets: today's behavior, unchanged.
type Options struct {
	// Conformance is the strictness the notation is judged at.
	Conformance diag.ConformanceMode
}

// NewContext builds a Context; newModel makes the semantic model on first use of Model().
func NewContext(name string, kind source.Kind, idx *symbols.Index, parseDiags []diag.Diagnostic, opts Options, newModel func(*resolve.Resolver) *semantics.Model) *Context {
	return &Context{Name: name, Kind: kind, Index: idx, ParseDiagnostics: parseDiags, Options: opts, newModel: newModel}
}

// Share hands the context a resolver, model and gathers that outlive it — a
// workspace's, kept across analyses — instead of the fresh ones it would make.
func (c *Context) Share(resolver *resolve.Resolver, model *semantics.Model, gathers *Gathers) {
	c.resolver, c.model, c.gathers = resolver, model, gathers
}

// InBatch places the context in batch, reading the gathers the batch shares
// instead of gathering alone; a nil batch leaves it analyzing on its own.
func (c *Context) InBatch(b *Batch) {
	c.Batch = b
	if b != nil {
		c.gathers = b.Gathers
	}
}

// Gathers is what the workspace-wide audits gathered per document, shared
// across analyses by a workspace and made afresh for a context outside one.
func (c *Context) Gathers() *Gathers {
	if c.gathers == nil {
		c.gathers = NewGathers()
	}
	return c.gathers
}

// SetFailures records the blocking spans of the tiers below the pass about to
// run. Only the registry calls it, once per pass.
func (c *Context) SetFailures(spans []source.Span) { c.failures = spans }

// DownstreamOfFailure reports whether a reference an element's meaning rests on
// — its type, its metaclass — carries a blocking fault from a lower tier, so a
// pass whose subject is that element has nothing sound to judge.
func (c *Context) DownstreamOfFailure(ref ast.Node) bool {
	if c == nil || ref == nil {
		return false
	}
	return c.DownstreamSpan(ref.Span())
}

// DownstreamSpan reports whether span contains a blocking failure.
func (c *Context) DownstreamSpan(span source.Span) bool {
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
		if c.newModel == nil {
			panic("kit: Context built without a model constructor")
		}
		c.model = c.newModel(c.Resolver())
		// Attach model to resolver for inheritance-aware member resolution
		c.Resolver().SetModel(c.model)
		if c.Batch != nil {
			c.model.SetSourceText(c.Batch.Source)
		}
	}
	return c.model
}
