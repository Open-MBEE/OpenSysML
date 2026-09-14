package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The `check` policy hands a step exactly the move the model checker selected —
// the executor to move, one token of an action, and the picks resolving the choice
// points the move draws in order — through the seam every policy resolves choice
// points by. It has no spelling: the checker constructs it, sets the move before
// each step and reads back the choices the step drew beyond the picks it was given.
// A step it selects no token for settles as a replayed step with no witness move
// does: every token is tried in turn, and none may act.

// ErrCheckRefused is the typed error every move the run could not make as the
// checker selected it wraps.
var ErrCheckRefused = errors.New("check refused")

// CheckMoveError reports a move the checker selected that the run did not make:
// the move, as the checker spells it, and what the run faced instead.
type CheckMoveError struct {
	Move  string
	Faced string
}

func (e *CheckMoveError) Error() string {
	return fmt.Sprintf("%v: %s: %s", ErrCheckRefused, e.Move, e.Faced)
}

// Is makes every CheckMoveError match ErrCheckRefused.
func (e *CheckMoveError) Is(target error) bool { return target == ErrCheckRefused }

// checkScript is the move the checker selected for the next step, shared by the
// policy and the checker: the executor whose step it is, the token to move (0 for
// none, a settling step or a state machine's), the index answering the due-order
// choice the step draws first, when several executors are due, and the picks
// answering the choice points the move draws after it, in order. A choice past
// the picks takes its first alternative and is reported back, so the checker
// learns what else the move could have picked. A step of any other executor — a
// typed action a token performs within its move — runs in declared order.
type checkScript struct {
	owner checkedExecutor
	token int64
	due   int
	picks []int
}

// checkPolicy is the `check` policy over the given script.
func checkPolicy(script *checkScript) SchedulePolicy {
	return SchedulePolicy{kind: scheduleCheck, check: script}
}

// set fixes the move the next step of owner makes.
func (s *checkScript) set(owner checkedExecutor, token int64, due int, picks []int) {
	s.owner, s.token, s.due, s.picks = owner, token, due, picks
}

// settle makes the next step select no token and draw no choice: every token is
// tried, none may act.
func (s *checkScript) settle() {
	s.token, s.due, s.picks = 0, -1, nil
}

// checkRun resolves one run's steps by the checker's script: the first move the
// run could not make as selected is its refusal.
type checkRun struct {
	script  *checkScript
	refused error
	// picked counts the picks the move under way consumed; drawn are the choice
	// points it faced past them, each taken at its first alternative.
	picked int
	drawn  []ChoicePoint
	// move is the step under way, nil between steps.
	move *checkMove
}

// begin starts a move: the picks are consumed from the first, nothing is drawn yet.
func (r *checkRun) begin() {
	r.picked, r.drawn, r.move = 0, nil, nil
}

// refuse records the first selected move the run could not make.
func (r *checkRun) refuse(faced string) {
	if r.refused == nil {
		r.refused = &CheckMoveError{Move: r.script.String(), Faced: faced}
	}
}

// String spells the scripted move: the executor, its token, or none, and its picks.
func (s *checkScript) String() string {
	move := "no token"
	if s.token != 0 {
		move = fmt.Sprintf("token %d", s.token)
	}
	if s.owner != nil {
		move = s.owner.dueLabel() + ", " + move
	}
	for _, pick := range s.picks {
		move += fmt.Sprintf(", pick %d", pick+1)
	}
	return move
}

// checkMove is one step under check: the tokens tried in order, and those able to
// act as the trace names them, for the choice the step notes.
type checkMove struct {
	run      *checkRun
	step     int
	selected bool
	nested   bool // a step of a run within the move, resolved in declared order
	order    []int64
	next     int
	moved    bool
	enabled  []string
	ids      []int64 // the tokens able to act, as enabled labels them
	taken    int
}

