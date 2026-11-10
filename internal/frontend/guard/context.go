package guard

import "context"

// principalKey is the context key the accepted Principal of a request is held under.
type principalKey struct{}

// WithPrincipal is ctx with principal as the one its request was accepted for,
// so a handler may read who it serves without parsing the credential again.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// PrincipalFrom is the Principal a request was accepted for, or false on a
// context whose request was admitted without a credential.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}
