package apikey

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// clock is a settable time for a store, so a test moves past RefreshInterval
// without waiting.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

// touch moves the file's modification time forward, as an editor or a mint
// does, whatever the file system's resolution.
func touch(t *testing.T, path string, by time.Duration) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime().Add(by), info.ModTime().Add(by)); err != nil {
		t.Fatal(err)
	}
}

// TestReloadOnModificationTime: a key minted after the store was opened is
// accepted once the file's modification time has moved and the interval has passed.
func TestReloadOnModificationTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	first, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	store, err := Open(path, slog.New(slog.NewTextHandler(&log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.now = c.now
	store.checked = c.t

	second, _, err := Mint(path, "bob", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	touch(t, path, time.Second)
	if _, refusal := store.Verify(context.Background(), "Bearer "+second, nil); refusal != guard.Mismatch {
		t.Errorf("within the interval the new key is %v, want a mismatch: the file is not looked at", refusal)
	}
	c.advance(RefreshInterval)
	if _, refusal := store.Verify(context.Background(), "Bearer "+second, nil); refusal != guard.Accepted {
		t.Errorf("after the interval the new key is %v, want accepted", refusal)
	}
	if _, refusal := store.Verify(context.Background(), "Bearer "+first, nil); refusal != guard.Accepted {
		t.Errorf("the first key is %v after the reload, want accepted", refusal)
	}
	if !strings.Contains(log.String(), "api keys reloaded") || !strings.Contains(log.String(), "keys=2") {
		t.Errorf("the reload was not logged:\n%s", log.String())
	}
	_, secret, _ := Split(second)
	if strings.Contains(log.String(), secret) || strings.Contains(log.String(), Hash(secret)) {
		t.Errorf("the log repeats a secret or its hash:\n%s", log.String())
	}

	// A revocation written by Revoke moves the file as well.
	id, _, _ := Split(first)
	if err := Revoke(path, id); err != nil {
		t.Fatal(err)
	}
	touch(t, path, time.Second)
	c.advance(RefreshInterval)
	if _, refusal := store.Verify(context.Background(), "Bearer "+first, nil); refusal != guard.Mismatch {
		t.Errorf("the revoked key is %v after the reload, want a mismatch", refusal)
	}
	if _, refusal := store.Verify(context.Background(), "Bearer "+second, nil); refusal != guard.Accepted {
		t.Errorf("the other key is %v after the reload, want accepted", refusal)
	}
}

// TestReloadOnTheSameSizeAndTime: a file replaced by one of the same size with
// its modification time kept, as cp -p does, is still read again.
func TestReloadOnTheSameSizeAndTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	first, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.now = c.now
	store.checked = c.t

	other := filepath.Join(t.TempDir(), "keys.jsonl")
	second, _, err := Mint(other, "bobby", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(replacement)) != before.Size() {
		t.Fatalf("the replacement is %d bytes, the file %d; the test needs them equal", len(replacement), before.Size())
	}
	if err := os.WriteFile(path, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	c.advance(RefreshInterval)
	if _, refusal := store.Verify(context.Background(), "Bearer "+first, nil); refusal != guard.Mismatch {
		t.Errorf("the key the replacement dropped is %v, want a mismatch", refusal)
	}
	if p, refusal := store.Verify(context.Background(), "Bearer "+second, nil); refusal != guard.Accepted || p.Subject != "bobby" {
		t.Errorf("the key the replacement holds is %v (%+v), want accepted for bobby", refusal, p)
	}
}

// TestReloadKeepsTheKeysOverABadFile: a file that stops reading keeps the keys
// held and is logged once, until it moves again.
func TestReloadKeepsTheKeysOverABadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	store, err := Open(path, slog.New(slog.NewTextHandler(&log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.now = c.now
	store.checked = c.t

	if err := os.WriteFile(path, []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	touch(t, path, time.Second)
	for i := 0; i < 3; i++ {
		c.advance(RefreshInterval)
		if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
			t.Errorf("round %d: the key is %v over a bad file, want accepted: the keys held are kept", i, refusal)
		}
	}
	if n := strings.Count(log.String(), "api keys not reloaded"); n != 1 {
		t.Errorf("the failure was logged %d times, want once:\n%s", n, log.String())
	}
	if err := store.Reload(); err == nil || !strings.Contains(err.Error(), ":1: not a key record") {
		t.Errorf("Reload over a bad file: %v", err)
	}
	if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
		t.Errorf("the key is %v after a failed Reload, want accepted", refusal)
	}

	// A file that is missing for a moment is the same: kept, logged once.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	c.advance(RefreshInterval)
	if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
		t.Errorf("the key is %v while the file is missing, want accepted", refusal)
	}
	if err := store.Reload(); err == nil {
		t.Error("Reload over a missing file: no error")
	}
	if n := strings.Count(log.String(), "api keys not reloaded"); n != 2 {
		t.Errorf("the failures were logged %d times, want twice (one per broken version):\n%s", n, log.String())
	}

	// The file restored to a good state is read again.
	if _, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(); err != nil {
		t.Errorf("Reload over the restored file: %v", err)
	}
	if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Mismatch {
		t.Errorf("the old key is %v after the file was replaced, want a mismatch", refusal)
	}
}

// TestReloadReadsTheFileOnce: within the interval the file is not looked at,
// and an unchanged file is not read again after it.
func TestReloadReadsTheFileOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	store, err := Open(path, slog.New(slog.NewTextHandler(&log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.now = c.now
	store.checked = c.t
	for i := 0; i < 5; i++ {
		c.advance(RefreshInterval)
		if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
			t.Errorf("round %d: %v", i, refusal)
		}
	}
	if log.Len() != 0 {
		t.Errorf("an unchanged file was reloaded:\n%s", log.String())
	}
}

// TestReloadLogsAMissingFileOnce: a file that stays missing is not logged at
// every interval, and its return is read and logged once.
func TestReloadLogsAMissingFileOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	key, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	store, err := Open(path, slog.New(slog.NewTextHandler(&log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.now = c.now
	store.checked = c.t
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		c.advance(RefreshInterval)
		if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Accepted {
			t.Errorf("round %d: the key is %v while the file is missing, want accepted", i, refusal)
		}
	}
	if n := strings.Count(log.String(), "api keys not reloaded"); n != 1 {
		t.Errorf("the missing file was logged %d times, want once:\n%s", n, log.String())
	}
	if _, _, err := Mint(path, "bob", []string{"sysml:read"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	c.advance(RefreshInterval)
	if _, refusal := store.Verify(context.Background(), "Bearer "+key, nil); refusal != guard.Mismatch {
		t.Errorf("the old key is %v after the file came back, want a mismatch", refusal)
	}
	if n := strings.Count(log.String(), "api keys reloaded"); n != 1 {
		t.Errorf("the file's return was logged %d times, want once:\n%s", n, log.String())
	}
	// The file gone again is a new failure, logged once more.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	c.advance(RefreshInterval)
	store.Verify(context.Background(), "Bearer "+key, nil)
	c.advance(RefreshInterval)
	store.Verify(context.Background(), "Bearer "+key, nil)
	if n := strings.Count(log.String(), "api keys not reloaded"); n != 2 {
		t.Errorf("the failures were logged %d times, want twice:\n%s", n, log.String())
	}
}