// beginStep resolves the step as a replayed one resolves a witness move: the
// selected token alone when two or more are able to act, else — one at most
// able to act — that one first and the rest after, as a settling step tries them.
func (r *checkRun) beginStep(tokens stepTokens) *checkMove {
	m := &checkMove{run: r, step: tokens.step, taken: -1, selected: r.script.token != 0}
	if tokens.owner != r.script.owner {
		m.nested, m.selected = true, false
	}
	r.move = m
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
	m.ids = enabled
	m.enabled = make([]string, len(enabled))
	for i, id := range enabled {
		m.enabled[i] = tokens.label(id)
		if m.selected && id == r.script.token {
			m.taken = i
		}
	}
	if !m.selected {
		m.order = slices.Concat(enabled, rest, held)
		return m
	}
	if m.taken < 0 {
		able := "none is able to act"
		if len(m.enabled) > 0 {
			able = "able to act: " + strings.Join(m.enabled, ", ")
		}
		var faced string
		switch {
		case !slices.Contains(tokens.ids, r.script.token):
			faced = fmt.Sprintf("token %d is not one the step may move", r.script.token)
		case tokens.held[r.script.token]:
			faced = tokens.label(r.script.token) + " is held"
		default:
			faced = tokens.label(r.script.token) + " is parked"
		}
		r.refuse(fmt.Sprintf("step %d: %s (%s)", tokens.step, faced, able))
		return m
	}
	if len(enabled) >= 2 {
		m.order = []int64{r.script.token}
	} else {
		m.order = slices.Concat(enabled, rest, held)
	}
	return m
}

// nextToken is the token to try next; false once one acted or none is left, which ends the step.
func (m *checkMove) nextToken() (int64, bool) {
	if m.moved || m.next >= len(m.order) {
		if m.run.move == m {
			m.run.move = nil
		}
		return 0, false
	}
	id := m.order[m.next]
	m.next++
	return id, true
}

// acted ends the step when the token acted. With two or more able to act, the
// selected token not acting — or one acting where none was selected — is a move
// the run made otherwise than the checker selected.
func (m *checkMove) acted(id int64, acted bool) {
	if acted {
		m.moved = true
		if m.nested {
			m.taken = slices.Index(m.ids, id)
			return
		}
		if !m.selected && len(m.enabled) >= 2 {
			m.run.refuse(fmt.Sprintf("step %d: token %d acted where the run had to pick one (able to act: %s)",
				m.step, id, strings.Join(m.enabled, ", ")))
		}
		return
	}
	if m.selected && len(m.enabled) >= 2 {
		m.run.refuse(fmt.Sprintf("step %d: %s did not act", m.step, m.enabled[m.taken]))
	}
}

// reported is the token-order choice the step notes: the tokens able to act when
// several were, and the one moved among them.
func (m *checkMove) reported() (alternatives []string, taken int, ok bool) {
	if !m.moved || len(m.enabled) < 2 || m.taken < 0 {
		return nil, 0, false
	}
	return m.enabled, m.taken, true
}

// choose resolves a pick among c.Alternatives: a due order by the index the
// script names, consumed by the one draw a move makes, any other choice by the
// next pick of the script, which must be one of the alternatives, or — the picks
// consumed — the first alternative, the choice kept for the checker. A pick
// within a nested step takes the first alternative, as a declared run does.
func (r *checkRun) choose(c ChoicePoint, whereOf func(i int) string) int {
	if r.move != nil && r.move.nested {
		return 0
	}
	n := len(c.Alternatives)
	if c.Kind == ChoiceDueOrder {
		if r.script.due < 0 || r.script.due >= n {
			if whereOf != nil {
				c.Where = whereOf(0)
			}
			r.refuse("the run faced " + c.Describe())
			return 0
		}
		due := r.script.due
		r.script.due = -1
		return due
	}
	if r.picked >= len(r.script.picks) {
		c.Taken = 0
		if whereOf != nil {
			c.Where = whereOf(0)
		}
		r.drawn = append(r.drawn, c)
		return 0
	}
	pick := r.script.picks[r.picked]
	r.picked++
	if pick >= n {
		if whereOf != nil {
			c.Where = whereOf(0)
		}
		r.refuse(fmt.Sprintf("the run faced %s and pick %d is not among them", c.Describe(), pick+1))
		return 0
	}
	return pick
}

// mark returns what a probe restores: whether a move was refused and what the
// step under way drew.
func (r *checkRun) mark() func() {
	refused, picked, drawn, move := r.refused, r.picked, len(r.drawn), r.move
	return func() { r.refused, r.picked, r.drawn, r.move = refused, picked, r.drawn[:drawn], move }
}
