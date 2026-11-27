//go:build !plan9

package source

import (
	"errors"
	"syscall"
)

// targetNotEmpty says whether a rename failed because its target is a directory
// with entries.
func targetNotEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY)
}
