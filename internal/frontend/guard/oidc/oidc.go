// Package oidc authenticates the access tokens an OpenID provider issues (RFC
// 9068, OpenID Connect Discovery 1.0) as one member of a guard.Chain.
package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// Source is the Principal.Source of a token the provider issued.
const Source = "oidc"

// DefaultClockSkew is how far a token's exp and nbf may disagree with this
// clock when Config.ClockSkew is zero.
const DefaultClockSkew = 30 * time.Second

// Config is how an Authenticator is built.
type Config struct {
	// Issuer is the provider's issuer identifier (Discovery § 3; https, or
	// loopback): <Issuer>/.well-known/openid-configuration is read at New.
	Issuer string
	// Audience is the value a token's aud must hold (RFC 9068 § 4): the MCP
	// resource URL (RFC 8707 § 2).
	Audience string
	// ScopeClaim names the claim the scopes are read from; "" reads the
	// space-separated scope claim (RFC 9068 § 2.2.3), then the scp array.
	ScopeClaim string
	// Roles maps a value of the roles claim to the scopes it grants.
	Roles map[string][]string
	// Scopes are what the resource advertises as scopes_supported.
	Scopes []string
	// ClockSkew is tolerated on exp and nbf; zero is DefaultClockSkew.
	ClockSkew time.Duration
	// Introspect is the RFC 7662 endpoint a token that is not a JWT is sent
	// to (https, or loopback); "" passes such tokens on to the next authenticator.
	Introspect string
	// Client authenticates the introspection requests (RFC 7662 § 2.1).
	Client Client
	// HTTPClient reaches the provider; nil is http.DefaultClient.
	HTTPClient *http.Client
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Authenticator verifies the provider's JWTs (and, when configured, has it
// introspect its opaque tokens); build one with New.
type Authenticator struct {
	cfg       Config
	verifier  *gooidc.IDTokenVerifier
	introspec *introspector
}

// New reads the provider's discovery document and prepares the verifier; its
// JWKS is fetched on first use and re-fetched when a token names an unknown key.
func New(ctx context.Context, cfg Config) (*Authenticator, error) {
	if cfg.Issuer == "" {
		return nil, errors.New("oidc: no issuer")
	}
	if err := secure("issuer", cfg.Issuer); err != nil {
		return nil, err
	}
	if cfg.Audience == "" {
		return nil, errors.New("oidc: no audience")
	}
	if cfg.ClockSkew < 0 {
		return nil, fmt.Errorf("oidc: negative clock skew %v", cfg.ClockSkew)
	}
	if (cfg.Introspect == "") != (cfg.Client == Client{}) {
		return nil, errors.New("oidc: introspection needs both an endpoint and a client")
	}
	if cfg.Introspect != "" {
		if err := secure("introspection endpoint", cfg.Introspect); err != nil {
			return nil, err
		}
		if cfg.Client.ID == "" || cfg.Client.Secret == "" {
			return nil, errors.New("oidc: the introspection client needs both an id and a secret")
		}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = DefaultClockSkew
	}
	// The key set outlives New's ctx: it is fetched during later requests.
	keys := gooidc.ClientContext(context.WithoutCancel(ctx), cfg.HTTPClient)
	provider, err := gooidc.NewProvider(gooidc.ClientContext(ctx, cfg.HTTPClient), cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery of %s: %w", cfg.Issuer, err)
	}
	var endpoints struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := provider.Claims(&endpoints); err != nil || endpoints.JWKSURI == "" {
		return nil, fmt.Errorf("oidc: discovery of %s names no jwks_uri", cfg.Issuer)
	}
	// The advertised scopes are those configured at New, whatever the caller does with its slice.
	cfg.Scopes = slices.Clone(cfg.Scopes)
	a := &Authenticator{cfg: cfg}
	// Expiry is checked here, with the configured skew, rather than by the library.
	a.verifier = gooidc.NewVerifier(cfg.Issuer, keySet{gooidc.NewRemoteKeySet(keys, endpoints.JWKSURI)}, &gooidc.Config{
		ClientID:             cfg.Audience,
		SupportedSigningAlgs: signingAlgorithms,
		SkipExpiryCheck:      true,
		Now:                  cfg.Now,
	})
	if cfg.Introspect != "" {
		a.introspec = newIntrospector(cfg)
	}
	return a, nil
}

// signingAlgorithms are the asymmetric JWS algorithms a token may be signed
// with (RFC 7518 § 3.1, RFC 8037 § 3.1); a shared-secret or unsigned one is refused.
var signingAlgorithms = []string{
	gooidc.RS256, gooidc.RS384, gooidc.RS512,
	gooidc.ES256, gooidc.ES384, gooidc.ES512,
	gooidc.PS256, gooidc.PS384, gooidc.PS512,
	gooidc.EdDSA,
}

// secure accepts an https URL, or an http one at a loopback host (a provider
// run beside the service for development); a secret is sent to it.
func secure(what, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("oidc: %s %q is not a URL", what, raw)
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && guard.IsLoopbackHost(u.Hostname()):
		return nil
	}
	return fmt.Errorf("oidc: %s %q must be https (or http at a loopback host)", what, raw)
}

