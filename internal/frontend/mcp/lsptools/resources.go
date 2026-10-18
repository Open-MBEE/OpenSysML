package lsptools

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mimeOf is the media type of a model document, by file extension.
func mimeOf(name string) string {
	if strings.EqualFold(path.Ext(name), ".kerml") {
		return "text/x-kerml"
	}
	return "text/x-sysml"
}

// registerResources serves the bundled standard library as one resource per
// document and the workspace's documents through a URI template.
func registerResources(s *mcp.Server, h *Host) {
	for _, name := range h.Workspace.LibraryNames() {
		name := name
		s.AddResource(&mcp.Resource{
			URI:         stdlibResourceURI(name),
			Name:        name,
			Title:       path.Base(name),
			Description: "The text of the bundled SysML v2 standard library document " + name + ".",
			MIMEType:    mimeOf(name),
		}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return h.readStdlib(req.Params.URI, name)
		})
	}
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: workspaceResourcePrefix + "{+path}",
		Name:        "workspace document",
		Title:       "Workspace document",
		Description: "The current text of a model document of the workspace: the open buffer when open_document holds one, else the file on disk. The path is relative to the workspace root.",
		MIMEType:    "text/x-sysml",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		rest, ok := strings.CutPrefix(req.Params.URI, workspaceResourcePrefix)
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		rel, err := url.PathUnescape(rest)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", req.Params.URI, err)
		}
		h.mu.RLock()
		defer h.mu.RUnlock()
		ref, err := h.document(ctx, rel)
		if err != nil {
			return nil, err
		}
		content, err := h.content(ref)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: mimeOf(ref.name), Text: string(content)}}}, nil
	})
}

// readStdlib answers one standard library resource.
func (h *Host) readStdlib(uri, name string) (*mcp.ReadResourceResult, error) {
	doc := h.Workspace.LibraryDocument(name)
	if doc == nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeOf(name), Text: string(doc.Content)}}}, nil
}
