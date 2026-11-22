//go:build sysml_re || sysml_nov1

package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A build made without the SysML v1 migration neither reports the migrate
// capability nor serves the call: it is refused as unimplemented, by name.
func TestMigrateIsRefusedWithoutTheMigration(t *testing.T) {
	if slices.Contains(Capabilities(), CapabilityMigrate) {
		t.Errorf("Capabilities() reports %q in a build without the migration", CapabilityMigrate)
	}
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(info.Capabilities, CapabilityMigrate) {
		t.Errorf("GetServerInfo reports %q in a build without the migration", CapabilityMigrate)
	}
	_, err = srv.Migrate(context.Background(), &pb.MigrateRequest{
		Source: &pb.MigrateRequest_Content{Content: []byte("<xmi/>")}, FromFormat: "xmi", ToFormat: "sysml",
	})
	if code := connect.CodeOf(err); err == nil || code != connect.CodeUnimplemented {
		t.Fatalf("Migrate = %v (code %v), want Unimplemented", err, code)
	}
	if msg := connectMessage(err); !strings.Contains(msg, `"`+CapabilityMigrate+`"`) {
		t.Errorf("message = %q, want it to name the capability %q", msg, CapabilityMigrate)
	}
}
