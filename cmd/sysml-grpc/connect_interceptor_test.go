package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestConnectAccessWarnsForLargeJSONOnly(t *testing.T) {
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	large := &pb.ParseFileResponse{ModelHash: strings.Repeat("x", 256*1024)}
	interceptor := connectAccess{}.WrapUnary
	next := func(_ context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(large), nil
	}
	request := connect.NewRequest(&pb.ParseFileRequest{})
	request.Header().Set("Content-Type", "application/json")
	_, err := interceptor(next)(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "large JSON response") {
		t.Fatalf("warning missing from logs: %s", logs.String())
	}

	logs.Reset()
	request.Header().Set("Content-Type", "application/proto")
	_, err = interceptor(next)(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), "large JSON response") {
		t.Fatalf("protobuf response emitted JSON warning: %s", logs.String())
	}
}
