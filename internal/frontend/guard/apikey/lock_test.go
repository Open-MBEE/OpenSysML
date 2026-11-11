package apikey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLockHeld: a lock another process left is waited for, then named so the
// operator can remove it; one released in time is taken.
func TestLockHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.jsonl")
	if _, _, err := Mint(path, "alice", []string{"sysml:read"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	defer func(w, p time.Duration) { lockWait, lockPoll = w, p }(lockWait, lockPoll)
	lockWait, lockPoll = 100*time.Millisecond, time.Millisecond
	if err := os.WriteFile(path+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := Mint(path, "bob", []string{"sysml:read"}, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "another mint or revoke") || !strings.Contains(err.Error(), path+".lock") {
		t.Errorf("Mint under a held lock: %v", err)
	}
	if records, _ := ReadFile(path); len(records) != 1 {
		t.Errorf("the file was edited under a held lock: %v", records)
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = os.Remove(path + ".lock")
	}()
	if _, _, err := Mint(path, "bob", []string{"sysml:read"}, time.Time{}); err != nil {
		t.Errorf("Mint after the lock was released: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Errorf("the lock is left behind: %v", err)
	}
}
