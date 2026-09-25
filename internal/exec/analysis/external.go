package analysis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/enginewire"
)

// externalEngine is a manifest `engine` entry as the framework runs it: a process of the
// plan's own spoken to over standard input, whose answers the host checks before they stand.
type externalEngine struct {
	entry   EngineEntry
	timeout func() time.Duration
	limit   func() int
}

// NewEngine is the engine of a manifest entry: registered whether or not this build serves
// it, listing the typed reason through Process and refusing every question through Covers.
func NewEngine(entry EngineEntry) Manifested {
	return externalEngine{entry: entry, timeout: toolTimeoutFromEnv, limit: outputLimitFromEnv}
}

// Name is the entry's.
func (e externalEngine) Name() string { return e.entry.Name }

// Describe is the entry's declaration at the authority this build admits.
func (e externalEngine) Describe() Description { return e.entry.Description() }

// Origin is the entry.
func (e externalEngine) Origin() Origin { return e.entry.Origin() }

// Process names the program found, or the typed reason the entry is not served or its
// program is absent; nothing is run.
func (e externalEngine) Process() (string, error) {
	if err := e.entry.Served(); err != nil {
		return "", err
	}
	if err := e.entry.Present(); err != nil {
		return "", err
	}
	if e.entry.Version != "" {
		return fmt.Sprintf("%s %s at %s", e.entry.Name, e.entry.Version, e.entry.Program()), nil
	}
	return e.entry.Name + " at " + e.entry.Program(), nil
}

// Probe starts the entry's process once, checks its describe against the entry and ends it:
// what -engines -probe reports. A build that does not serve the entry refuses without running.
func (e externalEngine) Probe() (string, error) {
	found, err := e.Process()
	if err != nil {
		return "", err
	}
	s, err := startSession(e.entry, e.limit(), e.timeout())
	if err != nil {
		return "", err
	}
	s.end(errProbed)
	return found + "; describe agrees", nil
}

// errProbed is why a probe's session ended: the handshake was all that was asked.
var errProbed = errors.New("probed")

// Covers takes a question of a kind and subject the entry declares, over a model that derives
// or holds the semantics it is sent as, when the engine's process answers covers with true; a
// refusal of the engine's own names its reason.
func (e externalEngine) Covers(model *Model, q Question) Coverage {
	if err := e.entry.Served(); err != nil {
		return refused(err)
	}
	if !e.Describe().Answers(q.Kind) {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if err := e.askable(q); err != nil {
		return refused(err)
	}
	if err := e.runnable(model, q); err != nil {
		return refused(err)
	}
	if len(e.entry.Subjects) > 0 {
		family := subjectFamily(model, q.Subject)
		if !containsString(e.entry.Subjects, family) {
			return refused(&SubjectError{Engine: e.Name(), Subject: q.Subject, Family: family, Subjects: e.entry.Subjects})
		}
	}
	params, err := e.coversParams(model, q, Budget{})
	if err != nil {
		return refused(err)
	}
	var answer enginewire.CoversResult
	if err := e.request(context.Background(), model, enginewire.MethodCovers, params, nil, &answer, nil); err != nil {
		return refused(err)
	}
	if !answer.Covers {
		return refused(&EngineRefusalError{Engine: e.Name(), Reason: answer.Reason})
	}
	return covered
}

// askable is the typed refusal of a question whose kind the host cannot check an answer to:
// a holds question without the action to replay a witness on, a satisfiable one without
// its queries.
func (e externalEngine) askable(q Question) error {
	switch q.Kind {
	case Holds, Sensitive:
		if q.Check == nil || q.Check.Start == nil {
			return &MalformedQuestionError{Kind: q.Kind, Missing: "a Check starting an action"}
		}
	case Outcomes:
		if q.Linearize == nil && (q.Check == nil || q.Check.Start == nil) {
			return &MalformedQuestionError{Kind: q.Kind, Missing: "a Linearize or a Check starting an action"}
		}
	case Satisfiable:
		if q.Solve == nil || len(q.Solve.Queries) == 0 {
			return &MalformedQuestionError{Kind: q.Kind, Missing: "a Solve with its queries"}
		}
	case Sweep:
		if q.Sweep == nil {
			return &MalformedQuestionError{Kind: q.Kind, Missing: "a Sweep"}
		}
	case Compute:
		if q.Compute == nil || q.Compute.Call == nil {
			return &MalformedQuestionError{Kind: q.Kind, Missing: "a Compute with a Call"}
		}
	}
	return nil
}

// runnable is whether the model serves the question: it derives or holds the semantics the
// engine is sent, and builds the run contexts a check question's witnesses replay in.
func (e externalEngine) runnable(model *Model, q Question) error {
	if _, err := model.semantics(); err != nil {
		return &NoRuntimeError{Engine: e.Name()}
	}
	if q.Check != nil && q.Check.Start != nil && !model.builds() {
		return &NoRuntimeError{Engine: e.Name()}
	}
	return nil
}

// Run puts the question to the engine's process in the plan and stands its answer: a claim
// the host checked at the strength the check earns, else not covered with the claim kept in
// the reason. Every failure of the process or the protocol is not covered too; only the
// plan's clock ending is an error.
func (e externalEngine) Run(ctx context.Context, model *Model, q Question, budget Budget) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := e.entry.Served(); err != nil {
		return Result{}, err
	}
	if err := e.runnable(model, q); err != nil {
		return Result{}, err
	}
	started := time.Now()
	result := Result{Question: q, Engine: e.Name(), Strength: NotCovered}
	params, err := e.runParams(model, q, budget)
	if err != nil {
		return e.uncovered(result, started, err), nil
	}
	var answer enginewire.Result
	var claim Claim
	var strength Strength
	shaped := func() error {
		var err error
		claim, strength, err = e.shape(q, answer)
		return err
	}
	sink := newProgressSink(e.Name(), ReporterFrom(ctx))
	if err := e.request(ctx, model, enginewire.MethodRun, params, sink, &answer, shaped); err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return e.uncovered(result, started, err), nil
	}
	result, err = e.stand(ctx, model, q, budget, answer, claim, strength)
	if err != nil {
		return Result{}, err
	}
	result.Elapsed = time.Since(started)
	return result, nil
}

