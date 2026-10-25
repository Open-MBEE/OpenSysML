package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
