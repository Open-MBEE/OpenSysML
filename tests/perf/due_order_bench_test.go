package perf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// ringModel is a ring of nodes passing a signal along under a timer each node
// also accepts: every clock instant has several executors due, so the due-order
// draw classifies each pair's places against the objects holding them.
const ringModel = `package Ring {
    private import ScalarValues::*;
    private import SI::*;
    item def Ping { attribute n : Integer; }
    port def P { out item o : Ping; in item i : Ping; }
    part def Node {
        port left : P; port right : ~P;
        attribute a0 : Integer = 0; attribute a1 : Integer = 0; attribute a2 : Integer = 0;
        attribute count : Integer = 0;
        exhibit state run { entry; then idle; state idle;
            transition first idle accept p : Ping via left
                do action { assign count := count + 1; assign a0 := p.n; send new Ping(n = p.n + 1) via right; } then idle;
            transition first idle accept after 1 [SI::s] do action { assign a1 := a1 + 1; } then idle;
        }
    }
    part def Sink { port left : P; attribute got : Integer = 0;
        exhibit state run { entry; then idle; state idle;
            transition first idle accept p : Ping via left do action { assign got := got + 1; } then idle;
        }
    }
    part def Starter { port out_ : ~P;
        exhibit state run { entry; then s0; state s0;
            transition first s0 accept after 1 [SI::s] do action { send new Ping(n = 0) via out_; } then s0;
        }
    }
    part ring { part s : Starter; part n1 : Node; part n2 : Node; part n3 : Node; part sink : Sink;
        connect s.out_ to n1.left; connect n1.right to n2.left; connect n2.right to n3.left; connect n3.right to sink.left;
    }
}
`

// BenchmarkExploreDueOrder explores the ring's sink for a few instants under an
// explore schedule, the path that draws the due order of every executor pair
// at every instant (Context.drawDueOrder through relation, slotHolder and
// ownsFeature). The model is loaded outside the timer.
func BenchmarkExploreDueOrder(b *testing.B) {
	path := filepath.Join(b.TempDir(), "ring.sysml")
	if err := os.WriteFile(path, []byte(ringModel), 0o600); err != nil {
		b.Fatal(err)
	}
	policy, err := runtime.ParseSchedulePolicy("explore:runs=2,depth=200")
	if err != nil {
		b.Fatal(err)
	}
	states := []repl.Behavior{{Name: "Ring::Sink::run", Performer: []string{"Ring::ring.sink"}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		s := loadedSession(b, path)
		if err := s.SetSchedule(policy); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		verdicts := s.RunFor(nil, states, 6)
		if len(verdicts) != len(states) {
			b.Fatalf("%d verdicts for %d behaviors", len(verdicts), len(states))
		}
		// The runs budget stops the exploration short by design, so the verdict is
		// unresolved; one that explored counts its runs, and none of them failed.
		for _, v := range verdicts {
			if x := v.Exploration; x == nil || x.Runs == 0 || x.FailedLinearizations != 0 {
				b.Fatalf("%s: %v", v.Subject, v.Lines)
			}
		}
	}
}
