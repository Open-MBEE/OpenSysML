package solve

import (
	"context"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CoreBudgetEnv names the environment variable that overrides how long a core
// may be shrunk for, as a Go duration ("5s", "500ms").
const CoreBudgetEnv = "OPENSYSML_SMT_CORE_BUDGET"

// DefaultCoreBudget is how long reduction is given in total, in the spirit of
// the runtime's step budgets: past it the solver's own core is reported as it
// came, said to be not necessarily minimal.
const DefaultCoreBudget = 30 * time.Second

// DefaultMaxCoreMembers is the largest core reduction is attempted over, since
// each dropped candidate costs a solver call.
const DefaultMaxCoreMembers = 64

// Core is a set of assertions whose conjunction is unsatisfiable: the conditions
// that conflict, each carrying the provenance of the condition it came from.
type Core struct {
	// Members are the conflicting assertions, in the query's assertion order.
	Members []Assertion

	// Indices are the positions those assertions hold in Query.Assertions.
	Indices []int

	// Minimal reports that dropping any one member was tried and left the rest
	// satisfiable, so every member is needed. It is false for a core reported as
	// the solver produced it, which is unsatisfiable but not necessarily
	// irreducible.
	Minimal bool

	// Note says why minimality was not established, empty when it was.
	Note string

	// Rounds is how many further solver calls reduction made.
	Rounds int

	// Elapsed is how long reduction took.
	Elapsed time.Duration
}

// Explain discovers a solver and asks it to explain the query.
func Explain(ctx context.Context, q *Query) (*Result, error) {
	solver, err := Discover()
	if err != nil {
		return nil, err
	}
	return solver.Explain(ctx, q)
}

// Explain asks the solver about the query and, when the verdict is unsat, for
// the assertions that conflict: the solver's own unsat core, shrunk to a minimal
// one while the budget lasts. sat and unknown carry no core, and a solver that
// refuses cores or names an assertion the query did not assert is an error
// rather than an empty or invented core.
func (s *Solver) Explain(ctx context.Context, q *Query) (*Result, error) {
	if err := s.require(ctx, q, "explaining a conflict", CapUnsatCores); err != nil {
		return nil, err
	}
	result, err := s.check(ctx, q, nil, true)
	if err != nil {
		return nil, err
	}
	if result.Status != StatusUnsat || result.Core == nil {
		return result, nil
	}
	core, err := s.reduce(ctx, q, result.Core.Indices)
	if err != nil {
		return nil, err
	}
	// Elapsed covers the whole explanation, shrinking the core included.
	result.Elapsed += core.Elapsed
	result.Core = core
	return result, nil
}

// check runs one labelled query in a fresh solver process, asking for the core
// when core is set and the verdict is unsat.
func (s *Solver) check(ctx context.Context, q *Query, include []int, core bool) (*Result, error) {
	return s.solve(ctx, q, func(sess *session) (*Result, error) {
		return sess.explain(q, include, core)
	})
}

// reduce shrinks the solver's core by dropping one member at a time, each round
// a fresh solver call, so a reported minimal core is one every member of is
// needed. Exhausting the budget or an undecided round reports the core reached
// so far, said not to be minimal.
func (s *Solver) reduce(ctx context.Context, q *Query, raw []int) (*Core, error) {
	started := time.Now()
	if len(raw) > s.maxCoreMembers() {
		return &Core{
			Members: membersOf(q, raw), Indices: raw,
			Note: "the core has more than " + strconv.Itoa(s.maxCoreMembers()) + " members, past the bound on shrinking one",
		}, nil
	}
	deadline := started.Add(s.coreBudget())
	kept, rounds := append([]int(nil), raw...), 0
	for _, candidate := range raw {
		if len(kept) <= 1 {
			// One assertion that is unsatisfiable alone is already irreducible.
			break
		}
		if !time.Now().Before(deadline) {
			return &Core{
				Members: membersOf(q, kept), Indices: kept, Rounds: rounds, Elapsed: time.Since(started),
				Note: "shrinking the core ran out of its " + s.coreBudget().String() + " budget",
			}, nil
		}
		trial := without(kept, candidate)
		result, err := s.check(ctx, q, trial, false)
		if err != nil {
			return nil, err
		}
		rounds++
		if result.Status == StatusUnknown {
			return &Core{
				Members: membersOf(q, kept), Indices: kept, Rounds: rounds, Elapsed: time.Since(started),
				Note: "the solver did not decide whether the core shrinks further: " + undecided(result),
			}, nil
		}
		if result.Status == StatusUnsat {
			kept = trial
		}
	}
	return &Core{
		Members: membersOf(q, kept), Indices: kept, Minimal: true,
		Rounds: rounds, Elapsed: time.Since(started),
	}, nil
}

// coreBudget is how long this solver may shrink a core for.
func (s *Solver) coreBudget() time.Duration {
	if s.CoreBudget > 0 {
		return s.CoreBudget
	}
	return coreBudgetFromEnv()
}

// maxCoreMembers is the largest core this solver shrinks.
func (s *Solver) maxCoreMembers() int {
	if s.MaxCoreMembers > 0 {
		return s.MaxCoreMembers
	}
	return DefaultMaxCoreMembers
}

// coreBudgetFromEnv reads the budget override, falling back to the default for
// an unset, unparsable or non-positive value.
func coreBudgetFromEnv() time.Duration {
	text := strings.TrimSpace(os.Getenv(CoreBudgetEnv))
	if text == "" {
		return DefaultCoreBudget
	}
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return DefaultCoreBudget
	}
	return d
}

