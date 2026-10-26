package lsp

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

//lint:file-ignore SA1019 rootUri/rootPath are deprecated, but see legacyRoot.

// maxScannedFileSize caps what the folder scan reads.
const maxScannedFileSize = 8 << 20

// setFolders records the folders the session was initialized with.
func (s *Server) setFolders(folders []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folders = folders
}

// setHoverMarkdown records whether the client renders Markdown hovers.
func (s *Server) setHoverMarkdown(ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hoverMarkdown = ok
}

// wantsMarkdownHover reports whether hovers should be rendered as Markdown.
func (s *Server) wantsMarkdownHover() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hoverMarkdown
}

// setCompletionMarkdown records whether the client renders Markdown completion
// documentation.
func (s *Server) setCompletionMarkdown(ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completionMarkdown = ok
}

// wantsMarkdownCompletion reports whether completion documentation should be
// rendered as Markdown.
func (s *Server) wantsMarkdownCompletion() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.completionMarkdown
}

// setCrossDocument records whether the client speaks the cross-document
// diagram contract.
func (s *Server) setCrossDocument(ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crossDocument = ok
}

// clientSpeaksCrossDocument reports whether renderings may name another
// document's declarations, which the client then pins its layouts to.
func (s *Server) clientSpeaksCrossDocument() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.crossDocument
}

// initializeFolders returns a session's folders, preferring workspaceFolders and
// falling back to the deprecated rootUri/rootPath older clients send instead.
func initializeFolders(params *protocol.InitializeParams) []string {
	var out []string
	seen := map[string]bool{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, folder := range params.WorkspaceFolders {
		add(uriToName(protocol.DocumentURI(folder.URI)))
	}
	if len(out) == 0 {
		add(legacyRoot(params))
	}
	return out
}

// legacyRoot returns the root a client that predates workspaceFolders sends;
// these deprecated fields are the only ones such a client populates.
func legacyRoot(params *protocol.InitializeParams) string {
	if params.RootURI != "" {
		return uriToName(params.RootURI)
	}
	return params.RootPath
}

// loadFolders indexes every model source under the session's folders, so a name
// declared in a file the editor never opened still resolves.
func (s *Server) loadFolders(ctx context.Context) {
	s.mu.Lock()
	// Copied: a folder change can rewrite the slice while this walk runs.
	folders := append([]string(nil), s.folders...)
	s.mu.Unlock()

	s.debugEdit(ctx, "", func() {
		for _, folder := range folders {
			s.loadFolder(folder)
		}
	})
	s.refreshOpenDiagnostics(ctx, "")
}

// loadFolder reads the model sources one folder holds, skipping hidden and
// vendored directories and passing over entries it cannot read, and those of
// the installed projects the folder's project uses.
func (s *Server) loadFolder(folder string) { s.scanFolder(folder, -1) }

// scanFolder is loadFolder over at most budget directories, every directory
// when budget is negative; a walk that runs out of budget stops where it is.
// The sources found are indexed as one batch, not one at a time: each document
// added alone would re-derive the imports of every package that reads it.
func (s *Server) scanFolder(folder string, budget int) {
	paths, _ := s.walkWorkspace(folder, budget)
	s.loadFilesFromDisk(paths)
}

// walkWorkspace is walkFolder over folder followed by the directories of the
// installed projects that the project at or above folder uses, each once; the
// dependency sources are recorded against folder so they can be dropped with it.
func (s *Server) walkWorkspace(folder string, budget int) (paths, unlisted []string) {
	paths, unlisted = walkFolder(folder, budget, nil)
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		seen[path] = true
	}
	var deps []string
	for _, used := range s.dependencyProjects(folder) {
		var more, failed []string
		if used.IsArchived() {
			more = used.Files()
		} else {
			// A project nested in an installed one is a project of its own,
			// read through its own installation.
			more, failed = walkFolder(used.Dir, -1, project.IsProjectDir)
		}
		unlisted = append(unlisted, failed...)
		for _, path := range more {
			if !seen[path] {
				seen[path] = true
				deps = append(deps, path)
			}
		}
	}
	s.mu.Lock()
	if s.dependencies == nil {
		s.dependencies = map[string][]string{}
	}
	if len(deps) == 0 {
		delete(s.dependencies, folder)
	} else {
		s.dependencies[folder] = deps
	}
	s.mu.Unlock()
	return append(paths, deps...), unlisted
}

