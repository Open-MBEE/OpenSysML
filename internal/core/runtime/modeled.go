package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A model states its own randomness — a weighted branch out of a decision, a
// RandomFunctions call — apart from the orders the library leaves open, which the
// scheduling policy resolves. Modeled draws come from a stream of their own, so
// the token order a seed fixes and the values the model draws are two knobs.

// modeledStream keys the stream of modeled draws off a seed, apart from the token-order stream.
const modeledStream uint64 = 0xD1B54A32D192ED03

// ErrUnseededDraw is the typed error a modeled draw raises when no seed fixes it.
var ErrUnseededDraw = errors.New("modeled randomness needs a seed")

// UnseededDrawError names the draw a run could not make for want of a seed.
type UnseededDrawError struct {
	What string
}

func (e *UnseededDrawError) Error() string {
	return fmt.Sprintf("%v: %s draws a random value; seed the run, as -seed <n> or %%seed <n>, or schedule it under seed:<n>", ErrUnseededDraw, e.What)
}

// Is makes every UnseededDrawError match ErrUnseededDraw.
func (e *UnseededDrawError) Is(target error) bool { return target == ErrUnseededDraw }

// ErrRandomDomain is the typed error a random function raises on arguments that
// bound no distribution: `uniform(hi, lo)`, a negative standard deviation.
var ErrRandomDomain = errors.New("random function arguments bound no distribution")

// ErrBranchWeights is the typed error a weighted decision raises when the weights
// its branches draw, once evaluated, are no probabilities.
var ErrBranchWeights = errors.New("invalid branch weights")

// modeledSource is the stream one run's modeled draws come from: a generator a
// seed fixes, or the draws a witness recorded, followed in order.
type modeledSource struct {
	pcg    *rand.PCG
	rng    *rand.Rand
	replay *replayRun
}

// newModeledSource starts the modeled stream seed fixes.
func newModeledSource(seed uint64) *modeledSource {
	pcg := rand.NewPCG(seed, seed^modeledStream)
	return modeledAt(*pcg)
}

// modeledAt resumes the modeled stream at a generator's position.
func modeledAt(state rand.PCG) *modeledSource {
	pcg := &state
	// #nosec G404 -- a replayable run needs a stated generator, not a cryptographic one.
	return &modeledSource{pcg: pcg, rng: rand.New(pcg)}
}

// modeledDraws follows the draws the run's witness recorded.
func modeledDraws(r *replayRun) *modeledSource {
	return &modeledSource{replay: r}
}

// seeded reports whether the source draws from a generator of its own.
func (m *modeledSource) seeded() bool { return m != nil && m.rng != nil }

// mark returns what a probe restores: the generator's state, or the witness's position.
func (m *modeledSource) mark() func() {
	if m == nil || m.pcg == nil {
		return func() { /* nothing drawn from a generator */ }
	}
	saved := *m.pcg
	return func() { *m.pcg = saved }
}

// position spells where the stream stands — what the run draws next — for a
// checker telling apart states alike in every other way; "" for a run that cannot draw.
func (m *modeledSource) position() string {
	switch {
	case m == nil:
		return ""
	case m.replay != nil:
		return fmt.Sprintf("witness draw %d", m.replay.nextDraw+1)
	}
	state, err := m.pcg.MarshalBinary()
	if err != nil {
		return "generator " + err.Error()
	}
	return fmt.Sprintf("generator %x", state)
}

// modelSeed is the seed a context's runs draw their modeled randomness from when
// set, whatever the scheduling policy; nil leaves it to a `seed:<n>` policy.
type modelSeed struct {
	seed uint64
	set  bool
}

// SetModelSeed fixes the seed the modeled draws of the runs started from now on
// come from, independent of the scheduling policy; ClearModelSeed unsets it.
func (ctx *Context) SetModelSeed(seed uint64) {
	ctx.modelSeed = modelSeed{seed: seed, set: true}
}

// ClearModelSeed leaves modeled draws to the scheduling policy's seed, if any.
func (ctx *Context) ClearModelSeed() {
	ctx.modelSeed = modelSeed{}
}

// ModelSeed is the seed set by SetModelSeed, and whether one is.
func (ctx *Context) ModelSeed() (uint64, bool) {
	return ctx.modelSeed.seed, ctx.modelSeed.set
}

