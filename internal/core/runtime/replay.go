package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The `replay:<file>` policy follows a witness — the choice lines `explore` prints,
// one per move — move for move, then goes on as `reverse` does, one token per step:
// the last token able to act, and at every other pick the alternative `reverse` takes.
// The runs of one context follow it in turn, as an exploration records them, so a
// behavior an object runs before the action takes its moves first (Context.ChoicesTaken).
// A move the run cannot make where the witness makes it, or one facing other
// alternatives than the witness names, is refused, naming the move: a witness that
// cannot be followed is never silently resolved.

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

// ReplayPolicy is the `replay` policy over a witness held in memory. Its spelling
// names no file, so it does not read back; write the choices out to name them.
func ReplayPolicy(choices []ChoiceTaken) SchedulePolicy {
	return SchedulePolicy{kind: scheduleReplay, replay: &replayScript{choices: slices.Clone(choices)}}
}

// Replay returns the witness of a `replay` policy, and whether the policy is one.
func (p SchedulePolicy) Replay() ([]ChoiceTaken, bool) {
	if p.kind != scheduleReplay {
		return nil, false
	}
	return slices.Clone(p.replay.choices), true
}

// Unfollowed is the first witness move the context's runs under a `replay` policy
// could not make — one refused, or one left over when the runs ended — as a
// ReplayError; nil when they followed the witness whole or ran under another policy.
func (ctx *Context) Unfollowed() error {
	if ctx.replaying == nil {
		return nil
	}
	return ctx.replaying.unfollowed("the run ended")
}

// Choice is the choice point as a witness lists it: what ChoiceTaken.String spells
// and ParseChoice reads back.
func (c ChoicePoint) Choice() ChoiceTaken {
	taken := ChoiceTaken{Kind: c.Kind, Step: c.Step, Where: c.Where, Alternatives: len(c.Alternatives), Taken: c.Taken, Among: slices.Clone(c.Alternatives)}
	if c.Taken >= 0 && c.Taken < len(c.Alternatives) {
		taken.Took = c.Alternatives[c.Taken]
	}
	return taken
}

// replayScript is the witness a replay policy follows and the file it was read from.
type replayScript struct {
	file    string
	choices []ChoiceTaken
}

// ParseChoices reads a witness header: choices as ChoiceTaken.String spells them, one
// per line or joined by `; `, ending at the first blank line after it; what follows is ignored.
func ParseChoices(text string) ([]ChoiceTaken, error) {
	var choices []ChoiceTaken
	begun := false
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if begun {
				break
			}
			continue
		}
		begun = true
		if line == "no choice points" {
			continue
		}
		for _, part := range strings.Split(line, "; ") {
			c, err := ParseChoice(part)
			if err != nil {
				var parse *ChoiceParseError
				if errors.As(err, &parse) {
					parse.Line = i + 1
				}
				return nil, err
			}
			choices = append(choices, c)
		}
	}
	return choices, nil
}

// ParseChoice reads one choice as ChoiceTaken.String spells it: `step N: T first
// of A, B` (a token order), `step N: decision D -> B` (a branch), `S -> T` (a
// transition) and `W: X first of A, B` (a region order, or a due order when W is an
// instant `t=…`). A branch or transition line names no alternatives, so the run
// resolves it against those it faces.
func ParseChoice(text string) (ChoiceTaken, error) {
	text = strings.TrimSpace(text)
	fail := func(reason string) (ChoiceTaken, error) {
		return ChoiceTaken{}, &ChoiceParseError{Text: text, Reason: reason}
	}
	step, rest := 0, text
	if after, ok := strings.CutPrefix(text, "step "); ok {
		digits, tail, found := strings.Cut(after, ": ")
		n, err := strconv.Atoi(digits)
		if !found || err != nil || n < 1 {
			return fail("step needs a positive number and a colon: step <n>: …")
		}
		step, rest = n, tail
	}
	if left, right, ok := strings.Cut(rest, " first of "); ok {
		among := strings.Split(right, ", ")
		c := ChoiceTaken{Kind: ChoiceTokenOrder, Step: step, Alternatives: len(among), Among: among, Took: left}
		if step == 0 {
			where, took, named := strings.Cut(left, ": ")
			if !named {
				return fail("an order outside a step needs where it was made: <where>: <took> first of …")
			}
			c.Kind, c.Where, c.Took = ChoiceRegionOrder, where, took
			if strings.HasPrefix(where, "t=") {
				c.Kind = ChoiceDueOrder
			}
		}
		c.Taken = slices.Index(among, c.Took)
		if c.Taken < 0 {
			return fail(fmt.Sprintf("%s is not among %s", c.Took, right))
		}
		return c, nil
	}
	if where, took, ok := strings.Cut(rest, " -> "); ok {
		if where == "" || took == "" {
			return fail("a branch or transition needs both sides of ->")
		}
		c := ChoiceTaken{Kind: ChoiceTransition, Where: where, Took: took}
		if step > 0 {
			c.Kind, c.Step = ChoiceDecisionBranch, step
		}
		return c, nil
	}
	return fail("not a token order, branch, transition or region order")
}

