// Package guard is what a service that other machines may reach shares with
// the others: who a bearer credential speaks for (RFC 6750), where the static
// token is read from, and the rule that a listener off loopback requires one.
package guard

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Token is the one access token a service accepts (RFC 6750), issued out of
// band by whoever runs it; the zero value is no token.
type Token struct {
	secret []byte
}

// New reads a token as RFC 6750 § 2.1 spells one (b64token), so a client can
// send what the server was given; anything else is refused.
func New(s string) (Token, error) {
	if s == "" {
		return Token{}, errors.New("the token is empty")
	}
	if !IsB64Token(s) {
		return Token{}, errors.New("a token is letters, digits and -._~+/ with trailing = only (RFC 6750 b64token), with no spaces or line breaks")
	}
	return Token{secret: []byte(s)}, nil
}

// Set reports whether a token is configured.
func (t Token) Set() bool { return len(t.secret) > 0 }

// Matches compares a presented token in constant time.
func (t Token) Matches(presented string) bool {
	return t.Set() && subtle.ConstantTimeCompare(t.secret, []byte(presented)) == 1
}

// IsB64Token reports whether s is a b64token of RFC 6750 § 2.1.
func IsB64Token(s string) bool {
	body := strings.TrimRight(s, "=")
	if body == "" {
		return false
	}
	for _, r := range body {
		switch {
		case 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z', '0' <= r && r <= '9':
		case r == '-', r == '.', r == '_', r == '~', r == '+', r == '/':
		default:
			return false
		}
	}
	return true
}

// Refusal is why an Authorization header was not accepted, or Accepted.
type Refusal int

const (
	// Accepted is a header carrying a credential the server accepts.
	Accepted Refusal = iota
	// NothingPresented is a request with no Authorization header; from one
	// Authenticator of a Chain, also a credential of a kind it does not read.
	NothingPresented
	// NotBearer is an Authorization header of another scheme.
	NotBearer
	// Malformed is a Bearer header whose credentials are not a b64token.
	Malformed
	// Mismatch is a well-formed bearer token that is not this server's.
	Mismatch
	// Expired is a bearer token of this server's whose lifetime has ended.
	Expired
	// InsufficientScope is a valid bearer token without a scope the server requires.
	InsufficientScope
)

// Bearer reads the bearer token of an Authorization header (RFC 6750 § 2.1):
// the b64token with Accepted, or why the header carries none.
func Bearer(authorization string) (string, Refusal) {
	scheme, credentials, _ := strings.Cut(strings.TrimSpace(authorization), " ")
	credentials = strings.TrimSpace(credentials)
	switch {
	case scheme == "":
		return "", NothingPresented
	case !strings.EqualFold(scheme, "Bearer"):
		return "", NotBearer
	case !IsB64Token(credentials):
		return "", Malformed
	}
	return credentials, Accepted
}

// Check classifies the Authorization header of a request (RFC 6750 § 2.1).
func (t Token) Check(authorization string) Refusal {
	credentials, refusal := Bearer(authorization)
	if refusal == Accepted && !t.Matches(credentials) {
		return Mismatch
	}
	return refusal
}

// HTTPStatus is the status a refusal is answered with: 400 for a header the
// server cannot read, 403 for a missing scope, 401 otherwise (RFC 6750 § 3.1).
func (r Refusal) HTTPStatus() int {
	switch r {
	case Malformed:
		return http.StatusBadRequest
	case InsufficientScope:
		return http.StatusForbidden
	}
	return http.StatusUnauthorized
}

// Code is the RFC 6750 § 3.1 error code, empty when nothing was presented.
func (r Refusal) Code() string {
	switch r {
	case NotBearer, Malformed:
		return "invalid_request"
	case Mismatch, Expired:
		return "invalid_token"
	case InsufficientScope:
		return "insufficient_scope"
	}
	return ""
}

// Description says what was wrong, empty when nothing was presented.
func (r Refusal) Description() string {
	switch r {
	case NotBearer:
		return "the Authorization scheme must be Bearer"
	case Malformed:
		return "the bearer token is malformed"
	case Mismatch:
		return "the bearer token is not this server's"
	case Expired:
		return "the bearer token has expired"
	case InsufficientScope:
		return "the bearer token lacks a scope this server requires"
	}
	return ""
}

// ChallengeParams are the error parameters of a WWW-Authenticate challenge,
// none when nothing was presented.
func (r Refusal) ChallengeParams() []string {
	if r.Code() == "" {
		return nil
	}
	return []string{fmt.Sprintf("error=%q", r.Code()), fmt.Sprintf("error_description=%q", r.Description())}
}

// Challenge is the WWW-Authenticate value for a refusal under a realm.
func (r Refusal) Challenge(realm string) string {
	return "Bearer " + strings.Join(append([]string{fmt.Sprintf("realm=%q", realm)}, r.ChallengeParams()...), ", ")
}

// Message is the refusal's text for the client.
func (r Refusal) Message() string {
	if r.Description() == "" {
		return "Unauthorized: this server requires a bearer token; send Authorization: Bearer TOKEN"
	}
	return http.StatusText(r.HTTPStatus()) + ": " + r.Description()
}

// String names the refusal for a log line; it never carries what was presented.
func (r Refusal) String() string {
	switch r {
	case Accepted:
		return "accepted"
	case NothingPresented:
		return "no token presented"
	case NotBearer:
		return "scheme is not Bearer"
	case Malformed:
		return "malformed token"
	case Mismatch:
		return "wrong token"
	case Expired:
		return "expired token"
	case InsufficientScope:
		return "insufficient scope"
	}
	return fmt.Sprintf("refusal(%d)", int(r))
}

// Read reads the token -token-file (one trailing line break dropped), -token
// or the environment variable env names, in that precedence; both flags set is
// refused, none and an empty variable is no token. The text is returned beside
// the token for a command that prints its configuration.
func Read(tokenFile, token, env string) (Token, string, error) {
	if token != "" && tokenFile != "" {
		return Token{}, "", errors.New("-token and -token-file both name the bearer token; give one")
	}
	source, value := "-token", token
	switch {
	case tokenFile != "":
		raw, err := os.ReadFile(tokenFile) // #nosec G304 -- the operator names the file with -token-file
		if err != nil {
			return Token{}, "", fmt.Errorf("-token-file: %w", err)
		}
		source, value = fmt.Sprintf("-token-file %q", tokenFile), strings.TrimSuffix(strings.TrimSuffix(string(raw), "\n"), "\r")
	case token == "":
		value = os.Getenv(env)
		if value == "" {
			return Token{}, "", nil
		}
		source = env
	}
	parsed, err := New(value)
	if err != nil {
		return Token{}, "", fmt.Errorf("%s: %w", source, err)
	}
	return parsed, value, nil
}
