//go:build !sysml_re

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project/install"
)

// runProjectInit writes .project.json and .meta.json in dir, named -name
// (the directory's base name when not given) and published by -publisher.
func runProjectInit(dir string) error {
	name := projectName
	if name == "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		name = filepath.Base(abs)
	}
	p, err := project.Init(dir, projectPublisher, name, "")
	if err != nil {
		return fmt.Errorf("cannot initialize %s: %w", dir, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s and %s in %s (%s %s)\n", project.ProjectFile, project.MetaFile, dir, p.Name, p.Version)
	return nil
}

// runProjectAdd appends a usage of the resource to the project in dir, then
// resolves, installs and locks it. A version after @ in the IRI becomes the
// usage's versionConstraint.
func runProjectAdd(ctx context.Context, dir, resource string) error {
	o, err := project.ParseOrigin(project.Usage{Resource: resource})
	if err != nil {
		return fmt.Errorf("cannot add %s: %w", resource, err)
	}
	u := project.Usage{Resource: o.IRI, VersionConstraint: o.Constraint}
	in := &install.Installer{Dir: dir, Log: os.Stderr}
	lock, err := in.Add(ctx, u)
	if err != nil {
		return installFailure(err)
	}
	e := lock.Entry(u.Resource)
	fmt.Fprintf(os.Stderr, "added %s (%s %s) to %s; %d projects locked\n", u.Resource, e.Name, e.Version, dir, len(lock.Entries))
	return nil
}

// runProjectInstall installs what .project.json and the lock in dir say.
func runProjectInstall(ctx context.Context, dir string) error {
	lock, err := (&install.Installer{Dir: dir, Log: os.Stderr}).Sync(ctx)
	if err != nil {
		return installFailure(err)
	}
	fmt.Fprintf(os.Stderr, "%s: %d projects installed and locked\n", dir, len(lock.Entries))
	return nil
}

// runProjectLock resolves every usage of the project in dir afresh,
// installs and rewrites the lock.
func runProjectLock(ctx context.Context, dir string) error {
	lock, err := (&install.Installer{Dir: dir, Log: os.Stderr}).Relock(ctx)
	if err != nil {
		return installFailure(err)
	}
	fmt.Fprintf(os.Stderr, "%s: %d projects locked\n", dir, len(lock.Entries))
	return nil
}

// installFailure reports an installer error as it is: an InstallError
// already names the IRI and the step.
func installFailure(err error) error {
	var ie *install.InstallError
	if errors.As(err, &ie) {
		return ie
	}
	return err
}
