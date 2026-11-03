//go:build !wasip1

package libs

// hostDeniedPath is false where no host hands out the filesystem piecemeal.
func hostDeniedPath(error) bool { return false }
