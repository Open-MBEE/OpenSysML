package opensysml

import (
	"context"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Graphs is the lowered graph of an action or state machine and of every
// behavior it performs, in the canonical `graphs:<Version>` JSON form an
// external analysis engine is sent.
type Graphs struct {
	// Content is the form as canonical JSON, ending in one newline.
	Content string
	// Version is the version of the form, the `version` field of the JSON.
	Version int
	// Subject is the qualified name of the behavior as resolved.
	Subject string
}

// ExportGraphs exports the lowered graph of the action or state machine the
// qualified name subject names.
func (c *client) ExportGraphs(ctx context.Context, model *Model, subject string) (*Graphs, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityExportGraphs); err != nil {
		return nil, err
	}
	response, err := c.caller.exportGraphs(ctx, &pb.ExportGraphsRequest{ModelHash: hash, Subject: subject})
	if err != nil {
		return nil, err
	}
	return &Graphs{Content: response.Content, Version: int(response.Version), Subject: response.Subject}, nil
}
