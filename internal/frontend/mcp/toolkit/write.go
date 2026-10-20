package toolkit

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// savedFileMode is the permission a saved file that did not exist yet is
// created with: readable like any document the user writes, as a model is no secret.
const savedFileMode = 0o644

// Target is the file a write to path reaches, relative to Real: path with its
// symlinks resolved, re-checked now since a link may have been repointed since.
func (r Root) Target(path string) (string, error) {
	res, err := r.Resolve(path)
	if err != nil {
		return "", err
	}
	return filepath.Rel(r.Real, res.Real)
}

// WriteFile replaces the regular file path names with content through an os.Root
// at Real, so a link repointed out of the root cannot carry the write outside it.
func (r Root) WriteFile(path string, content []byte) (replaced bool, err error) {
	name, err := r.Target(path)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(r.Real)
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return false, fmt.Errorf("cannot write %s: %w", path, err)
	case info.IsDir():
		return false, fmt.Errorf("cannot write %s: it is a directory", path)
	case !info.Mode().IsRegular():
		return false, fmt.Errorf("cannot write %s: it is not a regular file", path)
	default:
		replaced = true
	}
	if err := WriteFile(root, name, content, savedFileMode); err != nil {
		return false, fmt.Errorf("cannot write %s: %w", path, err)
	}
	return replaced, nil
}

// WriteFile replaces name, relative to root, with content atomically: a temporary
// file beside it is renamed over it. An existing file keeps its mode, a new one gets created.
func WriteFile(root *os.Root, name string, content []byte, created os.FileMode) error {
	mode := created
	if info, err := root.Stat(name); err == nil {
		mode = info.Mode().Perm()
	}
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(name), "."+filepath.Base(name)+"."+hex.EncodeToString(suffix[:])+".tmp")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	// Without this the rename can reach the disk before the bytes do, which
	// after a crash leaves an empty file where the previous one was.
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = root.Chmod(tmp, mode)
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	if err != nil {
		_ = root.Remove(tmp)
	}
	return err
}
