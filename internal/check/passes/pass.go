package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// PassLevel is the dependency tier of a validation pass.
type PassLevel = kit.PassLevel

const (
	LevelSyntax         = kit.LevelSyntax
	LevelNameResolution = kit.LevelNameResolution
	LevelType           = kit.LevelType
	LevelConstraint     = kit.LevelConstraint
)

// Pass is a single validation rule.
type Pass = kit.Pass

// Context carries shared state made available to every pass in a run.
type Context = kit.Context

// Options is the analysis configuration of one run.
type Options = kit.Options

// Batch is what a batch of analyses shares: its documents, gathers and source lookup.
type Batch = kit.Batch

// NewContext builds a Context for a document, in the default mode.
func NewContext(name string, idx *symbols.Index, parseDiags []diag.Diagnostic) *Context {
	return NewContextWithKind(name, source.KindOf(name), idx, parseDiags)
}

// NewContextWithKind builds a context with an explicit source language.
func NewContextWithKind(name string, kind source.Kind, idx *symbols.Index,
	parseDiags []diag.Diagnostic) *Context {
	return NewContextWithOptions(name, kind, idx, parseDiags, Options{})
}

// NewContextWithOptions builds a context that carries explicit analysis options.
func NewContextWithOptions(name string, kind source.Kind, idx *symbols.Index,
	parseDiags []diag.Diagnostic, opts Options) *Context {
	return kit.NewContext(name, kind, idx, parseDiags, opts, NewTypedModel)
}

// Shared is the semantic state a workspace keeps across analyses: the resolver
// and model it memoizes into, and what its audits gathered per document.
type Shared struct {
	Resolver *resolve.Resolver
	Model    *semantics.Model
	Gathers  *Gathers
}

// Share hands ctx the workspace's resolver, model and gathers in place of the
// fresh ones it would make.
func (s Shared) Share(ctx *Context) {
	ctx.Share(s.Resolver, s.Model, s.Gathers)
}
