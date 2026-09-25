package main

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestConnectCodeMapsEveryCodeName(t *testing.T) {
	tests := map[string]codes.Code{
		"canceled": codes.Canceled, "unknown": codes.Unknown,
		"invalid_argument": codes.InvalidArgument, "deadline_exceeded": codes.DeadlineExceeded,
		"not_found": codes.NotFound, "already_exists": codes.AlreadyExists,
		"permission_denied": codes.PermissionDenied, "resource_exhausted": codes.ResourceExhausted,
		"failed_precondition": codes.FailedPrecondition, "aborted": codes.Aborted,
		"out_of_range": codes.OutOfRange, "unimplemented": codes.Unimplemented,
		"internal": codes.Internal, "unavailable": codes.Unavailable,
		"data_loss": codes.DataLoss, "unauthenticated": codes.Unauthenticated,
	}
	for name, want := range tests {
		if got := connectCode(name); got != want {
			t.Errorf("connectCode(%q) = %s, want %s", name, got, want)
		}
	}
	if got := connectCode("not-a-code"); got != codes.Unknown {
		t.Errorf("unknown code = %s, want Unknown", got)
	}
}

func TestParseProtocols(t *testing.T) {
	got, err := parseProtocols(" grpc, connect-json,connect ")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"grpc", "connect-json", "connect"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("protocols = %v, want %v", got, want)
		}
	}
	for _, input := range []string{"", "grpc,grpc", "stdio", "connect,wat"} {
		if _, err := parseProtocols(input); err == nil {
			t.Errorf("parseProtocols(%q) accepted invalid input", input)
		}
	}
}

func TestGRPCTransportRejectsConnectProtocol(t *testing.T) {
	if err := validateProtocols([]string{"connect"}, transportGRPC); err == nil {
		t.Fatal("grpc transport accepted a Connect protocol")
	}
}
