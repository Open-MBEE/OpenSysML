// Package lsptools serves the language server's workspace over MCP: the
// documents under the root, their diagnostics, navigation, editing and
// rendering, as tools an agent calls instead of an editor.
package lsptools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// stdlibResourcePrefix is the resource URI prefix of the bundled standard
// library documents, each `sysml://stdlib/<file>`.
const stdlibResourcePrefix = "sysml://stdlib/"

// workspaceResourcePrefix is the resource URI prefix of workspace documents,
// `sysml://workspace/<path>` with the path relative to the root.
const workspaceResourcePrefix = "sysml://workspace/"

// Host is the language server the tools call, initialized over the workspace
// root with no editor attached: the tools are its client.
type Host struct {
	Root      string
	Server    *lsp.Server
	Workspace *model.Workspace

	root toolkit.Root // Root as spelled and with its symlinks resolved, which every document must lie under

	// mu holds the workspace for a tool: shared while it reads, exclusive
	// while it edits, so an edit is computed and written over one revision.
	mu sync.RWMutex
	// opened is the file on disk as it was when each open document was opened
	// or last written, so a buffer is not written over an edit made meanwhile.
	opened map[string]diskState
}

// diskState is a file's content on disk, or its absence.
type diskState struct {
	content []byte
	existed bool
}

// readDisk is the state of the file at name, which may not exist.
func readDisk(name string) (diskState, error) {
	content, err := os.ReadFile(name) // #nosec G304 -- a workspace document resolve admitted
	if errors.Is(err, fs.ErrNotExist) {
		return diskState{}, nil
	}
	if err != nil {
		return diskState{}, err
	}
	return diskState{content: content, existed: true}, nil
}

// same reports whether two disk states hold the same file.
func (d diskState) same(o diskState) bool {
	return d.existed == o.existed && bytes.Equal(d.content, o.content)
}

// NewHost starts a language server over root and indexes the model documents
// under it.
func NewHost(ctx context.Context, root string, ws *model.Workspace, version string) (*Host, error) {
	r, err := toolkit.NewRoot(root)
	if err != nil {
		return nil, err
	}
	srv := lsp.NewServer(ws, lsp.WithVersion(version), lsp.WithLibraryRoots(project.EnvLibraryRoots()))
	params := &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(root)), Name: root}},
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Hover: &protocol.HoverClientCapabilities{ContentFormat: []protocol.MarkupKind{protocol.MarkupKindMarkdown}},
			},
		},
	}
	if _, err := srv.Initialize(ctx, params); err != nil {
		return nil, fmt.Errorf("initializing the language server over %s: %w", root, err)
	}
	if err := srv.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		return nil, fmt.Errorf("indexing %s: %w", root, err)
	}
	return &Host{Root: root, Server: srv, Workspace: ws, root: r, opened: map[string]diskState{}}, nil
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

// docRef names a document the tools address: a workspace file by absolute
// path, or a bundled library document by its library-relative name.
type docRef struct {
	name    string
	library bool
}

func (d docRef) uri() uri.URI {
	if d.library {
		u := url.URL{Scheme: lsp.LibraryScheme, Path: "/" + filepath.ToSlash(d.name)}
		return uri.URI(u.String())
	}
	return uri.File(d.name)
}

// resolve turns a tool's path argument into a document reference: a path
// relative to the root or absolute, which must lie under the root, a
// `sysml://stdlib/<file>` resource URI or a `sysml-stdlib:` document URI of
// the standard library.
func (h *Host) resolve(path string) (docRef, error) {
	path = strings.TrimSpace(path)
	if name, ok := strings.CutPrefix(path, stdlibResourcePrefix); ok {
		name, err := url.PathUnescape(name)
		if err != nil {
			return docRef{}, fmt.Errorf("%s: %w", path, err)
		}
		return h.resolveLibrary(name)
	}
	if strings.HasPrefix(strings.ToLower(path), lsp.LibraryScheme+":") {
		parsed, err := url.Parse(path)
		if err != nil {
			return docRef{}, fmt.Errorf("%s: %w", path, err)
		}
		return h.resolveLibrary(strings.TrimPrefix(parsed.Path, "/"))
	}
	if strings.HasPrefix(path, "file://") {
		path = uri.URI(path).FsPath()
	}
	res, err := h.root.Resolve(path)
	if err != nil {
		return docRef{}, err
	}
	return docRef{name: res.Path}, nil
}

func (h *Host) resolveLibrary(name string) (docRef, error) {
	if h.Workspace.LibraryDocument(name) == nil {
		return docRef{}, fmt.Errorf("%s is not a standard library document", name)
	}
	return docRef{name: name, library: true}, nil
}

// document resolves a path after bringing the workspace up to date with disk,
// so the answer is about the file as the agent's own tools left it.
func (h *Host) document(ctx context.Context, path string) (docRef, error) {
	h.Refresh(ctx)
	return h.resolve(path)
}

// content is the text the workspace holds of ref: the open buffer, else the
// analyzed on-disk text; a library document's bundled text.
func (h *Host) content(ref docRef) ([]byte, error) {
	if ref.library {
		if doc := h.Workspace.LibraryDocument(ref.name); doc != nil {
			return doc.Content, nil
		}
		return nil, fmt.Errorf("%s is not a standard library document", ref.name)
	}
	content, _, ok := h.Workspace.AnalyzedContent(ref.name)
	if !ok {
		return nil, fmt.Errorf("%s is not a document of the workspace: it is not a .sysml or .kerml file under %s, or does not exist", h.display(ref.uri()), h.Root)
	}
	return content, nil
}

