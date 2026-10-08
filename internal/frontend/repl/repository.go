package repl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The repository commands drive a SysML v2 API server, as the pilot's notebook
// kernel does: %repo names it, %projects lists it, %load reads a project's
// branch into the session and %publish writes an element's tree back.

func repositoryLinked() bool { return replext.Repo() != nil }

const (
	usageRepo     = "usage: %repo [<base path>]"
	usageProjects = "usage: %projects"
	usageLoadRepo = "usage: %load [--id=<project id>] [--name=<name>] [--branch=<branch name or id>] [<name>]"
	usageLoadPath = "usage: %load [--cells <n,n-m|tag:<tag>>] <file|dir|glob|notebook>..."
	usagePublish  = "usage: %publish [-d] [--project=<project name>] [--branch=<branch name>] <name>"
)

// completionTimeout bounds a project listing asked for by Tab completion.
const completionTimeout = 2 * time.Second

// repoBase is the base URL the repository commands address: the one %repo set,
// else the environment's.
func (s *Session) repoBase() string {
	if s.repoURL != "" {
		return s.repoURL
	}
	return replext.Repo().DefaultURL()
}

func (s *Session) metaRepositoryCommand(fields []string, _ string) (metaResult, bool) {
	if !repositoryLinked() {
		return metaResult{}, false
	}
	switch fields[0] {
	case "%repo":
		return s.doRepo(fields[1:]), true
	case "%projects":
		return s.doProjects(fields[1:]), true
	case "%load":
		if !s.loadsRepository(fields[1:]) {
			return metaResult{}, false
		}
		return s.doLoadRepository(fields[1:]), true
	case "%publish":
		return s.doPublish(fields[1:]), true
	}
	return metaResult{}, false
}

func usageError(lines ...string) metaResult {
	return metaOut(nil, false, &UsageError{Lines: lines})
}

func (s *Session) doRepo(args []string) metaResult {
	switch len(args) {
	case 0:
		return metaOut([]string{"API base path: " + s.repoBase()}, false, nil)
	case 1:
		base := strings.TrimRight(nameText(args[0]), "/")
		if err := replext.Repo().CheckURL(base); err != nil {
			return metaOut(nil, false, err)
		}
		s.repoURL = base
		return metaOut([]string{"API base path: " + base}, false, nil)
	}
	return usageError(usageRepo)
}

func (s *Session) doProjects(args []string) metaResult {
	if len(args) != 0 {
		return usageError(usageProjects)
	}
	projects, err := replext.Repo().Projects(s.command.context(), s.repoBase())
	if err != nil {
		return metaOut(nil, false, err)
	}
	if len(projects) == 0 {
		return metaOut([]string{"no projects"}, false, nil)
	}
	lines := make([]string, 0, len(projects))
	for _, p := range projects {
		lines = append(lines, projectLine(p.Name, p.ID))
	}
	return metaOut(lines, false, nil)
}

func projectLine(name, id string) string { return fmt.Sprintf("%s (%s)", name, id) }

// loadsRepository tells a %load of a repository project from one of files: a
// repository option names a project or a branch, and a lone name that names no
// path names a project. Other options are the file load's to accept or refuse.
func (s *Session) loadsRepository(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--id") || strings.HasPrefix(arg, "--name") || strings.HasPrefix(arg, "--branch") {
			return true
		}
	}
	if len(args) != 1 {
		return false
	}
	return !namesPath(nameText(args[0]))
}

// namesPath tells a file argument from a project name: a path has a separator,
// a model file's extension, a glob's wildcard, a home or relative prefix, or
// is on disk. A project whose name reads as one is loaded by --name.
func namesPath(arg string) bool {
	if strings.ContainsAny(arg, `/\*?[`) || strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, ".") {
		return true
	}
	switch strings.ToLower(filepath.Ext(arg)) {
	case ".sysml", ".kerml", ".ipynb":
		return true
	}
	_, err := os.Stat(expandHome(arg))
	return err == nil
}

// repoFlag reads `--name=value` as value; ok is false when arg is no such flag.
func repoFlag(arg, name string) (value string, ok bool) {
	return strings.CutPrefix(arg, "--"+name+"=")
}

// setRepoFlag stores the value of a `--option=value` argument in the field it
// names, once: no field, no value and a second value are usage errors.
func setRepoFlag(field *string, arg, usage, command string) error {
	name, value, _ := strings.Cut(arg, "=")
	if field == nil || value == "" {
		return &UsageError{Lines: []string{usage, fmt.Sprintf("%s is not an option of %s", arg, command)}}
	}
	if *field != "" {
		return &UsageError{Lines: []string{usage, fmt.Sprintf("%s was given twice", name)}}
	}
	*field = value
	return nil
}

