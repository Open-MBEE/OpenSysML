package analysis

import (
	"context"
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Model is the model a question is about, reached through the runtime contexts a surface
// builds over it: an evaluation runs in the surface's own where it holds one, every other run
// in a context of the run's own, on the worker the plan builds (see Worker).
type Model struct {
	// Context is the surface's own runtime over the model; nil when the surface holds none.
	Context func() (*runtime.Context, error)
	// Semantics builds a worker's own model-derived part — resolver, semantic model and
	// the runtime's memo tables — over the shared frozen index.
	Semantics func() (*runtime.Model, error)
	// Fresh builds a run's own context on a worker, under the surface's limits.
	Fresh func(*Worker) (*runtime.Context, error)

	// worker is the plan's, built on first use; a plan's copy of the model owns its own.
	worker *Worker
	// tools is the plan's tool runner, put on every context its runs use; nil for none.
	tools runtime.ToolRunner
	// attached are the surface contexts carrying tools for the plan, released when it ends.
	attached []toolAttachment
}

// Engine is one registered way of answering questions about a model.
type Engine interface {
	// Name is the engine's stable identity.
	Name() string
	// Describe reports what the engine can do, fixed at registration.
	Describe() Description
	// Covers says, before any work, whether the engine can answer q for this model;
	// a refusal names what it cannot handle and is never a result.
	Covers(model *Model, q Question) Coverage
	// Run answers q within the budget, in a context of its own. It never
	// mutates model; it stops when ctx is done and reports how far it got.
	Run(ctx context.Context, model *Model, q Question, budget Budget) (Result, error)
}

// Description is an engine's capability declaration.
type Description struct {
	// Questions are the kinds the engine answers.
	Questions []Kind
	// Process names the external process the engine needs, empty for none.
	Process string
	// Bounds names the bounds the engine takes, in the order it reports them.
	Bounds []string
	// Replays reports whether the engine's witnesses replay through the interpreter.
	Replays bool
	// Authority is the strongest evidence the engine can ever produce.
	Authority Strength
}

// Answers reports whether the engine declares the kind.
func (d Description) Answers(k Kind) bool {
	for _, kind := range d.Questions {
		if kind == k {
			return true
		}
	}
	return false
}

// Coverage is what Covers answers: covered, or refused for a typed reason.
type Coverage struct {
	Covered bool
	// Refusal names the construct, condition or absent process the engine
	// cannot handle; nil when covered.
	Refusal error
}

// covered is the coverage of a question the engine answers.
var covered = Coverage{Covered: true}

// refused is the coverage of a question the engine will not take.
func refused(err error) Coverage { return Coverage{Refusal: err} }

// External is an engine that needs a process outside the build, and reports
// whether it found one.
type External interface {
	Engine
	// Process names the process found (`z3 at /usr/bin/z3`), or reports its
	// absence as the typed error Covers refuses with.
	Process() (string, error)
}

// ErrDuplicateEngine is the typed error Register returns for a name already registered.
var ErrDuplicateEngine = errors.New("engine already registered")

// DuplicateEngineError reports a second registration under one name.
type DuplicateEngineError struct {
	Name string
}

// Error names the engine.
func (e *DuplicateEngineError) Error() string {
	return fmt.Sprintf("analysis: engine %q is already registered", e.Name)
}

// Is matches ErrDuplicateEngine.
func (e *DuplicateEngineError) Is(target error) bool { return target == ErrDuplicateEngine }

// ErrNotAsked is the typed error an engine refuses with for a kind it does not answer.
var ErrNotAsked = errors.New("engine does not answer this kind of question")

// NotAskedError reports a question of a kind the engine does not answer.
type NotAskedError struct {
	Engine string
	Kind   Kind
}

// Error names the engine and the kind.
func (e *NotAskedError) Error() string {
	return fmt.Sprintf("%s does not answer %s questions", e.Engine, e.Kind)
}

// Is matches ErrNotAsked.
func (e *NotAskedError) Is(target error) bool { return target == ErrNotAsked }

// ErrFreedom is the typed error a concrete engine refuses with for a question
// that leaves free what the engine fixes.
var ErrFreedom = errors.New("engine cannot leave that free")

// FreedomError reports a question leaving free what the engine fixes: inputs to
// one that runs concrete values only, the schedule to one that runs under one policy.
type FreedomError struct {
	Engine string
	Free   Freedom
}

// Error names the engine and what it cannot leave free.
func (e *FreedomError) Error() string {
	return fmt.Sprintf("%s cannot leave %s free", e.Engine, e.Free)
}

// Is matches ErrFreedom.
func (e *FreedomError) Is(target error) bool { return target == ErrFreedom }

// ErrProcessAbsent is the typed error an external engine refuses with when its
// process is not found.
var ErrProcessAbsent = errors.New("engine's process is absent")

// ProcessAbsentError reports an external engine whose process was not found.
type ProcessAbsentError struct {
	Engine string
	// Process is what the engine needs, as its Description names it.
	Process string
	// Err is the probe's own report of the absence.
	Err error
}

// Error is the probe's own report, which already names what is absent and how
// to supply it; without one, Engine and Process say whose need it is.
func (e *ProcessAbsentError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s needs %s, which is absent", e.Engine, e.Process)
}

// Is matches ErrProcessAbsent.
func (e *ProcessAbsentError) Is(target error) bool { return target == ErrProcessAbsent }

// Unwrap returns the probe's report.
func (e *ProcessAbsentError) Unwrap() error { return e.Err }

// ErrMalformedQuestion is the typed error for a question missing the ask its kind needs.
var ErrMalformedQuestion = errors.New("question lacks the ask its kind needs")

// MalformedQuestionError reports a question whose kind's ask is missing.
type MalformedQuestionError struct {
	Kind Kind
	// Missing names the field the kind needs.
	Missing string
}

// Error names the kind and the field.
func (e *MalformedQuestionError) Error() string {
	return fmt.Sprintf("analysis: a %s question needs %s", e.Kind, e.Missing)
}

// Is matches ErrMalformedQuestion.
func (e *MalformedQuestionError) Is(target error) bool { return target == ErrMalformedQuestion }

// ErrNoEngine is the typed error Answer returns when no registered engine
// answers the question's kind at all.
var ErrNoEngine = errors.New("no engine answers this kind of question")

// NoEngineError reports a kind no registered engine declares.
type NoEngineError struct {
	Kind Kind
}

// Error names the kind.
func (e *NoEngineError) Error() string {
	return fmt.Sprintf("analysis: no registered engine answers %s questions", e.Kind)
}

// Is matches ErrNoEngine.
func (e *NoEngineError) Is(target error) bool { return target == ErrNoEngine }
