package hideo

import (
	"fmt"
	"sort"
	"strings"
)

// Result is one case of one suite as a run recorded it.
type Result struct {
	Outcome Outcome
	Detail  string
}

// Run is what one execution of the suites recorded: per suite, every case by id.
// A suite that wrote no results is absent.
type Run map[string]map[string]Result

// Move is one case of one suite whose outcome is not the baseline's.
type Move struct {
	Suite, ID string
	Was, Now  Outcome
	Detail    string
}

func (m Move) String() string {
	s := fmt.Sprintf("%s %s: %s -> %s", m.Suite, m.ID, m.Was, m.Now)
	if m.Detail != "" {
		s += " (" + bound(m.Detail, maxDetail) + ")"
	}
	return s
}

// Movement is how a run differs from the baseline, by what each difference means.
type Movement struct {
	// Provisioning names each suite whose results cannot be read as verdicts.
	Provisioning []string
	// Worse, Better and Changed are the cases that fell, rose, or moved between equal ranks.
	Worse, Better, Changed []Move
}

// Reproduces reports whether the run is exactly the baseline.
func (m Movement) Reproduces() bool {
	return len(m.Provisioning)+len(m.Worse)+len(m.Better)+len(m.Changed) == 0
}

// provisioningFailure says why a suite's results are not verdicts on the
// implementation, or "" when they are: no rows, or an error on every case.
func provisioningFailure(suite string, results map[string]Result) string {
	if len(results) == 0 {
		return suite + ": no results were recorded (a build, setup or service-start failure; see the suite's log)"
	}
	for _, r := range results {
		if r.Outcome != Error {
			return ""
		}
	}
	return fmt.Sprintf("%s: every one of its %d cases is an error (the suite could not reach the service; see the suite's log)", suite, len(results))
}

// Compare holds a run to the baseline. A suite that failed to provision, or that
// did not record exactly the baseline's cases, is reported as such and its cases are not compared.
func Compare(committed *Baseline, fresh Run) Movement {
	var m Movement
	ids := committed.IDs()
	for column, suite := range Suites {
		results := fresh[suite]
		if why := provisioningFailure(suite, results); why != "" {
			m.Provisioning = append(m.Provisioning, why)
			continue
		}
		if why := caseSetDifference(suite, ids, results); why != "" {
			m.Provisioning = append(m.Provisioning, why)
			continue
		}
		for _, id := range ids {
			was, now := committed.Cases[id][column], results[id]
			move := Move{Suite: suite, ID: id, Was: was, Now: now.Outcome, Detail: now.Detail}
			switch {
			case was == now.Outcome:
			case now.Outcome.rank() < was.rank():
				m.Worse = append(m.Worse, move)
			case now.Outcome.rank() > was.rank():
				m.Better = append(m.Better, move)
			default:
				m.Changed = append(m.Changed, move)
			}
		}
	}
	return m
}

// caseSetDifference names the cases a suite recorded beyond, or short of, the baseline's.
func caseSetDifference(suite string, ids []string, results map[string]Result) string {
	var missing, extra []string
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
		if _, ok := results[id]; !ok {
			missing = append(missing, id)
		}
	}
	for id := range results {
		if !known[id] {
			extra = append(extra, id)
		}
	}
	if len(missing)+len(extra) == 0 {
		return ""
	}
	sort.Strings(extra)
	return fmt.Sprintf("%s: recorded %d cases where the baseline holds %d (missing: %s; not in the baseline: %s)",
		suite, len(results), len(ids), sample(missing), sample(extra))
}

const (
	maxNamed  = 40
	maxDetail = 200
)

func sample(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	if len(ids) > 5 {
		return fmt.Sprintf("%s and %d more", strings.Join(ids[:5], ", "), len(ids)-5)
	}
	return strings.Join(ids, ", ")
}

func bound(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// Describe renders the movement for a reader who has to act on it: each class
// under its own heading, every moved case named up to a bound, then what to do.
func (m Movement) Describe() string {
	var b strings.Builder
	section := func(heading string, lines []string) {
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s (%d):\n", heading, len(lines))
		for i, line := range lines {
			if i == maxNamed {
				fmt.Fprintf(&b, "  … and %d more\n", len(lines)-maxNamed)
				break
			}
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	section("provisioning failure, not a verdict on the implementation: investigate the run, do not re-record", m.Provisioning)
	section("worse than the baseline: a regression to fix, or to adjudicate and record with -update -accept-regressions", moves(m.Worse))
	section("better than the baseline: adjudicate, then record with -update", moves(m.Better))
	section("changed without getting better or worse: adjudicate, then record with -update", moves(m.Changed))
	return b.String()
}

func moves(ms []Move) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.String()
	}
	return out
}

// Record turns a run into a baseline measured at pin. It refuses a run with a
// provisioning failure or with suites that disagree on which cases exist.
func Record(pin Pin, fresh Run) (*Baseline, error) {
	b := &Baseline{Pin: pin, Cases: make(map[string][]Outcome)}
	for id := range fresh[Suites[0]] {
		b.Cases[id] = make([]Outcome, len(Suites))
	}
	ids := b.IDs()
	var failures []string
	for column, suite := range Suites {
		results := fresh[suite]
		why := provisioningFailure(suite, results)
		if why == "" {
			why = caseSetDifference(suite, ids, results)
		}
		if why != "" {
			failures = append(failures, why)
			continue
		}
		for _, id := range ids {
			b.Cases[id][column] = results[id].Outcome
		}
	}
	if len(failures) > 0 {
		return nil, fmt.Errorf("this run cannot be recorded:\n  %s", strings.Join(failures, "\n  "))
	}
	return b, nil
}
