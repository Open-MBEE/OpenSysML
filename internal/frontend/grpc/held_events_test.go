package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A cached population keeps the most recent HeldEventsEnvVar records; an Events
// query reaching past them is FAILED_PRECONDITION, not a shortened relation.
func TestHeldEventsAreBounded(t *testing.T) {
	t.Setenv(HeldEventsEnvVar, "1")
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, "../../../tests/grpc/testdata/conformance/document_query_events.sysml")
	holdObject(t, srv, hash, "Lamps::lamp")
	holdObject(t, srv, hash, "Lamps::lamp")

	cached, _ := srv.cache.Get(hash)
	if dropped, _ := srv.objects(cached).rt.Trace().Dropped(); dropped != 1 {
		t.Fatalf("dropped %d records under a bound of 1, want the first lamp's entry", dropped)
	}
	_, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Lamps::Steps",
		Bindings: []*pb.DocumentQueryBinding{binding("root", objectByPath("Lamps::lamp"))},
	})
	if err == nil {
		t.Fatal("Steps over a truncated trace answered rows, want FAILED_PRECONDITION")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("code = %v, want %v: %v", connect.CodeOf(err), connect.CodeFailedPrecondition, err)
	}
	if !strings.Contains(err.Error(), "no longer keeps") {
		t.Errorf("error %q does not say the trace was truncated", err.Error())
	}
}

// TestMaxHeldEventsFromEnv: the bound is the positive integer the variable
// holds, the default when unset; anything else is refused at construction.
func TestMaxHeldEventsFromEnv(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{raw: "", want: DefaultMaxHeldEvents},
		{raw: " 12 ", want: 12},
		{raw: "0", wantErr: true},
		{raw: "-3", wantErr: true},
		{raw: "lots", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			t.Setenv(HeldEventsEnvVar, tc.raw)
			got, err := maxHeldEventsFromEnv()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("%q was accepted as %d", tc.raw, got)
				}
				if !strings.Contains(err.Error(), HeldEventsEnvVar) {
					t.Errorf("error does not name %s: %v", HeldEventsEnvVar, err)
				}
				if _, serr := NewService(4, "test"); serr == nil {
					t.Error("NewService accepted an unusable held events bound")
				}
				return
			}
			if err != nil {
				t.Fatalf("maxHeldEventsFromEnv(%q): %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("bound %d, want %d", got, tc.want)
			}
		})
	}
}
