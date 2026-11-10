package oidc

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"
)

// claims are the registered and profile claims a principal is built from (RFC
// 9068 § 2.2, OpenID Connect Core 1.0 § 5.1); the rest stay raw for the scope
// claim, which is named by configuration.
type claims struct {
	Subject           string   `json:"sub"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	NotBefore         jsonTime `json:"nbf"`
	raw               map[string]json.RawMessage
}

func (c *claims) UnmarshalJSON(data []byte) error {
	type plain claims
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*c = claims(p)
	return json.Unmarshal(data, &c.raw)
}

// name is the display name: preferred_username, else email, else name, else
// the subject.
func (c claims) name() string {
	for _, s := range []string{c.PreferredUsername, c.Email, c.Name} {
		if s != "" {
			return s
		}
	}
	return c.Subject
}

// scopes are what the token grants: the configured scope claim (or scope,
// then scp), plus what its roles map to, each once in order of appearance.
func (a *Authenticator) scopes(c claims) []string {
	var scopes []string
	if a.cfg.ScopeClaim != "" {
		scopes = scopeList(c.raw[a.cfg.ScopeClaim])
	} else if scopes = scopeList(c.raw["scope"]); scopes == nil {
		scopes = scopeList(c.raw["scp"])
	}
	for _, role := range scopeList(c.raw["roles"]) {
		scopes = append(scopes, a.cfg.Roles[role]...)
	}
	return dedupe(scopes)
}

// scopeList reads a claim that is a space-separated string (RFC 6749 § 3.3) or
// an array of strings; anything else holds no scopes.
func scopeList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.Fields(s)
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	return nil
}

func dedupe(scopes []string) []string {
	var out []string
	for _, s := range scopes {
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// jsonTime is a NumericDate (RFC 7519 § 2): seconds since the epoch, as a
// number; zero when the claim is absent.
type jsonTime struct {
	seconds float64
	set     bool
}

func (t *jsonTime) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "null" {
		return nil
	}
	seconds, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*t = jsonTime{seconds: seconds, set: true}
	return nil
}

func (t jsonTime) time() time.Time {
	if !t.set {
		return time.Time{}
	}
	sec, frac := int64(t.seconds), t.seconds-float64(int64(t.seconds))
	return time.Unix(sec, int64(frac*float64(time.Second)))
}
