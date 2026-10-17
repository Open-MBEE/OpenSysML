package mcp

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// CatalogInput narrows the catalog to one group; empty answers every tool
// the server serves.
type CatalogInput struct {
	Group string `json:"group,omitempty" jsonschema:"answer only this group's tools: lsp, runtime or session"`
}

// CatalogTool is one tool as tool_catalog describes it.
type CatalogTool struct {
	Name      string `json:"name"`
	Group     string `json:"group,omitempty"`
	Operation string `json:"operation"`
	Summary   string `json:"summary"`
}

// CatalogOutput is the catalog of the tools the server serves.
type CatalogOutput struct {
	Groups []string      `json:"groups"`
	Tools  []CatalogTool `json:"tools"`
}

// registerCatalog adds tool_catalog, which answers the catalog entries of the
// groups this server serves, so a client discovers the surface and what each
// tool stands for without the manual page.
func registerCatalog(s *sdk.Server, groups []Group) {
	sdk.AddTool(s, &sdk.Tool{
		Name: "tool_catalog",
		Description: "List the tools this server serves: each tool's name, group, the sysml-grpc RPC or `sysml` CLI flag it " +
			"stands for, and a one-line summary. Pass `group` (lsp, runtime, session) to narrow the list. The same catalog " +
			"generates the Tools section of the sysml-mcp manual page.",
		Annotations: toolkit.ReadOnly("Tool catalog"),
	}, func(_ context.Context, _ *sdk.CallToolRequest, in CatalogInput) (*sdk.CallToolResult, CatalogOutput, error) {
		served := groups
		if in.Group != "" {
			var found bool
			for _, g := range groups {
				if string(g) == in.Group {
					served, found = []Group{g}, true
				}
			}
			if !found {
				return nil, CatalogOutput{}, fmt.Errorf("this server does not serve the group %q; it serves %s", in.Group, groupList(groups))
			}
		}
		out := CatalogOutput{Groups: []string{}}
		for _, g := range served {
			out.Groups = append(out.Groups, string(g))
		}
		var text strings.Builder
		for _, e := range catalog.Entries(served...) {
			out.Tools = append(out.Tools, CatalogTool{Name: e.Name, Group: string(e.Group), Operation: e.Operation(), Summary: e.Summary})
			fmt.Fprintf(&text, "%s (%s): %s\n", e.Name, e.Operation(), e.Summary)
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: strings.TrimRight(text.String(), "\n")}}}, out, nil
	})
}

func groupList(groups []Group) string {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = string(g)
	}
	return strings.Join(names, ", ")
}
