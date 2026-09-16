package analysis

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// wired is the JSON the host writes for q, as an engine reads it.
func wired(t *testing.T, model *Model, q Question) string {
	t.Helper()
	out, err := externalEngine{}.wireQuestion(model, q, Budget{})
	if err != nil {
		t.Fatalf("wire: %v", err)
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A sweep's seed goes on the wire whenever rows are drawn from it, a seed of 0 included,
// and stays off it for a swept table, which draws nothing; an engine reading the line can
// tell an unseeded sweep from one seeded with 0.
func TestWireSweepKeepsAZeroSeed(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	sweep := func(plan runtime.SweepPlan) Question {
		return Question{Kind: Sweep, Subject: "test::Double", Schedule: ctx.Schedule(), Sweep: &SweepAsk{Plan: plan, Row: doubleRow(t, f)}}
	}
	stepped := doublePlan(t, f, ctx)
	sampled := stepped
	sampled.Sampled, sampled.Samples, sampled.Seed = true, 4, 0
	cases := []struct {
		name string
		plan runtime.SweepPlan
		want string
	}{
		{"stepped", stepped, `"from":1,"to":3}]}}`},
		{"sampled seed 0", sampled, `"sampled":true,"samples":4,"seed":0}`},
		{"runs seed 0", runtime.MonteCarloPlan(3, 0), `"sweep":{"ranges":[],"seed":0,"runs":3}`},
		{"runs seed 7", runtime.MonteCarloPlan(3, 7), `"sweep":{"ranges":[],"seed":7,"runs":3}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line := wired(t, f.building(), sweep(c.plan))
			if !strings.Contains(line, c.want) {
				t.Fatalf("the host wrote %s, want it to carry %s", line, c.want)
			}
			var back struct {
				Sweep struct {
					Seed *uint64 `json:"seed"`
				} `json:"sweep"`
			}
			if err := json.Unmarshal([]byte(line), &back); err != nil {
				t.Fatal(err)
			}
			if got, want := back.Sweep.Seed != nil, c.plan.Drawn(); got != want {
				t.Fatalf("seed present = %v, want %v for %+v", got, want, c.plan)
			}
		})
	}
}

// A question carries the model seed its runs draw from apart from the schedule — 0 as
// much as any other — and none when no seed is set, so the two knobs the surface has
// reach the engine as two.
func TestWireQuestionCarriesTheModelSeed(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	perform := func(*runtime.Context) (Answer, error) { return Answer{Claim: ClaimHolds}, nil }
	cases := []struct {
		name string
		seed ModelSeed
		want string
	}{
		{"unset", ModelSeed{}, `"schedule":"seed:5","free":[]`},
		{"zero", ModelSeed{Set: true}, `"schedule":"seed:5","modelSeed":0,"free":[]`},
		{"eleven", ModelSeed{Seed: 11, Set: true}, `"schedule":"seed:5","modelSeed":11,"free":[]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := Question{Kind: Evaluate, Subject: "test::Tank::low", Schedule: policy(t, "seed:5"), ModelSeed: c.seed, Perform: perform}
			if line := wired(t, Held(ctx), q); !strings.Contains(line, c.want) {
				t.Fatalf("the host wrote %s, want it to carry %s", line, c.want)
			}
		})
	}
	if strings.Contains(wired(t, Held(ctx), Question{Kind: Evaluate, Subject: "test::Tank::low", Schedule: ctx.Schedule(), Perform: perform}), "modelSeed") {
		t.Fatal("a question with no model seed set named one")
	}
}

// The model seed a held context was given is the one its question carries, and the
// registry hands a request's seed to the question it asks.
func TestModelSeedOfAContextAndARequest(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	if got := ModelSeedOf(ctx); got.Set {
		t.Fatalf("a fresh context has model seed %+v, want none", got)
	}
	if got := ModelSeedOf(nil); got.Set {
		t.Fatalf("no context has model seed %+v, want none", got)
	}
	ctx.SetModelSeed(0)
	if got, want := ModelSeedOf(ctx), (ModelSeed{Set: true}); got != want {
		t.Fatalf("model seed %+v, want %+v", got, want)
	}
	ctx.SetModelSeed(11)
	if got, want := ModelSeedOf(ctx), (ModelSeed{Seed: 11, Set: true}); got != want {
		t.Fatalf("model seed %+v, want %+v", got, want)
	}
	ctx.ClearModelSeed()
	if got := ModelSeedOf(ctx); got.Set {
		t.Fatalf("a cleared context has model seed %+v, want none", got)
	}
}
