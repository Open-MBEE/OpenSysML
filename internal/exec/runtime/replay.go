package runtime

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The `replay:<file>` policy fixes a witness's input lines before the run's first move, follows
// its choice lines move for move, then picks as `reverse` does, still one token a step; a move the run cannot make where
// the witness makes it is refused, naming the move, as is an input the run cannot fix.

// ErrReplayRefused is the typed error every refused replay move wraps.
var ErrReplayRefused = errors.New("replay refused")

// ReplayError reports a witness move the run could not follow: which move, the
// move itself, and what the run faced instead.
type ReplayError struct {
	// Move is the 1-based position of the move in the witness.
	Move   int
	Choice ChoiceTaken
	Faced  string
}

func (e *ReplayError) Error() string {
	return fmt.Sprintf("%v: move %d (%s): %s", ErrReplayRefused, e.Move, e.Choice, e.Faced)
}

// Is makes every ReplayError match ErrReplayRefused.
func (e *ReplayError) Is(target error) bool { return target == ErrReplayRefused }

// ErrInvalidChoice is the typed error every unparseable choice line wraps.
var ErrInvalidChoice = errors.New("invalid choice")

// ChoiceParseError reports a line of a witness that spells no choice, with why.
type ChoiceParseError struct {
	// Line is the 1-based line of the witness, 0 for a line parsed on its own.
	Line   int
	Text   string
	Reason string
}

func (e *ChoiceParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%v: line %d %q: %s", ErrInvalidChoice, e.Line, e.Text, e.Reason)
	}
	return fmt.Sprintf("%v: %q: %s", ErrInvalidChoice, e.Text, e.Reason)
}

// Is makes every ChoiceParseError match ErrInvalidChoice.
func (e *ChoiceParseError) Is(target error) bool { return target == ErrInvalidChoice }

// ErrInvalidInput is the typed error every unparseable input line wraps.
var ErrInvalidInput = errors.New("invalid input")

// InputParseError reports a line of a witness that spells no input, with why.
type InputParseError struct {
	// Line is the 1-based line of the witness, 0 for a line parsed on its own.
	Line   int
	Text   string
	Reason string
}

func (e *InputParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%v: line %d %q: %s", ErrInvalidInput, e.Line, e.Text, e.Reason)
	}
	return fmt.Sprintf("%v: %q: %s", ErrInvalidInput, e.Text, e.Reason)
}

// Is makes every InputParseError match ErrInvalidInput.
func (e *InputParseError) Is(target error) bool { return target == ErrInvalidInput }

// ErrInvalidObject is the typed error every unparseable object line wraps.
var ErrInvalidObject = errors.New("invalid object")

// ObjectParseError reports a line of a witness that names no object, with why.
type ObjectParseError struct {
	// Line is the 1-based line of the witness, 0 for a line parsed on its own.
	Line   int
	Text   string
	Reason string
}

func (e *ObjectParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%v: line %d %q: %s", ErrInvalidObject, e.Line, e.Text, e.Reason)
	}
	return fmt.Sprintf("%v: %q: %s", ErrInvalidObject, e.Text, e.Reason)
}

// Is makes every ObjectParseError match ErrInvalidObject.
func (e *ObjectParseError) Is(target error) bool { return target == ErrInvalidObject }

// ErrWitnessObject is the typed error for an object a witness names that the
// replaying run has no object at the path of.
var ErrWitnessObject = errors.New("witness object")

// WitnessObjectError reports an object a witness names by path that the run
// replaying it did not make.
type WitnessObjectError struct {
	Object ObjectNamed
	Reason string
}

func (e *WitnessObjectError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrWitnessObject, e.Object, e.Reason)
}

// Is makes every WitnessObjectError match ErrWitnessObject.
func (e *WitnessObjectError) Is(target error) bool { return target == ErrWitnessObject }

// ErrWitnessInput is the typed error every witness input the run cannot fix wraps.
var ErrWitnessInput = errors.New("witness input refused")

// WitnessInputError reports a witness input the run could not fix before its
// first move: the feature named, and why.
type WitnessInputError struct {
	Feature string
	Reason  string
}

func (e *WitnessInputError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrWitnessInput, e.Feature, e.Reason)
}

// Is makes every WitnessInputError match ErrWitnessInput.
func (e *WitnessInputError) Is(target error) bool { return target == ErrWitnessInput }

// InputTaken is one input a witness fixes before the run's first move: a feature of
// the action or its performer and its value, spelt `input <feature> = <value>`.
type InputTaken struct {
	Feature string
	// Value is the value, when the witness was made in memory; ValInvalid for one
	// read from a file, whose Written is evaluated where the action's defaults are.
	Value Value
	// Written is the value as the notation spells it: what a witness file holds.
	Written string
}

// InputOf is the input fixing feature at value, spelt as the value formats.
func InputOf(feature string, value Value) InputTaken {
	return InputTaken{Feature: feature, Value: value, Written: FormatValue(value)}
}

// String spells the input as a witness lists it and ParseInput reads it back.
func (in InputTaken) String() string {
	feature := in.Feature
	if labelNeedsQuoting(feature) || strings.ContainsAny(feature, " \t") {
		feature = source.UnrestrictedNameText(feature)
	}
	return inputPrefix + feature + " = " + in.Written
}

// inputPrefix opens an input line of a witness.
const inputPrefix = "input "