func (s *Session) doLoadRepository(args []string) metaResult {
	var req replext.LoadRequest
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--"):
			var field *string
			switch {
			case strings.HasPrefix(arg, "--id="):
				field = &req.ProjectID
			case strings.HasPrefix(arg, "--name="):
				field = &req.Name
			case strings.HasPrefix(arg, "--branch="):
				field = &req.Branch
			}
			if err := setRepoFlag(field, arg, usageLoadRepo, "%load"); err != nil {
				return metaOut(nil, false, err)
			}
		case req.Name != "" || req.ProjectID != "":
			return usageError(usageLoadRepo, "name the project once: by --id, by --name or by itself")
		default:
			req.Name = nameText(arg)
		}
	}
	if req.Name == "" && req.ProjectID == "" {
		return usageError(usageLoadRepo, "name the project to load, or the files")
	}
	if req.Name != "" && req.ProjectID != "" {
		return usageError(usageLoadRepo, "name the project once: by --id, by --name or by itself")
	}
	loaded, err := replext.Repo().Load(s.command.context(), s.repoBase(), req)
	if err != nil {
		return metaOut(nil, false, err)
	}
	lines := []string{fmt.Sprintf("loaded project %s at branch %s, commit %s",
		projectLine(loaded.State.ProjectName, loaded.State.ProjectID), branchLine(loaded.State), loaded.State.LastSeenCommit)}
	for _, warning := range loaded.Warnings {
		lines = append(lines, "warning: "+warning)
	}
	file := SourceFile{Name: repositoryDocName(loaded.State), Text: string(loaded.Notation), Kind: source.KindSysML}
	found, declared := renderSplit(s.submitFiles([]SourceFile{file}), s.verbosity)
	state := loaded.State
	s.repoState = &state
	return metaOut(append(append(lines, found...), declared...), false, nil)
}

func branchLine(state replext.ProjectState) string {
	if state.BranchName == "" {
		return state.Branch
	}
	return projectLine(state.BranchName, state.Branch)
}

// repositoryDocName is the session document a loaded project becomes: one per
// project, so loading it again replaces the earlier load.
func repositoryDocName(state replext.ProjectState) string {
	return "project:" + state.ProjectID
}

func (s *Session) doPublish(args []string) metaResult {
	req := replext.PublishRequest{Origin: sessionOrigin}
	for _, arg := range args {
		switch {
		case arg == "-d":
			req.Derived = true
		case strings.HasPrefix(arg, "-"):
			var field *string
			switch {
			case strings.HasPrefix(arg, "--project="):
				field = &req.Project
			case strings.HasPrefix(arg, "--branch="):
				field = &req.Branch
			}
			if err := setRepoFlag(field, arg, usagePublish, "%publish"); err != nil {
				return metaOut(nil, false, err)
			}
		case req.Root != "":
			return usageError(usagePublish, "name one element to publish")
		default:
			req.Root = arg
		}
	}
	if req.Root == "" {
		return usageError(usagePublish, "name the element to publish")
	}
	_, fqn, err := s.lookupSymbol(req.Root)
	if err != nil {
		return metaOut([]string{errPrefix + err.Error()}, false, nil)
	}
	req.Root = fqn
	req.Source = []byte(s.text())
	req.State = s.repoState
	published, err := replext.Repo().Publish(s.command.context(), s.repoBase(), req)
	if err != nil {
		return metaOut(nil, false, err)
	}
	var lines []string
	if published.NewProject {
		lines = append(lines, "created project "+projectLine(published.State.ProjectName, published.State.ProjectID))
	}
	if published.Commit == "" {
		lines = append(lines, fmt.Sprintf("nothing to publish: branch %s already holds %s", branchLine(published.State), fqn))
	} else {
		lines = append(lines, fmt.Sprintf("commit %s on branch %s of %s: %d created, %d updated, %d deleted",
			published.Commit, branchLine(published.State), projectLine(published.State.ProjectName, published.State.ProjectID),
			published.Created, published.Updated, published.Deleted))
	}
	lines = append(lines, published.Notes...)
	state := published.State
	s.repoState = &state
	return metaOut(lines, false, nil)
}

// repositoryCompletions answers Tab after a repository command: the flags a
// `-` starts, project names otherwise, from the server when it answers in time.
func (s *Session) repositoryCompletions(command, word string) ([]string, bool) {
	if !repositoryLinked() {
		return nil, false
	}
	flags := map[string][]string{
		"%load":    {"--id=", "--name=", "--branch="},
		"%publish": {"-d", "--project=", "--branch="},
	}[command]
	if flags == nil {
		return nil, false
	}
	if strings.HasPrefix(word, "-") {
		if value, ok := repoFlag(word, "name"); ok {
			return prefixed("--name=", matchingPrefix(s.projectNames(), value)), true
		}
		if value, ok := repoFlag(word, "project"); ok {
			return prefixed("--project=", matchingPrefix(s.projectNames(), value)), true
		}
		if value, ok := repoFlag(word, "id"); ok {
			return prefixed("--id=", matchingPrefix(s.projectIDs(), value)), true
		}
		return matchingPrefix(flags, word), true
	}
	if command == "%load" {
		return append(pathCompletions(word), matchingPrefix(s.projectNames(), word)...), true
	}
	return nil, false
}

func (s *Session) projectNames() []string {
	return s.projectField(func(p replext.ProjectInfo) string { return p.Name })
}

func (s *Session) projectIDs() []string {
	return s.projectField(func(p replext.ProjectInfo) string { return p.ID })
}

func (s *Session) projectField(field func(replext.ProjectInfo) string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	defer cancel()
	projects, err := replext.Repo().Projects(ctx, s.repoBase())
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, field(p))
	}
	return out
}