// modeledUnder is the stream the run's modeled draws come from under policy: the
// witness's draws under replay, else the model seed's stream, else the schedule
// seed's; nil for a run that cannot draw.
func (ctx *Context) modeledUnder(policy SchedulePolicy, replay *replayRun) *modeledSource {
	if policy.kind == scheduleReplay && replay != nil {
		return modeledDraws(replay)
	}
	if ctx.modelSeed.set {
		return newModeledSource(ctx.modelSeed.seed)
	}
	if policy.kind == scheduleSeeded {
		return newModeledSource(policy.seed)
	}
	return nil
}

// DrawTaken is one random value a run drew: the call that drew it with its
// arguments as evaluated, and the value, spelt `draw uniform(1, 80) = 42.5`.
type DrawTaken struct {
	What  string
	Value semantics.Value
}

// DrawDiagnosticCode is the code a diagnostic about a random draw carries.
const DrawDiagnosticCode = "random-draw"

// DrawPoint is a random draw as a run notes it, in the trace where it was made.
type DrawPoint struct {
	Draw DrawTaken
}

// Describe renders the draw for a diagnostic.
func (d DrawPoint) Describe() string { return d.Draw.Describe() }

// String is the trace line the draw is recorded as: its witness line.
func (d DrawPoint) String() string { return d.Draw.String() }

// Location is where the draw was made; a call inside an expression names no file.
func (d DrawPoint) Location() (string, source.Span) { return "", source.Span{} }

// Diagnostic is the draw as an informational finding about the run.
func (d DrawPoint) Diagnostic() diag.Diagnostic {
	return diag.Diagnostic{
		Severity: diag.SeverityInfo,
		Message:  "random draw: " + d.Describe(),
		Code:     DrawDiagnosticCode,
		Source:   "runtime",
	}
}

// distribution is what one random call draws: a value from the generator, and the
// values it could draw at all, which a witness's recorded draw is checked against.
type distribution struct {
	draw   func(rng *rand.Rand) semantics.Value
	admits func(v semantics.Value) bool
}

// draw makes the random draw the call what asks for from the run's modeled stream,
// noting it for the trace and the witness; a probe's draw is undone with the probe.
func (ctx *Context) draw(what string, dist distribution) (semantics.Value, error) {
	val, err := ctx.scheduling().draw(what, dist)
	if err != nil {
		return semantics.Value{}, err
	}
	taken := DrawTaken{What: what, Value: val}
	if ctx.probes == 0 {
		ctx.draws = append(ctx.draws, taken)
	}
	ctx.note(DrawPoint{Draw: taken})
	return val, nil
}

// DrawsTaken returns the random draws every run of the context made, in order, as
// a witness lists them: what a `replay` policy over them hands the same calls.
func (ctx *Context) DrawsTaken() []DrawTaken {
	return slices.Clone(ctx.draws)
}

// drawPrefix opens a draw line of a witness.
const drawPrefix = "draw "

// String spells the draw as a witness lists it and ParseDraw reads it back.
func (d DrawTaken) String() string {
	return drawPrefix + d.What + " = " + formatDrawn(d.Value)
}

// Describe renders the draw for a diagnostic.
func (d DrawTaken) Describe() string {
	return d.What + " drew " + formatDrawn(d.Value)
}

// formatDrawn spells a drawn number so it reads back as the kind it is: a Real
// always carries a point or an exponent.
func formatDrawn(v semantics.Value) string {
	if v.Kind == semantics.ValInt {
		return strconv.FormatInt(v.Int, 10)
	}
	s := strconv.FormatFloat(v.Real, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eIN") {
		s += ".0"
	}
	return s
}

// ErrInvalidDraw is the typed error every unreadable draw line wraps.
var ErrInvalidDraw = errors.New("invalid draw")

// DrawParseError reports a draw line ParseDraw could not read, with why.
type DrawParseError struct {
	Text   string
	Line   int
	Reason string
}

func (e *DrawParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%v: line %d: %q: %s", ErrInvalidDraw, e.Line, e.Text, e.Reason)
	}
	return fmt.Sprintf("%v: %q: %s", ErrInvalidDraw, e.Text, e.Reason)
}

// Is makes every DrawParseError match ErrInvalidDraw.
func (e *DrawParseError) Is(target error) bool { return target == ErrInvalidDraw }

