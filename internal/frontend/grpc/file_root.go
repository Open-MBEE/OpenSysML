package grpc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
)

// fileRoot is the directory a confined service reads request-named files under.
type fileRoot struct {
	// dir is absolute, with its symbolic links followed.
	dir string
}

func newFileRoot(dir string) (*fileRoot, error) {
	if dir == "" {
		return nil, errors.New("the file root needs a directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("file root %q: %w", dir, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("file root %q: %w", dir, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("file root %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("file root %q is not a directory", dir)
	}
	return &fileRoot{dir: resolved}, nil
}

// path is name as the root takes it: a relative name from the root, uncleaned, so
// a ".." after a link leaves the link's target as it does for the system.
func (r *fileRoot) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return r.dir + string(filepath.Separator) + name
}

// read reads the file name resolves to when that lies under the root. The read goes
// through the root itself, so a link put on the way after the check cannot lead it out.
func (r *fileRoot) read(name string) ([]byte, error) {
	resolved, err := filepath.EvalSymlinks(r.path(name))
	if err != nil {
		return nil, refusedFile(name)
	}
	rel, err := filepath.Rel(r.dir, resolved)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, refusedFile(name)
	}
	root, err := os.OpenRoot(r.dir)
	if err != nil {
		return nil, refusedFile(name)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(rel)
	if err != nil {
		return nil, refusedFile(name)
	}
	return data, nil
}

// refusedFile answers alike for a file outside the root and one that is not there,
// so the answer does not tell a caller what exists outside the root.
func refusedFile(name string) error {
	return statusErrorf(connect.CodePermissionDenied,
		"file not readable: %q is not a file under the file root", name)
}
