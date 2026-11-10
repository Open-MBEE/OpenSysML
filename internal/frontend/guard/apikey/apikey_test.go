package apikey_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/apikey"
)

// mint issues a key into the file at path, failing the test when it cannot.
func mint(t *testing.T, path, subject string, scopes []string, expires time.Time) (string, apikey.Record) {
	t.Helper()
	key, record, err := apikey.Mint(path, subject, scopes, expires)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	return key, record
}

// open reads the file at path into a store logging to the returned buffer.
func open(t *testing.T, path string) (*apikey.Store, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	store, err := apikey.Open(path, slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return store, &log
}

func bearer(key string) string { return "Bearer " + key }

// TestMintAndVerify is the roundtrip: a minted key is accepted as its
// subject with its scopes, and the file holds a record without the secret.
func TestMintAndVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, record := mint(t, path, "alice", []string{"sysml:read", "sysml:write"}, time.Time{})
	id, secret, ok := apikey.Split(key)
	if !ok || !strings.HasPrefix(key, apikey.Prefix) || id != record.ID || len(id) != 12 || len(secret) != 43 {
		t.Fatalf("Split(%q) = %q, %q, %v; want the record's id %q, a 43-character secret", key, id, secret, ok, record.ID)
	}
	if !guard.IsB64Token(key) {
		t.Errorf("the key %q is not a b64token", key)
	}
	if record.SHA256 != apikey.Hash(secret) || record.Subject != "alice" || len(record.Scopes) != 2 || !record.Expires.IsZero() || record.Disabled {
		t.Errorf("record = %+v, want alice's hashed, open-ended, enabled record", record)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Errorf("the file holds the secret:\n%s", raw)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the file's mode is %v, %v; want 0600", info.Mode(), err)
	}

	store, log := open(t, path)
	principal, refusal := store.Verify(context.Background(), bearer(key), nil)
	if refusal != guard.Accepted {
		t.Fatalf("Verify = %v, want Accepted", refusal)
	}
	want := guard.Principal{Subject: "alice", Name: "api key " + record.ID, Scopes: []string{"sysml:read", "sysml:write"}, Source: apikey.Source}
	if principal.Subject != want.Subject || principal.Name != want.Name || strings.Join(principal.Scopes, ",") != strings.Join(want.Scopes, ",") || principal.Source != want.Source || !principal.Expires.IsZero() {
		t.Errorf("principal = %+v, want %+v", principal, want)
	}
	principal.Scopes[0] = "changed"
	if again, _ := store.Verify(context.Background(), bearer(key), nil); again.Scopes[0] != "sysml:read" {
		t.Errorf("a principal's scopes alias the store's")
	}
	if store.Len() != 1 || store.Path() != path {
		t.Errorf("Len, Path = %d, %q; want 1, %q", store.Len(), store.Path(), path)
	}
	if strings.Contains(log.String(), secret) || strings.Contains(log.String(), record.SHA256) {
		t.Errorf("the log repeats the secret or its hash:\n%s", log.String())
	}
}

// TestVerifyRefusals: a wrong secret, an unknown id, a disabled key and a key
// of the right prefix but no shape are Mismatch; an expired key Expired; a
// token that is no key is NothingPresented; a header that is no bearer token is read as such.
func TestVerifyRefusals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, _ := mint(t, path, "alice", []string{"sysml:read"}, time.Time{})
	revoked, revokedRecord := mint(t, path, "bob", []string{"sysml:read"}, time.Time{})
	if err := apikey.Revoke(path, revokedRecord.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	expired, _ := mint(t, path, "carol", []string{"sysml:read"}, time.Now().Add(-time.Minute))
	live, _ := mint(t, path, "dave", []string{"sysml:read"}, time.Now().Add(time.Hour))
	id, secret, _ := apikey.Split(key)
	store, _ := open(t, path)
	for name, tc := range map[string]struct {
		authorization string
		want          guard.Refusal
	}{
		"the key":                      {bearer(key), guard.Accepted},
		"a live key with an expiry":    {bearer(live), guard.Accepted},
		"a wrong secret":               {bearer(apikey.Prefix + id + "." + strings.Repeat("A", len(secret))), guard.Mismatch},
		"a secret one character off":   {bearer(key[:len(key)-1] + flip(key[len(key)-1])), guard.Mismatch},
		"an unknown id":                {bearer(apikey.Prefix + "unknownid000." + secret), guard.Mismatch},
		"a disabled key":               {bearer(revoked), guard.Mismatch},
		"an expired key":               {bearer(expired), guard.Expired},
		"a key with no secret":         {bearer(apikey.Prefix + id), guard.Mismatch},
		"a key with no id":             {bearer(apikey.Prefix + "." + secret), guard.Mismatch},
		"a key with two dots":          {bearer(key + ".more"), guard.Mismatch},
		"the bare prefix":              {bearer(apikey.Prefix), guard.Mismatch},
		"another kind of bearer token": {bearer("s3cret"), guard.NothingPresented},
		"a JWT":                        {bearer("eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJhbGljZSJ9.c2ln"), guard.NothingPresented},
		"no header":                    {"", guard.NothingPresented},
		"another scheme":               {"Basic " + key, guard.NotBearer},
		"a malformed bearer token":     {"Bearer not a token", guard.Malformed},
	} {
		t.Run(name, func(t *testing.T) {
			principal, refusal := store.Verify(context.Background(), tc.authorization, nil)
			if refusal != tc.want {
				t.Errorf("Verify = %v, want %v", refusal, tc.want)
			}
			if (refusal == guard.Accepted) != (principal.Subject != "") {
				t.Errorf("principal = %+v with %v", principal, refusal)
			}
		})
	}
}

// flip is another character of the base64url alphabet than b.
func flip(b byte) string {
	if b == 'A' {
		return "B"
	}
	return "A"
}

