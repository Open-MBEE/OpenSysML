package hideo

import (
	"strings"
	"testing"
)

var testPin = Pin{Repository: "https://example.test/suite.git", Commit: "0123456789abcdef0123456789abcdef01234567"}

// uniform is a run in which every suite records the same outcome per case.
func uniform(cases map[string]Outcome) Run {
	run := make(Run, len(Suites))
	for _, suite := range Suites {
		run[suite] = make(map[string]Result, len(cases))
		for id, o := range cases {
			run[suite][id] = Result{Outcome: o}
		}
	}
	return run
}

func baselineOf(t *testing.T, cases map[string]Outcome) *Baseline {
	t.Helper()
	b, err := Record(testPin, uniform(cases))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var threeCases = map[string]Outcome{"A01-001": Pass, "A01-002": Unsupported, "B02-001": Fail}

func TestCompareReproduces(t *testing.T) {
	m := Compare(baselineOf(t, threeCases), uniform(threeCases))
	if !m.Reproduces() {
		t.Fatalf("an identical run moved:\n%s", m.Describe())
	}
}

func TestCompareClassifiesEveryMove(t *testing.T) {
	tests := []struct {
		was, now Outcome
		class    string
	}{
		{Pass, Fail, "worse"},
		{Pass, Error, "worse"},
		{Pass, Unsupported, "worse"},
		{Pass, Skip, "worse"},
		{Unsupported, Fail, "worse"},
		{Unsupported, Error, "worse"},
		{Skip, Fail, "worse"},
		{Unsupported, Pass, "better"},
		{Fail, Pass, "better"},
		{Error, Pass, "better"},
		{Fail, Unsupported, "better"},
		{Fail, Error, "changed"},
		{Error, Fail, "changed"},
		{Unsupported, Skip, "changed"},
	}
	for _, tt := range tests {
		t.Run(string(tt.was)+"_to_"+string(tt.now), func(t *testing.T) {
			committed := baselineOf(t, map[string]Outcome{"A01-001": tt.was, "A01-002": Pass})
			fresh := uniform(map[string]Outcome{"A01-001": tt.was, "A01-002": Pass})
			fresh[Suites[1]]["A01-001"] = Result{Outcome: tt.now, Detail: "why"}
			m := Compare(committed, fresh)
			got := map[string][]Move{"worse": m.Worse, "better": m.Better, "changed": m.Changed}
			want := Move{Suite: Suites[1], ID: "A01-001", Was: tt.was, Now: tt.now, Detail: "why"}
			for class, moves := range got {
				if class == tt.class {
					if len(moves) != 1 || moves[0] != want {
						t.Errorf("%s = %v, want exactly %v", class, moves, want)
					}
				} else if len(moves) != 0 {
					t.Errorf("%s = %v, want none", class, moves)
				}
			}
			if len(m.Provisioning) != 0 || m.Reproduces() {
				t.Errorf("provisioning = %v, reproduces = %v", m.Provisioning, m.Reproduces())
			}
			if text := m.Describe(); !strings.Contains(text, want.String()) {
				t.Errorf("the description does not name the case:\n%s", text)
			}
		})
	}
}

func TestCompareProvisioningFailureIsNotARegression(t *testing.T) {
	committed := baselineOf(t, threeCases)

	t.Run("every case an error", func(t *testing.T) {
		fresh := uniform(threeCases)
		for id := range fresh[Suites[0]] {
			fresh[Suites[0]][id] = Result{Outcome: Error, Detail: "connection refused"}
		}
		m := Compare(committed, fresh)
		if len(m.Provisioning) != 1 || !strings.HasPrefix(m.Provisioning[0], Suites[0]+": every one of its 3 cases is an error") {
			t.Errorf("provisioning = %v", m.Provisioning)
		}
		if len(m.Worse)+len(m.Better)+len(m.Changed) != 0 || m.Reproduces() {
			t.Errorf("a suite that could not run was compared case by case:\n%s", m.Describe())
		}
	})
	t.Run("no results", func(t *testing.T) {
		fresh := uniform(threeCases)
		delete(fresh, Suites[2])
		m := Compare(committed, fresh)
		if len(m.Provisioning) != 1 || !strings.HasPrefix(m.Provisioning[0], Suites[2]+": no results") || len(m.Worse) != 0 {
			t.Errorf("movement = %+v", m)
		}
	})
	t.Run("a different case set", func(t *testing.T) {
		fresh := uniform(threeCases)
		delete(fresh[Suites[3]], "B02-001")
		fresh[Suites[3]]["Z99-001"] = Result{Outcome: Pass}
		m := Compare(committed, fresh)
		if len(m.Provisioning) != 1 || !strings.Contains(m.Provisioning[0], "missing: B02-001; not in the baseline: Z99-001") || len(m.Worse) != 0 {
			t.Errorf("movement = %+v", m)
		}
	})
	t.Run("one error among verdicts is a regression", func(t *testing.T) {
		fresh := uniform(threeCases)
		fresh[Suites[0]]["A01-001"] = Result{Outcome: Error}
		m := Compare(committed, fresh)
		if len(m.Provisioning) != 0 || len(m.Worse) != 1 {
			t.Errorf("movement = %+v", m)
		}
	})
}

func TestRecordRefusesAnUnprovisionedRun(t *testing.T) {
	fresh := uniform(threeCases)
	for id := range fresh[Suites[1]] {
		fresh[Suites[1]][id] = Result{Outcome: Error}
	}
	if _, err := Record(testPin, fresh); err == nil || !strings.Contains(err.Error(), Suites[1]+": every one") {
		t.Errorf("err = %v", err)
	}
	if _, err := Record(testPin, Run{}); err == nil {
		t.Error("an empty run was recorded")
	}
}

func TestDescribeBoundsItsLists(t *testing.T) {
	var m Movement
	for range maxNamed + 3 {
		m.Worse = append(m.Worse, Move{Suite: Suites[0], ID: "A01-001", Was: Pass, Now: Fail, Detail: strings.Repeat("x", maxDetail+50)})
	}
	text := m.Describe()
	if !strings.Contains(text, "… and 3 more") || strings.Count(text, "A01-001") != maxNamed {
		t.Errorf("unbounded description:\n%s", text)
	}
	if strings.Contains(text, strings.Repeat("x", maxDetail+1)) {
		t.Error("a detail was not bounded")
	}
}
