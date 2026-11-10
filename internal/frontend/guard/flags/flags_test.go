package flags

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/apikey"
)

const env = "OPENSYSML_TEST_TOKEN"

// parse registers the flags on a fresh set and parses args.
func parse(t *testing.T, args ...string) *Options {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	o := Register(fs, env)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return o
}

// The flags are declared with the environment variable in their help, and
// the first one given is named for a transport that refuses them.
func TestRegisterAndGiven(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	Register(fs, env)
	for _, name := range []string{"token", "token-file", "api-keys", "oidc-issuer", "oidc-audience", "oidc-scope-claim", "oidc-introspect", "oidc-client-file"} {
		if fs.Lookup(name) == nil {
			t.Errorf("-%s is not declared", name)
		}
	}
	if usage := fs.Lookup("token").Usage; !strings.Contains(usage, env) {
		t.Errorf("-token's help does not name %s: %q", env, usage)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"-token", "abc"}, "-token"},
		{[]string{"-token-file", "x"}, "-token-file"},
		{[]string{"-token-file", "x", "-token", "abc"}, "-token"},
		{[]string{"-api-keys", "keys.jsonl"}, "-api-keys"},
	} {
		if got := parse(t, test.args...).Given(); got != test.want {
			t.Errorf("Given(%q) = %q, want %q", test.args, got, test.want)
		}
	}
}

// Nothing configured resolves to an empty chain a service may serve open.
func TestResolveNothing(t *testing.T) {
	t.Setenv(env, "")
	a, err := parse(t).Resolve(env)
	if err != nil {
		t.Fatal(err)
	}
	if a.Configured() || a.Chain() != nil || a.TokenText() != "" {
		t.Errorf("nothing given resolved to %+v", a)
	}
	if a.String() != "none" {
		t.Errorf("String() = %q, want none", a.String())
	}
}

// A static token, from the flag, the file or the environment, is a chain of
// one that accepts it with the scopes given and refuses another token.
func TestResolveStaticToken(t *testing.T) {
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		env   string
		args  []string
		token string
	}{
		{"flag", "", []string{"-token", "from-flag"}, "from-flag"},
		{"file", "", []string{"-token-file", file}, "from-file"},
		{"environment", "from-env", nil, "from-env"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(env, test.env)
			a, err := parse(t, test.args...).Resolve(env, "sysml:read", "sysml:write")
			if err != nil {
				t.Fatal(err)
			}
			if !a.Configured() || len(a.Chain()) != 1 || a.String() != "bearer token" || a.TokenText() != test.token {
				t.Fatalf("resolved to %+v (%s), want one static token %q", a, a, test.token)
			}
			principal, refusal := a.Chain().Verify(context.Background(), "Bearer "+test.token, nil)
			if refusal != guard.Accepted {
				t.Fatalf("the token was refused: %v", refusal)
			}
			if principal.Subject != guard.StaticTokenSubject || principal.Source != guard.SourceStaticToken || strings.Join(principal.Scopes, " ") != "sysml:read sysml:write" {
				t.Errorf("principal = %+v", principal)
			}
			if _, refusal := a.Chain().Verify(context.Background(), "Bearer other", nil); refusal != guard.Mismatch {
				t.Errorf("another token: %v, want Mismatch", refusal)
			}
			if _, refusal := a.Chain().Verify(context.Background(), "", nil); refusal != guard.NothingPresented {
				t.Errorf("no header: %v, want NothingPresented", refusal)
			}
		})
	}
}

// What guard.Read refuses, Resolve refuses in the same words.
func TestResolveRefusals(t *testing.T) {
	t.Setenv(env, "")
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"both flags", []string{"-token", "a", "-token-file", "x"}, "give one"},
		{"malformed", []string{"-token", "two words"}, "RFC 6750"},
		{"missing file", []string{"-token-file", filepath.Join(t.TempDir(), "none")}, "-token-file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parse(t, test.args...).Resolve(env)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v, want it to say %q", err, test.want)
			}
		})
	}
}