// Verify accepts a JWT the provider signed for this audience within its validity
// (RFC 9068 § 4); a token that is not a JWT is introspected, or NothingPresented.
func (a *Authenticator) Verify(ctx context.Context, authorization string, _ *http.Request) (guard.Principal, guard.Refusal) {
	token, refusal := guard.Bearer(authorization)
	if refusal != guard.Accepted {
		return guard.Principal{}, refusal
	}
	if !isJWT(token) {
		if a.introspec == nil {
			return guard.Principal{}, guard.NothingPresented
		}
		return a.introspec.verify(ctx, token)
	}
	var outage bool
	idToken, err := a.verifier.Verify(context.WithValue(ctx, outageKey{}, &outage), token)
	switch {
	case outage:
		return guard.Principal{}, guard.Unavailable
	case err != nil:
		return guard.Principal{}, guard.Mismatch
	}
	var c claims
	if err := idToken.Claims(&c); err != nil {
		return guard.Principal{}, guard.Mismatch
	}
	// An access token carries exp (RFC 9068 § 2.2); one without never ends.
	if idToken.Expiry.IsZero() {
		return guard.Principal{}, guard.Mismatch
	}
	now := a.cfg.Now()
	if idToken.Expiry.Add(a.cfg.ClockSkew).Before(now) {
		return guard.Principal{}, guard.Expired
	}
	if nbf := c.NotBefore.time(); !nbf.IsZero() && nbf.Add(-a.cfg.ClockSkew).After(now) {
		return guard.Principal{}, guard.Mismatch
	}
	if c.Subject == "" {
		return guard.Principal{}, guard.Mismatch
	}
	return guard.Principal{
		Subject: c.Subject,
		Name:    c.name(),
		Scopes:  a.scopes(c),
		Expires: idToken.Expiry,
		Source:  Source,
	}, guard.Accepted
}

// Metadata names the issuer as the authorization server and the configured
// scopes (RFC 9728 § 2).
func (a *Authenticator) Metadata() guard.ResourceMetadata {
	return guard.ResourceMetadata{
		AuthorizationServers: []string{a.cfg.Issuer},
		ScopesSupported:      append([]string(nil), a.cfg.Scopes...),
	}
}

// isJWT reports whether token is a JWS in compact serialization (RFC 7515 § 7.1):
// three segments, the first a base64url JSON object; nothing else is parsed.
func isJWT(token string) bool {
	header, rest, ok := strings.Cut(token, ".")
	if !ok || strings.Count(rest, ".") != 1 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil
}

// outageKey carries, through the library's Verify, whether the key set could
// not be fetched: the library folds that into its signature error.
type outageKey struct{}

// keySet is the provider's JWKS; it tells a key fetch that failed (the
// provider unreachable) apart from a signature no key verifies.
type keySet struct {
	remote *gooidc.RemoteKeySet
}

func (k keySet) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	payload, err := k.remote.VerifySignature(ctx, jwt)
	if err != nil {
		var urlErr *url.Error
		if outage, ok := ctx.Value(outageKey{}).(*bool); ok && errors.As(err, &urlErr) {
			*outage = true
		}
	}
	return payload, err
}
