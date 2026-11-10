package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard/apikey"
)

// keyCommand is what -mint-api-key or -revoke-api-key asked for: a key to mint
// for a subject with scopes and a lifetime, or the id of a key to revoke.
type keyCommand struct {
	file    string
	mint    bool
	subject string
	scopes  []string
	expires time.Duration
	revoke  string
}

// readKeyCommand validates the key flags together: nil when neither command
// was asked for, and then -api-keys, -subject, -scopes and -expires are refused.
func readKeyCommand(o *options, fs *flag.FlagSet) (*keyCommand, error) {
	revoke := false
	fs.Visit(func(f *flag.Flag) { revoke = revoke || f.Name == "revoke-api-key" })
	if revoke && o.revokeAPIKey == "" {
		return nil, errors.New("-revoke-api-key needs the id of the key to disable; an empty id names none")
	}
	if !o.mintAPIKey && !revoke {
		if o.apiKeys != "" {
			return nil, errors.New("-api-keys applies to -mint-api-key and -revoke-api-key; give one")
		}
		return nil, mintQualifiers(o, "-mint-api-key; give it")
	}
	if o.mintAPIKey && revoke {
		return nil, errors.New("-mint-api-key and -revoke-api-key ask for different things; give one")
	}
	which := "-mint-api-key"
	if !o.mintAPIKey {
		which = "-revoke-api-key"
	}
	for _, f := range []struct {
		name string
		set  bool
	}{{"-http", o.http != ""}, {"-stdio", o.stdio}, {"-print-config", o.printConfig.set}} {
		if f.set {
			return nil, fmt.Errorf("%s edits the key file and exits; it is not given with %s", which, f.name)
		}
	}
	if o.apiKeys == "" {
		return nil, fmt.Errorf("%s needs the key file; name it with -api-keys", which)
	}
	cmd := &keyCommand{file: o.apiKeys}
	if !o.mintAPIKey {
		cmd.revoke = o.revokeAPIKey
		return cmd, mintQualifiers(o, "-mint-api-key, not -revoke-api-key")
	}
	cmd.mint = true
	if o.subject == "" {
		return nil, errors.New("-mint-api-key needs the key's subject; name it with -subject")
	}
	if strings.Trim(o.scopes, ", \t") == "" {
		return nil, errors.New("-mint-api-key needs the key's scopes; name them with -scopes, comma-separated")
	}
	for _, scope := range strings.Split(o.scopes, ",") {
		scope = strings.TrimSpace(scope)
		if !apikey.ValidScope(scope) {
			return nil, fmt.Errorf("-scopes: %q is not a scope token; a scope is printable characters without spaces, quotes or backslashes (RFC 6749 § 3.3)", scope)
		}
		cmd.scopes = append(cmd.scopes, scope)
	}
	if o.expires < 0 {
		return nil, fmt.Errorf("-expires: %v is not a lifetime; give a duration such as 720h, or none for a key that does not expire", o.expires)
	}
	cmd.subject, cmd.expires = o.subject, o.expires
	return cmd, nil
}

// mintQualifiers refuses -subject, -scopes and -expires where they do not
// apply, saying what they apply to.
func mintQualifiers(o *options, applyTo string) error {
	for _, f := range []struct {
		name string
		set  bool
	}{{"-subject", o.subject != ""}, {"-scopes", o.scopes != ""}, {"-expires", o.expires != 0}} {
		if f.set {
			return fmt.Errorf("%s applies to %s", f.name, applyTo)
		}
	}
	return nil
}

// run mints the key, printing it once to stdout and a note of its id to
// stderr, or revokes the key named; the file is created by a mint when absent.
func (c *keyCommand) run(stdout, stderr io.Writer) error {
	if !c.mint {
		if err := apikey.Revoke(c.file, c.revoke); err != nil {
			return fmt.Errorf("-revoke-api-key: %w", err)
		}
		fmt.Fprintf(stderr, "%skey %s disabled in %s\n", commandPrefix, c.revoke, c.file)
		return nil
	}
	var expires time.Time
	if c.expires > 0 {
		expires = time.Now().Add(c.expires)
	}
	key, record, err := apikey.Mint(c.file, c.subject, c.scopes, expires)
	if err != nil {
		return fmt.Errorf("-mint-api-key: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, key); err != nil {
		// A key nobody received must not stay good: the record is disabled again.
		if rerr := apikey.Revoke(c.file, record.ID); rerr != nil {
			return fmt.Errorf("-mint-api-key: the key was not written to stdout (%v) and its record %s could not be disabled: %w", err, record.ID, rerr)
		}
		return fmt.Errorf("-mint-api-key: the key was not written to stdout, so its record %s was disabled: %w", record.ID, err)
	}
	lifetime := "does not expire"
	if !record.Expires.IsZero() {
		lifetime = "expires " + record.Expires.Format(time.RFC3339)
	}
	fmt.Fprintf(stderr, "%skey %s for %s (%s) appended to %s; it is printed once, the file holds only its hash\n",
		commandPrefix, record.ID, record.Subject, lifetime, c.file)
	return nil
}
