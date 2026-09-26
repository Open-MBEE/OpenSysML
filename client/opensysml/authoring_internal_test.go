package opensysml

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func (o *oldCaller) applyEdits(context.Context, *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	o.t.Fatal("an edit was sent without its required capability")
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

func TestAddConnectionIsNotSentWithoutItsCapabilities(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	tests := map[string]struct {
		capabilities []string
		missing      string
	}{
		"predates connection_authoring": {
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConnectionAuthoring,
		},
		"predates authoring": {
			capabilities: []string{CapabilityApplyEdits, CapabilityConnectionAuthoring},
			missing:      CapabilityAuthoring,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			old := &oldCaller{t: t, capabilities: test.capabilities}
			c := &client{caller: old}
			_, err := c.ApplyEdits(ctx, model, AddConnection{
				Owner: "Demo::System", Kind: "allocation", From: "a", To: "b",
			})
			wantUnimplemented(t, "ApplyEdits AddConnection", err)
			var status *StatusError
			if !errors.As(err, &status) || !strings.Contains(status.Message, test.missing) {
				t.Errorf("ApplyEdits error = %v, want missing capability %q", err, test.missing)
			}
		})
	}
}
