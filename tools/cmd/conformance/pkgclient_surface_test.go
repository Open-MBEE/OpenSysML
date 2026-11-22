package main

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestPkgClientDispatchesEveryRPC holds the public-API protocol to the proto:
// an RPC it cannot dispatch is skipped by the runner, so one must fail here.
func TestPkgClientDispatchesEveryRPC(t *testing.T) {
	client := newPkgClient("pkg", nil)
	services := pb.File_sysml_proto.Services()
	rpcs := 0
	for i := 0; i < services.Len(); i++ {
		methods := services.Get(i).Methods()
		for j := 0; j < methods.Len(); j++ {
			rpcs++
			name := string(methods.Get(j).Name())
			if _, ok := client.lookup(name); !ok {
				t.Errorf("the public Go API protocol does not dispatch %s", name)
			}
		}
	}
	if rpcs == 0 {
		t.Fatal("the proto declares no RPC")
	}
}