// ParseDraw reads one draw as DrawTaken.String spells it: `draw <call> = <number>`.
func ParseDraw(text string) (DrawTaken, error) {
	text = strings.TrimSpace(text)
	fail := func(reason string) (DrawTaken, error) {
		return DrawTaken{}, &DrawParseError{Text: text, Reason: reason}
	}
	rest, ok := strings.CutPrefix(text, drawPrefix)
	if !ok {
		return fail("a draw line starts with `draw `: draw <call> = <number>")
	}
	at := strings.LastIndex(rest, " = ")
	if at < 0 {
		return fail("a draw needs ` = ` between the call and its value: draw <call> = <number>")
	}
	what, written := strings.TrimSpace(rest[:at]), strings.TrimSpace(rest[at+len(" = "):])
	if what == "" {
		return fail("a draw names the call that drew it before ` = `")
	}
	if n, err := strconv.ParseInt(written, 10, 64); err == nil {
		return DrawTaken{What: what, Value: semantics.Value{Kind: semantics.ValInt, Int: n}}, nil
	}
	x, err := strconv.ParseFloat(written, 64)
	if err != nil || written == "" {
		return fail("a draw's value is a number: draw <call> = <number>")
	}
	return DrawTaken{What: what, Value: semantics.Value{Kind: semantics.ValReal, Real: x}}, nil
}

// ErrWitnessDraw is the typed error a replay raises at a draw the run cannot
// consume: one the witness records that the run does not make, or one the run
// makes that the witness does not record.
var ErrWitnessDraw = errors.New("witness draw not consumable")

// WitnessDrawError names the draw a replay could not follow and why.
type WitnessDrawError struct {
	// Draw is the 1-based position of the draw in the witness, 0 past its last.
	Draw   int
	What   string
	Reason string
}

func (e *WitnessDrawError) Error() string {
	if e.Draw > 0 {
		return fmt.Sprintf("%v: draw %d: %s: %s", ErrWitnessDraw, e.Draw, e.What, e.Reason)
	}
	return fmt.Sprintf("%v: %s: %s", ErrWitnessDraw, e.What, e.Reason)
}

// Is makes every WitnessDrawError match ErrWitnessDraw.
func (e *WitnessDrawError) Is(target error) bool { return target == ErrWitnessDraw }

// draw is the value the call what draws: the witness's next recorded draw under
// replay, which must be of the same call and one the call could draw, else a draw
// from the seeded generator.
func (m *modeledSource) draw(what string, dist distribution) (semantics.Value, error) {
	if m == nil {
		return semantics.Value{}, &UnseededDrawError{What: what}
	}
	if m.replay != nil {
		return m.replay.takeDraw(what, dist.admits)
	}
	return dist.draw(m.rng), nil
}

// unit is a draw in [0, 1) deciding a weighted branch; a replay decides it by the
// witness's choice line instead, so it draws none.
func (m *modeledSource) unit() (float64, bool) {
	if !m.seeded() {
		return 0, false
	}
	return m.rng.Float64(), true
}

// weightedPick is the alternative a unit draw u in [0, 1) selects among weights
// summing to their total: the first whose cumulative weight exceeds u.
func weightedPick(weights []float64, total, u float64) int {
	target := u * total
	acc := 0.0
	for i, w := range weights {
		acc += w
		if target < acc && w > 0 {
			return i
		}
	}
	for i := len(weights) - 1; i >= 0; i-- {
		if weights[i] > 0 {
			return i
		}
	}
	return 0
}

// mostProbable is the first alternative of the greatest weight.
func mostProbable(weights []float64) int {
	best := 0
	for i, w := range weights {
		if w > weights[best] {
			best = i
		}
	}
	return best
}

// checkWeights refuses weights that are no probabilities: one outside [0, 1], or
// none of them positive.
func checkWeights(where string, weights []float64) (float64, error) {
	total := 0.0
	for i, w := range weights {
		if math.IsNaN(w) || w < 0 || w > 1 {
			return 0, fmt.Errorf("%w: %s: branch %d weighs %s, not a probability in [0, 1]",
				ErrBranchWeights, where, i, formatWeight(w))
		}
		total += w
	}
	if total <= 0 {
		return 0, fmt.Errorf("%w: %s: no holding branch has a positive weight", ErrBranchWeights, where)
	}
	return total, nil
}

// formatWeight spells a weight as a trace reports it.
func formatWeight(w float64) string {
	return strconv.FormatFloat(w, 'g', -1, 64)
}
