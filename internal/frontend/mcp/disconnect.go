package mcp

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// requestHeader carries the number liveRequests gave an HTTP request, which is
// how a tools/call finds the request that delivered it.
const requestHeader = "Sysml-Mcp-Request"

// liveRequests holds the context of each protocol POST being served. The zero
// value is ready to use.
type liveRequests struct {
	mu   sync.Mutex
	last uint64
	ctx  map[string]context.Context
}

// track numbers each POST where the SDK hands it to the request's handlers,
// over anything the client sent under that name, for as long as it is served.
func (l *liveRequests) track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(requestHeader)
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		l.mu.Lock()
		l.last++
		id := strconv.FormatUint(l.last, 10)
		if l.ctx == nil {
			l.ctx = map[string]context.Context{}
		}
		l.ctx[id] = r.Context()
		l.mu.Unlock()
		defer func() {
			l.mu.Lock()
			delete(l.ctx, id)
			l.mu.Unlock()
		}()
		r.Header.Set(requestHeader, id)
		next.ServeHTTP(w, r)
	})
}

// bound is ctx ending when the POST that delivered req does: no stream is kept
// to resume, so a call whose client hung up has nobody to answer.
func (l *liveRequests) bound(ctx context.Context, req sdk.Request) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	extra := req.GetExtra()
	if extra == nil || extra.Header == nil {
		return ctx, cancel
	}
	id := extra.Header.Get(requestHeader)
	if id == "" {
		return ctx, cancel
	}
	l.mu.Lock()
	post := l.ctx[id]
	l.mu.Unlock()
	if post == nil {
		// The POST ended before its call was dispatched.
		cancel()
		return ctx, cancel
	}
	stop := context.AfterFunc(post, cancel)
	return ctx, func() { stop(); cancel() }
}