// ParseInput reads one input as InputTaken.String spells it: `input <feature> = <value>`,
// the value any expression the notation reads, evaluated when the run begins.
func ParseInput(text string) (InputTaken, error) {
	text = strings.TrimSpace(text)
	fail := func(reason string) (InputTaken, error) {
		return InputTaken{}, &InputParseError{Text: text, Reason: reason}
	}
	rest, ok := strings.CutPrefix(text, inputPrefix)
	if !ok {
		return fail("an input line starts with `input `: input <feature> = <value>")
	}
	feature, mark, written, ok := readLabel(rest, " = ")
	written = strings.TrimSpace(written)
	quoted := strings.HasPrefix(rest, "'")
	switch {
	case !ok:
		return fail("the feature's quoted name is left open")
	case mark == "":
		return fail("an input needs ` = ` between the feature and its value: input <feature> = <value>")
	case feature == "" || !quoted && strings.ContainsAny(feature, " \t"):
		return fail("an input names one feature of the action or its performer before ` = `")
	case written == "":
		return fail("an input needs a value after ` = `")
	}
	return InputTaken{Feature: feature, Written: written}, nil
}

// ObjectNamed is an object the moves of a witness name by the number its run gave
// it, bound to its materialization path, `object #2 = Plant::spare#1`, so the run
// replaying the witness finds the object among its own.
type ObjectNamed struct {
	ID   int64
	Path string
}

// String spells the binding as a witness lists it and ParseObject reads it back.
func (o ObjectNamed) String() string {
	return fmt.Sprintf("%s%d = %s", objectPrefix, o.ID, o.Path)
}

// Number is how the moves name the object: `object #<id>`.
func (o ObjectNamed) Number() string { return objectPrefix + strconv.FormatInt(o.ID, 10) }

// objectPrefix opens an object line of a witness, and names an object in a move.
const objectPrefix = "object #"

// ParseObject reads one object binding as ObjectNamed.String spells it.
func ParseObject(text string) (ObjectNamed, error) {
	text = strings.TrimSpace(text)
	fail := func(reason string) (ObjectNamed, error) {
		return ObjectNamed{}, &ObjectParseError{Text: text, Reason: reason}
	}
	rest, ok := strings.CutPrefix(text, objectPrefix)
	if !ok {
		return fail("an object line starts with `object #`: object #<n> = <path>")
	}
	digits, path, found := strings.Cut(rest, " = ")
	id, err := strconv.ParseInt(digits, 10, 64)
	if !found || err != nil || id < 1 {
		return fail("an object is numbered from 1 and bound with ` = `: object #<n> = <path>")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fail("the object's path is missing: object #<n> = <path>")
	}
	return ObjectNamed{ID: id, Path: path}, nil
}

// Witness is one schedule as a witness file holds it: the objects its moves name
// bound to their paths, the inputs the run fixes before its first move, the values
// its random draws took, the choices that fix the schedule as a replay follows them,
// and the trace the run leaves, as the trace recorder writes it.
type Witness struct {
	Objects []ObjectNamed
	Inputs  []InputTaken
	// DrawPolicy is the policy the draws were resolved under; the line is written
	// only for a fixed policy, so a random run's witness reads as before.
	DrawPolicy DrawPolicy
	// ClockStep is the step, in seconds, the run's clock ticked by; the line is
	// written only for a stepped clock, so a continuous run's witness reads as before.
	ClockStep float64
	Draws     []DrawTaken
	Choices   []ChoiceTaken
	Trace     string
	// Property names the property false at the state the schedule reaches, or
	// whose evaluation there fails as Fails says; empty for a state or a run's failure.
	Property string
	// Fails is the deadlock or failure the schedule ends in, as the executor
	// spells it — or the property's evaluation raised; empty for a state the run goes on from.
	Fails string
}

// The lines closing a witness after its trace: the property it claims, the failure it ends in.
const (
	propertyPrefix = "property: "
	failsPrefix    = "fails: "
)