// undecided says why a round was not decided, as the reason the solver gave or
// the timeout it hit.
func undecided(result *Result) string {
	switch {
	case result.Reason != "":
		return result.Reason
	case result.TimedOut:
		return "it ran out of time"
	default:
		return "it gave no reason"
	}
}

// membersOf reads the assertions the indices name, which is where a core's
// provenance comes from: the query itself, not a table beside it.
func membersOf(q *Query, indices []int) []Assertion {
	out := make([]Assertion, 0, len(indices))
	for _, i := range indices {
		out = append(out, q.Assertions[i])
	}
	return out
}

// without is the indices with one dropped, keeping their order.
func without(indices []int, drop int) []int {
	out := make([]int, 0, len(indices))
	for _, i := range indices {
		if i != drop {
			out = append(out, i)
		}
	}
	return out
}

// coreError builds a failure to explain an unsat verdict, which is never a core.
func (s *Solver) coreError(detail, stderr string) error {
	return &CoreError{Solver: s.Name, Detail: detail, Stderr: stderr}
}

// explain holds the core-producing dialogue: the labelled script, the verdict,
// and then the core or the reason the solver did not decide.
func (s *session) explain(q *Query, include []int, core bool) (*Result, error) {
	if err := s.send(coreScript(q, include, false)); err != nil {
		return nil, err
	}
	status, err := s.verdict()
	if err != nil {
		return nil, err
	}
	result := &Result{Status: status}
	switch status {
	case StatusUnsat:
		if !core {
			return result, nil
		}
		indices, err := s.unsatCore(q, include)
		if err != nil {
			return nil, err
		}
		result.Core = &Core{Members: membersOf(q, indices), Indices: indices}
	case StatusUnknown:
		result.Reason = s.reasonUnknown()
	}
	return result, nil
}

// unsatCore asks for the core and reads the labels back to the assertions they
// name. A label the query did not issue, a duplicate, an unreadable reply or an
// empty core is a failure: a core is either the solver's own or nothing.
func (s *session) unsatCore(q *Query, include []int) ([]int, error) {
	if err := s.send("(get-unsat-core)\n"); err != nil {
		return nil, err
	}
	reply, err := s.read("get-unsat-core")
	if err != nil {
		return nil, err
	}
	if msg, ok := reply.isError(); ok {
		return nil, s.solver.coreError("it would not report an unsat core: "+msg, s.stderrText())
	}
	if !reply.IsList {
		return nil, s.solver.coreError("its core is "+quoteReply(reply)+" rather than a list of assertion names", s.stderrText())
	}
	issued, seen := indexSet(include), make(map[int]bool, len(reply.List))
	out := make([]int, 0, len(reply.List))
	for _, elem := range reply.List {
		// A label is written literally, and `!` cannot occur in a name a script
		// declares, so the atom is the label as it was issued.
		i, ok := coreLabelIndex(elem.Atom)
		if elem.IsList || !ok || i >= len(q.Assertions) || (issued != nil && !issued[i]) {
			return nil, s.solver.coreError("its core names "+quoteReply(elem)+", which the query did not assert", s.stderrText())
		}
		if seen[i] {
			return nil, s.solver.coreError("its core names "+elem.Atom+" twice", s.stderrText())
		}
		seen[i] = true
		out = append(out, i)
	}
	if len(out) == 0 {
		return nil, s.solver.coreError("its core is empty, though it answered unsat", s.stderrText())
	}
	sort.Ints(out)
	return out, nil
}
