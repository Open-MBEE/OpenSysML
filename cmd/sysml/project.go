package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// projectCommandRequested reports whether a run manages a project rather
// than loading a model: -project-init, -add, -install, -sync or -lock.
func projectCommandRequested() bool {
	return projectInit || addResource != "" || installProject || syncProject || lockProject ||
		flagGiven("add") || flagGiven("publisher") || flagGiven("name")
}

// runProjectCommand runs the one project command asked for on the project
// directory named as the sole argument (the working directory when none)
// and returns the exit status. The commands write .project.json, the lock
// and lib/ and load no model, so they run on their own.
func runProjectCommand(args []string) int {
	var asked []string
	for _, c := range []struct {
		name string
		on   bool
	}{{"project-init", projectInit}, {"add", addResource != "" || flagGiven("add")}, {"install", installProject}, {"sync", syncProject}, {"lock", lockProject}} {
		if c.on {
			asked = append(asked, "-"+c.name)
		}
	}
	if len(asked) > 1 && !(len(asked) == 2 && installProject && syncProject) {
		fmt.Fprintf(os.Stderr, "sysml: %s each manage a project in their own way; ask for one per run\n", strings.Join(asked, ", "))
		return exitUnevaluable
	}
	if len(asked) == 0 {
		fmt.Fprintln(os.Stderr, "sysml: -publisher and -name name the project -project-init writes; give -project-init with them")
		return exitUnevaluable
	}
	if modelChecks.requested() || convertFormat != "" || migrateFormat != "" || renderView != "" || renderDoc != "" || len(evalExprs) > 0 || bundleDir != "" || unbundleFile != "" {
		fmt.Fprintf(os.Stderr, "sysml: %s manages a project and loads no model; run it on its own\n", asked[0])
		return exitUnevaluable
	}
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "sysml: %s takes the project directory as its one argument, not %d\n", asked[0], len(args))
		return exitUnevaluable
	}
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}
	if (flagGiven("publisher") || flagGiven("name")) && !projectInit {
		fmt.Fprintf(os.Stderr, "sysml: -publisher and -name name the project -project-init writes; %s reads the name from %s\n", asked[0], project.ProjectFile)
		return exitUnevaluable
	}
	if flagGiven("add") && addResource == "" {
		fmt.Fprintln(os.Stderr, "sysml: -add names the resource to use: pkg:github/<org>/<repo>[@<constraint>], a git URL, a .kpar URL or a local path")
		return exitUnevaluable
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var err error
	switch {
	case projectInit:
		err = runProjectInit(dir)
	case addResource != "":
		err = runProjectAdd(ctx, dir, addResource)
	case lockProject:
		err = runProjectLock(ctx, dir)
	default:
		err = runProjectInstall(ctx, dir)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml: %v\n", err)
		return exitUnevaluable
	}
	return exitHolds
}

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
	in := &project.Installer{Dir: dir, Log: os.Stderr}
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
	lock, err := (&project.Installer{Dir: dir, Log: os.Stderr}).Sync(ctx)
	if err != nil {
		return installFailure(err)
	}
	fmt.Fprintf(os.Stderr, "%s: %d projects installed and locked\n", dir, len(lock.Entries))
	return nil
}

// runProjectLock resolves every usage of the project in dir afresh,
// installs and rewrites the lock.
func runProjectLock(ctx context.Context, dir string) error {
	lock, err := (&project.Installer{Dir: dir, Log: os.Stderr}).Relock(ctx)
	if err != nil {
		return installFailure(err)
	}
	fmt.Fprintf(os.Stderr, "%s: %d projects locked\n", dir, len(lock.Entries))
	return nil
}

// installFailure reports an installer error as it is: an InstallError
// already names the IRI and the step.
func installFailure(err error) error {
	var ie *project.InstallError
	if errors.As(err, &ie) {
		return ie
	}
	return err
}
