package oidc_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// issuer is an in-process OpenID provider: discovery, a JWKS of one RSA key,
// and an introspection endpoint whose answers the test chooses.
type issuer struct {
	*httptest.Server
	key        *rsa.PrivateKey
	kid        string
	ecKey      *ecdsa.PrivateKey
	jwksHits   atomic.Int32
	introspect func(w http.ResponseWriter, r *http.Request)
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	is := &issuer{key: key, kid: "k1", ecKey: ecKey}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 is.URL,
			"authorization_endpoint": is.URL + "/authorize",
			"token_endpoint":         is.URL + "/token",
			"jwks_uri":               is.URL + "/keys",
			"introspection_endpoint": is.URL + "/introspect",
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		is.jwksHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk(&is.key.PublicKey, is.kid), ecJWK(t, &is.ecKey.PublicKey)}})
	})
	mux.HandleFunc("/introspect", func(w http.ResponseWriter, r *http.Request) {
		if is.introspect == nil {
			http.Error(w, "no introspection", http.StatusNotFound)
			return
		}
		is.introspect(w, r)
	})
	is.Server = httptest.NewServer(mux)
	t.Cleanup(is.Close)
	return is
}

func jwk(pub *rsa.PublicKey, kid string) map[string]any {
	return map[string]any{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func ecJWK(t *testing.T, pub *ecdsa.PublicKey) map[string]any {
	t.Helper()
	point, err := pub.Bytes() // uncompressed: 0x04, x, y (SEC 1 § 2.3.3)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"kty": "EC", "use": "sig", "alg": "ES256", "crv": "P-256", "kid": "ec1",
		"x": base64.RawURLEncoding.EncodeToString(point[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(point[33:65]),
	}
}

// signES256 is the ES256 compact JWS of c under the issuer's EC key (RFC 7518 § 3.4).
func (is *issuer) signES256(t *testing.T, c claims) string {
	t.Helper()
	signingInput := signingInput(t, "ES256", "ec1", c)
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, is.ecKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// rotate replaces the signing key, as a provider does; tokens signed before
// name the old kid.
func (is *issuer) rotate(t *testing.T) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	is.key, is.kid = key, is.kid+"'"
}

// claims is the payload of a token; nil values are omitted.
type claims map[string]any

// standard is a token for aud at the issuer, valid for an hour from now.
func (is *issuer) standard(aud string) claims {
	now := time.Now()
	return claims{
		"iss": is.URL, "aud": aud, "sub": "alice", "preferred_username": "alice@example.org",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

func (c claims) with(k string, v any) claims {
	out := claims{}
	for key, val := range c {
		out[key] = val
	}
	if v == nil {
		delete(out, k)
	} else {
		out[k] = v
	}
	return out
}

// sign is the RS256 compact JWS of c under key (RFC 7515 § 7.1, RFC 7518 § 3.3).
func sign(t *testing.T, key *rsa.PrivateKey, kid string, c claims) string {
	t.Helper()
	signingInput := signingInput(t, "RS256", kid, c)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// signingInput is the header and payload of a JWS (RFC 7515 § 5.1).
func signingInput(t *testing.T, alg, kid string, c claims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": alg, "typ": "JWT", "kid": kid})
	payload, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
}

func (is *issuer) token(t *testing.T, c claims) string {
	t.Helper()
	return sign(t, is.key, is.kid, c)
}

func bearer(token string) string { return "Bearer " + token }

// answer serves one RFC 7662 answer, counting the requests and checking the
// client's credential (RFC 6749 § 2.3.1).
func (is *issuer) answer(t *testing.T, hits *atomic.Int32, status int, body map[string]any) {
	t.Helper()
	is.introspect = func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if user, pass, ok := r.BasicAuth(); !ok || user != "sysml-mcp" || pass != "s3cret%2F" {
			t.Errorf("introspection credential = %q %q %v", user, pass, ok)
		}
		if err := r.ParseForm(); err != nil || r.PostForm.Get("token") == "" || r.PostForm.Get("token_type_hint") != "access_token" {
			t.Errorf("introspection form = %v (%v)", r.PostForm, err)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			t.Errorf("introspection Content-Type = %q", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// zero reports whether p is the zero Principal a refusal carries.
func zero(p guard.Principal) bool {
	return p.Subject == "" && p.Name == "" && p.Scopes == nil && p.Expires.IsZero() && p.Source == ""
}