// An API key file ahead of the static token: a key is accepted as its subject,
// the token still as the token, and a stranger to both is a mismatch.
func TestResolveAPIKeys(t *testing.T) {
	t.Setenv(env, "")
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, _, err := apikey.Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := parse(t, "-api-keys", path, "-token", "static").Resolve(env, "sysml:read", "sysml:write")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Configured() || len(a.Chain()) != 2 || a.String() != "api keys, bearer token" || a.TokenText() != "static" {
		t.Fatalf("resolved to %+v (%s), want api keys then the bearer token", a, a)
	}
	principal, refusal := a.Chain().Verify(context.Background(), "Bearer "+key, nil)
	if refusal != guard.Accepted || principal.Subject != "alice" || principal.Source != apikey.Source || strings.Join(principal.Scopes, " ") != "sysml:read" {
		t.Errorf("the key: %v, %+v", refusal, principal)
	}
	if principal, refusal := a.Chain().Verify(context.Background(), "Bearer static", nil); refusal != guard.Accepted || principal.Source != guard.SourceStaticToken {
		t.Errorf("the token: %v, %+v", refusal, principal)
	}
	if _, refusal := a.Chain().Verify(context.Background(), "Bearer osk_nosuch.secret", nil); refusal != guard.Mismatch {
		t.Errorf("an unknown key: %v, want Mismatch", refusal)
	}
	if _, refusal := a.Chain().Verify(context.Background(), "Bearer other", nil); refusal != guard.Mismatch {
		t.Errorf("a stranger: %v, want Mismatch", refusal)
	}

	// Keys alone guard the service too, and Reload reads a revocation at once.
	alone, err := parse(t, "-api-keys", path).Resolve(env)
	if err != nil {
		t.Fatal(err)
	}
	if !alone.Configured() || len(alone.Chain()) != 1 || alone.String() != "api keys" || alone.TokenText() != "" {
		t.Fatalf("keys alone resolved to %+v (%s)", alone, alone)
	}
	id, _, _ := apikey.Split(key)
	if err := apikey.Revoke(path, id); err != nil {
		t.Fatal(err)
	}
	if err := alone.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, refusal := alone.Chain().Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Mismatch {
		t.Errorf("the revoked key after Reload: %v, want Mismatch", refusal)
	}
	if err := a.Reload(); err != nil {
		t.Errorf("Reload of the pair: %v", err)
	}
	if err := (Authenticators{}).Reload(); err != nil {
		t.Errorf("Reload with nothing to reload: %v", err)
	}
}

// A key file that does not read, or is missing, refuses to resolve naming the
// flag; one that breaks after Resolve keeps its keys and Reload says why.
func TestResolveAPIKeysRefusals(t *testing.T) {
	t.Setenv(env, "")
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(bad, []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, path, want string }{
		{"missing", filepath.Join(dir, "none.jsonl"), "-api-keys: stat "},
		{"not records", bad, "-api-keys: " + bad + ":1: not a key record"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parse(t, "-api-keys", test.path).Resolve(env)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v, want it to say %q", err, test.want)
			}
		})
	}
	path := filepath.Join(dir, "keys.jsonl")
	key, _, err := apikey.Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	o := parse(t, "-api-keys", path)
	var log strings.Builder
	o.Logger = slog.New(slog.NewTextHandler(&log, nil))
	a, err := o.Resolve(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err == nil || !strings.Contains(err.Error(), ":1: not a key record") {
		t.Errorf("Reload over a broken file: %v", err)
	}
	if _, refusal := a.Chain().Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
		t.Errorf("the key over a broken file: %v, want accepted (kept)", refusal)
	}
	_, secret, _ := apikey.Split(key)
	if strings.Contains(log.String(), secret) || strings.Contains(log.String(), apikey.Hash(secret)) {
		t.Errorf("the log carries the secret or its hash:\n%s", log.String())
	}
}

