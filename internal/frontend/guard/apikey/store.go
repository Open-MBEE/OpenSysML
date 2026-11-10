package apikey

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// RefreshInterval bounds how often a request has the store look at the file's
// modification time.
const RefreshInterval = time.Second

// Store is the keys of one file, as a guard.Authenticator; it re-reads the
// file when its modification time moves and on Reload, so a revocation needs no restart.
type Store struct {
	path   string
	logger *slog.Logger
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]entry
	// modTime and size are the file's as last read, failed or not.
	modTime time.Time
	size    int64
	// checked is when the file was last looked at.
	checked time.Time
	// failure is the last reload error logged, so the same one is not logged again.
	failure string
}

// entry is a record with its hash decoded, compared on every request.
type entry struct {
	Record
	sum [sha256.Size]byte
}

// Open reads the file at path once, failing as ReadFile does; logger (nil for
// the default) hears each later reload and the one line a failed one logs.
func Open(path string, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Store{path: path, logger: logger, now: time.Now}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if err := s.load(info); err != nil {
		return nil, err
	}
	return s, nil
}

// Path is the file the store reads.
func (s *Store) Path() string { return s.path }

// Len is the number of records held, disabled and expired ones included.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// Reload re-reads the file now, as on SIGHUP; a file that does not read keeps
// the keys held and is returned as the error.
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	return s.load(info)
}

// load reads the file described by info under the lock and remembers its
// modification time and size, so the same content is not read twice.
func (s *Store) load(info os.FileInfo) error {
	s.modTime, s.size, s.checked = info.ModTime(), info.Size(), s.now()
	records, err := ReadFile(s.path)
	if err != nil {
		return err
	}
	entries := make(map[string]entry, len(records))
	for _, r := range records {
		sum, _ := hex.DecodeString(r.SHA256)
		e := entry{Record: r}
		copy(e.sum[:], sum)
		entries[r.ID] = e
	}
	s.entries, s.failure = entries, ""
	return nil
}

// refresh re-reads the file when, at most once per RefreshInterval, its
// modification time or size is found moved; a failed read keeps the keys held and logs once.
func (s *Store) refresh() {
	now := s.now()
	if now.Sub(s.checked) < RefreshInterval {
		return
	}
	s.checked = now
	info, err := os.Stat(s.path)
	if err != nil {
		s.logFailure(err)
		return
	}
	if info.ModTime().Equal(s.modTime) && info.Size() == s.size {
		return
	}
	if err := s.load(info); err != nil {
		s.logFailure(err)
		return
	}
	s.logger.Info("api keys reloaded", "path", s.path, "keys", len(s.entries))
}

// logFailure reports a reload that failed, once per failure: a file that stays
// missing or broken is not logged again at every interval.
func (s *Store) logFailure(err error) {
	if err.Error() == s.failure {
		return
	}
	s.failure = err.Error()
	s.logger.Error("api keys not reloaded, keeping the keys held", "path", s.path, "err", err)
}

// lookup is the entry of id, with the file refreshed first; found is false
// for an unknown id, whose entry is empty.
func (s *Store) lookup(id string) (entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refresh()
	e, ok := s.entries[id]
	return e, ok
}

// Verify accepts a key of the file as its subject (RFC 6750 § 2.1): a bearer token that is
// no key is NothingPresented; an unknown id, wrong secret or disabled key Mismatch; a dead one Expired.
func (s *Store) Verify(_ context.Context, authorization string, _ *http.Request) (guard.Principal, guard.Refusal) {
	token, refusal := guard.Bearer(authorization)
	if refusal != guard.Accepted {
		return guard.Principal{}, refusal
	}
	id, secret, ok := Split(token)
	if !ok {
		if len(token) >= len(Prefix) && token[:len(Prefix)] == Prefix {
			return guard.Principal{}, guard.Mismatch
		}
		return guard.Principal{}, guard.NothingPresented
	}
	e, found := s.lookup(id)
	sum := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(sum[:], e.sum[:]) != 1 || !found || e.Disabled {
		return guard.Principal{}, guard.Mismatch
	}
	if e.Expired(s.now()) {
		return guard.Principal{}, guard.Expired
	}
	return guard.Principal{
		Subject: e.Subject,
		Name:    "api key " + e.ID,
		Scopes:  slices.Clone(e.Scopes),
		Expires: e.Expires,
		Source:  Source,
	}, guard.Accepted
}

// Metadata is empty: no authorization server issues a key, and no scope is
// requested for one.
func (*Store) Metadata() guard.ResourceMetadata { return guard.ResourceMetadata{} }