// String renders the witness as a file holds it: its objects one per line, its
// inputs one per line, `draws by <policy>` for a fixed draw policy, `clock steps by <seconds>` for a stepped clock, its
// draws one per line, its choices one per line — or `no choice points` — a blank line, the trace, and after a blank line the claims closing it: `property: <name>`
// for a property's, `fails: <the failure>` for a schedule ending in a failure, last.
func (w Witness) String() string {
	var b strings.Builder
	for _, o := range w.Objects {
		b.WriteString(o.String())
		b.WriteByte('\n')
	}
	for _, in := range w.Inputs {
		b.WriteString(in.String())
		b.WriteByte('\n')
	}
	if w.DrawPolicy.Fixed() {
		b.WriteString(drawPolicyPrefix + w.DrawPolicy.String() + "\n")
	}
	if w.ClockStep > 0 {
		b.WriteString(clockStepPrefix + semantics.FormatReal(w.ClockStep) + "\n")
	}
	for _, d := range w.Draws {
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	if len(w.Choices) == 0 {
		b.WriteString("no choice points\n")
	}
	for _, c := range w.Choices {
		b.WriteString(c.String())
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(w.Trace)
	if w.Property != "" || w.Fails != "" {
		b.WriteString("\n")
	}
	if w.Property != "" {
		b.WriteString("\n" + propertyPrefix + w.Property)
	}
	if w.Fails != "" {
		b.WriteString("\n" + failsPrefix + w.Fails)
	}
	return b.String()
}

// Empty reports whether the witness fixes no input, records no draw and takes no choice.
func (w Witness) Empty() bool { return len(w.Inputs) == 0 && len(w.Draws) == 0 && len(w.Choices) == 0 }

// objectNumber matches an object a move names by number.
var objectNumber = regexp.MustCompile(regexp.QuoteMeta(objectPrefix) + `(\d+)`)

// objectsNamed binds every object the choices name by number to its path in this
// context, in order of first mention, so a witness of them replays in another run.
func (ctx *Context) objectsNamed(choices []ChoiceTaken) []ObjectNamed {
	var objects []ObjectNamed
	seen := make(map[int64]bool)
	name := func(label string) {
		for _, m := range objectNumber.FindAllStringSubmatch(label, -1) {
			id, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil || seen[id] {
				continue
			}
			seen[id] = true
			objects = append(objects, ObjectNamed{ID: id, Path: ctx.objectPath(id)})
		}
	}
	for _, c := range choices {
		name(c.Where)
		for _, alt := range c.Among {
			name(alt)
		}
		name(c.Took)
	}
	return objects
}

// ReplayPolicy is the `replay` policy over choices held in memory, fixing no
// input. Its spelling names no file, so it does not read back; write the
// witness out to name it.
func ReplayPolicy(choices []ChoiceTaken) SchedulePolicy {
	return ReplayOf(Witness{Choices: choices})
}

// ReplayOf is the `replay` policy over a witness held in memory: its inputs are
// fixed before the first move of the run, its choices followed move for move.
func ReplayOf(w Witness) SchedulePolicy {
	return SchedulePolicy{kind: scheduleReplay, replay: &replayScript{witness: cloneWitness(w)}}
}

func cloneWitness(w Witness) Witness {
	w.Objects, w.Inputs, w.Draws, w.Choices = slices.Clone(w.Objects), slices.Clone(w.Inputs), slices.Clone(w.Draws), slices.Clone(w.Choices)
	return w
}

// Replay returns the choices of a `replay` policy's witness, and whether the policy is one.
func (p SchedulePolicy) Replay() ([]ChoiceTaken, bool) {
	if p.kind != scheduleReplay {
		return nil, false
	}
	return slices.Clone(p.replay.witness.Choices), true
}

// Witness returns the witness a `replay` policy follows, and whether the policy is one.
func (p SchedulePolicy) Witness() (Witness, bool) {
	if p.kind != scheduleReplay {
		return Witness{}, false
	}
	return cloneWitness(p.replay.witness), true
}

// Unfollowed is the first witness move the last run under a `replay` policy could
// not make — one refused, or one left over when the run ended — as a ReplayError;
// nil when the run followed its witness whole or ran under another policy.
func (ctx *Context) Unfollowed() error {
	return ctx.run.scheduler.unfollowed("the run ended")
}

// Choice is the choice point as a witness lists it: what ChoiceTaken.String spells
// and ParseChoice reads back.
func (c ChoicePoint) Choice() ChoiceTaken {
	taken := ChoiceTaken{Kind: c.Kind, Step: c.Step, Where: c.Where, Alternatives: len(c.Alternatives), Taken: c.Taken, Among: slices.Clone(c.Alternatives)}
	if c.Taken >= 0 && c.Taken < len(c.Alternatives) {
		taken.Took = c.Alternatives[c.Taken]
	}
	if c.Weighted() {
		taken.Weights, taken.Drew, taken.Drawn = slices.Clone(c.Weights), c.Drew, c.Drawn
	}
	return taken
}

// replayScript is the witness a replay policy follows and the file it was read from.
type replayScript struct {
	file    string
	witness Witness
}

// ParseChoices reads the choices of a witness header naming no object and spelling
// no input: as ChoiceTaken.String spells them, one per line or joined by `; `,
// ending at the first blank line after it; what follows is ignored.
func ParseChoices(text string) ([]ChoiceTaken, error) {
	w, _, err := readHeader(text)
	if err != nil {
		return nil, err
	}
	if len(w.Objects) > 0 {
		return nil, &ObjectParseError{Text: w.Objects[0].String(), Reason: "a witness naming objects is read by ParseWitness"}
	}
	if len(w.Inputs) > 0 {
		return nil, &InputParseError{Text: w.Inputs[0].String(), Reason: "a witness with inputs is read by ParseWitness"}
	}
	if len(w.Draws) > 0 {
		return nil, &DrawParseError{Text: w.Draws[0].String(), Reason: "a witness with draws is read by ParseWitness"}
	}
	return w.Choices, nil
}

// ParseWitness reads a witness as Witness.String writes it: the header of input
// and choice lines, after the blank line ending it the trace, exact, and after a
// blank line ending that the claims closing it, if any.
func ParseWitness(text string) (Witness, error) {
	w, _, err := readWitness(text)
	return w, err
}

// readWitness reads a witness as ParseWitness does and says whether the text has
// a header: one of `no choice points` alone spells a run with no choice to make.
func readWitness(text string) (Witness, bool, error) {
	w, headed, err := readHeader(text)
	if err != nil {
		return Witness{}, headed, err
	}
	lines := strings.SplitAfter(text, "\n")
	begun := false
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			if begun {
				w.Trace = strings.Join(lines[i+1:], "")
				w.readClaims()
				break
			}
			continue
		}
		begun = true
	}
	return w, headed, nil
}

// readClaims splits the claims closing the witness off its trace.
func (w *Witness) readClaims() {
	if trace, claims, found := strings.Cut(w.Trace, "\n\n"+propertyPrefix); found {
		w.Trace = trace
		w.Property, w.Fails, _ = strings.Cut(strings.TrimSuffix(claims, "\n"), "\n"+failsPrefix)
		return
	}
	if trace, fails, found := strings.Cut(w.Trace, "\n\n"+failsPrefix); found {
		w.Trace, w.Fails = trace, strings.TrimSuffix(fails, "\n")
	}
}

// drawPolicyPrefix opens the witness line naming the draw policy of its draws.
const drawPolicyPrefix = "draws by "

// clockStepPrefix opens the witness line naming the step its clock ticked by.
const clockStepPrefix = "clock steps by "

