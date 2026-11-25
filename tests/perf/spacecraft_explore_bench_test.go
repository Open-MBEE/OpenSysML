package perf

import (
	"fmt"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// spacecraftExploreModel is the CLI case's t=79 race model. The benchmark
// drives repl in process, so -benchmem measures the exploration itself.
const spacecraftExploreModel = "../../examples/runtime-showcase/spacecraft-comms.sysml"

// BenchmarkExploreSpacecraftRace varies runs and depth, including the CLI's
// 500/1024 case. It measures where time and allocations go, not coverage.
func BenchmarkExploreSpacecraftRace(b *testing.B) {
	if _, err := os.Stat(spacecraftExploreModel); err != nil {
		b.Fatalf("read %s: %v", spacecraftExploreModel, err)
	}
	cases := []struct{ runs, depth int }{
		{runs: 8, depth: 64},
		{runs: 64, depth: 128},
		{runs: 300, depth: 1024},
		{runs: 500, depth: 1024},
	}
	states := []repl.Behavior{{
		Name:      "SpacecraftComms::SpacecraftVehicle::modes",
		Performer: []string{"SpacecraftComms::mission.spacecraftVehicle"},
	}}
	for _, tc := range cases {
		b.Run(fmt.Sprintf("runs=%d/depth=%d", tc.runs, tc.depth), func(b *testing.B) {
			policy, err := runtime.ParseSchedulePolicy(fmt.Sprintf("explore:runs=%d,depth=%d", tc.runs, tc.depth))
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := loadedSession(b, spacecraftExploreModel)
				if _, err := s.InstantiateNamed("SpacecraftComms::mission"); err != nil {
					b.Fatalf("instantiate mission: %v", err)
				}
				if err := s.SetSchedule(policy); err != nil {
					b.Fatalf("SetSchedule: %v", err)
				}
				b.StartTimer()
				verdicts := s.RunFor(nil, states, 80)
				if len(verdicts) != len(states) {
					b.Fatalf("%d verdicts for %d behaviors", len(verdicts), len(states))
				}
				for _, verdict := range verdicts {
					explored := verdict.Exploration
					if explored == nil || explored.Runs != tc.runs || explored.FailedLinearizations != 0 {
						b.Fatalf("%s: %+v, want %d runs and no failed linearization", verdict.Subject, explored, tc.runs)
					}
				}
			}
		})
	}
}
