// Package fsutil holds the file-system operations more than one package
// spells alike.
package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// Replace renames source over target, atomically where the platform allows.
// Where the platform refuses a rename over what is at target, it retries
// after removing that; any other failure — the source missing, another
// device — leaves the target as it was. A target that is a link is replaced,
// never followed.
func Replace(source, target string) error {
	err := os.Rename(source, target)
	if err == nil || !targetInTheWay(err) {
		return err
	}
	if _, statErr := os.Lstat(source); statErr != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(source, target)
}

// targetInTheWay says whether a rename failed because of what is at its target
// rather than because of its source or the operation itself.
func targetInTheWay(err error) bool {
	return errors.Is(err, fs.ErrExist) ||
		errors.Is(err, syscall.EISDIR) ||
		errors.Is(err, syscall.ENOTEMPTY)
}
