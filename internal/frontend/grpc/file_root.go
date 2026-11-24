package grpc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"connectrpc.com/connect"
)

// fileRoot is the directory a confined service reads request-named files under.
type fileRoot struct {
	// dir is absolute, with its symbolic links followed.
	dir string
	// given is the absolute name the root was configured by, which may reach dir by links.
	given string
}

// maxFileRootLinks bounds the symbolic links one name may pass, as the system does.
const maxFileRootLinks = 255

func newFileRoot(dir string) (*fileRoot, error) {
	if dir == "" {
		return nil, errors.New("the file root needs a directory")
	}
	given, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("file root %q: %w", dir, err)
	}
	resolved, err := filepath.EvalSymlinks(given)
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
	return &fileRoot{dir: resolved, given: given}, nil
}

// path is name as the root takes it: a relative name from the root, uncleaned, so
// a ".." after a link leaves the link's target as it does for the system.
func (r *fileRoot) path(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return r.dir + string(filepath.Separator) + name
}

// within is the part of an absolute path below the root, by either of the root's
// names, as elements; ok is false for a path that does not start in the root. The
// name the root was configured by counts only while it still leads to the root.
func (r *fileRoot) within(path string) (rest []string, ok bool) {
	if filepath.VolumeName(path) != filepath.VolumeName(r.dir) {
		return nil, false
	}
	elements := namedElements(path)
	for _, root := range []string{r.dir, r.given} {
		prefix := pathElements(root)
		if len(elements) < len(prefix) || !slices.Equal(elements[:len(prefix)], prefix) {
			continue
		}
		if root != r.dir {
			if now, err := filepath.EvalSymlinks(root); err != nil || now != r.dir {
				continue
			}
		}
		return elements[len(prefix):], true
	}
	return nil, false
}

// resolve walks name one element at a time and returns the file it names, relative to
// the root. It fails when any step is outside the root, even one that a later link
// brings back in, so a name tells nothing of what lies outside.
func (r *fileRoot) resolve(name string) (string, error) {
	pending := namedElements(name)
	if filepath.IsAbs(name) {
		rest, ok := r.within(name)
		if !ok {
			return "", errOutsideFileRoot
		}
		pending = rest
	}
	var at []string
	for links := 0; len(pending) > 0; {
		element := pending[0]
		pending = pending[1:]
		switch element {
		case ".":
			continue
		case "..":
			if len(at) == 0 {
				return "", errOutsideFileRoot
			}
			at = at[:len(at)-1]
			continue
		}
		next := filepath.Join(r.dir, filepath.Join(at...), element)
		info, err := os.Lstat(next)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if len(pending) > 0 && !info.IsDir() {
				return "", errOutsideFileRoot
			}
			at = append(at, element)
			continue
		}
		if links++; links > maxFileRootLinks {
			return "", errOutsideFileRoot
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		follow := namedElements(target)
		if filepath.IsAbs(target) {
			rest, ok := r.within(target)
			if !ok {
				return "", errOutsideFileRoot
			}
			at, follow = nil, rest
		}
		pending = append(follow, pending...)
	}
	if len(at) == 0 {
		return "", errOutsideFileRoot
	}
	return filepath.Join(at...), nil
}

// namedElements is the elements of a path a request or a link names, with a "." for
// a trailing separator, which names what is before it as a directory.
func namedElements(path string) []string {
	elements := pathElements(path)
	if path != "" && os.IsPathSeparator(path[len(path)-1]) {
		elements = append(elements, ".")
	}
	return elements
}

// pathElements splits a path at its separators, dropping the empty elements.
func pathElements(path string) []string {
	return strings.FieldsFunc(path, func(c rune) bool { return c < 0x80 && os.IsPathSeparator(uint8(c)) })
}

var errOutsideFileRoot = errors.New("outside the file root")

// read reads the file name resolves to when every step to it lies under the root. The
// read goes through the root itself, so a link put on the way after the check cannot
// lead it out.
func (r *fileRoot) read(name string) ([]byte, error) {
	rel, err := r.resolve(name)
	if err != nil {
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
