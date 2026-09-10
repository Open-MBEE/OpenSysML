// Package analysis is the analysis framework: one contract every way of
// answering a question about a model implements, one scale for the strength of
// an answer, a registry of engines and the dispatch that turns a question into
// the engines that answer it.
//
// A Question names a subject, what is asked of it and what is left free. An
// Engine covers a set of questions and answers one with a Result whose Strength
// says how the claim is supported. A Registry holds the engines a binary knows,
// and Registry.Answer dispatches a question under `auto`: the strongest covering
// engine answers, a not-covered result advances to the next, and the Plan
// records every engine consulted.
//
// The engines here — run, explore, sweep and solve — are adapters over the
// interpreter, runtime.Explore, Context.RunSweep and internal/core/solve.
package analysis
