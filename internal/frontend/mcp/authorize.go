package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// CodeInsufficientScope is the JSON-RPC error code of a tools/call refused for
// a scope the principal lacks.
const CodeInsufficientScope = toolkit.CodeInsufficientScope

// methodCallTool is the request the scopes gate.
const methodCallTool = "tools/call"

// principalKey is the TokenInfo.Extra entry the transport stores the verified
// guard.Principal under, so a request's handlers read the whole principal.
const principalKey = "principal"

// remoteAddrHeader carries the connection's remote address, which the SDK does
// not hand a request's handlers, from the HTTP transport to the audit line.
const remoteAddrHeader = "Sysml-Mcp-Remote-Addr"

// stdioRemote is the remote address of a call made over stdin and stdout.
const stdioRemote = "stdio"

// TokenInfoFor is the SDK's record of a verified bearer token for p: its
// UserID is p.ID(), which binds an MCP session to the principal that opened it.
func TokenInfoFor(p guard.Principal) *auth.TokenInfo {
	return &auth.TokenInfo{Scopes: p.Scopes, Expiration: p.Expires, UserID: p.ID(), Extra: map[string]any{principalKey: p}}
}

// principalOf is the principal a request was accepted for: the one the HTTP
// transport verified, or the local principal when it authenticated nobody.
func principalOf(req sdk.Request) guard.Principal {
	extra := req.GetExtra()
	if extra == nil || extra.TokenInfo == nil {
		return toolkit.LocalPrincipal()
	}
	if p, ok := extra.TokenInfo.Extra[principalKey].(guard.Principal); ok {
		return p
	}
	// A token verified by other code: the SDK's record is all there is to go by.
	return guard.Principal{Subject: extra.TokenInfo.UserID, Scopes: extra.TokenInfo.Scopes, Expires: extra.TokenInfo.Expiration, Source: "bearer"}
}

// remoteOf is where a request came from: the address the HTTP transport
// stamped, or stdio when there was no HTTP request.
func remoteOf(req sdk.Request) string {
	extra := req.GetExtra()
	if extra == nil || extra.Header == nil {
		return stdioRemote
	}
	return extra.Header.Get(remoteAddrHeader)
}

// stampRemoteAddr records each request's remote address where the SDK hands it
// to the request's handlers, over anything the client sent under that name.
func stampRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set(remoteAddrHeader, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// authorize is the receiving middleware: handlers see the caller's principal in
// ctx, a tools/call without the tool's scope is refused, and every tools/call is logged.
func (s *Server) authorize(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		p := principalOf(req)
		ctx = guard.WithPrincipal(ctx, p)
		if method != methodCallTool {
			return next(ctx, method, req)
		}
		params, _ := req.GetParams().(*sdk.CallToolParamsRaw)
		var tool string
		if params != nil {
			tool = params.Name
		}
		// A tool this server does not serve is the SDK's to answer (unknown tool).
		entry, served := s.served[tool]
		start := time.Now()
		var (
			res     sdk.Result
			err     error
			outcome = "ok"
		)
		switch {
		case served && !toolkit.HasScope(p, entry.Scope):
			err = toolkit.InsufficientScope(tool, entry.Scope)
		default:
			res, err = next(ctx, method, req)
		}
		scope := entry.Scope
		if refused, ok := toolkit.RefusedScope(err); ok {
			// A handler refuses too, for a command needing more than its tool's floor.
			outcome, scope = "refused", refused
		} else if err != nil || failed(res) {
			outcome = "error"
		}
		logger := s.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.LogAttrs(ctx, slog.LevelInfo, "tools/call",
			slog.String("principal", p.Subject),
			slog.String("source", p.Source),
			slog.String("remote", remoteOf(req)),
			slog.String("tool", tool),
			slog.String("scope", string(scope)),
			slog.String("outcome", outcome),
			slog.Duration("duration", time.Since(start)),
		)
		return res, err
	}
}

// failed reports a tools/call the tool answered with an error result.
func failed(res sdk.Result) bool {
	r, ok := res.(*sdk.CallToolResult)
	return ok && r != nil && r.IsError
}