// contentOf is the text of a document a result names by URI, nil when the
// workspace does not hold it.
func (h *Host) contentOf(u uri.URI) []byte {
	ref := h.refOf(u)
	content, err := h.content(ref)
	if err != nil {
		return nil
	}
	return content
}

// refOf is the document reference of a URI the server produced.
func (h *Host) refOf(u uri.URI) docRef {
	if parsed, err := url.Parse(string(u)); err == nil && strings.EqualFold(parsed.Scheme, lsp.LibraryScheme) {
		return docRef{name: strings.TrimPrefix(parsed.Path, "/"), library: true}
	}
	return docRef{name: uri.URI(u).FsPath()}
}

// display is how results name a document: the path relative to the root when
// under it, the absolute path otherwise, and the `sysml://stdlib/<file>`
// resource URI for a standard library document.
func (h *Host) display(u uri.URI) string {
	ref := h.refOf(u)
	if ref.library {
		return stdlibResourceURI(ref.name)
	}
	if toolkit.Under(h.Root, ref.name) {
		rel, _ := filepath.Rel(h.Root, ref.name)
		return filepath.ToSlash(rel)
	}
	return ref.name
}

func stdlibResourceURI(name string) string {
	segs := strings.Split(filepath.ToSlash(name), "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return stdlibResourcePrefix + strings.Join(segs, "/")
}

func slices(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// pendingWrite is one file of a change: the text it is to hold, and the text
// the change was computed from.
type pendingWrite struct {
	name    string
	content []byte
	base    []byte
}

// writeAll puts every pending write on disk, or none: a file whose text on
// disk is no longer what its change was computed from, or that changed under
// an open buffer since the buffer was opened, refuses the whole change, and a
// failed write restores the files written before it. Every file operation
// goes through an os.Root at the resolved root, so a link repointed while the
// change is in flight cannot carry a write outside it. The index, and an open
// buffer of a written document, then follow the new text.
func (h *Host) writeAll(ctx context.Context, writes []pendingWrite) error {
	root, err := os.OpenRoot(h.root.Real)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	type original struct {
		target string
		diskState
	}
	originals := make(map[string]original, len(writes))
	for _, w := range writes {
		target, err := h.root.Target(w.name)
		if err != nil {
			return err
		}
		prev, err := root.ReadFile(target)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		now := diskState{content: prev, existed: err == nil}
		if seen, open := h.opened[w.name]; open {
			if !now.same(seen) {
				return fmt.Errorf("%s changed on disk while it was held open; close_document and open it again with the text to keep", h.display(uri.File(w.name)))
			}
		} else if !now.same(diskState{content: w.base, existed: true}) {
			return fmt.Errorf("%s changed on disk since it was read; call the tool again", h.display(uri.File(w.name)))
		}
		originals[w.name] = original{target: target, diskState: now}
	}
	var written []string
	for _, w := range writes {
		if err := toolkit.WriteFile(root, originals[w.name].target, w.content, 0o600); err != nil {
			var restore []string
			for _, name := range written {
				o := originals[name]
				var rerr error
				if o.existed {
					rerr = toolkit.WriteFile(root, o.target, o.content, 0o600)
				} else {
					rerr = root.Remove(o.target)
				}
				if rerr != nil {
					restore = append(restore, h.display(uri.File(name)))
				}
			}
			if len(restore) > 0 {
				return fmt.Errorf("writing %s: %w; and %s could not be restored, check them", h.display(uri.File(w.name)), err, strings.Join(restore, ", "))
			}
			return fmt.Errorf("writing %s: %w (no file was changed)", h.display(uri.File(w.name)), err)
		}
		written = append(written, w.name)
	}
	for _, w := range writes {
		if _, open := h.opened[w.name]; open {
			h.opened[w.name] = diskState{content: w.content, existed: true}
			if err := h.replace(ctx, w.name, string(w.content)); err != nil {
				return err
			}
		}
	}
	h.Refresh(ctx)
	return nil
}

// replace makes text the content of the open document name, as the next
// version of its buffer.
func (h *Host) replace(ctx context.Context, name, text string) error {
	doc := h.Workspace.Document(name)
	version := int32(doc.Version + 1) // #nosec G115 -- document versions are small counters
	return h.Server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(name)}, Version: version},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Range: fullRange(doc.Content), Text: text}},
	})
}

// WorkspaceInfoInput takes no arguments.
type WorkspaceInfoInput struct{}

// WorkspaceInfo describes the workspace the server holds.
type WorkspaceInfo struct {
	Root      string   `json:"root" jsonschema:"the workspace folder the server indexed"`
	Documents []string `json:"documents" jsonschema:"paths of the model documents indexed, the standard library excluded"`
	Open      []string `json:"open,omitempty" jsonschema:"documents whose text is held open with edits not yet on disk"`
}

// Register adds the workspace tools and resources to s.
func Register(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "workspace_info",
		Description: "Describe the SysML v2 workspace the server has indexed: its root folder and the model documents found under it.",
		Annotations: toolkit.ReadOnly("Workspace info"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, _ WorkspaceInfoInput) (*mcp.CallToolResult, WorkspaceInfo, error) {
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
	}))
	registerQueries(s, h)
	registerEdits(s, h)
	registerRendering(s, h)
	registerSync(s, h)
	registerResources(s, h)
}

// reading runs a tool holding the workspace shared with other readers.
func reading[In, Out any](h *Host, fn mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		h.mu.RLock()
		defer h.mu.RUnlock()
		return fn(ctx, req, in)
	}
}

// writing runs a tool holding the workspace exclusively, from the read its
// edit is computed on to the write, so no other tool changes it in between.
func writing[In, Out any](h *Host, fn mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		return fn(ctx, req, in)
	}
}

// text wraps prose as the readable content of a result whose structured
// content is the typed output.
func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