// readHeader reads a witness header: object lines as ObjectNamed.String spells
// them, input lines as InputTaken.String spells them, a `draws by <policy>` line, a `clock steps by <seconds>` line,
// draw lines as DrawTaken.String spells them, then choices as ChoiceTaken.String spells them, one per line or joined by `; `, ending at the
// first blank line after it. It says whether the text has a header.
func readHeader(text string) (w Witness, headed bool, err error) {
	policied, stepped := false, false
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if headed {
				break
			}
			continue
		}
		headed = true
		if line == "no choice points" {
			continue
		}
		if strings.HasPrefix(line, objectPrefix) {
			o, err := ParseObject(line)
			if err == nil && (len(w.Inputs) > 0 || len(w.Draws) > 0 || len(w.Choices) > 0) {
				err = &ObjectParseError{Text: line, Reason: "objects come before the inputs and the moves, and before the draws"}
			}
			if err == nil && slices.ContainsFunc(w.Objects, func(seen ObjectNamed) bool { return seen.ID == o.ID }) {
				err = &ObjectParseError{Text: line, Reason: o.Number() + " is bound twice"}
			}
			if err != nil {
				var parse *ObjectParseError
				if errors.As(err, &parse) {
					parse.Line = i + 1
				}
				return Witness{}, true, err
			}
			w.Objects = append(w.Objects, o)
			continue
		}
		if strings.HasPrefix(line, inputPrefix) {
			in, err := ParseInput(line)
			if err == nil && (len(w.Draws) > 0 || len(w.Choices) > 0) {
				err = &InputParseError{Text: line, Reason: "inputs come before the moves, and before the draws"}
			}
			if err != nil {
				var parse *InputParseError
				if errors.As(err, &parse) {
					parse.Line = i + 1
				}
				return Witness{}, true, err
			}
			w.Inputs = append(w.Inputs, in)
			continue
		}
		if rest, ok := strings.CutPrefix(line, drawPolicyPrefix); ok {
			policy, err := ParseDrawPolicy(rest)
			if err == nil && (len(w.Draws) > 0 || len(w.Choices) > 0) {
				err = &DrawParseError{Text: line, Reason: "the draw policy comes before the draws and the moves"}
			}
			if err == nil && policied {
				err = &DrawParseError{Text: line, Reason: "the draw policy is named twice, and a witness draws by one"}
			}
			if err != nil {
				var parse *DrawParseError
				if !errors.As(err, &parse) {
					parse = &DrawParseError{Text: line, Reason: err.Error()}
				}
				parse.Line = i + 1
				return Witness{}, true, parse
			}
			w.DrawPolicy, policied = policy, true
			continue
		}
		if rest, ok := strings.CutPrefix(line, clockStepPrefix); ok {
			step, err := ParseClockStep(rest)
			if err == nil && step == 0 {
				err = &ClockStepParseError{Text: line, Reason: "a continuous clock writes no clock step line"}
			}
			if err == nil && (len(w.Draws) > 0 || len(w.Choices) > 0) {
				err = &ClockStepParseError{Text: line, Reason: "the clock step comes before the draws and the moves"}
			}
			if err == nil && stepped {
				err = &ClockStepParseError{Text: line, Reason: "the clock step is named twice, and a run's clock steps by one"}
			}
			if err != nil {
				var parse *ClockStepParseError
				if !errors.As(err, &parse) {
					parse = &ClockStepParseError{Text: line, Reason: err.Error()}
				}
				parse.Line = i + 1
				return Witness{}, true, parse
			}
			w.ClockStep, stepped = step, true
			continue
		}
		if strings.HasPrefix(line, drawPrefix) {
			d, err := ParseDraw(line)
			if err == nil && len(w.Choices) > 0 {
				err = &DrawParseError{Text: line, Reason: "draws come before the moves"}
			}
			if err != nil {
				var parse *DrawParseError
				if errors.As(err, &parse) {
					parse.Line = i + 1
				}
				return Witness{}, true, err
			}
			w.Draws = append(w.Draws, d)
			continue
		}
		for _, part := range splitChoices(line) {
			c, err := ParseChoice(part)
			if err != nil {
				var parse *ChoiceParseError
				if errors.As(err, &parse) {
					parse.Line = i + 1
				}
				return Witness{}, true, err
			}
			w.Choices = append(w.Choices, c)
		}
	}
	return w, headed, nil
}

// ParseChoice reads one choice as ChoiceTaken.String spells it: `step N: T first of A, B`,
// `step N: decision D -> B`, `S -> T`, `W: X first of A, B` (a region order, a due
// order at `t=…`, or a dispatch order among `events at t=…`).
func ParseChoice(text string) (ChoiceTaken, error) {
	text = strings.TrimSpace(text)
	fail := func(reason string) (ChoiceTaken, error) {
		return ChoiceTaken{}, &ChoiceParseError{Text: text, Reason: reason}
	}
	step, rest := 0, text
	if after, ok := strings.CutPrefix(text, "step "); ok {
		digits, tail, found := strings.Cut(after, markWhere)
		n, err := strconv.Atoi(digits)
		if !found || err != nil || n < 1 {
			return fail("step needs a positive number and a colon: step <n>: …")
		}
		step, rest = n, tail
	}
	first, mark, after, ok := readLabel(rest, markFirstOf, markArrow, markWhere)
	if !ok {
		return fail(unclosedQuote)
	}
	switch mark {
	case markFirstOf, markWhere:
		return parseOrderChoice(fail, step, first, mark, after)
	case markArrow:
		return parseTransitionChoice(fail, step, first, after)
	}
	return fail(notAChoice)
}

// parseOrderChoice reads the order after `<took> first of ` (a step's token order)
// or `<where>: <took> first of ` (a region or due order) as ParseChoice found it.
func parseOrderChoice(fail func(string) (ChoiceTaken, error), step int, first, mark, after string) (ChoiceTaken, error) {
	c := ChoiceTaken{Kind: ChoiceTokenOrder, Step: step, Took: first}
	if mark == markWhere {
		if step > 0 {
			return fail("a step's order names the token first: step <n>: <took> first of …")
		}
		c.Kind, c.Where = ChoiceRegionOrder, first
		switch {
		case strings.HasPrefix(first, "t="):
			c.Kind = ChoiceDueOrder
		case strings.HasPrefix(first, dispatchWherePrefix):
			c.Kind = ChoiceDispatchOrder
		case strings.HasPrefix(first, enteringWherePrefix), strings.HasPrefix(first, forkWherePrefix):
			c.Kind = ChoiceEntryOrder
		case strings.HasPrefix(first, exitingWherePrefix):
			c.Kind = ChoiceExitOrder
		case strings.HasPrefix(first, stepWherePrefix):
			c.Kind = ChoiceStepOrder
		}
		var ok bool
		if c.Took, mark, after, ok = readLabel(after, markFirstOf); !ok {
			return fail(unclosedQuote)
		}
		if mark == "" {
			return fail(notAChoice)
		}
	} else if step == 0 {
		return fail("an order outside a step needs where it was made: <where>: <took> first of …")
	}
	among, ok := splitLabels(after, markList)
	if !ok {
		return fail(unclosedQuote)
	}
	c.Among, c.Alternatives, c.Taken = among, len(among), slices.Index(among, c.Took)
	if c.Taken < 0 {
		return fail(fmt.Sprintf("%s is not among %s", choiceLabel(c.Took), choiceLabels(among)))
	}
	return c, nil
}

