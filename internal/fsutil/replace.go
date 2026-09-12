// Package fsutil holds the file-system operations more than one package
// spells alike.
package fsutil

import "os"

// Replace renames source over target, atomically where the platform allows;
// Windows refuses a rename over an existing file, so it retries after removing
// the target, keeping the target until the retry begins. A target that is a
// link is replaced, never followed.
func Replace(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(source, target)
}
