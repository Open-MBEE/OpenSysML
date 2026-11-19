package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/apikey"
)

// TestMintAndRevokeAPIKey: -mint-api-key prints the key once to stdout and
// appends its hash to the file, -revoke-api-key disables it, and nothing else is written.
func TestMintAndRevokeAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	var stdout, stderr bytes.Buffer
	if status := run([]string{"-mint-api-key", "-api-keys", path, "-subject", "alice@example.org", "-scopes", "sysml:read, sysml:write", "-expires", "720h"}, &stdout, &stderr); status != exitServed {
		t.Fatalf("mint: exit status %d\n%s", status, stderr.String())
	}
	key := strings.TrimSuffix(stdout.String(), "\n")
	if strings.Contains(key, "\n") || !strings.HasPrefix(key, apikey.Prefix) {
		t.Fatalf("stdout is not the key alone:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "sysml-mcp: key ") || !strings.Contains(stderr.String(), "for alice@example.org (expires ") || !strings.Contains(stderr.String(), path) {
		t.Errorf("stderr does not note the key's id, subject, lifetime and file:\n%s", stderr.String())
	}
	_, secret, _ := apikey.Split(key)
	if strings.Contains(stderr.String(), secret) {
		t.Errorf("stderr repeats the secret:\n%s", stderr.String())
	}
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(file), secret) || !strings.Contains(string(file), apikey.Hash(secret)) {
		t.Errorf("the file holds the secret or not its hash:\n%s", file)
	}
	records, err := apikey.ReadFile(path)
	if err != nil || len(records) != 1 {
		t.Fatalf("ReadFile: %v, %v", records, err)
	}
	r := records[0]
	if r.Subject != "alice@example.org" || strings.Join(r.Scopes, " ") != "sysml:read sysml:write" || r.Disabled {
		t.Errorf("the record is %+v", r)
	}
	if left := time.Until(r.Expires); left < 719*time.Hour || left > 721*time.Hour {
		t.Errorf("the record expires %v, want in 720h", r.Expires)
	}
	store, err := apikey.Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, refusal := store.Verify(context.Background(), "Bearer "+key, nil)
	if refusal != guard.Accepted || p.Subject != "alice@example.org" || p.Source != apikey.Source {
		t.Errorf("the minted key is %v, %+v", refusal, p)
	}

	stdout.Reset()
	stderr.Reset()
	if status := run([]string{"-revoke-api-key", r.ID, "-api-keys", path}, &stdout, &stderr); status != exitServed {
		t.Fatalf("revoke: exit status %d\n%s", status, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("revoke wrote to stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "sysml-mcp: key "+r.ID+" disabled in "+path) {
		t.Errorf("revoke's note:\n%s", stderr.String())
	}
	if err := store.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Mismatch {
		t.Errorf("the revoked key is %v, want a mismatch", refusal)
	}

	stderr.Reset()
	if status := run([]string{"-revoke-api-key", r.ID, "-api-keys", path}, &stdout, &stderr); status != exitUnservable {
		t.Errorf("a second revoke: exit status %d", status)
	}
	if !strings.Contains(stderr.String(), "sysml-mcp: -revoke-api-key: the key "+r.ID+" is disabled already") || strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("a second revoke is reported without the usage:\n%s", stderr.String())
	}

	// A key without a lifetime does not expire.
	stdout.Reset()
	if status := run([]string{"-mint-api-key", "-api-keys", path, "-subject", "ci", "-scopes", "sysml:read"}, &stdout, &stderr); status != exitServed {
		t.Fatalf("second mint: exit status %d\n%s", status, stderr.String())
	}
	if records, _ := apikey.ReadFile(path); len(records) != 2 || !records[1].Expires.IsZero() || records[1].Subject != "ci" {
		t.Errorf("after the second mint the file holds %+v", records)
	}
	if !strings.Contains(stderr.String(), "for ci (does not expire)") {
		t.Errorf("the second mint's note:\n%s", stderr.String())
	}
}

// TestAPIKeyFlagsMisused: each misuse of the key flags is a usage error with
// a message naming the flag at fault, and an error of the file is reported alone.
func TestAPIKeyFlagsMisused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keys.jsonl")
	if err := os.WriteFile(filepath.Join(dir, "bad.jsonl"), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		args   []string
		usage  bool
		stderr string
	}{
		{"-api-keys alone", []string{"-api-keys", path}, true, "-api-keys applies to -http, -mint-api-key and -revoke-api-key; give one"},
		{"-subject alone", []string{"-subject", "alice"}, true, "-subject applies to -mint-api-key; give it"},
		{"-scopes alone", []string{"-scopes", "a"}, true, "-scopes applies to -mint-api-key; give it"},
		{"-expires alone", []string{"-expires", "1h"}, true, "-expires applies to -mint-api-key; give it"},
		{"-subject with a revoke", []string{"-revoke-api-key", "x", "-api-keys", path, "-subject", "alice"}, true, "-subject applies to -mint-api-key, not -revoke-api-key"},
		{"both commands", []string{"-mint-api-key", "-revoke-api-key", "x", "-api-keys", path}, true, "-mint-api-key and -revoke-api-key ask for different things; give one"},
		{"an empty revoke id", []string{"-revoke-api-key", ""}, true, "-revoke-api-key needs the id of the key to disable; an empty id names none"},
		{"an empty revoke id with the file", []string{"-revoke-api-key", "", "-api-keys", path}, true, "-revoke-api-key needs the id of the key to disable; an empty id names none"},
		{"an empty revoke id with a mint", []string{"-mint-api-key", "-revoke-api-key", "", "-api-keys", path, "-subject", "a", "-scopes", "b"}, true, "-revoke-api-key needs the id of the key to disable; an empty id names none"},
		{"a mint with -http", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "b", "-http", "8765"}, true, "-mint-api-key edits the key file and exits; it is not given with -http"},
		{"a revoke with -stdio", []string{"-revoke-api-key", "x", "-api-keys", path, "-stdio"}, true, "-revoke-api-key edits the key file and exits; it is not given with -stdio"},
		{"a mint with -print-config", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "b", "-print-config", "cursor"}, true, "-mint-api-key edits the key file and exits; it is not given with -print-config"},
		{"a mint with -token", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "b", "-token", "t"}, true, "-mint-api-key edits the key file and exits; it is not given with -token"},
		{"a revoke with -token-file", []string{"-revoke-api-key", "x", "-api-keys", path, "-token-file", path}, true, "-revoke-api-key edits the key file and exits; it is not given with -token-file"},
		{"a mint without the file", []string{"-mint-api-key", "-subject", "a", "-scopes", "b"}, true, "-mint-api-key needs the key file; name it with -api-keys"},
		{"a revoke without the file", []string{"-revoke-api-key", "x"}, true, "-revoke-api-key needs the key file; name it with -api-keys"},
		{"a mint without a subject", []string{"-mint-api-key", "-api-keys", path, "-scopes", "b"}, true, "-mint-api-key needs the key's subject; name it with -subject"},
		{"a mint without scopes", []string{"-mint-api-key", "-api-keys", path, "-subject", "a"}, true, "-mint-api-key needs the key's scopes; name them with -scopes"},
		{"a mint with blank scopes", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", " , "}, true, "-mint-api-key needs the key's scopes; name them with -scopes"},
		{"a scope with a space", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "read all"}, true, `-scopes: "read all" is not a scope token`},
		{"an empty scope in the list", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "read,,write"}, true, `-scopes: "" is not a scope token`},
		{"a negative lifetime", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "b", "-expires", "-1h"}, true, "-expires: -1h0m0s is not a lifetime"},
		{"a lifetime that is not one", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "b", "-expires", "soon"}, true, `invalid value "soon" for flag -expires`},
		{"an argument after the command", []string{"-mint-api-key", "-api-keys", path, "-subject", "a", "-scopes", "b", "extra"}, true, `unexpected argument "extra" after a key command`},
		{"a revoke of an unknown id", []string{"-revoke-api-key", "nosuch", "-api-keys", filepath.Join(dir, "bad.jsonl")}, false, "sysml-mcp: -revoke-api-key: " + filepath.Join(dir, "bad.jsonl") + ":1: not a key record"},
		{"a revoke over a missing file", []string{"-revoke-api-key", "nosuch", "-api-keys", filepath.Join(dir, "missing.jsonl")}, false, "sysml-mcp: -revoke-api-key: open " + filepath.Join(dir, "missing.jsonl")},
		{"a mint over a bad file", []string{"-mint-api-key", "-api-keys", filepath.Join(dir, "bad.jsonl"), "-subject", "a", "-scopes", "b"}, false, "sysml-mcp: -mint-api-key: " + filepath.Join(dir, "bad.jsonl") + ":1: not a key record"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := run(tc.args, &stdout, &stderr); status != exitUnservable {
				t.Errorf("exit status = %d, want %d\n%s", status, exitUnservable, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout is not empty:\n%s", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.stderr) {
				t.Errorf("stderr is missing %q:\n%s", tc.stderr, stderr.String())
			}
			if usage := strings.Contains(stderr.String(), "Usage: sysml-mcp [options]"); usage != tc.usage {
				t.Errorf("the usage is printed: %v, want %v\n%s", usage, tc.usage, stderr.String())
			}
		})
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a refused mint created the file: %v", err)
	}
}

// failingWriter fails every write, as a closed or full stdout does.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }

// TestMintWithoutStdout: a key that could not be written to stdout is not
// left good in the file, since nobody has it; the failure is reported with exit status 2.
func TestMintWithoutStdout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	var stderr bytes.Buffer
	if status := run([]string{"-mint-api-key", "-api-keys", path, "-subject", "alice", "-scopes", "sysml:read"}, failingWriter{}, &stderr); status != exitUnservable {
		t.Errorf("exit status %d, want %d\n%s", status, exitUnservable, stderr.String())
	}
	if !strings.Contains(stderr.String(), "sysml-mcp: -mint-api-key: the key was not written to stdout, so its record ") || !strings.Contains(stderr.String(), "no space left on device") {
		t.Errorf("stderr:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "appended to") {
		t.Errorf("stderr reports the mint as done:\n%s", stderr.String())
	}
	records, err := apikey.ReadFile(path)
	if err != nil || len(records) != 1 || !records[0].Disabled {
		t.Errorf("the file holds %+v, %v; want the one record disabled", records, err)
	}
}