// parseTransitionChoice reads `<where> -> <took>`, a decision branch inside a step
// and a transition outside one, with the alternatives, weights and draw a weighted
// one carries after ` among `.
func parseTransitionChoice(fail func(string) (ChoiceTaken, error), step int, first, after string) (ChoiceTaken, error) {
	took, mark, after, ok := readLabel(after, markAmong)
	if !ok {
		return fail(unclosedQuote)
	}
	if first == "" || took == "" {
		return fail("a branch or transition needs both sides of ->")
	}
	c := ChoiceTaken{Kind: ChoiceTransition, Where: first, Took: took}
	if step > 0 {
		c.Kind, c.Step = ChoiceDecisionBranch, step
	}
	if mark == markAmong {
		return parseWeighted(fail, c, after)
	}
	return c, nil
}

// parseWeighted reads what follows ` among ` on a weighted line — `<alt> p=<w>, …`
// then ` drew <u>` when a draw selected the branch — into c.
func parseWeighted(fail func(string) (ChoiceTaken, error), c ChoiceTaken, text string) (ChoiceTaken, error) {
	const shape = "a weighted branch lists every alternative with its weight: … among <alt> p=<w>, <alt> p=<w> drew <u>"
	for {
		alt, mark, rest, ok := readLabel(text, markWeight)
		if !ok {
			return fail(unclosedQuote)
		}
		if mark == "" {
			return fail(shape)
		}
		number, sep := rest, ""
		if at, m := indexMark(rest, markList, markDrew); at >= 0 {
			number, sep, rest = rest[:at], m, rest[at+len(m):]
		}
		w, err := strconv.ParseFloat(number, 64)
		if err != nil {
			return fail("a weight is a number: <alt> p=<w>")
		}
		c.Among, c.Weights = append(c.Among, alt), append(c.Weights, w)
		if sep == markList {
			text = rest
			continue
		}
		if sep == markDrew {
			u, err := strconv.ParseFloat(rest, 64)
			if err != nil || !(0 <= u && u < 1) {
				return fail("the draw selecting a weighted branch is a number in [0, 1): … drew <u>")
			}
			c.Drew, c.Drawn = u, true
		}
		break
	}
	c.Alternatives, c.Taken = len(c.Among), slices.Index(c.Among, c.Took)
	if c.Taken < 0 {
		return fail(fmt.Sprintf("%s is not among %s", choiceLabel(c.Took), choiceLabels(c.Among)))
	}
	return c, nil
}

// The names a choice line carries — tokens, branches, states, where the choice was
// made — are written as they are unless the line's own punctuation occurs in them;
// then the name is written quoted, 'like this', escaped as an unrestricted name is.

// The marks a choice line's grammar reads as structure.
const (
	markChoices = "; "
	markFirstOf = " first of "
	markArrow   = " -> "
	markList    = ", "
	markWhere   = ": "
	markAmong   = " among "
	markWeight  = " p="
	markDrew    = " drew "
)

// linePunctuation is what a choice line's grammar reads as structure.
var linePunctuation = []string{markChoices, markFirstOf, markArrow, markList, markWhere, markAmong, markWeight, markDrew}

const (
	unclosedQuote = "a quoted name needs its closing quote, followed by the line's punctuation"
	notAChoice    = "not a token order, branch, transition, region, due or dispatch order"
)

// choiceLabel spells a name as a choice line carries it.
func choiceLabel(name string) string {
	if labelNeedsQuoting(name) {
		return source.UnrestrictedNameText(name)
	}
	return name
}

// choiceLabels spells a list of names as a choice line carries them.
func choiceLabels(names []string) string {
	labels := make([]string, len(names))
	for i, name := range names {
		labels[i] = choiceLabel(name)
	}
	return strings.Join(labels, markList)
}

// labelNeedsQuoting reports a name a line could not read back as it is: empty,
// punctuated like the line, quote-led, step-led, or with whitespace to lose.
func labelNeedsQuoting(name string) bool {
	if name == "" || name[0] == '\'' || strings.HasPrefix(name, "step ") || strings.TrimSpace(name) != name {
		return true
	}
	for _, mark := range linePunctuation {
		if strings.Contains(name, mark) {
			return true
		}
	}
	return strings.ContainsAny(name, "\n\r")
}

// readLabel reads the name text starts with — a quoted one to its closing quote, a
// plain one to the first of the marks — with the mark that ended it ("" at the end of
// text) and what follows the mark; false for a quote left open or not followed by a mark.
func readLabel(text string, marks ...string) (name, mark, after string, ok bool) {
	if !strings.HasPrefix(text, "'") {
		at, mark := indexMark(text, marks...)
		if at < 0 {
			return text, "", "", true
		}
		return text[:at], mark, text[at+len(mark):], true
	}
	end := closingQuote(text, 0)
	if end < 0 {
		return "", "", "", false
	}
	name, after = source.StringValue(text[:end+1]), text[end+1:]
	if after == "" {
		return name, "", "", true
	}
	for _, m := range marks {
		if rest, found := strings.CutPrefix(after, m); found {
			return name, m, rest, true
		}
	}
	return "", "", "", false
}

