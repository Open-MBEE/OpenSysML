package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// drawPolicyModel calls every RandomFunctions function but normal once, and waits
// a drawn duration, so each policy resolves four draws.
const drawPolicyModel = `
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import RandomFunctions::*;
	action draw {
		attribute u : Real = uniform(2.0, 6.0);
		attribute n : Integer = uniformInteger(1, 6);
		attribute tri : Real = triangular(1.0, 4.0, 10.0);
		first start;
		then action wait accept after uniform(1, 80) [s];
		then done;
	}
}`

// realOut is the Real an output holds.
func realOut(t *testing.T, out map[string]Value, name string) float64 {
	t.Helper()
	v, ok := out[name]
	if !ok || v.Const.Kind != semantics.ValReal {
		t.Fatalf("%s = %v, want a Real", name, v)
	}
	return v.Const.Real
}

// formatDraws spells draws one per line, as a witness lists them.
func formatDraws(draws []DrawTaken) string {
	var b strings.Builder
	for _, d := range draws {
		b.WriteString(d.String() + "\n")
	}
	return b.String()
}

// runUnderDraws runs the action named on a fresh context under the draw policy,
// unseeded unless a seed is given.
func runUnderDraws(t *testing.T, m *exploreModel, name string, policy DrawPolicy, seed ...uint64) (*Context, map[string]Value, error) {
	t.Helper()
	ctx, _ := m.fresh()
	ctx.SetDrawPolicy(policy)
	if len(seed) > 0 {
		ctx.SetModelSeed(seed[0])
	}
	out, err := ctx.ExecuteAction(m.action(t, name))
	return ctx, out, err
}

// A draw policy spells and parses as -draws takes it, random being the default
// and the only policy that is not fixed.
func TestDrawPolicySpellsAndParses(t *testing.T) {
	for _, policy := range []DrawPolicy{DrawRandom, DrawMin, DrawMax, DrawAverage} {
		parsed, err := ParseDrawPolicy(policy.String())
		if err != nil || parsed != policy {
			t.Errorf("ParseDrawPolicy(%q) = %v, %v; want %v", policy.String(), parsed, err, policy)
		}
		if policy.Fixed() != (policy != DrawRandom) {
			t.Errorf("%s.Fixed() = %v", policy, policy.Fixed())
		}
	}
	if _, err := ParseDrawPolicy("median"); !errors.Is(err, ErrDrawPolicy) || !strings.Contains(err.Error(), "random, min, max, average") {
		t.Errorf("ParseDrawPolicy(median) = %v, want ErrDrawPolicy naming the policies", err)
	}
	if _, err := ParseDrawPolicy(""); !errors.Is(err, ErrDrawPolicy) {
		t.Errorf("ParseDrawPolicy(\"\") = %v, want ErrDrawPolicy", err)
	}
	var ctx Context
	if ctx.DrawPolicy() != DrawRandom {
		t.Errorf("a fresh context draws by %s, want random", ctx.DrawPolicy())
	}
}

// A fixed policy resolves every call to the point of its distribution the policy
// names, needs no seed, and records the draws it made as a witness lists them.
func TestFixedDrawPoliciesResolveEveryCallWithoutASeed(t *testing.T) {
	m := parseLibraryModel(t, drawPolicyModel)
	cases := []struct {
		policy DrawPolicy
		u, tri float64
		n      int64
		wait   string
	}{
		{DrawMin, 2.0, 1.0, 1, "draw uniform(1, 80) = 1.0"},
		{DrawMax, 6.0, 10.0, 6, "draw uniform(1, 80) = 80.0"},
		{DrawAverage, 4.0, 5.0, 4, "draw uniform(1, 80) = 40.5"},
	}
	for _, tc := range cases {
		t.Run(tc.policy.String(), func(t *testing.T) {
			ctx, out, err := runUnderDraws(t, m, "draw", tc.policy)
			if err != nil {
				t.Fatal(err)
			}
			if got := realOut(t, out, "u"); got != tc.u {
				t.Errorf("u = %v, want %v", got, tc.u)
			}
			if got := realOut(t, out, "tri"); got != tc.tri {
				t.Errorf("tri = %v, want %v", got, tc.tri)
			}
			if got := takenInt(t, out, "n"); got != tc.n {
				t.Errorf("n = %d, want %d", got, tc.n)
			}
			draws := ctx.DrawsTaken()
			if len(draws) != 4 || draws[3].String() != tc.wait {
				t.Errorf("recorded %v, want four draws ending in %q", draws, tc.wait)
			}
			if ctx.DrawPolicyTaken() != tc.policy {
				t.Errorf("the run drew by %s, want %s", ctx.DrawPolicyTaken(), tc.policy)
			}
			again, _, err := runUnderDraws(t, m, "draw", tc.policy, 5)
			if err != nil {
				t.Fatal(err)
			}
			if a, b := formatDraws(ctx.DrawsTaken()), formatDraws(again.DrawsTaken()); a != b {
				t.Errorf("a seed changed the fixed draws:\n%s\n%s", a, b)
			}
		})
	}
}