// -oidc-issuer adds the provider's authenticator ahead of the static token,
// advertising the scopes given; nothing is read from the provider to resolve.
func TestResolveOpenIDProvider(t *testing.T) {
	t.Setenv(env, "")
	o := parse(t, "-oidc-issuer", "https://sso.example.org/realms/models", "-token", "from-flag")
	o.Resource = "https://models.example.org/mcp"
	a, err := o.Resolve(env, "sysml:read", "sysml:write")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Chain()) != 2 || a.String() != "openid provider https://sso.example.org/realms/models, bearer token" {
		t.Fatalf("resolved to %+v (%s), want the provider then the token", a, a)
	}
	md := a.Chain().Metadata()
	if strings.Join(md.AuthorizationServers, " ") != "https://sso.example.org/realms/models" || strings.Join(md.ScopesSupported, " ") != "sysml:read sysml:write" {
		t.Errorf("metadata = %+v", md)
	}
	// The static token, not being a JWT, passes the provider to the token behind it.
	if principal, refusal := a.Chain().Verify(context.Background(), "Bearer from-flag", nil); refusal != guard.Accepted || principal.Source != guard.SourceStaticToken {
		t.Errorf("the static token through the chain: %+v, %v", principal, refusal)
	}
	// A provider alone guards the service too.
	o = parse(t, "-oidc-issuer", "http://localhost:8080/realms/models", "-oidc-audience", "api://models")
	if a, err := o.Resolve(env); err != nil || !a.Configured() || len(a.Chain()) != 1 || a.TokenText() != "" {
		t.Errorf("a provider alone resolved to %+v, %v", a, err)
	}
	if got := o.Given(); got != "-oidc-issuer" {
		t.Errorf("Given() = %q, want -oidc-issuer", got)
	}
}

// The -oidc-* flags are refused when they cannot name a usable provider.
func TestResolveOpenIDProviderRefusals(t *testing.T) {
	t.Setenv(env, "")
	issuer := []string{"-oidc-issuer", "https://sso.example.org/realms/models", "-oidc-audience", "api://models"}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"qualifier alone", []string{"-oidc-audience", "api://models"}, "-oidc-audience applies to single sign-on; name the provider with -oidc-issuer"},
		{"scope claim alone", []string{"-oidc-scope-claim", "scp"}, "-oidc-scope-claim applies to single sign-on"},
		{"no audience", []string{"-oidc-issuer", "https://sso.example.org/realms/models"}, "-oidc-issuer needs the audience a token is issued for"},
		{"cleartext issuer", []string{"-oidc-issuer", "http://sso.example.org/realms/models", "-oidc-audience", "api://models"}, `the issuer "http://sso.example.org/realms/models" must be https (or http at a loopback host)`},
		{"half an introspection pair", append(issuer, "-oidc-introspect", "https://sso.example.org/introspect"), "-oidc-introspect and -oidc-client-file must be supplied together"},
		{"client file alone", append(issuer, "-oidc-client-file", "client.json"), "must be supplied together"},
		{"missing client file", append(issuer, "-oidc-introspect", "https://sso.example.org/introspect", "-oidc-client-file", filepath.Join(t.TempDir(), "none.json")), "-oidc-client-file: open "},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parse(t, test.args...).Resolve(env)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v, want it to say %q", err, test.want)
			}
		})
	}
	file := filepath.Join(t.TempDir(), "client.json")
	if err := os.WriteFile(file, []byte(`{"client_id":"svc","client_secret":"s3cret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := parse(t, append(issuer, "-oidc-introspect", "https://sso.example.org/introspect", "-oidc-client-file", file)...).Resolve(env)
	if err != nil || len(a.Chain()) != 1 {
		t.Errorf("a complete introspection pair resolved to %+v, %v", a, err)
	}
}
