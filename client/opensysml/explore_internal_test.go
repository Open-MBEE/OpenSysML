package opensysml

import (
	"reflect"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// The coverage of every run is read from the wire beside the status, with an
// absent coverage covering nothing.
func TestExplorationReadsTheCoverageOfEveryRun(t *testing.T) {
	status := &pb.ExplorationStatus{Complete: true, Runs: 2, Coverage: &pb.Coverage{
		States: []*pb.CoverageEntry{
			{Path: "Cov::sys#1.m.run: viaA", Declaration: "Cov::M::run::viaA", Runs: 1},
			{Path: "Cov::sys#1.m.run: viaB", Declaration: "Cov::M::run::viaB", Runs: 1},
		},
		Transitions: []*pb.CoverageEntry{{Path: "Cov::sys#1.m.run: s0 -> viaB", Declaration: "Cov::M::run::s0 -> viaB", Runs: 1}},
	}}
	got := explorationFromProto(nil, status).Coverage
	want := Coverage{
		States: []CoverageEntry{
			{Path: "Cov::sys#1.m.run: viaA", Declaration: "Cov::M::run::viaA", Runs: 1},
			{Path: "Cov::sys#1.m.run: viaB", Declaration: "Cov::M::run::viaB", Runs: 1},
		},
		Transitions: []CoverageEntry{{Path: "Cov::sys#1.m.run: s0 -> viaB", Declaration: "Cov::M::run::s0 -> viaB", Runs: 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("coverage %+v, want %+v", got, want)
	}
	if bare := explorationFromProto(nil, &pb.ExplorationStatus{Runs: 1}).Coverage; len(bare.States) != 0 || len(bare.Transitions) != 0 {
		t.Errorf("an absent coverage read as %+v", bare)
	}
}
