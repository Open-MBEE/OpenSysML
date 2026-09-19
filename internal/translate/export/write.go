package export

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// newFileMode is the permission a model that did not exist yet is created with:
// readable like any other document the user writes, since a model is not a
// secret. A file that does exist keeps the permissions it has.
const newFileMode = 0o644

// WriteFile writes a converted model to path, reporting whether it replaced a
// file that was already there.
//
// A regular file is written atomically: the bytes go to a temporary file in the
// same directory, are flushed, and are renamed over the destination, so an
// interrupted write leaves the previous model intact rather than a truncated or
// empty one. An existing file keeps its permissions, and a symlink is written
// through rather than replaced.
//
// Anything that is not a regular file — a terminal, a pipe, /dev/null, a
// process substitution — is written directly, since there is no previous
// content to protect and replacing it by rename would destroy the pipe or
// device the user pointed at.
func WriteFile(path string, data []byte) (replaced bool, err error) {
	target, info, err := Destination(path)
	if err != nil {
		return false, err
	}
	switch {
	case info == nil:
		return false, writeAtomic(path, target, newFileMode, data)
	case info.IsDir():
		return false, fmt.Errorf("cannot write %s: it is a directory", path)
	case !info.Mode().IsRegular():
		return false, writeThrough(path, target, 0, data)
	}
	err = writeAtomic(path, target, info.Mode().Perm(), data)
	if errors.Is(err, os.ErrPermission) {
		// The file is writable but its directory is not, so the temporary file
		// cannot be created; the model is what the user wants saved.
		if writeThrough(path, target, os.O_TRUNC, data) == nil {
			return true, nil
		}
	}
	return err == nil, err
}

// writeAtomic writes data beside target and renames it over target.
func writeAtomic(path, target string, mode os.FileMode, data []byte) (err error) {
	dir := filepath.Dir(target)
	info, serr := os.Stat(dir)
	switch {
	case os.IsNotExist(serr):
		return fmt.Errorf("cannot write %s: directory %s does not exist", path, dir)
	case serr != nil:
		return serr
	case !info.IsDir():
		return fmt.Errorf("cannot write %s: %s is not a directory", path, dir)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".*")
	if err != nil {
		return writeError(path, err)
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(name)
			err = writeError(path, err)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	// Without this the rename can reach the disk before the bytes do, which
	// after a crash leaves an empty file where the previous model was.
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	// CreateTemp makes the file 0600, which is not what a saved document is.
	// #nosec G302 -- deliberate: see newFileMode.
	if err = os.Chmod(name, mode); err != nil {
		return err
	}
	if err = os.Rename(name, target); err != nil {
		return err
	}
	syncDir(dir)
	return nil
}

// writeThrough writes data into target as it stands, without replacing it. A
// pipe or device takes no O_TRUNC, so the caller passes the flags.
func writeThrough(path, target string, flags int, data []byte) error {
	// #nosec G304 -- target is the destination the caller asked to write.
	f, err := os.OpenFile(target, os.O_WRONLY|flags, 0)
	if err != nil {
		return writeError(path, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return writeError(path, err)
	}
	if err := f.Close(); err != nil {
		return writeError(path, err)
	}
	return nil
}

// syncDir flushes the directory entry the rename created. Best-effort: the
// model is already written, so a filesystem that refuses is not a failed save.
func syncDir(dir string) {
	// #nosec G304 -- dir is the directory of the path the caller asked to write.
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}

// Destination returns the file a save should write and what is there now, which
// is nil when nothing is. A symlink resolves to what it points at, so saving
// over a linked model updates the model instead of unlinking it; a link that
// resolves to nothing is written where it points.
func Destination(path string) (target string, info os.FileInfo, err error) {
	info, err = os.Stat(path)
	switch {
	case os.IsNotExist(err):
		// A dangling symlink is written where it points, so the link survives.
		return resolveRegular(path), nil, nil
	case err != nil:
		return "", nil, err
	case !info.Mode().IsRegular():
		// A pipe or device is opened as named: resolving it would land on
		// something like /proc/self/fd's pipe:[…], which cannot be opened.
		return path, info, nil
	}
	return resolveRegular(path), info, nil
}

// resolveRegular returns what path points at when it is a symlink, and path
// itself otherwise.
func resolveRegular(path string) string {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return path
	}
	return readLink(path)
}

// readLink follows a chain of symlinks as far as it resolves, so that a link to
// a link to a model still writes the model.
func readLink(path string) string {
	for range 64 {
		next, err := os.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(next) {
			next = filepath.Join(filepath.Dir(path), next)
		}
		path = next
	}
	return path
}

// writeError reports a failure against the path the caller named, dropping the
// *os.PathError whose path may be the temporary file it never asked for.
func writeError(path string, err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return fmt.Errorf("cannot write %s: %w", path, err)
}