// closingQuote is the index of the quote closing the name opened at text[open],
// past its backslash escapes; -1 when the name is left open.
func closingQuote(text string, open int) int {
	for i := open + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '\'':
			return i
		}
	}
	return -1
}

// indexMark is where the first of the marks occurs in text outside its quoted names,
// with the mark; -1 when none does. A quote opens a name only where a name may begin.
func indexMark(text string, marks ...string) (int, string) {
	for i := 0; i < len(text); i++ {
		if text[i] == '\'' && nameMayBegin(text[:i]) {
			if i = closingQuote(text, i); i < 0 {
				return -1, ""
			}
			continue
		}
		for _, m := range marks {
			if strings.HasPrefix(text[i:], m) {
				return i, m
			}
		}
	}
	return -1, ""
}

// nameMayBegin reports whether a name may begin after before: at the start of the
// text or right after the line's punctuation.
func nameMayBegin(before string) bool {
	if before == "" {
		return true
	}
	for _, mark := range linePunctuation {
		if strings.HasSuffix(before, mark) {
			return true
		}
	}
	return false
}

// splitChoices splits a line into the choices it joins by `; `, outside quoted names.
func splitChoices(line string) []string {
	var parts []string
	for {
		at, _ := indexMark(line, markChoices)
		if at < 0 {
			return append(parts, line)
		}
		parts, line = append(parts, line[:at]), line[at+len(markChoices):]
	}
}

// splitLabels reads the names text lists separated by sep; false for a quote left open.
func splitLabels(text, sep string) ([]string, bool) {
	var names []string
	for {
		name, mark, after, ok := readLabel(text, sep)
		if !ok {
			return nil, false
		}
		names = append(names, name)
		if mark == "" {
			return names, true
		}
		text = after
	}
}

// replayRun follows one run's witness: the objects its moves name to bind to the
// run's own, the inputs to fix before its first move, the draws to hand its random
// calls in turn, the moves left and the first it refused.
type replayRun struct {
	// unbound are the objects the witness names that the run has not made yet;
	// renumber maps the witness's numbers of those it has to the run's own.
	unbound  []ObjectNamed
	renumber map[string]string
	inputs   []InputTaken
	policy   DrawPolicy
	draws    []DrawTaken
	nextDraw int
	choices  []ChoiceTaken
	next     int
	refused  error
	// ctx is the context whose run follows the witness.
	ctx *Context
}

func newReplayRun(w Witness) *replayRun {
	return &replayRun{
		unbound:  slices.Clone(w.Objects),
		renumber: make(map[string]string, len(w.Objects)),
		inputs:   slices.Clone(w.Inputs),
		policy:   w.DrawPolicy,
		draws:    slices.Clone(w.Draws),
		choices:  slices.Clone(w.Choices),
	}
}

// bind binds the objects the witness names that the run has made by now, at their
// paths: a run makes an object when it first reaches it, which may be moves in.
func (r *replayRun) bind() {
	if r.ctx == nil {
		return
	}
	r.unbound = slices.DeleteFunc(r.unbound, func(o ObjectNamed) bool {
		inst, err := r.ctx.objectAt(o.Path)
		if err != nil {
			return false
		}
		r.renumber[o.Number()] = objectPrefix + strconv.FormatInt(inst.ID, 10)
		return true
	})
}

// current is the next move with its objects numbered as this run numbers them, and
// the first object it names that the run has not made, nil when it names none.
func (r *replayRun) current() (ChoiceTaken, *ObjectNamed) {
	r.bind()
	c := r.choices[r.next]
	c.Where, c.Took = r.relabel(c.Where), r.relabel(c.Took)
	c.Among = slices.Clone(c.Among)
	for i, alt := range c.Among {
		c.Among[i] = r.relabel(alt)
	}
	for i, o := range r.unbound {
		if c.names(o.Number()) {
			return c, &r.unbound[i]
		}
	}
	return c, nil
}

// refuseUnbound refuses the witness at a move naming an object the run has not made.
func (r *replayRun) refuseUnbound(o ObjectNamed) {
	if r.refused == nil {
		_, err := r.ctx.objectAt(o.Path)
		r.refused = &WitnessObjectError{Object: o, Reason: err.Error()}
	}
}

// relabel renumbers the objects a label names to the run's own numbers.
func (r *replayRun) relabel(label string) string {
	return objectNumber.ReplaceAllStringFunc(label, func(number string) string {
		if renumbered, ok := r.renumber[number]; ok {
			return renumbered
		}
		return number
	})
}

// names reports whether the choice's place or any alternative spells the object number.
func (c ChoiceTaken) names(number string) bool {
	mentions := func(label string) bool {
		for _, found := range objectNumber.FindAllString(label, -1) {
			if found == number {
				return true
			}
		}
		return false
	}
	return mentions(c.Where) || mentions(c.Took) || slices.ContainsFunc(c.Among, mentions)
}

// takeInputs hands the run's witness inputs to the performance beginning it, once.
func (r *replayRun) takeInputs() []InputTaken {
	inputs := r.inputs
	r.inputs = nil
	return inputs
}

// drawsByPolicy reports whether the witness leaves its draws to its fixed policy: it
// names one and records no draw, so each call resolves to its fixed point as the run would.
func (r *replayRun) drawsByPolicy() bool {
	return r.policy.Fixed() && len(r.draws) == 0
}