// dependencyProjects lists the installed projects the project at or above
// folder uses, found under the well-known library directories of its project
// and the configured roots, a root given relative to the folder resolved
// against it; a project may be a directory or an archive.
func (s *Server) dependencyProjects(folder string) []*project.Interchange {
	if folder == "" {
		return nil
	}
	s.mu.Lock()
	roots := make([]string, 0, len(s.libraryRoots))
	for _, root := range s.libraryRoots {
		if !filepath.IsAbs(root) {
			root = filepath.Join(folder, root)
		}
		roots = append(roots, root)
	}
	s.mu.Unlock()
	return project.Discovery{Roots: roots}.Around([]string{folder}).Used
}

// dependencySources lists the sources the installed projects contributed for
// every folder and opened directory, each once.
func (s *Server) dependencySources() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]bool{}
	for _, paths := range s.dependencies {
		for _, path := range paths {
			out[path] = true
		}
	}
	return out
}

// reloadDependencies walks the folders and the opened directories again after
// the library roots changed, dropping the dependency sources no project
// contributes any more, and republishes the open documents.
func (s *Server) reloadDependencies(ctx context.Context) {
	before := s.dependencySources()
	s.mu.Lock()
	folders := append([]string(nil), s.folders...)
	var openDirs []string
	for dir := range s.openDirs {
		openDirs = append(openDirs, dir)
	}
	s.mu.Unlock()
	s.debugEdit(ctx, "", func() {
		walked := map[string]bool{}
		var paths []string
		for _, folder := range folders {
			found, _ := s.walkWorkspace(folder, -1)
			paths = append(paths, found...)
		}
		for _, dir := range openDirs {
			found, _ := s.walkWorkspace(dir, maxOpenedDirs)
			paths = append(paths, found...)
		}
		for _, path := range paths {
			walked[path] = true
		}
		s.loadFilesFromDisk(paths)
		for name := range before {
			if !walked[name] && !s.ws.IsOpen(name) {
				s.ws.DeleteOnDisk(name)
			}
		}
	})
	s.refreshOpenDiagnostics(ctx, "")
}

// forgetDependencies drops the dependency records of the keys — folders or
// opened directories — and returns the sources they listed and the set of
// sources the remaining records still hold, which must stay indexed.
func (s *Server) forgetDependencies(keys []string) (dropped []string, still map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		dropped = append(dropped, s.dependencies[key]...)
		delete(s.dependencies, key)
	}
	still = map[string]bool{}
	for _, paths := range s.dependencies {
		for _, path := range paths {
			still[path] = true
		}
	}
	return dropped, still
}

// dropDependencies forgets the dependency sources recorded against the keys
// that no other key contributed, keeping an open buffer and a source under a
// remaining folder, and returns what the remaining records still hold.
func (s *Server) dropDependencies(keys, kept []string) map[string]bool {
	dropped, still := s.forgetDependencies(keys)
	for _, name := range dropped {
		if !still[name] && !s.ws.IsOpen(name) && !underAnyFolder(name, kept) {
			s.ws.DeleteOnDisk(name)
		}
	}
	return still
}

// projectOrDir is the directory an opened document's siblings are scanned
// from: its project's directory when it lies in one, so an import of a
// declaration elsewhere in the project resolves as it does on the command
// line, else its own directory.
func projectOrDir(name string) string {
	if projects := project.EnclosingProjects(name); len(projects) > 0 {
		return projects[0]
	}
	return filepath.Dir(name)
}

