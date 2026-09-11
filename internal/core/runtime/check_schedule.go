package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The `check` policy hands a step exactly the move the model checker selected —
// one token, and at a decision the branch to take — through the seam every
// policy resolves choice points by. It has no spelling: the checker constructs
// it, sets the move before each step and reads back what the step made of it.
// A step it selects no token for settles as a replayed step with no witness move
// does: every token is tried in turn, and none may act.

// ErrCheckRefused is the typed error every move the run could not make as the
// checker selected it wraps.
var ErrCheckRefused = errors.New("check refused")

// CheckMoveError reports a move the checker selected that the run did not make:
// the move and what the run faced instead.
type CheckMoveError struct {
	Token  int64
	Branch int
	Faced  string
}

func (e *CheckMoveError) Error() string {
	move := fmt.Sprintf("token %d", e.Token)
	if e.Token == 0 {
		move = "no token"
	}
	if e.Branch >= 0 {
		move += fmt.Sprintf(", branch %d", e.Branch+1)
	}
	return fmt.Sprintf("%v: %s: %s", ErrCheckRefused, move, e.Faced)
}

// Is makes every CheckMoveError match ErrCheckRefused.
func (e *CheckMoveError) Is(target error) bool { return target == ErrCheckRefused }

// checkScript is the move the checker selected for the next step, shared by the
// policy and the checker: the token to move (0 for none, a settling step) and,
// when its node is a decision, the index among the holding branches to take; -1
// takes the first, so the checker learns how many hold from the choice the step notes.
type checkScript struct {
	token  int64
	branch int
}

// checkPolicy is the `check` policy over the given script.
func checkPolicy(script *checkScript) SchedulePolicy {
	return SchedulePolicy{kind: scheduleCheck, check: script}
}

// set fixes the move the next step makes.
func (s *checkScript) set(token int64, branch int) {
	s.token, s.branch = token, branch
}

// settle makes the next step select no token: every token is tried, none may act.
func (s *checkScript) settle() {
	s.token, s.branch = 0, -1
}

// checkRun resolves one run's steps by the checker's script: the first move the
// run could not make as selected is its refusal.
type checkRun struct {
	script  *checkScript
	refused error
	// decided is the decision the step under way faced, nil for none yet.
	decided *ChoicePoint
	// move is the step under way, nil between steps.
	move *checkMove
}

// refuse records the first selected move the run could not make.
func (r *checkRun) refuse(faced string) {
	if r.refused == nil {
		r.refused = &CheckMoveError{Token: r.script.token, Branch: r.script.branch, Faced: faced}
	}
}

// checkMove is one step under check: the tokens tried in order, and those able to
// act as the trace names them, for the choice the step notes.
type checkMove struct {
	run      *checkRun
	step     int
	selected bool
	order    []int64
	next     int
	moved    bool
	enabled  []string
	taken    int
}

// beginStep resolves the step as a replayed one resolves a witness move: the
// selected token alone when two or more are able to act, else — one at most
// able to act — that one first and the rest after, as a settling step tries them.
func (r *checkRun) beginStep(tokens stepTokens) *checkMove {
	m := &checkMove{run: r, step: tokens.step, taken: -1, selected: r.script.token != 0}
	r.decided, r.move = nil, m
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

// nextToken is the token to try next; false once one acted or none is left.
func (m *checkMove) nextToken() (int64, bool) {
	if m.moved || m.next >= len(m.order) {
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

// choose resolves a pick among c.Alternatives by the selected branch, which must be
// one of them at a decision (the first when none is selected); any other pick faced
// is a move the checker did not select. The decision faced is kept for the checker.
func (r *checkRun) choose(c ChoicePoint, whereOf func(i int) string) int {
	n := len(c.Alternatives)
	if c.Kind != ChoiceDecisionBranch {
		if whereOf != nil {
			c.Where = whereOf(0)
		}
		r.refuse("the run faced " + c.Describe())
		return 0
	}
	if r.decided != nil {
		r.refuse(fmt.Sprintf("the run faced %s after %s in one step", c.Describe(), r.decided.Describe()))
		return 0
	}
	pick := 0
	if r.script.branch >= n {
		r.refuse(fmt.Sprintf("the run faced %s and branch %d is not among them", c.Describe(), r.script.branch+1))
	} else if r.script.branch >= 0 {
		pick = r.script.branch
	}
	c.Taken = pick
	r.decided = &c
	return pick
}

// mark returns what a probe restores: whether a move was refused and what the
// step under way faced.
func (r *checkRun) mark() func() {
	refused, decided, move := r.refused, r.decided, r.move
	return func() { r.refused, r.decided, r.move = refused, decided, move }
}