// replayRun follows a context's witness: the moves left and the first it refused.
type replayRun struct {
	choices []ChoiceTaken
	next    int
	refused error
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

// unfollowed is the refusal of a run that ended with moves left, nil otherwise.
func (r *replayRun) unfollowed(how string) error {
	if r.following() {
		r.refuse(how)
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
	ids     []int64
	taken   int
}

// beginStep resolves the step: two tokens able to act take the witness's next move
// (a token order at this step, among exactly them) or refuse it; fewer is no choice
// and takes no move. With the witness spent the tokens are tried as `reverse` tries them.
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
	m.ids = enabled
	if !r.following() || len(enabled) < 2 {
		m.order = slices.Concat(enabled, rest, held)
		slices.Reverse(m.order[:len(enabled)])
		slices.Reverse(m.order[len(enabled) : len(enabled)+len(rest)])
		slices.Reverse(m.order[len(enabled)+len(rest):])
		return m
	}
	able := "able to act: " + strings.Join(m.enabled, ", ")
	c := &r.choices[r.next]
	switch {
	case c.Kind == ChoiceTokenOrder && c.Step == tokens.step:
	case c.Step > 0 && c.Step < tokens.step:
		r.refuse(fmt.Sprintf("the run is at step %d and step %d had no such move", tokens.step, c.Step))
		return m
	default:
		r.refuse(fmt.Sprintf("step %d: the run must pick a token (%s) and the witness names none", tokens.step, able))
		return m
	}
	for _, alt := range c.Among {
		if !slices.Contains(m.enabled, alt) {
			r.refuse(fmt.Sprintf("step %d: %s is not able to act (%s)", tokens.step, alt, able))
			return m
		}
	}
	if !sameAlternatives(c.Among, m.enabled) {
		r.refuse(fmt.Sprintf("step %d: the witness names %s and the run has more (%s)", tokens.step, strings.Join(c.Among, ", "), able))
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
func (m *replayMove) acted(id int64, acted bool) {
	if acted {
		m.moved = true
		if i := slices.Index(m.ids, id); i >= 0 {
			m.taken = i
		}
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

// sameAlternatives reports whether the witness names exactly the alternatives the
// run faces, in any order.
func sameAlternatives(named, faced []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(named)), slices.Sorted(slices.Values(faced)))
}

// choose resolves a pick among c.Alternatives by the witness's next move, which
// must be a choice of the same kind at the same place, among exactly them where it
// names them, taking one of them; whereOf is the place as the run reports it once
// alternative i is taken, nil for c.Where. The caller picks as `reverse` does once
// the witness is spent.
func (r *replayRun) choose(c ChoicePoint, whereOf func(i int) string) int {
	w := r.choices[r.next]
	alts := strings.Join(c.Alternatives, ", ")
	taken := slices.Index(c.Alternatives, w.Took)
	if w.Kind != c.Kind {
		r.refuse("the run faced " + c.Describe())
		return 0
	}
	if whereOf != nil && taken < 0 {
		r.refuse(fmt.Sprintf("%s is not enabled (enabled: %s)", w.Took, alts))
		return 0
	}
	if whereOf != nil {
		c.Where = whereOf(taken)
	}
	if w.Where != c.Where {
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
	if len(w.Among) > 0 && !sameAlternatives(w.Among, c.Alternatives) {
		r.refuse(fmt.Sprintf("the witness names %s and more are enabled (enabled: %s)", strings.Join(w.Among, ", "), alts))
		return 0
	}
	if taken < 0 {
		r.refuse(fmt.Sprintf("%s is not enabled (enabled: %s)", w.Took, alts))
		return 0
	}
	r.next++
	return taken
}

// mark returns what a probe restores: the run's position in the witness.
func (r *replayRun) mark() func() {
	next, refused := r.next, r.refused
	return func() { r.next, r.refused = next, refused }
}