// walkFolder lists the model sources under folder, visiting at most budget
// directories (every directory when budget is negative) and skipping hidden and
// vendored directories, unreadable entries and files too large to serve. It also
// lists the directories it could not enumerate, whose sources it cannot vouch for.
// A directory below folder that stop admits, when stop is given, is not entered.
func walkFolder(folder string, budget int, stop func(dir string) bool) (paths, unlisted []string) {
	if folder == "" {
		return nil, nil
	}
	_ = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				unlisted = append(unlisted, path)
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != folder && (skipDir(d.Name()) || project.IsLibraryDir(filepath.Dir(path), d.Name()) || (stop != nil && stop(path))) {
				return fs.SkipDir
			}
			if budget == 0 {
				return fs.SkipAll
			}
			budget--
			return nil
		}
		if !model.IsModelSource(path) {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 && !linksWithin(path, folder) {
			return nil
		}
		if info, statErr := os.Stat(path); statErr != nil || info.Size() > maxScannedFileSize {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	return paths, unlisted
}

// Rescan reconciles the workspace with the session's folders as they are on
// disk now, for a client with no file watcher: sources added or rewritten since
// the last walk are reindexed as one batch, and closed sources gone from disk
// are dropped. An open buffer stays authoritative, as for a watched change.
func (s *Server) Rescan(ctx context.Context) {
	s.mu.Lock()
	folders := append([]string(nil), s.folders...)
	s.mu.Unlock()
	if len(folders) == 0 {
		return
	}
	s.debugEdit(ctx, "", func() {
		before := s.dependencySources()
		onDisk := map[string]bool{}
		var changed, unlisted []string
		for _, folder := range folders {
			paths, failed := s.walkWorkspace(folder, -1)
			unlisted = append(unlisted, failed...)
			for _, path := range paths {
				onDisk[path] = true
				if s.changedOnDisk(path) {
					changed = append(changed, path)
				}
			}
		}
		s.loadFilesFromDisk(changed)
		for _, name := range s.ws.DocumentNames() {
			if onDisk[name] || s.ws.IsLibraryDocument(name) || s.ws.IsOpen(name) || !(underFolders(name, folders) || before[name]) {
				continue
			}
			// Not walked: gone, grown past the size limit, or moved under a
			// directory the walk skips. readFromDisk drops the first two as a
			// watched change would; the third is dropped as the walk never
			// indexed it, unless its directory could not be listed, when the
			// source is still there as far as anyone can tell. A file that
			// cannot be read right now is kept.
			if _, readable := s.readFromDisk(name); readable && !underFolders(name, unlisted) {
				s.ws.DeleteOnDisk(name)
			}
		}
	})
	s.refreshOpenDiagnostics(ctx, "")
}

// changedOnDisk reports whether path's bytes differ from those the workspace
// recorded for it, a file it has not recorded included.
func (s *Server) changedOnDisk(path string) bool {
	held, ok := s.ws.OnDisk(path)
	if !ok {
		return true
	}
	_, content, err := project.ReadFile(path)
	return err != nil || !bytes.Equal(content, held)
}

// linksWithin reports whether the symlink at path resolves to a file under
// folder: a link out of the workspace is not the workspace's to index, so a
// query over the whole workspace shows nothing a path argument would refuse.
func linksWithin(path, folder string) bool {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	root, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return false
	}
	return underFolders(target, []string{root})
}

