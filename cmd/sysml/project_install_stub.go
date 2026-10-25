//go:build sysml_re

package main

import (
	"context"
	"errors"
)

// The browser REPL has no file system of its own to install into and no
// network to fetch from, so the project commands are left out of its build.
var errProjectNotLinked = errors.New("managing a project's usages is not available in this build (built without project install)")

func runProjectInit(string) error { return errProjectNotLinked }

func runProjectAdd(context.Context, string, string) error { return errProjectNotLinked }

func runProjectInstall(context.Context, string) error { return errProjectNotLinked }

func runProjectLock(context.Context, string) error { return errProjectNotLinked }
