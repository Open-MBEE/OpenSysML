package guard

import (
	"context"
	"net/http"
	"slices"
	"time"
)

// Principal is who an accepted credential speaks for.
type Principal struct {
	// Subject identifies the principal stably and is never empty: an OIDC
	// sub, an API key's subject, StaticTokenSubject.
	Subject string
	// Name is how the principal is displayed: an e-mail address, a key's label.
	Name string
	// Scopes are the scopes the credential grants (RFC 6749 § 3.3).
	Scopes []string
	// Expires is when the credential stops being accepted; zero is never.
	Expires time.Time
	// Source names the kind of authenticator that accepted the credential and
	// holds no colon, so that ID reads one way.
	Source string
}

// ID identifies the principal across authenticators as "<source>:<subject>", a
// subject being unique only within its source; a service keys what a caller owns by it.
func (p Principal) ID() string { return p.Source + ":" + p.Subject }

// principalKey is the context key of the principal a request was accepted for.
type principalKey struct{}

// WithPrincipal is ctx carrying p, the principal a transport accepted the
// request for, to the request's handlers.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom is the principal the request of ctx was accepted for, to key what
// its caller owns by ID; there is none on a transport that authenticates nobody.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// ResourceMetadata is what an authenticator has the resource advertise in its
// protected resource metadata (RFC 9728 § 2).
type ResourceMetadata struct {
	// AuthorizationServers are the issuers of its tokens (authorization_servers).
	AuthorizationServers []string
	// ScopesSupported are the scopes a client may request (scopes_supported).
	ScopesSupported []string
}

// Authenticator decides who an Authorization header speaks for; a service
// asks the ones it is configured with through a Chain.
type Authenticator interface {
	// Verify classifies an Authorization header (RFC 6750 § 2.1) and names the
	// principal of an Accepted one; r is nil on a transport with no HTTP request.
	Verify(ctx context.Context, authorization string, r *http.Request) (Principal, Refusal)
	// Metadata is what the authenticator has the resource advertise.
	Metadata() ResourceMetadata
}

// StaticTokenSubject is the subject of the principal a static token speaks for.
const StaticTokenSubject = "token"

// SourceStaticToken is the Principal.Source of a static token.
const SourceStaticToken = "static-token"

// StaticToken authenticates the one token a service was given out of band
// (RFC 6750); it reads every bearer token, so it ends the Chain it is in.
type StaticToken struct {
	token  Token
	scopes []string
}

// NewStaticToken is the authenticator of token, whose principal holds scopes:
// every scope the service defines, the token being its operator's.
func NewStaticToken(token Token, scopes ...string) StaticToken {
	return StaticToken{token: token, scopes: slices.Clone(scopes)}
}

// Verify accepts the token as StaticTokenSubject with no expiry and refuses
// anything else as Token.Check does; an unset token accepts nothing.
func (s StaticToken) Verify(_ context.Context, authorization string, _ *http.Request) (Principal, Refusal) {
	if refusal := s.token.Check(authorization); refusal != Accepted {
		return Principal{}, refusal
	}
	return Principal{Subject: StaticTokenSubject, Name: "static token", Scopes: slices.Clone(s.scopes), Source: SourceStaticToken}, Accepted
}

// Metadata is empty: no authorization server issues a static token, and no
// scope is requested for one.
func (StaticToken) Metadata() ResourceMetadata { return ResourceMetadata{} }

// Chain asks authenticators in order and answers with the first verdict that
// is not NothingPresented, so each claims the credentials of its own kind.
type Chain []Authenticator

// Verify is the first claiming authenticator's verdict; a credential none
// claims is refused as its header reads (RFC 6750 § 3.1), a bearer token as Mismatch.
func (c Chain) Verify(ctx context.Context, authorization string, r *http.Request) (Principal, Refusal) {
	principal, refusal := c.claim(ctx, authorization, r)
	if refusal != NothingPresented {
		return principal, refusal
	}
	if _, refusal = Bearer(authorization); refusal == Accepted {
		refusal = Mismatch
	}
	return Principal{}, refusal
}

// claimer is a Chain as another holds it, by value or by pointer: asked for
// its claim, it leaves a credential none of its own claims to the next.
type claimer interface {
	claim(ctx context.Context, authorization string, r *http.Request) (Principal, Refusal)
}

// claim is the first verdict that is not NothingPresented, which is the answer
// when none claims; a chain within asks its own in turn rather than deciding.
func (c Chain) claim(ctx context.Context, authorization string, r *http.Request) (Principal, Refusal) {
	for _, a := range c {
		var principal Principal
		var refusal Refusal
		if inner, ok := a.(claimer); ok {
			principal, refusal = inner.claim(ctx, authorization, r)
		} else {
			principal, refusal = a.Verify(ctx, authorization, r)
		}
		switch refusal {
		case NothingPresented:
		case Accepted:
			return principal, Accepted
		default:
			return Principal{}, refusal
		}
	}
	return Principal{}, NothingPresented
}

// Metadata is the authenticators' metadata together, in their order and with
// each value once.
func (c Chain) Metadata() ResourceMetadata {
	var all ResourceMetadata
	for _, a := range c {
		m := a.Metadata()
		all.AuthorizationServers = appendNew(all.AuthorizationServers, m.AuthorizationServers)
		all.ScopesSupported = appendNew(all.ScopesSupported, m.ScopesSupported)
	}
	return all
}

// appendNew appends the values of more that dst does not hold yet.
func appendNew(dst, more []string) []string {
	for _, v := range more {
		if !slices.Contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}
