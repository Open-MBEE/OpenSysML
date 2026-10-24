package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// runBundle writes the project directory bundleDir as a project interchange
// archive, to -o or to <name>-<version>.kpar beside it.
func runBundle(dir string) error {
	data, p, err := project.Bundle(dir)
	if err != nil {
		return fmt.Errorf("cannot bundle %s: %w", dir, err)
	}
	out := outputPath
	if out == "" {
		out = archiveFileName(p)
	}
	if inside, err := isWithin(dir, out); err != nil {
		return err
	} else if inside {
		return fmt.Errorf("%s lies inside the project %s it would archive; write the archive elsewhere", out, dir)
	}
	if err := writeWhole(out, data); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s %s, %d files)\n", out, p.Name, p.Version, len(archiveMembers(data, out)))
	return nil
}

// isWithin reports whether path is dir or lies under it, both resolved.
func isWithin(dir, path string) (bool, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	if resolved, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(absPath)); err == nil {
		absPath = filepath.Join(resolved, filepath.Base(absPath))
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false, nil
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

// writeWhole writes data to path through a temporary file beside it, so a
// failed write leaves whatever was at path as it was.
func writeWhole(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// runUnbundle materializes the archive as a project directory, at -o or at
// <name>-<version> beside it.
func runUnbundle(file string) error {
	a, err := project.OpenArchive(file)
	if err != nil {
		var ae *project.ArchiveError
		if errors.As(err, &ae) {
			err = ae.Err
		}
		return fmt.Errorf("cannot read %s: %w", file, err)
	}
	out := outputPath
	if out == "" {
		out = strings.TrimSuffix(archiveFileName(a.Project), project.ArchiveExt)
	}
	if err := project.Unbundle(a, out); err != nil {
		return fmt.Errorf("cannot unbundle %s: %w", file, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%s %s, %d files)\n", out, a.Project.Name, a.Project.Version, len(a.Names()))
	return nil
}

// archiveFileName names an archive as the OMG publishes them: the project's
// name with its spaces as underscores, its version, and the extension.
func archiveFileName(p *project.Interchange) string {
	name := strings.ReplaceAll(p.Name, " ", "_")
	return filepath.Base(name + "-" + p.Version + project.ArchiveExt)
}

// archiveMembers lists the members of the archive bytes just written.
func archiveMembers(data []byte, path string) []string {
	a, err := project.ParseArchive(path, data)
	if err != nil {
		return nil
	}
	return a.Names()
}
