// Package flags declares the command-line flags that configure a service's
// authenticators once, so every command reads the same ones and resolves them alike.
package flags

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/oidc"
)

// Options is what the authenticator flags resolve to; the zero value configures
// no authenticator.
type Options struct {
	// TokenFile is -token-file, the file holding the static bearer token.
	TokenFile string
	// Token is -token, the static bearer token itself.
	Token string
	// OIDCIssuer is -oidc-issuer, the OpenID provider whose access tokens are accepted.
	OIDCIssuer string
	// OIDCAudience is -oidc-audience, what a token's aud must name; "" is Resource.
	OIDCAudience string
	// OIDCScopeClaim is -oidc-scope-claim, the claim scopes are read from.
	OIDCScopeClaim string
	// OIDCIntrospect is -oidc-introspect, the RFC 7662 endpoint for opaque tokens.
	OIDCIntrospect string
	// OIDCClientFile is -oidc-client-file, the introspection client's credential.
	OIDCClientFile string
	// Resource is the URL the command serves its protected resource at (RFC
	// 8707), the audience when -oidc-audience is not given; the command sets it.
	Resource string
}

// Register declares the authenticator flags on fs; env is the variable the
// command reads a static token from when no flag names one.
func Register(fs *flag.FlagSet, env string) *Options {
	var o Options
	fs.StringVar(&o.TokenFile, "token-file", "",
		"File holding the bearer token every call must carry as Authorization: Bearer TOKEN "+
			"(one trailing line break is dropped); with it the token stays out of the process list")
	fs.StringVar(&o.Token, "token", "",
		"The bearer token itself, for a shell whose command line is private; -token-file keeps "+
			"it out of the process list, and "+env+" is read when neither flag names one")
	fs.StringVar(&o.OIDCIssuer, "oidc-issuer", "",
		"Accept the access tokens of this OpenID provider, given as its issuer URL (https, or http on loopback): its discovery "+
			"document and signing keys are read, a token's signature, iss, aud, exp and nbf verified, and the issuer advertised "+
			"in the resource metadata so a host runs Authorization Code with PKCE against it")
	fs.StringVar(&o.OIDCAudience, "oidc-audience", "",
		"Audience (aud) a token must name to be accepted; default the URL the service is reached at")
	fs.StringVar(&o.OIDCScopeClaim, "oidc-scope-claim", "",
		"Claim a token's scopes are read from, a space-separated string or an array; default scope, then scp")
	fs.StringVar(&o.OIDCIntrospect, "oidc-introspect", "",
		"Introspection endpoint (RFC 7662) asked about a bearer token that is not a JWT, with -oidc-client-file; "+
			"an active answer is kept until the token expires")
	fs.StringVar(&o.OIDCClientFile, "oidc-client-file", "",
		"JSON file holding the client_id and client_secret of the provider's client the introspection requests are made as")
	return &o
}

// Given names the first authenticator flag given, so a transport they are not
// for can refuse it by name; empty when none is.
func (o *Options) Given() string {
	for _, f := range []struct{ name, value string }{{"-token", o.Token}, {"-token-file", o.TokenFile}, {"-oidc-issuer", o.OIDCIssuer}, {"-oidc-audience", o.OIDCAudience}, {"-oidc-scope-claim", o.OIDCScopeClaim}, {"-oidc-introspect", o.OIDCIntrospect}, {"-oidc-client-file", o.OIDCClientFile}} {
		if f.value != "" {
			return f.name
		}
	}
	return ""
}

// Resolve reads the credentials the flags (or env) name into the chain a service
// verifies through: each kind ahead of the static token, whose scopes these are.
func (o *Options) Resolve(env string, scopes ...string) (Authenticators, error) {
	var a Authenticators
	provider, err := o.provider(scopes)
	if err != nil {
		return Authenticators{}, err
	}
	if provider != nil {
		a.add(provider, "openid provider "+o.OIDCIssuer)
	}
	token, text, err := guard.Read(o.TokenFile, o.Token, env)
	if err != nil {
		return Authenticators{}, err
	}
	if token.Set() {
		a.add(guard.NewStaticToken(token, scopes...), "bearer token")
		a.tokenText = text
	}
	return a, nil
}

// provider is the authenticator of -oidc-issuer, advertising scopes; nil without
// the flag, whose qualifiers alone, half an introspection pair or no audience is refused.
func (o *Options) provider(scopes []string) (*oidc.Authenticator, error) {
	if o.OIDCIssuer == "" {
		for _, f := range []struct{ name, value string }{{"-oidc-audience", o.OIDCAudience}, {"-oidc-scope-claim", o.OIDCScopeClaim}, {"-oidc-introspect", o.OIDCIntrospect}, {"-oidc-client-file", o.OIDCClientFile}} {
			if f.value != "" {
				return nil, fmt.Errorf("%s applies to single sign-on; name the provider with -oidc-issuer", f.name)
			}
		}
		return nil, nil
	}
	cfg := oidc.Config{Issuer: o.OIDCIssuer, Audience: o.OIDCAudience, ScopeClaim: o.OIDCScopeClaim, Introspect: o.OIDCIntrospect, Scopes: scopes}
	if cfg.Audience == "" {
		if o.Resource == "" {
			return nil, errors.New("-oidc-issuer needs the audience a token is issued for: the URL the service is reached at, or -oidc-audience")
		}
		cfg.Audience = o.Resource
	}
	if (o.OIDCIntrospect == "") != (o.OIDCClientFile == "") {
		return nil, errors.New("-oidc-introspect and -oidc-client-file must be supplied together")
	}
	if o.OIDCClientFile != "" {
		client, err := oidc.ReadClientFile(o.OIDCClientFile)
		if err != nil {
			return nil, fmt.Errorf("-oidc-client-file: %w", err)
		}
		cfg.Client = client
	}
	if err := cfg.Check(); err != nil {
		return nil, fmt.Errorf("-oidc-issuer: %w", err)
	}
	return oidc.New(cfg)
}

// Authenticators is the chain the flags resolve to and what a command says
// about it: nothing is configured when the chain is empty.
type Authenticators struct {
	chain     guard.Chain
	kinds     []string
	tokenText string
}

// add appends an authenticator and the words a startup line names it by.
func (a *Authenticators) add(authenticator guard.Authenticator, kind string) {
	a.chain = append(a.chain, authenticator)
	a.kinds = append(a.kinds, kind)
}

// Chain is the authenticators in the order a request is asked of them; nil
// when none is configured, which a service serves open where it may.
func (a Authenticators) Chain() guard.Chain { return a.chain }

// Configured reports whether any authenticator guards the service.
func (a Authenticators) Configured() bool { return len(a.chain) > 0 }

// TokenText is the static token as it was given, for a command that prints
// its configuration; empty without one.
func (a Authenticators) TokenText() string { return a.tokenText }

// String names what guards the service for a startup line: the kinds
// configured, comma-separated, or none.
func (a Authenticators) String() string {
	if len(a.kinds) == 0 {
		return "none"
	}
	return strings.Join(a.kinds, ", ")
}