// underFolders reports whether name lies in one of the folders.
func underFolders(name string, folders []string) bool {
	for _, folder := range folders {
		if rel, err := filepath.Rel(folder, name); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// loadFilesFromDisk is loadFromDisk over the paths as one batch of the workspace.
func (s *Server) loadFilesFromDisk(paths []string) {
	inputs := make([]model.Input, 0, len(paths))
	for _, path := range paths {
		if content, ok := s.readFromDisk(path); ok {
			inputs = append(inputs, model.Input{Name: path, Content: content})
		}
	}
	s.ws.SetOnDiskAll(inputs)
}

// skipDir reports whether a directory holds no model sources worth indexing.
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || name == "node_modules"
}

// loadFromDisk records a file's bytes as its on-disk content, reindexing it
// unless an open buffer is authoritative. A file that is gone, or too large to
// serve, is forgotten; any other read failure leaves what was indexed in place,
// since it may be transient.
func (s *Server) loadFromDisk(path string) {
	if content, ok := s.readFromDisk(path); ok {
		s.ws.SetOnDisk(path, content)
	}
}

// readFromDisk reads a file's bytes for the workspace, forgetting a file that is
// gone or too large to serve; it reports whether there is content to record.
func (s *Server) readFromDisk(path string) ([]byte, bool) {
	// A member of a project archive is read from the archive, in memory.
	if _, _, ok := project.SplitMemberPath(path); ok {
		_, content, err := project.ReadFile(path)
		if err != nil || len(content) > maxScannedFileSize {
			s.ws.DeleteOnDisk(path)
			return nil, false
		}
		return content, true
	}
	if info, err := os.Stat(path); err == nil && info.Size() > maxScannedFileSize {
		s.ws.DeleteOnDisk(path)
		return nil, false
	}
	// #nosec G304 -- path comes from the folder the client asked to serve.
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.ws.DeleteOnDisk(path)
		}
		return nil, false
	}
	return content, true
}

// DidChangeWatchedFiles reindexes model files changed outside the editor.
// A deletion leaves an open buffer alone; it is still authoritative. A project
// interchange file changed — a usage added, a project installed or removed —
// has the projects the folders use found again.
func (s *Server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	var sources []*protocol.FileEvent
	projects := false
	for _, event := range params.Changes {
		name := uriToName(event.URI)
		switch {
		case model.IsModelSource(name):
			sources = append(sources, event)
		case filepath.Base(name) == project.ProjectFile, project.IsArchive(name):
			projects = true
		}
	}
	if projects {
		s.reloadDependencies(ctx)
	}
	if len(sources) == 0 {
		return nil
	}
	s.debugEdit(ctx, "", func() {
		for _, event := range sources {
			if event.Type == protocol.FileChangeTypeDeleted {
				s.ws.DeleteOnDisk(uriToName(event.URI))
			} else {
				s.loadFromDisk(uriToName(event.URI))
			}
		}
	})
	for _, event := range sources {
		if event.Type == protocol.FileChangeTypeDeleted {
			s.publishDiagnostics(ctx, uriToName(event.URI))
		}
	}
	s.refreshOpenDiagnostics(ctx, "")
	return nil
}

// DidChangeWorkspaceFolders indexes the model sources a folder added to the
// session holds, and drops those a removed folder contributed. The file watcher
// reports changes only, so a newly added folder has to be walked here.
func (s *Server) DidChangeWorkspaceFolders(ctx context.Context, params *protocol.DidChangeWorkspaceFoldersParams) error {
	s.debugEdit(ctx, "", func() {
		for _, folder := range params.Event.Removed {
			s.dropFolder(uriToName(protocol.DocumentURI(folder.URI)))
		}
		for _, folder := range params.Event.Added {
			s.addFolder(uriToName(protocol.DocumentURI(folder.URI)))
		}
		// A document whose folder was just removed is now a lone file.
		for _, name := range s.ws.OpenNames() {
			s.indexOpenedDirectory(name)
		}
	})
	s.refreshOpenDiagnostics(ctx, "")
	return nil
}

// maxOpenedDirs bounds the directories the scan for an opened document's
// siblings visits: the document's directory was not chosen as a workspace, and
// may be a home directory.
const maxOpenedDirs = 2000

// indexOpenedDirectory indexes the directory of a document opened outside every
// folder, so sibling imports resolve; it is rescanned once all its documents
// close. The filesystem root is never indexed: walking it can take minutes.
func (s *Server) indexOpenedDirectory(name string) {
	if !filepath.IsAbs(name) {
		return
	}
	dir := projectOrDir(name)
	if dir == filepath.VolumeName(name)+string(filepath.Separator) {
		return
	}
	s.mu.Lock()
	if underAnyFolder(name, s.folders) || s.openDirs[dir] {
		s.mu.Unlock()
		return
	}
	if s.openDirs == nil {
		s.openDirs = map[string]bool{}
	}
	s.openDirs[dir] = true
	s.mu.Unlock()
	s.scanFolder(dir, maxOpenedDirs)
}