// The average of a uniformInteger is its midpoint, a half rounded toward hi, and
// stays exact at the ends of the Integer range; the average of a normal is its mean.
func TestAverageDrawsOfDiscreteAndUnboundedCalls(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;
			private import RandomFunctions::*;
			action draw {
				attribute even : Integer = uniformInteger(1, 5);
				attribute odd : Integer = uniformInteger(1, 6);
				attribute negative : Integer = uniformInteger(-6, -1);
				attribute wide : Integer = uniformInteger(-9223372036854775807, 9223372036854775807);
				attribute g : Real = normal(12.0, 3.0);
				first start; then done;
			}
		}`)
	_, out, err := runUnderDraws(t, m, "draw", DrawAverage)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int64{"even": 3, "odd": 4, "negative": -3, "wide": 0} {
		if got := takenInt(t, out, name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	if got := realOut(t, out, "g"); got != 12.0 {
		t.Errorf("g = %v, want the mean 12", got)
	}
}

// Under random the policy changes nothing: the seeded draws are today's, and the
// witness carries no policy line.
func TestRandomDrawPolicyIsTheSeededStream(t *testing.T) {
	m := parseLibraryModel(t, drawPolicyModel)
	ctx, out, err := runAction(t, m, "draw", "declared", 7)
	if err != nil {
		t.Fatal(err)
	}
	random, again, err := runUnderDraws(t, m, "draw", DrawRandom, 7)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"u", "n", "tri"} {
		if out[name].Const != again[name].Const {
			t.Errorf("%s = %v under random, want %v", name, again[name], out[name])
		}
	}
	if u := realOut(t, out, "u"); u < 2 || u >= 6 {
		t.Errorf("u = %v, want a draw in [2, 6)", u)
	}
	w := Witness{DrawPolicy: random.DrawPolicyTaken(), Draws: ctx.DrawsTaken(), Choices: ctx.ChoicesTaken()}
	if strings.Contains(w.String(), drawPolicyPrefix) {
		t.Errorf("a random run's witness names a policy:\n%s", w)
	}
}

// A witness of a fixed-policy run names the policy, reads back, and replays to the
// same values with no seed on the replaying context; a witness that names a policy
// after its draws, or names one twice, is refused with a typed error.
func TestWitnessCarriesTheDrawPolicy(t *testing.T) {
	m := parseLibraryModel(t, drawPolicyModel)
	ctx, out, err := runUnderDraws(t, m, "draw", DrawMax)
	if err != nil {
		t.Fatal(err)
	}
	w := Witness{DrawPolicy: ctx.DrawPolicyTaken(), Draws: ctx.DrawsTaken(), Choices: ctx.ChoicesTaken()}
	text := w.String()
	if !strings.HasPrefix(text, "draws by max\ndraw uniform(2.0, 6.0) = 6.0\n") {
		t.Fatalf("the witness does not open with the policy and the first draw:\n%s", text)
	}
	parsed, err := ParseWitness(text)
	if err != nil {
		t.Fatalf("the witness does not read back: %v\n%s", err, text)
	}
	if parsed.DrawPolicy != DrawMax || formatDraws(parsed.Draws) != formatDraws(w.Draws) {
		t.Fatalf("the witness reads back as %s over %s", parsed.DrawPolicy, formatDraws(parsed.Draws))
	}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(parsed))
	got, err := replay.ExecuteAction(m.action(t, "draw"))
	if err == nil {
		err = replay.Unfollowed()
	}
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	for _, name := range []string{"u", "n", "tri"} {
		if got[name].Const != out[name].Const {
			t.Errorf("replay gave %s = %v, want %v", name, got[name], out[name])
		}
	}
	if replay.DrawPolicyTaken() != DrawMax {
		t.Errorf("the replay drew by %s, want the witness's max", replay.DrawPolicyTaken())
	}

	misplaced := strings.Replace(text, "draws by max\n", "", 1)
	misplaced = strings.Replace(misplaced, "\nno choice points", "\ndraws by max\nno choice points", 1)
	var parse *DrawParseError
	if _, err := ParseWitness(misplaced); !errors.As(err, &parse) || !strings.Contains(err.Error(), "comes before the draws") {
		t.Errorf("a policy after the draws parsed: %v", err)
	}
	if _, err := ParseWitness("draws by median\nno choice points\n"); !errors.As(err, &parse) || !errors.Is(err, ErrDrawPolicy) && !strings.Contains(err.Error(), "median") {
		t.Errorf("an unknown policy parsed: %v", err)
	}
	for _, twice := range []string{
		"draws by min\ndraws by max\ndraw uniform(0.0, 1.0) = 1.0\nno choice points\n",
		"draws by max\ndraws by max\ndraw uniform(0.0, 1.0) = 1.0\nno choice points\n",
		"draws by random\ndraws by random\nno choice points\n",
	} {
		_, err := ParseWitness(twice)
		if !errors.As(err, &parse) || !strings.Contains(err.Error(), "named twice") || parse.Line != 2 {
			t.Errorf("a witness naming its policy twice parsed: %v\n%s", err, twice)
		}
	}
}

// A replay admits a draw only where the witness's policy could have made it: at hi
// under max, though uniform never draws hi at random; not at the midpoint under
// max, though uniform draws it at random.
func TestReplayAdmitsDrawsUnderTheWitnessPolicy(t *testing.T) {
	m := parseLibraryModel(t, drawingModel)
	realValue := func(x float64) semantics.Value { return semantics.Value{Kind: semantics.ValReal, Real: x} }
	integer := func(n int64) semantics.Value { return semantics.Value{Kind: semantics.ValInt, Int: n} }
	cases := []struct {
		name   string
		policy DrawPolicy
		d, n   semantics.Value
		ok     bool
		reason string
	}{
		{"max at the fixed points", DrawMax, realValue(1), integer(6), true, ""},
		{"min at the fixed points", DrawMin, realValue(0), integer(1), true, ""},
		{"average at the fixed points", DrawAverage, realValue(0.5), integer(4), true, ""},
		{"random at hi", DrawRandom, realValue(1), integer(6), false, "records 1.0, which the call cannot draw"},
		{"max off hi", DrawMax, realValue(0.5), integer(6), false, "records 0.5, which the call cannot draw under max"},
		{"max off the greatest integer", DrawMax, realValue(1), integer(5), false, "records 5, which the call cannot draw under max"},
		{"average off the midpoint", DrawAverage, realValue(0.5), integer(3), false, "records 3, which the call cannot draw under average"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := Witness{DrawPolicy: tc.policy, Draws: []DrawTaken{{What: "uniform(0.0, 1.0)", Value: tc.d}, {What: "uniformInteger(1, 6)", Value: tc.n}}}
			replay, _ := m.fresh()
			mustSchedule(t, replay, ReplayOf(w))
			_, err := replay.ExecuteAction(m.action(t, "draw"))
			if err == nil {
				err = replay.Unfollowed()
			}
			if tc.ok {
				if err != nil {
					t.Fatalf("replay refused a draw the policy makes: %v", err)
				}
				return
			}
			var refused *WitnessDrawError
			if !errors.As(err, &refused) {
				t.Fatalf("error = %v, want a WitnessDrawError", err)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("error %q does not say %q", err, tc.reason)
			}
		})
	}
}

// A weighted decision draws from the seed under every policy, and unseeded takes
// the most probable branch: the policy resolves durations, not decisions.
func TestWeightedDecisionsDrawTheSameUnderEveryPolicy(t *testing.T) {
	m := parseLibraryModel(t, weightedRouteModel)
	_, seeded, err := runAction(t, m, "route", "declared", 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []DrawPolicy{DrawMin, DrawMax, DrawAverage} {
		_, out, err := runUnderDraws(t, m, "route", policy, 3)
		if err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		if got, want := takenInt(t, out, "taken"), takenInt(t, seeded, "taken"); got != want {
			t.Errorf("%s under seed 3 took %d, want the random policy's %d", policy, got, want)
		}
		ctx, out, err := runUnderDraws(t, m, "route", policy)
		if err != nil {
			t.Fatalf("%s unseeded: %v", policy, err)
		}
		if got := takenInt(t, out, "taken"); got != 1 {
			t.Errorf("%s unseeded took %d, want the 0.7 branch", policy, got)
		}
		if choices := ctx.Choices(); len(choices) != 1 || choices[0].Drawn {
			t.Errorf("%s unseeded: choices %v, want one undrawn decision", policy, choices)
		}
	}
}

// A witness naming a fixed policy and recording no draw leaves them to the policy: the
// replay resolves each call to its fixed point, as the run did, and records what it took;
// a partial record, or none under random, is still a witness the replay refuses.
func TestFixedPolicyWitnessWithoutDrawsReplaysByThePolicy(t *testing.T) {
	m := parseLibraryModel(t, drawPolicyModel)
	for _, tc := range []struct {
		policy DrawPolicy
		u, tri float64
		n      int64
	}{{DrawMin, 2, 1, 1}, {DrawMax, 6, 10, 6}, {DrawAverage, 4, 5, 4}} {
		w, err := ParseWitness(drawPolicyPrefix + tc.policy.String() + "\nno choice points\n")
		if err != nil {
			t.Fatal(err)
		}
		replay, _ := m.fresh()
		mustSchedule(t, replay, ReplayOf(w))
		out, err := replay.ExecuteAction(m.action(t, "draw"))
		if err == nil {
			err = replay.Unfollowed()
		}
		if err != nil {
			t.Fatalf("%s: replay refused a witness leaving its draws to the policy: %v", tc.policy, err)
		}
		if u, tri, n := realOut(t, out, "u"), realOut(t, out, "tri"), takenInt(t, out, "n"); u != tc.u || tri != tc.tri || n != tc.n {
			t.Errorf("%s replayed u = %v, tri = %v, n = %d, want %v, %v, %d", tc.policy, u, tri, n, tc.u, tc.tri, tc.n)
		}
		if replay.DrawPolicyTaken() != tc.policy || len(replay.DrawsTaken()) != 4 {
			t.Errorf("%s replay drew by %s over %s, want the policy and its four draws", tc.policy, replay.DrawPolicyTaken(), formatDraws(replay.DrawsTaken()))
		}
	}
	partial, err := ParseWitness("draws by max\ndraw uniform(2.0, 6.0) = 6.0\nno choice points\n")
	if err != nil {
		t.Fatal(err)
	}
	replay, _ := m.fresh()
	mustSchedule(t, replay, ReplayOf(partial))
	_, err = replay.ExecuteAction(m.action(t, "draw"))
	var refused *WitnessDrawError
	if !errors.As(err, &refused) || !strings.Contains(err.Error(), "records no draw left for it") {
		t.Fatalf("a witness recording one draw of four replayed: %v", err)
	}
}

// The clock of a run whose durations are all drawn is deterministic under a
// fixed policy: the same duration every run, at the point the policy names.
func TestFixedDrawPolicyMakesTheClockDeterministic(t *testing.T) {
	m := parseLibraryModel(t, drawPolicyModel)
	for _, tc := range []struct {
		policy DrawPolicy
		clock  float64
	}{{DrawMin, 1}, {DrawMax, 80}, {DrawAverage, 40.5}} {
		var clocks []float64
		for i := 0; i < 3; i++ {
			ctx, _, err := runUnderDraws(t, m, "draw", tc.policy)
			if err != nil {
				t.Fatal(err)
			}
			clocks = append(clocks, ctx.Clock().Now())
		}
		for _, c := range clocks {
			if math.Abs(c-tc.clock) > 1e-9 {
				t.Errorf("%s: the clock ended at %v, want %v every run (%v)", tc.policy, c, tc.clock, clocks)
			}
		}
	}
}
