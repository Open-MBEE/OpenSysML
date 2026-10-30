package opensysml

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func (o *oldCaller) convert(context.Context, *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	o.t.Fatal("a conversion was sent without its required capability")
	return nil, nil
}

func (o *oldCaller) parseSources(context.Context, *pb.ParseSourcesRequest) (*pb.ParseSourcesResponse, error) {
	o.t.Fatal("a base model was sent without its required capability")
	return nil, nil
}

func wantMissingCapability(t *testing.T, op string, err error, capability string) {
	t.Helper()
	wantUnimplemented(t, op, err)
	var status *StatusError
	if !errors.As(err, &status) || !strings.Contains(status.Message, capability) {
		t.Errorf("%s error = %v, want missing capability %q", op, err, capability)
	}
}

// Library stubs are refused before the request leaves the client when the
// service lacks convert_library_stubs, since such a service would write the
// bare references instead of the stubs asked for.
func TestLibraryStubsAreNotRequestedWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	for name, old := range map[string]*oldCaller{
		"predates convert_library_stubs": {capabilities: []string{CapabilityConvert}},
		"predates GetServerInfo":         {infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.ConvertSource(ctx, "package P;", FormatAPIJSON, WithFromFormat(FormatSysML), WithLibraryStubs())
			wantMissingCapability(t, "ConvertSource", err, CapabilityConvertLibraryStubs)
		})
	}
}

// A base model is refused the same way without parse_sources_affected, since a
// service without it would parse the documents and report no affected set.
func TestABaseModelIsNotSentWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	for name, old := range map[string]*oldCaller{
		"predates parse_sources_affected": {capabilities: []string{CapabilityParseSources}},
		"predates GetServerInfo":          {infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.ParseDocuments(ctx, []Document{Source("a.sysml", "package A;")}, WithBaseModel(&Model{Hash: "h"}))
			wantMissingCapability(t, "ParseDocuments", err, CapabilityParseSourcesAffected)
		})
	}
}
