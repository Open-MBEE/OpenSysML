package apikey_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/apikey"
)

// TestConcurrentRevokes: revocations of different keys at once all hold, none
// undoing another by writing back a snapshot taken before it.
func TestConcurrentRevokes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	const n = 8
	ids := make([]string, n)
	for i := range ids {
		_, r := mint(t, path, "alice", []string{"sysml:read"}, time.Time{})
		ids[i] = r.ID
	}
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = apikey.Revoke(path, id)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("Revoke(%s): %v", ids[i], err)
		}
	}
	records, err := apikey.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if !r.Disabled {
			t.Errorf("the key %s is still enabled after its revocation", r.ID)
		}
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("the revocations left files behind: %v", entries)
	}
}

// TestConcurrentMints: keys minted at once are all in the file, under distinct ids.
func TestConcurrentMints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	const n = 8
	var wg sync.WaitGroup
	keys := make([]string, n)
	for i := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, _, err := apikey.Mint(path, "alice", []string{"sysml:read"}, time.Time{})
			if err != nil {
				t.Error(err)
			}
			keys[i] = key
		}()
	}
	wg.Wait()
	store, err := apikey.Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if store.Len() != n {
		t.Errorf("the file holds %d keys, want %d", store.Len(), n)
	}
	for _, key := range keys {
		if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
			t.Errorf("a key minted alongside others is %v", refusal)
		}
	}
}

// TestMintAfterAMissingLineBreak: a record appended to a file whose last line
// was left unended, as by an editor, starts a line of its own.
func TestMintAfterAMissingLineBreak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	if _, _, err := apikey.Mint(path, "alice", []string{"sysml:read"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.TrimSuffix(content, []byte("\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := apikey.Mint(path, "bob", []string{"sysml:read"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	records, err := apikey.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].Subject != "alice" || records[1].Subject != "bob" {
		t.Errorf("records %+v, want alice then bob", records)
	}
}

// TestMintTightensTheFile: a key file left readable by others is made the
// owner's alone when a key is appended.
func TestMintTightensTheFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not POSIX on Windows")
	}
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	mint(t, path, "alice", []string{"sysml:read"}, time.Time{})
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	mint(t, path, "bob", []string{"sysml:read"}, time.Time{})
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the file's mode is %v, %v; want 0600", info.Mode(), err)
	}
}

// TestShortLifetime: a key minted to expire within the second is good until
// then, its expiry being rounded up to the whole second, not down to one passed.
func TestShortLifetime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	expires := time.Now().Add(300 * time.Millisecond)
	key, record := mint(t, path, "alice", []string{"sysml:read"}, expires)
	if record.Expires.Before(expires) {
		t.Errorf("the record expires %v, before the %v asked for", record.Expires, expires)
	}
	if record.Expires.Sub(expires) >= time.Second {
		t.Errorf("the record expires %v, a second or more after the %v asked for", record.Expires, expires)
	}
	if record.Expires.Nanosecond() != 0 {
		t.Errorf("the record expires %v, not on a whole second", record.Expires)
	}
	store, err := apikey.Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
		t.Errorf("a key minted for the next moment is %v, want accepted", refusal)
	}
}

// TestReadFileRefusesTrailingContent: a line with anything after its record
// fails the file at that line, so corruption is not read as a good file.
func TestReadFileRefusesTrailingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	_, r := mint(t, path, "alice", []string{"sysml:read"}, time.Time{})
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSuffix(string(good), "\n")
	for _, trailing := range []string{" garbage", " }", " " + strings.ReplaceAll(line, r.ID, "other-id0000"), ` {"id":"x"}`, " ]"} {
		if err := os.WriteFile(path, []byte(line+trailing+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := apikey.ReadFile(path); err == nil || !strings.Contains(err.Error(), ":1: more than one record on the line") {
			t.Errorf("ReadFile over %q: %v, want the line refused", trailing, err)
		}
		if _, err := apikey.Open(path, nil); err == nil {
			t.Errorf("Open over %q: no error", trailing)
		}
	}
	if err := os.WriteFile(path, []byte(line+"   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if records, err := apikey.ReadFile(path); err != nil || len(records) != 1 {
		t.Errorf("ReadFile over a record with trailing blanks: %v, %v", records, err)
	}
}
