package lsp

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/buildinfo"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Server is the SysML v2 language server. It wraps a single model.Workspace and
// serves the Language Server Protocol over a jsonrpc2 stream.
type Server struct {
	protocol.UnimplementedServer
	ws     *model.Workspace
	client protocol.Client
	// notifier sends the notifications the protocol library does not know, of
	// which opensysml/renderChanged is one.
	notifier notifier

	// exited is closed by Exit so that Run returns from the read loop rather
	// than a handler tearing the connection down under itself.
	exited   chan struct{}
	exitOnce sync.Once

	// crossDoc coalesces the sweep that republishes the other open documents, so
	// a burst of keystrokes costs one pass over them instead of one per change.
	crossDoc *model.Debouncer

	// renderNotify coalesces opensysml/renderChanged the same way, so a diagram
	// client redraws from settled text rather than from every keystroke.
	renderNotify *model.Debouncer
	// debug holds the live opensysml/debug sessions.
	debug *debugService

	// pubMu serializes analyze-and-send, so the sweep's timer goroutine and a
	// notification handler cannot deliver one document's diagnostics out of order.
	pubMu sync.Mutex
	// findingURIs are the documents not open in the editor that the workspace
	// findings were last published on, to withdraw from when they move.
	findingURIs map[uri.URI]bool

	mu               sync.Mutex
	shutdownReceived bool
	exitReceived     bool
	folders          []string
	// openDirs are the directories indexed for documents opened under no folder.
	openDirs map[string]bool
	// notebooks are the open notebooks, each analyzed as one document.
	notebooks notebooks
	// libraryRoots are the directories installed projects are looked for
	// under, besides the well-known ones beneath a folder's project.
	libraryRoots []string
	// dependencies records, per folder or opened directory, the sources the
	// installed projects it uses contributed, so they are dropped with it.
	dependencies map[string][]string
	// hoverMarkdown records that the client advertised Markdown for hover.
	hoverMarkdown bool
	// completionMarkdown records that the client advertised Markdown for
	// completion item documentation.
	completionMarkdown bool
	// crossDocument records that the client advertised CrossDocumentCapability.
	crossDocument bool
	// version is what initialize reports as the server's version.
	version string
}

// Option configures a Server at construction.
type Option func(*Server)

// WithVersion sets the version initialize reports in serverInfo, which is the
// version of the binary serving: the one its -version flag prints.
func WithVersion(version string) Option {
	return func(s *Server) { s.version = version }
}

// WithLibraryRoots sets the directories installed projects are looked for
// under, besides the well-known ones beneath a folder's project; the
// libraryRoots setting replaces them.
func WithLibraryRoots(roots []string) Option {
	return func(s *Server) { s.libraryRoots = append([]string(nil), roots...) }
}

// notifier sends a notification by method name, which a client connection does.
type notifier interface {
	Notify(ctx context.Context, method string, params interface{}) error
}

// crossDocRefreshWindow is how long an edit burst has to settle before the
// other open documents are re-analyzed.
const crossDocRefreshWindow = 200 * time.Millisecond

// NewServer returns a Server bound to ws; without WithVersion it reports an
// unversioned build.
func NewServer(ws *model.Workspace, opts ...Option) *Server {
	s := &Server{
		ws:           ws,
		exited:       make(chan struct{}),
		crossDoc:     model.NewDebouncer(crossDocRefreshWindow),
		renderNotify: model.NewDebouncer(crossDocRefreshWindow),
		debug:        newDebugService(),
		version:      buildinfo.Unversioned,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run wires the server to a stdio-style stream and blocks until the connection
// is done.
//
// The connection is built here rather than with protocol.NewServer because that
// helper starts the read loop with its own handler chain; runHandler is the
// chain this server needs, with the lifecycle ordered ahead of the asynchronous
// dispatch.
func (s *Server) Run(ctx context.Context, rwc io.ReadWriteCloser) error {
	stream := jsonrpc2.NewStream(rwc)
	conn := jsonrpc2.NewConn(stream, jsonrpc2.WithCodec(lspCodec{}))
	client := protocol.ClientDispatcher(conn)
	ctx = protocol.WithClient(ctx, client)
	s.client = client
	s.notifier = conn
	conn.Go(ctx, runHandler(s))
	select {
	case <-conn.Done():
		return conn.Err()
	case <-s.exited:
		// The exit notification ends the session. The stream is closed rather
		// than the connection: closing the connection waits for the read loop,
		// which a blocking stdin never releases.
		_ = stream.Close()
		return nil
	}
}

// lspCodec encodes the wire as the protocol library does, so its optional and
// union types are written as the specification says; the library wires that
// codec into protocol.NewServer alone, which Run does not use.
type lspCodec struct{}

func (lspCodec) Marshal(v any) ([]byte, error)      { return protocol.Marshal(v) }
func (lspCodec) Unmarshal(data []byte, v any) error { return protocol.Unmarshal(data, v) }

// runHandler is the chain a served session reads with: cancellation, dispatch
// off the read loop so one slow request cannot stall the stream, and the custom
// methods ahead of the protocol's.
func runHandler(s *Server) jsonrpc2.Handler {
	serve := s.stdlibHandler(s.debugHandler(s.renderHandler(s.importDiagramHandler(s.modelEditHandler(notificationErrorsStayLocal(s.changeHandler(s.cellHandler(protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)))))))))
	// The lifecycle wrapper runs outside inOrder so that shutdown, exit and
	// the messages after them are ordered as the client sent them.
	return s.lifecycleHandler(protocol.CancelHandler(inOrder(answerCancelled(serve))))
}

// inOrder frees the read loop at once but runs the handlers one at a time in
// wire order; a request cancelled while it waits answers without running.
func inOrder(handler jsonrpc2.Handler) jsonrpc2.Handler {
	var mu sync.Mutex
	previous := make(chan struct{})
	close(previous)
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		// Taken on the read loop, before it is released: the queue is wire order.
		mu.Lock()
		wait := previous
		done := make(chan struct{})
		previous = done
		mu.Unlock()
		jsonrpc2.Async(ctx)
		select {
		case <-wait:
		case <-ctx.Done():
			if req.IsCall() {
				// The ticket still closes in turn, so nothing behind overtakes.
				go func() { <-wait; close(done) }()
				return nil, protocol.ErrRequestCancelled
			}
			<-wait
		}
		defer close(done)
		return handler(ctx, req)
	}
}

// answerCancelled gives a request cancelled while it was served the reply the
// protocol asks for, when the handler produced none of its own.
func answerCancelled(handler jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		result, err := handler(ctx, req)
		if err == nil && req.IsCall() && ctx.Err() != nil {
			return nil, protocol.ErrRequestCancelled
		}
		return result, err
	}
}

// notificationErrorsStayLocal keeps a notification's failure out of the
// connection: a notification has no reply to carry it, the handlers have told
// the client what they can (a library edit shows a message), and the library
// would otherwise end the session over it. Unknown notifications are ignored,
// as the protocol asks.
func notificationErrorsStayLocal(handler jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		result, err := handler(ctx, req)
		if err != nil && !req.IsCall() {
			return nil, nil
		}
		return result, err
	}
}

// decodeParams reads a request's params as T; what does not decode is a parse
// error, as the protocol library answers its own methods.
func decodeParams[T any](req *jsonrpc2.Request) (*T, error) {
	var params T
	if err := protocol.Unmarshal(req.Params(), &params); err != nil {
		return nil, fmt.Errorf("%w: %w", jsonrpc2.ErrParse, err)
	}
	return &params, nil
}
