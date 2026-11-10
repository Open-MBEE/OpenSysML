package toolkit

import (
	"context"
	"slices"

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
