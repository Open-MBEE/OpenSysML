package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// Client is the credential the resource introspects tokens with (RFC 7662
// § 2.1): a confidential client of the provider.
type Client struct {
	ID     string `json:"client_id"`
	Secret string `json:"client_secret"`
}

// ReadClientFile reads a Client from the JSON file at path, an object with
// client_id and client_secret.
func ReadClientFile(path string) (Client, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the operator names the file
	if err != nil {
		return Client{}, err
	}
	var c Client
	if err := json.Unmarshal(data, &c); err != nil {
		return Client{}, fmt.Errorf("%s: %w", path, err)
	}
	if c.ID == "" || c.Secret == "" {
		return Client{}, fmt.Errorf("%s: client_id and client_secret are both needed", path)
	}
	return c, nil
}

// introspector asks the provider about tokens that are not JWTs (RFC 7662) and
// keeps each active answer until the token expires.
type introspector struct {
	cfg     Config
	mu      sync.Mutex
	cache   map[[sha256.Size]byte]guard.Principal
	sweepAt int
}

// sweepFloor is the cache size below which expired entries are only dropped
// when their token is presented again.
const sweepFloor = 64

func newIntrospector(cfg Config) *introspector {
	return &introspector{cfg: cfg, cache: map[[sha256.Size]byte]guard.Principal{}, sweepAt: sweepFloor}
}

// introspection is the provider's answer (RFC 7662 § 2.2).
type introspection struct {
	Active   bool            `json:"active"`
	Scope    string          `json:"scope"`
	Subject  string          `json:"sub"`
	Username string          `json:"username"`
	Expires  jsonTime        `json:"exp"`
	Issuer   string          `json:"iss"`
	Audience json.RawMessage `json:"aud"`
}

func (i *introspector) verify(ctx context.Context, token string) (guard.Principal, guard.Refusal) {
	key := sha256.Sum256([]byte(token))
	now := i.cfg.Now()
	i.mu.Lock()
	p, cached := i.cache[key]
	if cached && !p.Expires.After(now) {
		delete(i.cache, key)
		cached = false
	}
	i.mu.Unlock()
	if cached {
		return p, guard.Accepted
	}
	answer, err := i.ask(ctx, token)
	if err != nil {
		return guard.Principal{}, guard.Unavailable
	}
	// A token the provider does not know is not its: the next authenticator may know it.
	if !answer.Active {
		return guard.Principal{}, guard.NothingPresented
	}
	if answer.Issuer != "" && answer.Issuer != i.cfg.Issuer {
		return guard.Principal{}, guard.Mismatch
	}
	// The token must have been issued for this resource (RFC 8707 § 2).
	if !slices.Contains(scopeList(answer.Audience), i.cfg.Audience) {
		return guard.Principal{}, guard.Mismatch
	}
	if answer.Subject == "" {
		return guard.Principal{}, guard.Mismatch
	}
	expires := answer.Expires.time()
	if !expires.IsZero() {
		if expires = expires.Add(i.cfg.ClockSkew); expires.Before(now) {
			return guard.Principal{}, guard.Expired
		}
	}
	p = guard.Principal{
		Subject: answer.Subject,
		Name:    firstOf(answer.Username, answer.Subject),
		Scopes:  dedupe(strings.Fields(answer.Scope)),
		Expires: expires,
		Source:  Source,
	}
	// An answer with no exp is not kept: nothing would ever let it go.
	if !expires.IsZero() {
		i.keep(key, p, now)
	}
	return p, guard.Accepted
}

// keep caches p; each time the cache has doubled since the last sweep, the
// entries whose tokens expired are dropped, so it holds only live tokens.
func (i *introspector) keep(key [sha256.Size]byte, p guard.Principal, now time.Time) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cache[key] = p
	if len(i.cache) < i.sweepAt {
		return
	}
	for k, cached := range i.cache {
		if !cached.Expires.After(now) {
			delete(i.cache, k)
		}
	}
	i.sweepAt = max(2*len(i.cache), sweepFloor)
}

// ask posts the token to the introspection endpoint as the client (RFC 7662
// § 2.1, RFC 6749 § 2.3.1) and decodes the answer.
func (i *introspector) ask(ctx context.Context, token string) (introspection, error) {
	form := url.Values{"token": {token}, "token_type_hint": {"access_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, i.cfg.Introspect, strings.NewReader(form.Encode()))
	if err != nil {
		return introspection{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(url.QueryEscape(i.cfg.Client.ID), url.QueryEscape(i.cfg.Client.Secret))
	resp, err := i.cfg.HTTPClient.Do(req)
	if err != nil {
		return introspection{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return introspection{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return introspection{}, fmt.Errorf("introspection answered %s", resp.Status)
	}
	var answer introspection
	if err := json.Unmarshal(body, &answer); err != nil {
		return introspection{}, errors.New("introspection answered no JSON object")
	}
	return answer, nil
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
