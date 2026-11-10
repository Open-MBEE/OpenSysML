// Package flags declares the command-line flags that configure a service's
// authenticators once, so every command reads the same ones and resolves them alike.
package flags

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/apikey"
)

// Options is what the authenticator flags resolve to; the zero value configures
// no authenticator.
type Options struct {
	// TokenFile is -token-file, the file holding the static bearer token.
	TokenFile string
	// Token is -token, the static bearer token itself.
	Token string
	// APIKeys is -api-keys, the file of hashed API key records.
	APIKeys string
	// Logger hears the API key store's reloads; nil is the default logger.
	Logger *slog.Logger
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
	fs.StringVar(&o.APIKeys, "api-keys", "",
		"File of API keys accepted besides the bearer token, one JSON record per line holding the SHA-256 of a key "+
			"with its subject, scopes, expiry and whether it is disabled; re-read when it changes and on SIGHUP")
	return &o
}

// Given names the first authenticator flag given, so a transport they are not
// for can refuse it by name; empty when none is.
func (o *Options) Given() string {
	for _, f := range []struct{ name, value string }{{"-token", o.Token}, {"-token-file", o.TokenFile}, {"-api-keys", o.APIKeys}} {
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
	if o.APIKeys != "" {
		store, err := apikey.Open(o.APIKeys, o.Logger)
		if err != nil {
			return Authenticators{}, fmt.Errorf("-api-keys: %w", err)
		}
		a.add(store, "api keys")
		a.reloaders = append(a.reloaders, store)
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

// Authenticators is the chain the flags resolve to and what a command says
// about it: nothing is configured when the chain is empty.
type Authenticators struct {
	chain     guard.Chain
	kinds     []string
	tokenText string
	reloaders []reloader
}

// reloader is an authenticator that re-reads its credentials on demand, as the
// API key store does on SIGHUP.
type reloader interface{ Reload() error }

// add appends an authenticator and the words a startup line names it by.
func (a *Authenticators) add(authenticator guard.Authenticator, kind string) {
	a.chain = append(a.chain, authenticator)
	a.kinds = append(a.kinds, kind)
}

// Chain is the authenticators in the order a request is asked of them; nil
// when none is configured, which a service serves open where it may.
func (a Authenticators) Chain() guard.Chain { return a.chain }

// Reload has every authenticator that reads a file read it again, as on SIGHUP;
// the errors are joined, each file that fails keeping what it held.
func (a Authenticators) Reload() error {
	var errs []error
	for _, r := range a.reloaders {
		errs = append(errs, r.Reload())
	}
	return errors.Join(errs...)
}

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
