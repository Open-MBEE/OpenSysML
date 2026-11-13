package grpc

import (
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// coverageProto converts what every run entered and took into its wire form.
func coverageProto(c runtime.Coverage) *pb.Coverage {
	return &pb.Coverage{States: coverageEntriesProto(c.States), Transitions: coverageEntriesProto(c.Transitions)}
}

func coverageEntriesProto(entries []runtime.CoverageEntry) []*pb.CoverageEntry {
	out := make([]*pb.CoverageEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, &pb.CoverageEntry{Path: e.Path, Declaration: e.Declaration, Runs: int32Clamp(e.Runs)})
	}
	return out
}
