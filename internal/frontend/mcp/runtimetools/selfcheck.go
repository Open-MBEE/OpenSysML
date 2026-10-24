package runtimetools

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// SelfCheckInput applies the reflective validation constraints to a model.
type SelfCheckInput struct {
	ModelInput
	SelfCheckPackage []string `json:"selfCheckPackage,omitempty" jsonschema:"qualified names of packages whose constraint definitions apply after the bundled SysMLValidation ones (-self-check-package); each must be a package the model or its libraries declare"`
}

func (g *group) registerSelfCheck(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "self_check",
		Description: "Apply the SysMLValidation constraints, and those of any rule packages named, reflectively to every element of a " +
			"model (the SelfCheck RPC, the CLI's -self-check and -self-check-package): each element is handed to every constraint " +
			"whose input its metaclass conforms to. `status` is holds, fails or unresolved; `findings` carry each violation, " +
			"evaluation the runtime could not make, error and skipped rule constraint with the element's location and the line the " +
			"CLI prints; the counts and `applications` say how much was checked. The result is the SelfCheckResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Self-check"),
	}, func(ctx context.Context, call *mcp.CallToolRequest, in SelfCheckInput) (*mcp.CallToolResult, Result, error) {
		req := &pb.SelfCheckRequest{}
		var resp *pb.SelfCheckResponse
		return g.verify(ctx, in, in.ModelInput, req, func() (proto.Message, error) {
			err := withProgress(ctx, call, "self_check", func() (err error) {
				resp, err = g.svc.SelfCheck(ctx, req)
				return err
			})
			return resp, err
		}, func() string {
			var b strings.Builder
			b.WriteString(errorText(resp.GetError()))
			for _, f := range resp.GetFindings() {
				b.WriteString(f.GetLine())
				b.WriteByte('\n')
			}
			if resp.GetSummary() != "" {
				b.WriteString(resp.GetSummary())
				b.WriteByte('\n')
			}
			b.WriteString(diagnosticsText(resp.GetDiagnostics()))
			return b.String()
		})
	})
}