// takeDraw hands the call what the witness's next recorded draw, which must be of
// the same call and a value the call admits; a draw the witness does not record,
// records for another call, or records outside the call's distribution refuses
// the witness and fails the call.
// takeDraw hands out the witness's next recorded draw for the call what, refusing
// one the call cannot make: under the witness's fixed policy, any but the fixed point.
func (r *replayRun) takeDraw(what string, dist distribution) (semantics.Value, error) {
	if r.nextDraw >= len(r.draws) {
		err := &WitnessDrawError{What: what, Reason: "the witness records no draw left for it"}
		if r.refused == nil {
			r.refused = err
		}
		return semantics.Value{}, err
	}
	d := r.draws[r.nextDraw]
	if d.What != what {
		err := &WitnessDrawError{Draw: r.nextDraw + 1, What: d.What, Reason: "the run drew " + what + " instead"}
		if r.refused == nil {
			r.refused = err
		}
		return semantics.Value{}, err
	}
	if !dist.admitsUnder(r.policy, d.Value) {
		reason := "the witness records " + formatDrawn(d.Value) + ", which the call cannot draw"
		if r.policy.Fixed() {
			reason += " under " + r.policy.String()
		}
		err := &WitnessDrawError{Draw: r.nextDraw + 1, What: d.What, Reason: reason}
		if r.refused == nil {
			r.refused = err
		}
		return semantics.Value{}, err
	}
	r.nextDraw++
	return d.Value, nil
}

// following reports whether moves are left to follow and none was refused.
func (r *replayRun) following() bool {
	return r.refused == nil && r.next < len(r.choices)
}

// refuse records the first move the run could not follow, with what it faced.
func (r *replayRun) refuse(faced string) {
	if r.refused == nil {
		r.refused = &ReplayError{Move: r.next + 1, Choice: r.choices[r.next], Faced: faced}
	}
}

// unfollowed is the refusal of a run that ended with moves or draws left, or that
// began no performance to fix its inputs on; nil otherwise.
func (r *replayRun) unfollowed(how string) error {
	if r.refused == nil && len(r.inputs) > 0 {
		r.refused = &WitnessInputError{Feature: r.inputs[0].Feature, Reason: how + " with no action performance begun to fix it on"}
	}
	if r.following() {
		r.refuse(how)
	}
	if r.refused == nil && r.nextDraw < len(r.draws) {
		d := r.draws[r.nextDraw]
		r.refused = &WitnessDrawError{Draw: r.nextDraw + 1, What: d.What, Reason: how + " without drawing it"}
	}
	return r.refused
}

// replayMove is one step under replay: the token the witness moves, or with no
// move at this step the tokens tried as an exploring step tries them.
type replayMove struct {
	run    *replayRun
	step   int
	order  []int64
	next   int
	moved  bool
	choice *ChoiceTaken
	// enabled labels the tokens able to act, sorted by ID; taken indexes the one moved.
	enabled []string
	taken   int
	// kept names a token of the witness's move present but not yet able to act: the move waits for the clock's retry.
	kept string
}

// beginStep resolves the step by the witness's move at it: taken when each token named is able to act, kept for
// the clock's retry when one at most is and the rest are present but parked or held; a token absent is refused.
// Past the witness the step is still one token's move, the last able to act, as `reverse` orders them.
func (r *replayRun) beginStep(tokens stepTokens) *replayMove {
	m := &replayMove{run: r, step: tokens.step}
	var enabled, rest, held []int64
	for _, id := range tokens.ids {
		switch {
		case tokens.held[id]:
			held = append(held, id)
		case tokens.enabled(id):
			enabled = append(enabled, id)
		default:
			rest = append(rest, id)
		}
	}
	slices.Sort(enabled)
	slices.Sort(rest)
	slices.Sort(held)
	m.enabled = make([]string, len(enabled))
	for i, id := range enabled {
		m.enabled[i] = tokens.label(id)
	}
	if !r.following() {
		if len(enabled) < 2 {
			m.order = slices.Concat(enabled, rest, held)
			return m
		}
		m.taken = len(enabled) - 1
		m.order = []int64{enabled[m.taken]}
		return m
	}
	able := m.able()
	r.hoistOrder(tokens)
	current, unbound := r.current()
	c := &current
	// With one token at most able to act, an order over tokens this flow lacks is
	// another performance's step of the same number, made within this move or after it.
	if c.Kind == ChoiceTokenOrder && c.Step == tokens.step && (len(enabled) >= 2 || tokens.hasAll(c.Among)) {
		if unbound != nil {
			r.refuseUnbound(*unbound)
			return m
		}
		for _, alt := range c.Among {
			switch {
			case slices.Contains(m.enabled, alt):
			case len(enabled) < 2 && tokens.has(alt):
				if m.kept == "" {
					m.kept = alt
				}
			default:
				r.refuse(fmt.Sprintf("step %d: %s is not able to act (%s)", tokens.step, alt, able))
				return m
			}
		}
		if m.kept != "" {
			m.order = slices.Concat(enabled, rest, held)
			return m
		}
		m.taken = slices.Index(m.enabled, c.Took)
		if m.taken < 0 {
			r.refuse(fmt.Sprintf("step %d: %s is not able to act (%s)", tokens.step, c.Took, able))
			return m
		}
		r.next++
		m.choice = c
		m.order = []int64{enabled[m.taken]}
		return m
	}
	switch {
	case len(enabled) < 2:
		m.order = slices.Concat(enabled, rest, held)
	case c.Step > 0 && c.Step < tokens.step:
		r.refuse(fmt.Sprintf("the run is at step %d and step %d had no such move", tokens.step, c.Step))
	default:
		r.refuse(fmt.Sprintf("step %d: the run must pick a token (%s) and the witness names none", tokens.step, able))
	}
	return m
}

// hoistOrder moves the step's token order, which a witness may spell after the
// choices the token's move made, ahead of them: it is resolved first when replaying.
func (r *replayRun) hoistOrder(tokens stepTokens) {
	for j := r.next; j < len(r.choices); j++ {
		c := r.choices[j]
		if c.Step != tokens.step {
			return
		}
		if c.Kind == ChoiceTokenOrder && tokens.hasAll(c.Among) {
			copy(r.choices[r.next+1:j+1], r.choices[r.next:j])
			r.choices[r.next] = c
			return
		}
	}
}

