//go:build !sysml_re && !sysml_nofmi

package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// ListEngines names every registered engine in name order, with its authority,
// the questions it answers and whether it can run here.
func TestListEnginesNamesEveryEngine(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	resp, err := srv.ListEngines(context.Background(), &pb.ListEnginesRequest{})
	if err != nil {
		t.Fatalf("ListEngines: %v", err)
	}
	want := []struct {
		name, authority string
		answers         []string
	}{
		{"check", "bounded", []string{"outcomes", "holds", "sensitive"}},
		{"explore", "proved", []string{"outcomes"}},
		{"run", "observed", []string{"evaluate"}},
		{"smt", "proved", []string{"holds", "sensitive"}},
		{"solve", "proved", []string{"satisfiable", "holds"}},
		{"sweep", "observed", []string{"sweep"}},
		{"tool:fmi", "observed", []string{"compute"}},
	}
	if len(resp.Engines) != len(want) {
		t.Fatalf("engines = %v, want %d", resp.Engines, len(want))
	}
	for i, w := range want {
		got := resp.Engines[i]
		if got.Name != w.name || got.Authority != w.authority || strings.Join(got.Answers, ",") != strings.Join(w.answers, ",") {
			t.Errorf("engine %d = %v, want %s %s %v", i, got, w.name, w.authority, w.answers)
		}
		if got.Name != "solve" && got.Name != "smt" && got.Name != "tool:fmi" && (!got.Ready || got.Process != "" || got.Unavailable != "") {
			t.Errorf("in-process engine %s = %v, want ready with no process", got.Name, got)
		}
	}
	for _, external := range []*pb.EngineInfo{resp.Engines[3], resp.Engines[4], resp.Engines[6]} {
		if external.Process == "" || external.Ready == (external.Unavailable != "") {
			t.Errorf("%s = %v, want a process and ready or a reason", external.Name, external)
		}
		if external.Ready && external.ProcessFound == "" {
			t.Errorf("%s is ready but names no found process: %v", external.Name, external)
		}
	}
}