// TestRevoke: revoking marks the record disabled and keeps the others, an
// unknown or already disabled id is an error, and the file stays owner-readable.
func TestRevoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	_, a := mint(t, path, "alice", []string{"sysml:read"}, time.Time{})
	_, b := mint(t, path, "bob", []string{"sysml:read"}, time.Now().Add(time.Hour))
	if err := apikey.Revoke(path, a.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	records, err := apikey.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].ID != a.ID || !records[0].Disabled || records[1].ID != b.ID || records[1].Disabled || !records[1].Expires.Equal(b.Expires) {
		t.Errorf("after Revoke the file holds %+v", records)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the file's mode is %v, %v; want 0600", info.Mode(), err)
	}
	if err := apikey.Revoke(path, a.ID); err == nil || !strings.Contains(err.Error(), "disabled already") {
		t.Errorf("Revoke of a disabled key: %v", err)
	}
	if err := apikey.Revoke(path, "nosuch"); err == nil || !strings.Contains(err.Error(), "no key has the id nosuch") {
		t.Errorf("Revoke of an unknown id: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("Revoke left files behind: %v", entries)
	}
}

// TestReadFile: blank lines are skipped, and a line that is not a record, a
// record with a bad field or a repeated id fails the file at its line.
func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	hash := apikey.Hash("secret")
	good := `{"id":"abc","sha256":"` + hash + `","subject":"alice","scopes":["sysml:read"]}`
	records, err := apikey.ReadFile(write("good", "\n"+good+"\n\n"))
	if err != nil || len(records) != 1 || records[0].ID != "abc" {
		t.Errorf("ReadFile = %+v, %v", records, err)
	}
	if _, err := apikey.ReadFile(filepath.Join(dir, "nosuch")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadFile of a missing file: %v", err)
	}
	for name, tc := range map[string]struct{ content, want string }{
		"not json":             {good + "\nnope\n", "nosuch:2: not a key record"},
		"an array":             {"[" + good + "]\n", "nosuch:1: not a key record"},
		"an unknown field":     {`{"id":"abc","sha256":"` + hash + `","subject":"alice","scopes":["sysml:read"],"secret":"x"}`, `nosuch:1: not a key record: json: unknown field "secret"`},
		"no id":                {`{"sha256":"` + hash + `","subject":"alice","scopes":["sysml:read"]}`, "nosuch:1: the id is empty"},
		"an id with a dot":     {`{"id":"a.b","sha256":"` + hash + `","subject":"alice","scopes":["sysml:read"]}`, `nosuch:1: the id "a.b" is not`},
		"a short hash":         {`{"id":"abc","sha256":"abcd","subject":"alice","scopes":["sysml:read"]}`, `nosuch:1: the sha256 "abcd" is not 64 hex digits`},
		"no subject":           {`{"id":"abc","sha256":"` + hash + `","scopes":["sysml:read"]}`, "nosuch:1: the subject is empty"},
		"no scopes":            {`{"id":"abc","sha256":"` + hash + `","subject":"alice"}`, "nosuch:1: the scopes are empty"},
		"a scope with a space": {`{"id":"abc","sha256":"` + hash + `","subject":"alice","scopes":["sysml read"]}`, `nosuch:1: the scope "sysml read" is not a scope token`},
		"a repeated id":        {good + "\n" + good + "\n", "nosuch:2: the id abc is held by line 1 already"},
	} {
		t.Run(name, func(t *testing.T) {
			path := write("nosuch", tc.content)
			_, err := apikey.ReadFile(path)
			if err == nil || !strings.Contains(err.Error(), strings.Replace(tc.want, "nosuch", path, 1)) {
				t.Errorf("ReadFile = %v, want %q", err, tc.want)
			}
		})
	}
}

// TestOpenRefusesABadFile: a store is not opened over a file that does not read.
func TestOpenRefusesABadFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := apikey.Open(filepath.Join(dir, "nosuch"), nil); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open of a missing file: %v", err)
	}
	path := filepath.Join(dir, "bad")
	if err := os.WriteFile(path, []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := apikey.Open(path, nil); err == nil || !strings.Contains(err.Error(), ":1: not a key record") {
		t.Errorf("Open of a bad file: %v", err)
	}
}

// TestChainFallsThroughToTheStaticToken: in a chain before a static token, the
// store claims keys and leaves every other bearer token to the static token.
func TestChainFallsThroughToTheStaticToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, _ := mint(t, path, "alice", []string{"sysml:read"}, time.Time{})
	store, _ := open(t, path)
	token, err := guard.New("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	chain := guard.Chain{store, guard.NewStaticToken(token, "sysml:read", "sysml:admin")}
	for name, tc := range map[string]struct {
		authorization string
		want          guard.Refusal
		source        string
	}{
		"the key":          {bearer(key), guard.Accepted, apikey.Source},
		"the static token": {bearer("s3cret"), guard.Accepted, guard.SourceStaticToken},
		"another token":    {bearer("other"), guard.Mismatch, ""},
		"an unknown key":   {bearer(apikey.Prefix + "unknownid000.secret"), guard.Mismatch, ""},
		"no header":        {"", guard.NothingPresented, ""},
	} {
		t.Run(name, func(t *testing.T) {
			principal, refusal := chain.Verify(context.Background(), tc.authorization, nil)
			if refusal != tc.want || principal.Source != tc.source {
				t.Errorf("Verify = %+v, %v; want source %q, %v", principal, refusal, tc.source, tc.want)
			}
		})
	}
	if m := chain.Metadata(); len(m.AuthorizationServers) != 0 || len(m.ScopesSupported) != 0 {
		t.Errorf("Metadata = %+v, want nothing advertised", m)
	}
}
