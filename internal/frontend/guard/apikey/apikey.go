// Package apikey authenticates the access keys a service hands people and CI:
// bearer tokens osk_<id>.<secret> (RFC 6750), checked against a file of hashed records.
package apikey

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

// Prefix heads every key, so an authenticator tells one from any other bearer token.
const Prefix = "osk_"

// Source is the Principal.Source of a key.
const Source = "api-key"

// idBytes and secretBytes are the random bytes behind a key's id and secret;
// base64url without padding spells them as 12 and 43 characters.
const (
	idBytes     = 9
	secretBytes = 32
)

// Record is one key as the file holds it: the SHA-256 of the secret in hex,
// never the secret, so reading the file does not give the key away.
type Record struct {
	ID       string    `json:"id"`
	SHA256   string    `json:"sha256"`
	Subject  string    `json:"subject"`
	Scopes   []string  `json:"scopes"`
	Expires  time.Time `json:"expires,omitzero"`
	Disabled bool      `json:"disabled,omitempty"`
}

// Expired reports whether the record's lifetime had ended at now; a record
// with no expiry never expires.
func (r Record) Expired(now time.Time) bool {
	return !r.Expires.IsZero() && !now.Before(r.Expires)
}

// validate checks a record as the file or a mint states it.
func (r Record) validate() error {
	switch {
	case r.ID == "":
		return errors.New("the id is empty")
	case strings.Contains(r.ID, ".") || !guard.IsB64Token(r.ID):
		return fmt.Errorf("the id %q is not letters, digits and -_~+/", r.ID)
	case r.Subject == "":
		return errors.New("the subject is empty")
	case !printable(r.Subject):
		return fmt.Errorf("the subject %q has a control character", r.Subject)
	case len(r.Scopes) == 0:
		return errors.New("the scopes are empty")
	}
	if sum, err := hex.DecodeString(r.SHA256); err != nil || len(sum) != sha256.Size {
		return fmt.Errorf("the sha256 %q is not 64 hex digits", r.SHA256)
	}
	for _, scope := range r.Scopes {
		if !ValidScope(scope) {
			return fmt.Errorf("the scope %q is not a scope token (RFC 6749 § 3.3: printable ASCII without space, quote or backslash)", scope)
		}
	}
	return nil
}

// ValidScope reports whether s is a scope-token of RFC 6749 § 3.3.
func ValidScope(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x21 || r > 0x7e || r == '"' || r == '\\' {
			return false
		}
	}
	return true
}

// printable reports whether s has no control character.
func printable(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Split reads a key as osk_<id>.<secret>; ok is false for any other string.
func Split(key string) (id, secret string, ok bool) {
	rest, found := strings.CutPrefix(key, Prefix)
	if !found {
		return "", "", false
	}
	id, secret, found = strings.Cut(rest, ".")
	if !found || id == "" || secret == "" || strings.Contains(secret, ".") {
		return "", "", false
	}
	return id, secret, true
}

// Hash is what the file holds of a secret: its SHA-256 in hex.
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Mint issues a key for subject with scopes, expiring at expires (zero for
// never), and appends its record to the file at path, created if absent; the
// key is returned once and is not recoverable from the file.
func Mint(path, subject string, scopes []string, expires time.Time) (string, Record, error) {
	records, err := ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", Record{}, err
	}
	ids := make(map[string]bool, len(records))
	for _, r := range records {
		ids[r.ID] = true
	}
	id := random(idBytes)
	for ids[id] {
		id = random(idBytes)
	}
	secret := random(secretBytes)
	record := Record{ID: id, SHA256: Hash(secret), Subject: subject, Scopes: append([]string(nil), scopes...)}
	if !expires.IsZero() {
		record.Expires = expires.UTC().Truncate(time.Second)
	}
	if err := record.validate(); err != nil {
		return "", Record{}, err
	}
	if err := appendRecord(path, record); err != nil {
		return "", Record{}, err
	}
	return Prefix + id + "." + secret, record, nil
}

// random is n random bytes in base64url without padding (RFC 4648 § 5).
func random(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// appendRecord writes one record as one line at the end of the file, which
// only the owner may read.
func appendRecord(path string, record Record) error {
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600) // #nosec G304 -- the operator names the file with -api-keys
	if err != nil {
		return err
	}
	if _, err = f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Revoke marks the record of id disabled in the file at path, which is
// rewritten whole; a key that is not there, or is disabled already, is an error.
func Revoke(path, id string) error {
	records, err := ReadFile(path)
	if err != nil {
		return err
	}
	found := false
	for i := range records {
		if records[i].ID != id {
			continue
		}
		if records[i].Disabled {
			return fmt.Errorf("the key %s is disabled already", id)
		}
		records[i].Disabled = true
		found = true
	}
	if !found {
		return fmt.Errorf("no key has the id %s", id)
	}
	return writeFile(path, records)
}

// writeFile replaces the file with the records, one per line, through a
// rename so a reader sees the old file or the new one and never a part.
func writeFile(path string, records []Record) error {
	var buf strings.Builder
	for _, r := range records {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if _, err = tmp.WriteString(buf.String()); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}

// ReadFile reads the records of the file at path, one JSON object per line
// with blank lines allowed; a line that is not a valid record, or an id held
// twice, fails the whole file with its line number.
func ReadFile(path string) ([]Record, error) {
	f, err := os.Open(path) // #nosec G304 -- the operator names the file with -api-keys
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var records []Record
	ids := map[string]int{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<20)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r Record
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("%s:%d: not a key record: %w", path, n, err)
		}
		if err := r.validate(); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		if first, dup := ids[r.ID]; dup {
			return nil, fmt.Errorf("%s:%d: the id %s is held by line %d already", path, n, r.ID, first)
		}
		ids[r.ID] = n
		records = append(records, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return records, nil
}
