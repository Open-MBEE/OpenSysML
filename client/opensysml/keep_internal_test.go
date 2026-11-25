package opensysml

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// countingParseCaller counts parse requests and ServerInfo preflights, so an
// extra preflight before each request cannot hide a capability check.
type countingParseCaller struct {
	caller
	requests []*pb.ParseSourcesRequest
	infos    int
}

func (c *countingParseCaller) serverInfo(context.Context) (*pb.ServerInfoResponse, error) {
	c.infos++
	return &pb.ServerInfoResponse{Capabilities: []string{
		CapabilityParseSources, CapabilityParseSourcesKeep, CapabilityParseSourcesAffected,
		CapabilityStrictConformance, CapabilityInlineLanguage,
	}}, nil
}

func (c *countingParseCaller) parseSources(_ context.Context, req *pb.ParseSourcesRequest) (*pb.ParseSourcesResponse, error) {
	c.requests = append(c.requests, req)
	return &pb.ParseSourcesResponse{ModelHash: "h"}, nil
}

func (c *countingParseCaller) close() error { return nil }

// The option is absent on an ordinary parse and explicit only when a caller
// asks for a transient or a deliberately kept parse.
func TestWithKeepReachesTheWire(t *testing.T) {
	ctx := context.Background()
	counting := &countingParseCaller{}
	c := &client{caller: counting}
	documents := []Document{Source("a.sysml", "package A;")}
	for _, opts := range [][]ParseOption{nil, {WithKeep(false)}, {WithKeep(true)}} {
		if _, err := c.ParseDocuments(ctx, documents, opts...); err != nil {
			t.Fatalf("ParseDocuments: %v", err)
		}
	}
	if len(counting.requests) != 3 {
		t.Fatalf("parse requests = %d, want 3", len(counting.requests))
	}
	if counting.infos != 1 {
		t.Errorf("ServerInfo preflights = %d, want one for the session", counting.infos)
	}
	want := []*bool{nil, boolPtr(false), boolPtr(true)}
	for i, req := range counting.requests {
		switch {
		case want[i] == nil && req.Keep != nil:
			t.Errorf("request %d sent keep=%v without the option", i, req.GetKeep())
		case want[i] != nil && req.Keep == nil:
			t.Errorf("request %d did not send keep", i)
		case want[i] != nil && req.GetKeep() != *want[i]:
			t.Errorf("request %d sent keep=%v, want %v", i, req.GetKeep(), *want[i])
		}
	}
}

func boolPtr(value bool) *bool { return &value }

// WithKeep names a ParseSources option: a one-document parse is always kept and
// must refuse it rather than silently ignore it.
func TestWithKeepOnOneDocumentParseIsRefused(t *testing.T) {
	ctx := context.Background()
	c := &client{caller: &countingParseCaller{}}
	_, err := c.ParseSource(ctx, "package A;", WithKeep(false))
	var status *StatusError
	if !errors.As(err, &status) || status.Code != CodeInvalidArgument || !strings.Contains(status.Message, "WithKeep") {
		t.Errorf("ParseSource with WithKeep: error %v, want INVALID_ARGUMENT naming WithKeep", err)
	}
}
