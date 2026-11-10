package toolkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/catalog"
)

// LocalSource is the Source of the principal a transport authenticates nobody
// for; no authenticator uses it, so it collides with no credential's.
const LocalSource = "local"

// LocalSubject is the Subject of that principal.
const LocalSubject = "local"

// LocalPrincipal is the one caller of a transport that authenticates nobody:
// the host over stdio, or a loopback -http client with no token. It holds every scope.
func LocalPrincipal() guard.Principal {
	return guard.Principal{Subject: LocalSubject, Name: "local caller", Scopes: catalog.ScopeNames(), Source: LocalSource}
}

// Caller is the principal a tool call is made by: the one the server put in
// ctx (guard.WithPrincipal), or the local principal when the transport named none.
func Caller(ctx context.Context) guard.Principal {
	if p, ok := guard.PrincipalFrom(ctx); ok {
		return p
	}
	return LocalPrincipal()
}

// HasScope reports whether p holds scope (RFC 6749 § 3.3: scopes are
// case-sensitive strings).
func HasScope(p guard.Principal, scope catalog.Scope) bool {
	return slices.Contains(p.Scopes, string(scope))
}

// CodeInsufficientScope refuses a tools/call by a principal without the tool's
// scope: implementation-defined (JSON-RPC 2.0 § 5.1), after RFC 6750 § 3.1's insufficient_scope.
const CodeInsufficientScope = -32030

// scopeRefusal is the data of the refusal: the tool and the scope it needs.
type scopeRefusal struct {
	Tool  string `json:"tool"`
	Scope string `json:"scope"`
}

// InsufficientScope is the refusal of a call of tool by a principal without
// scope: a protocol error naming both in its message and its data.
func InsufficientScope(tool string, scope catalog.Scope) *jsonrpc.Error {
	data, err := json.Marshal(scopeRefusal{Tool: tool, Scope: string(scope)})
	if err != nil {
		data = nil
	}
	return &jsonrpc.Error{
		Code:    CodeInsufficientScope,
		Message: fmt.Sprintf("insufficient_scope: %s requires the scope %s, which this credential does not grant", tool, scope),
		Data:    data,
	}
}

// RefusedScope answers the scope a refusal names, and whether err is one.
func RefusedScope(err error) (catalog.Scope, bool) {
	var jerr *jsonrpc.Error
	if !errors.As(err, &jerr) || jerr.Code != CodeInsufficientScope {
		return "", false
	}
	var data scopeRefusal
	if json.Unmarshal(jerr.Data, &data) != nil {
		return "", true
	}
	return catalog.Scope(data.Scope), true
}

// RequireScope is nil when the caller of ctx holds scope, else the refusal of tool.
func RequireScope(ctx context.Context, tool string, scope catalog.Scope) error {
	if HasScope(Caller(ctx), scope) {
		return nil
	}
	return InsufficientScope(tool, scope)
}
