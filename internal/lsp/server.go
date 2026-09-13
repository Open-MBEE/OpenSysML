package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// Server is the SysML v2 language server. It wraps a single model.Workspace and
// serves the Language Server Protocol over a jsonrpc2 stream.
type Server struct {
	baseServer
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

	// pubMu serializes analyze-and-send, so the sweep's timer goroutine and a
	// notification handler cannot deliver one document's diagnostics out of order.
	pubMu sync.Mutex

	mu               sync.Mutex
	shutdownReceived bool
	exitReceived     bool
	folders          []string
	// hoverMarkdown records that the client advertised Markdown for hover.
	hoverMarkdown bool
	// completionMarkdown records that the client advertised Markdown for
	// completion item documentation.
	completionMarkdown bool
}

// notifier sends a notification by method name, which a client connection does.
type notifier interface {
	Notify(ctx context.Context, method string, params interface{}) error
}

// crossDocRefreshWindow is how long an edit burst has to settle before the
// other open documents are re-analyzed.
const crossDocRefreshWindow = 200 * time.Millisecond

// NewServer returns a Server bound to ws.
func NewServer(ws *model.Workspace) *Server {
	return &Server{
		ws:           ws,
		exited:       make(chan struct{}),
		crossDoc:     model.NewDebouncer(crossDocRefreshWindow),
		renderNotify: model.NewDebouncer(crossDocRefreshWindow),
	}
}

// Run wires the server to a stdio-style stream and blocks until the connection
// is done.
//
// The connection is built here rather than with protocol.NewServer because that
// helper already starts a read loop; a second conn.Go would run two concurrent
// readers over the same stream and corrupt the framing. runHandler adds back
// what that helper wrapped around the handler.
func (s *Server) Run(ctx context.Context, rwc io.ReadWriteCloser) error {
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(rwc))
	client := protocol.ClientDispatcher(conn, zap.NewNop())
	ctx = protocol.WithClient(ctx, client)
	s.client = client
	s.notifier = conn
	conn.Go(ctx, runHandler(s))
	select {
	case <-conn.Done():
		return conn.Err()
	case <-s.exited:
		// The exit notification ends the session: the stream is released here,
		// where nothing is waiting on it, and the session is not in error.
		_ = conn.Close()
		return nil
	}
}

// runHandler is the chain a served session reads with: cancellation, async
// dispatch so one slow request cannot stall the stream, and a reply per request.
func runHandler(s *Server) jsonrpc2.Handler {
	serve := s.stdlibHandler(s.renderHandler(s.modelEditHandler(s.changeHandler(protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)))))
	// The lifecycle wrapper runs outside AsyncHandler so that shutdown, exit and
	// the messages after them are ordered as the client sent them.
	return s.lifecycleHandler(cancelHandler(jsonrpc2.AsyncHandler(jsonrpc2.ReplyHandler(serve))))
}

// cancelHandler is protocol.CancelHandler with an id decode that accepts the
// JSON numbers clients send; that helper rejects them all as malformed.
func cancelHandler(handler jsonrpc2.Handler) jsonrpc2.Handler {
	handler, cancel := jsonrpc2.CancelHandler(handler)
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() != protocol.MethodCancelRequest {
			// A cancelled request still owes the client a reply, on a context
			// that outlives the cancellation.
			answer := func(ctx context.Context, result any, err error) error {
				if err == nil && ctx.Err() != nil {
					err = protocol.ErrRequestCancelled
				}
				return reply(context.WithoutCancel(ctx), result, err)
			}
			return handler(ctx, answer, req)
		}
		var params struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(req.Params(), &params); err != nil {
			return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
		}
		id, err := requestID(params.ID)
		if err != nil {
			return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
		}
		cancel(id)
		return reply(ctx, nil, nil)
	}
}

// requestID decodes a JSON-RPC id, which is either a number or a string.
func requestID(raw json.RawMessage) (jsonrpc2.ID, error) {
	var number int32
	if err := json.Unmarshal(raw, &number); err == nil {
		return jsonrpc2.NewNumberID(number), nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return jsonrpc2.NewStringID(text), nil
	}
	return jsonrpc2.ID{}, fmt.Errorf("request ID %s malformed", raw)
}
