// Package lsptools is the workspace group of sysml-mcp tools: the language
// server's diagnostics, navigation, editing and rendering, served over MCP from
// one sysml-lsp Server held in process over a workspace root.
package lsptools

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Host is the language server the tools call, initialized over one workspace
// folder as an editor would initialize it.
type Host struct {
	Root      string
	Server    *lsp.Server
	Workspace *model.Workspace
}

// NewHost starts a language server over ws rooted at root: the folder is
// indexed before the host returns, so the first tool call sees every document.
func NewHost(ctx context.Context, root string, ws *model.Workspace, version string) (*Host, error) {
	srv := lsp.NewServer(ws, lsp.WithVersion(version))
	params := &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(root)), Name: root}},
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Hover: &protocol.HoverTextDocumentClientCapabilities{ContentFormat: []protocol.MarkupKind{protocol.Markdown}},
			},
		},
	}
	if _, err := srv.Initialize(ctx, params); err != nil {
		return nil, fmt.Errorf("initializing the language server over %s: %w", root, err)
	}
	if err := srv.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return nil, fmt.Errorf("indexing %s: %w", root, err)
	}
	return &Host{Root: root, Server: srv, Workspace: ws}, nil
}

// Refresh reconciles the workspace with the root as it is on disk now. An
// editor reports file changes to the language server as they happen; an agent
// edits files with its own tools and tells no one, so every tool that reads the
// workspace refreshes it first.
func (h *Host) Refresh(ctx context.Context) { h.Server.Rescan(ctx) }

// Close ends the language server session, releasing what it holds.
func (h *Host) Close(ctx context.Context) error {
	return h.Server.Shutdown(ctx)
}

// URI is the document URI of a path, as the language server names documents.
func (h *Host) URI(path string) protocol.DocumentURI { return uri.File(path) }

// WorkspaceInfoInput takes no arguments.
type WorkspaceInfoInput struct{}

// WorkspaceInfo describes the workspace the server holds.
type WorkspaceInfo struct {
	Root      string   `json:"root" jsonschema:"the workspace folder the server indexed"`
	Documents []string `json:"documents" jsonschema:"paths of the model documents indexed, the standard library excluded"`
	Open      []string `json:"open,omitempty" jsonschema:"documents whose text is held open with edits not yet on disk"`
}

// Register adds the workspace tools to s.
func Register(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "workspace_info",
		Description: "Describe the SysML v2 workspace the server has indexed: its root folder and the model documents found under it.",
		Annotations: toolkit.ReadOnly("Workspace info"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ WorkspaceInfoInput) (*mcp.CallToolResult, WorkspaceInfo, error) {
		h.Refresh(ctx)
		info := WorkspaceInfo{Root: h.Root, Documents: []string{}}
		for _, name := range h.Workspace.DocumentNames() {
			if !h.Workspace.IsLibraryDocument(name) {
				info.Documents = append(info.Documents, name)
			}
		}
		sort.Strings(info.Documents)
		info.Open = h.Workspace.OpenNames()
		return nil, info, nil
	})
}
