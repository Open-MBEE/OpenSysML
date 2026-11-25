package opensysml

import (
	"context"
	"errors"
	"slices"
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

// The inherited member lists are refused the same way without convert_inherited_members.
func TestInheritedMembersAreNotRequestedWithoutTheCapability(t *testing.T) {
	old := &oldCaller{t: t, capabilities: []string{CapabilityConvert}}
	c := &client{caller: old}
	_, err := c.ConvertSource(context.Background(), "package P;", FormatAPIJSON, WithFromFormat(FormatSysML), WithInheritedMembers())
	wantMissingCapability(t, "ConvertSource", err, CapabilityConvertInheritedMembers)
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

// A nil base after a real one asks for a plain parse again, not the earlier comparison.
func TestANilBaseModelClearsTheEarlierOne(t *testing.T) {
	var o parseOptions
	for _, apply := range []ParseOption{WithBaseModel(&Model{Hash: "h"}), WithBaseModel(nil)} {
		apply(&o)
	}
	if o.base != "" {
		t.Fatalf("base after WithBaseModel(nil) = %q, want none", o.base)
	}
}

// A relative direction is refused the same way without symbol_member_direction,
// since a service without it would answer the direction the feature declares.
func TestAMemberDirectionIsNotAskedWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	for name, old := range map[string]*oldCaller{
		"predates symbol_member_direction": {capabilities: []string{CapabilitySymbolMembers}},
		"predates GetServerInfo":           {infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.LookupSymbol(ctx, &Model{Hash: "h"}, "P::Port::cmd", AsMemberOf("P::Device::pIn"))
			wantMissingCapability(t, "LookupSymbol", err, CapabilitySymbolMemberDirection)
		})
	}
}

// Each Convert option the service gates is refused before the request leaves the client when the
// service lacks its capability, since such a service would ignore the field and answer as if unasked.
func TestConvertOptionsAreNotRequestedWithoutTheirCapability(t *testing.T) {
	ctx := context.Background()
	others := []string{CapabilityConvert, CapabilityConvertDocuments, CapabilityConvertCompact, CapabilityConvertCheckDiagnostics, CapabilityConvertProjection}
	for _, tc := range []struct {
		name       string
		capability string
		options    []ConvertOption
	}{
		{"WithDocuments", CapabilityConvertDocuments, []ConvertOption{WithDocuments("a.sysml")}},
		{"WithCompact", CapabilityConvertCompact, []ConvertOption{WithCompact()}},
		{"WithOmitDerived", CapabilityConvertCompact, []ConvertOption{WithOmitDerived()}},
		{"WithOmitDerived keeping", CapabilityConvertCompact, []ConvertOption{WithOmitDerived("qualifiedName")}},
		{"WithCheckDiagnostics", CapabilityConvertCheckDiagnostics, []ConvertOption{WithCheckDiagnostics()}},
		{"WithProjection", CapabilityConvertProjection, []ConvertOption{WithProjection(Projection{QualifiedNames: []string{"P::A"}})}},
	} {
		without := slices.DeleteFunc(slices.Clone(others), func(name string) bool { return name == tc.capability })
		for name, old := range map[string]*oldCaller{
			"predates " + tc.capability: {capabilities: without},
			"predates GetServerInfo":    {infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
		} {
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				old.t = t
				c := &client{caller: old}
				_, err := c.Convert(ctx, &Model{Hash: "h"}, FormatAPIJSON, tc.options...)
				wantMissingCapability(t, "Convert", err, tc.capability)
			})
		}
	}
}

// recordingCaller announces every capability and keeps the conversion requests it is sent.
type recordingCaller struct {
	caller
	requests []*pb.ConvertRequest
}

func (r *recordingCaller) serverInfo(context.Context) (*pb.ServerInfoResponse, error) {
	return &pb.ServerInfoResponse{Version: "new", Capabilities: []string{
		CapabilityConvert, CapabilityConvertDocuments, CapabilityConvertCompact, CapabilityConvertCheckDiagnostics,
		CapabilityConvertProjection,
	}}, nil
}

func (r *recordingCaller) convert(_ context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	r.requests = append(r.requests, req)
	return &pb.ConvertResponse{Content: "{}", Diagnostics: []*pb.Diagnostic{
		{Severity: SeverityWarning, Message: "unused import", Code: "unused-import"},
		{Severity: SeverityError, Message: "unresolved name", Code: "unresolved-reference"},
	}}, nil
}

// The Convert options reach the wire as the fields they name, a plain conversion sets none of
// them, and each diagnostic comes back with the severity and code the service gave it.
func TestConvertOptionsReachTheWire(t *testing.T) {
	ctx := context.Background()
	recorded := &recordingCaller{}
	c := &client{caller: recorded}
	keep := []string{"qualifiedName", "owner"}
	projection := Projection{QualifiedNames: []string{"P::A"}, Metaclasses: []string{"PartUsage"}}
	asked, err := c.Convert(ctx, &Model{Hash: "h"}, FormatAPIJSON,
		WithDocuments("a.sysml", "b.sysml"), WithCompact(), WithOmitDerived(keep...), WithCheckDiagnostics(),
		WithProjection(projection))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	keep[0] = "changed after the option was built"
	projection.QualifiedNames[0] = "changed after the option was built"
	if _, err := c.Convert(ctx, &Model{Hash: "h"}, FormatAPIJSON); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	wanted, plain := recorded.requests[0], recorded.requests[1]
	if !slices.Equal(wanted.Documents, []string{"a.sysml", "b.sysml"}) || !wanted.Compact || !wanted.OmitDerived ||
		!slices.Equal(wanted.KeepDerived, []string{"qualifiedName", "owner"}) || !wanted.IncludeCheckDiagnostics ||
		wanted.GetProjection().GetQualifiedNames()[0] != "P::A" || wanted.GetProjection().GetMetaclasses()[0] != "PartUsage" {
		t.Errorf("request with every option = %v", wanted)
	}
	if plain.Documents != nil || plain.Compact || plain.OmitDerived || plain.KeepDerived != nil ||
		plain.IncludeCheckDiagnostics || plain.Projection != nil {
		t.Errorf("request with no option = %v", plain)
	}
	want := []Diagnostic{
		{Severity: SeverityWarning, Message: "unused import", Code: "unused-import"},
		{Severity: SeverityError, Message: "unresolved name", Code: "unresolved-reference"},
	}
	if !slices.EqualFunc(asked.Diagnostics, want, func(a, b Diagnostic) bool { return a == b }) {
		t.Errorf("diagnostics = %+v, want %+v", asked.Diagnostics, want)
	}
}

// WithBaseModel names the model as it was when the option was made.
func TestWithBaseModelSnapshotsTheHash(t *testing.T) {
	base := &Model{Hash: "h"}
	option := WithBaseModel(base)
	base.Hash = "moved"
	var o parseOptions
	option(&o)
	if o.base != "h" {
		t.Fatalf("base = %q, want the hash the option was made with", o.base)
	}
}
