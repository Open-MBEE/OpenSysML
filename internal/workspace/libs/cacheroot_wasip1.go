//go:build wasip1

package libs

import (
	"errors"
	"syscall"
)

// hostDeniedPath reports WASI's ENOTCAPABLE: the path lies outside every
// directory the host preopened for the process.
func hostDeniedPath(err error) bool {
	return errors.Is(err, syscall.ENOTCAPABLE)
}