// able spells the tokens able to act, as a refusal names them.
func (m *replayMove) able() string {
	if len(m.enabled) == 0 {
		return "none is able to act"
	}
	return "able to act: " + strings.Join(m.enabled, ", ")
}

// ended settles a kept move: the clock retrying the step faces it then; a step ending any other way refuses it.
func (m *replayMove) ended(retry bool) {
	if m.kept != "" && !retry {
		m.run.refuse(fmt.Sprintf("step %d: %s is not able to act (%s)", m.step, m.kept, m.able()))
	}
}

// nextToken is the token to try next; false once one acted or none is left.
func (m *replayMove) nextToken() (int64, bool) {
	if m.moved || m.next >= len(m.order) {
		return 0, false
	}
	id := m.order[m.next]
	m.next++
	return id, true
}

// acted ends the step when the token acted; the witness's token not acting is a
// move the run could not make.
func (m *replayMove) acted(acted bool) {
	if acted {
		m.moved = true
		return
	}
	if m.choice != nil {
		m.run.refuse(fmt.Sprintf("step %d: %s did not act", m.step, m.choice.Took))
	}
}

// reported is the token-order choice the step notes: the witness's when it names
// several, else the tokens able to act when several were and the one moved.
func (m *replayMove) reported() (alternatives []string, taken int, ok bool) {
	if !m.moved {
		return nil, 0, false
	}
	if m.choice != nil && len(m.choice.Among) >= 2 {
		return m.choice.Among, m.choice.Taken, true
	}
	if len(m.enabled) >= 2 {
		return m.enabled, m.taken, true
	}
	return nil, 0, false
}

// choose resolves a pick among c.Alternatives by the witness's next move, which
// must be a choice of the same kind at the same place naming one of them; whereOf
// is the place as the run reports it once alternative i is taken, nil for c.Where.
func (r *replayRun) choose(c ChoicePoint, whereOf func(i int) string) int {
	w, unbound := r.current()
	if unbound != nil {
		r.refuseUnbound(*unbound)
		return 0
	}
	alts := strings.Join(c.Alternatives, ", ")
	taken := slices.Index(c.Alternatives, w.Took)
	if whereOf != nil && taken >= 0 {
		c.Where = whereOf(taken)
	}
	if w.Kind != c.Kind || w.Where != c.Where {
		r.refuse("the run faced " + c.Describe())
		return 0
	}
	if w.Step != c.Step {
		r.refuse(fmt.Sprintf("the run is at step %d and step %d had no such move", c.Step, w.Step))
		return 0
	}
	for _, alt := range w.Among {
		if !slices.Contains(c.Alternatives, alt) {
			r.refuse(fmt.Sprintf("%s is not enabled (enabled: %s)", alt, alts))
			return 0
		}
	}
	if taken < 0 {
		r.refuse(fmt.Sprintf("%s is not enabled (enabled: %s)", w.Took, alts))
		return 0
	}
	if (w.Weighted() || c.Weighted()) && !r.weighedAlike(w, c, taken) {
		return 0
	}
	r.next++
	return taken
}

// chooseWeighted follows the witness's move at a weighted decision, the run's choice
// carrying the draw the witness records where one selected the branch.
func (r *replayRun) chooseWeighted(c *ChoicePoint) int {
	w, unbound := r.current()
	taken := r.choose(*c, nil)
	if unbound == nil && r.refused == nil && w.Drawn {
		c.Drew, c.Drawn = w.Drew, true
	}
	return taken
}

// weighedAlike reports whether the witness's weighted move w fits the decision the run
// faces: the same branches weighed the same, and a recorded draw that selects the branch
// taken; a move that does not fit is refused.
func (r *replayRun) weighedAlike(w ChoiceTaken, c ChoicePoint, taken int) bool {
	if !c.Weighted() {
		r.refuse("the run's decision weighs no branch")
		return false
	}
	if !w.Weighted() || len(w.Among) != len(c.Alternatives) {
		r.refuse("the run's decision weighs every branch: " + weightedLabels(c.Alternatives, c.Weights))
		return false
	}
	for i, alt := range w.Among {
		if slices.Index(w.Among, alt) != i {
			r.refuse(fmt.Sprintf("the move weighs %s twice: %s", choiceLabel(alt), choiceLabels(w.Among)))
			return false
		}
		at := slices.Index(c.Alternatives, alt)
		if at < 0 {
			r.refuse(fmt.Sprintf("%s is not among the run's branches: %s", choiceLabel(alt), weightedLabels(c.Alternatives, c.Weights)))
			return false
		}
		if got := c.Weights[at]; got != w.Weights[i] {
			r.refuse(fmt.Sprintf("%s weighs p=%s, not p=%s", choiceLabel(alt), formatWeight(got), formatWeight(w.Weights[i])))
			return false
		}
	}
	if !w.Drawn {
		return true
	}
	if math.IsNaN(w.Drew) || w.Drew < 0 || w.Drew >= 1 {
		r.refuse(fmt.Sprintf("the draw %s is no unit draw in [0, 1)", formatWeight(w.Drew)))
		return false
	}
	total := 0.0
	for _, weight := range c.Weights {
		total += weight
	}
	if pick := weightedPick(c.Weights, total, w.Drew); pick != taken {
		r.refuse(fmt.Sprintf("the draw %s selects %s, not %s", formatWeight(w.Drew), choiceLabel(c.Alternatives[pick]), choiceLabel(w.Took)))
		return false
	}
	return true
}

// mark returns what a probe restores: the run's position in the witness and the
// objects it had bound, which the probe's run may have made and unmade.
func (r *replayRun) mark() func() {
	next, nextDraw, refused := r.next, r.nextDraw, r.refused
	unbound, renumber := slices.Clone(r.unbound), maps.Clone(r.renumber)
	return func() {
		r.next, r.nextDraw, r.refused, r.unbound, r.renumber = next, nextDraw, refused, unbound, renumber
	}
}
