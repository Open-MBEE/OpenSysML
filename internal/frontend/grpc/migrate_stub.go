//go:build sysml_re || sysml_nov1

package grpc

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// unlinkedCapabilities are the capabilities this build never reports: it is
// made without the SysML v1 migration.
var unlinkedCapabilities = []string{CapabilityMigrate}

// Migrate refuses the call in a build made without the SysML v1 migration, as
// any call for a capability the service does not report is refused.
func (s *Service) Migrate(context.Context, *pb.MigrateRequest) (*pb.MigrateResponse, error) {
	if err := s.requireCapability(CapabilityMigrate); err != nil {
		return nil, err
	}
	return nil, statusError(connect.CodeUnimplemented, convert.ErrMigrationNotLinked.Error())
}
