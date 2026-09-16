package opensysml

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func (o *oldCaller) applyEdits(context.Context, *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	o.t.Fatal("a document name was sent to a service without edit_documents")
	return nil, nil
}

// A document name is refused before it leaves the client when the service lacks
// edit_documents, since such a service would ignore the name and edit its sole
// document instead: whether it predates the capability or GetServerInfo itself.
func TestADocumentNameIsNotSentWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	for name, old := range map[string]*oldCaller{
		"predates edit_documents": {t: t, capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring}},
		"predates GetServerInfo":  {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.ApplyDocumentEdits(ctx, model, "typo.sysml", Rename{Target: "P::x", NewName: "y"})
			wantUnimplemented(t, "ApplyDocumentEdits", err)
		})
	}
}
