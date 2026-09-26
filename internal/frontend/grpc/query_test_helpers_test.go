package grpc

import "github.com/Open-MBEE/OpenSysML/internal/translate/export"

func isPositionalIdentity(name string) bool {
	return export.IsPositionalIdentity(name)
}