// uncovered is the not-covered result of a run the engine did not answer, naming why.
func (e externalEngine) uncovered(result Result, started time.Time, err error) Result {
	result.Claim, result.Strength, result.Reason = ClaimNone, NotCovered, err.Error()
	result.Elapsed = time.Since(started)
	return result
}

// request sends one request on a session of the plan's pool and decodes its answer into
// out, shaped checking its form; an answer that does not decode or has the wrong form ends
// the session as a protocol break.
func (e externalEngine) request(ctx context.Context, model *Model, method string, params any, sink *progressSink, out any, shaped func() error) error {
	pool, err := model.engineSessions(e.Name(), func() *enginePool {
		return newEnginePool(e.entry, e.limit(), e.timeout())
	})
	if err != nil {
		return err
	}
	s, err := pool.acquire()
	if err != nil {
		return err
	}
	defer pool.put(s)
	raw, err := s.call(ctx, method, params, sink)
	if err != nil {
		return err
	}
	if err := decodeOne(raw, out); err != nil {
		broke := &ProtocolError{Engine: e.Name(), Detail: fmt.Sprintf("the result of %s does not decode: %v", method, err)}
		s.end(broke)
		return broke
	}
	if shaped != nil {
		if err := shaped(); err != nil {
			s.end(err)
			return err
		}
	}
	return nil
}

// ErrSubject is the typed error for a question about a subject the entry does not name.
var ErrSubject = errors.New("engine does not answer for the subject")

// SubjectError reports a question about a subject whose declaration kind is outside the
// entry's subjects; Family is the kind found, empty for a subject the model does not declare.
type SubjectError struct {
	Engine   string
	Subject  string
	Family   string
	Subjects []string
}

// Error names the engine, the subject with its kind, and the kinds the engine answers for.
func (e *SubjectError) Error() string {
	if e.Family == "" {
		return fmt.Sprintf("engine %q answers for %s, and %s is not a declaration of the model", e.Engine, spellList(e.Subjects), e.Subject)
	}
	return fmt.Sprintf("engine %q answers for %s, not the %s %s", e.Engine, spellList(e.Subjects), e.Family, e.Subject)
}

// Is matches ErrSubject.
func (e *SubjectError) Is(target error) bool { return target == ErrSubject }

// ErrEngineRefusal is the typed error for an engine answering covers with false.
var ErrEngineRefusal = errors.New("engine does not cover the question")

// EngineRefusalError is the engine's own refusal of a question, with the reason it gave.
type EngineRefusalError struct {
	Engine string
	Reason string
}

// Error names the engine and its reason.
func (e *EngineRefusalError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("engine %q does not cover the question", e.Engine)
	}
	return fmt.Sprintf("engine %q does not cover the question: %s", e.Engine, e.Reason)
}

// Is matches ErrEngineRefusal.
func (e *EngineRefusalError) Is(target error) bool { return target == ErrEngineRefusal }