// releaseOpenedDirectories forgets every indexed directory no open document lies
// under any more, with the siblings only it contributed, so the next document
// opened there is scanned afresh and a deleted sibling does not linger. The
// document just closed stays indexed, as one closed under a folder does.
func (s *Server) releaseOpenedDirectories(closed string) {
	open := s.ws.OpenNames()
	s.mu.Lock()
	var released, kept []string
	for dir := range s.openDirs {
		if underAnyOpen(dir, open) {
			kept = append(kept, dir)
		} else {
			released = append(released, dir)
			delete(s.openDirs, dir)
		}
	}
	folders := append(kept, s.folders...)
	s.mu.Unlock()
	if len(released) == 0 {
		return
	}
	still := s.dropDependencies(released, folders)
	for _, name := range s.ws.DocumentNames() {
		if name != closed && underAnyFolder(name, released) && !underAnyFolder(name, folders) && !s.ws.IsOpen(name) && !still[name] {
			s.ws.DeleteOnDisk(name)
		}
	}
}

// underAnyOpen reports whether one of the open documents lies inside dir.
func underAnyOpen(dir string, open []string) bool {
	for _, name := range open {
		if underFolder(name, dir) {
			return true
		}
	}
	return false
}

// addFolder records a folder and indexes the model sources under it.
func (s *Server) addFolder(folder string) {
	if folder == "" {
		return
	}
	s.mu.Lock()
	for _, known := range s.folders {
		if known == folder {
			s.mu.Unlock()
			return
		}
	}
	s.folders = append(s.folders, folder)
	s.mu.Unlock()
	s.loadFolder(folder)
}

// dropFolder forgets a folder and the documents only it contributed; folders can
// nest, and an open buffer stays regardless, since the editor still shows it.
func (s *Server) dropFolder(folder string) {
	if folder == "" {
		return
	}
	s.mu.Lock()
	kept := make([]string, 0, len(s.folders))
	for _, known := range s.folders {
		if known != folder {
			kept = append(kept, known)
		}
	}
	s.folders = kept
	s.mu.Unlock()

	// A source another folder's project uses stays, wherever it lies.
	still := s.dropDependencies([]string{folder}, kept)
	for _, name := range s.ws.DocumentNames() {
		if underFolder(name, folder) && !underAnyFolder(name, kept) && !still[name] {
			s.ws.DeleteOnDisk(name)
		}
	}
}

// underAnyFolder reports whether name lies inside one of the folders.
func underAnyFolder(name string, folders []string) bool {
	for _, folder := range folders {
		if underFolder(name, folder) {
			return true
		}
	}
	return false
}

// underFolder reports whether name lies inside folder.
func underFolder(name, folder string) bool {
	rel, err := filepath.Rel(folder, name)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// refreshOpenDiagnostics republishes diagnostics for the open documents, whose
// analysis depends on the rest of the workspace, skipping the name a caller has
// just published on its own.
func (s *Server) refreshOpenDiagnostics(ctx context.Context, except string) {
	for _, name := range s.ws.OpenNames() {
		if name != except {
			// A tab closed mid-sweep has had its markers withdrawn already.
			s.publishOpenDiagnostics(ctx, name)
		}
	}
	s.publishFindings(ctx)
}

// queueOpenDiagnostics is refreshOpenDiagnostics once an editor burst settles;
// a refresh re-analyzes every open document the edit reached and republishes
// the others from the workspace's cache, still too much to pay per keystroke.
func (s *Server) queueOpenDiagnostics(ctx context.Context, except string) {
	if s.crossDoc == nil {
		s.refreshOpenDiagnostics(ctx, except)
		return
	}
	// The sweep outlives the notification, whose context is cancelled on return.
	ctx = context.WithoutCancel(ctx)
	s.crossDoc.Trigger(except, func() { s.refreshOpenDiagnostics(ctx, except) })
}
