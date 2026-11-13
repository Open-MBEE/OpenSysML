package grpc

import "github.com/Open-MBEE/OpenSysML/internal/semantic/naming"

func isPositionalIdentity(name string) bool {
	return naming.IsPositionalIdentity(name)
}
