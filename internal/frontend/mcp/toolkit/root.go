package toolkit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// Root is the workspace folder every file a tool touches must lie under. A
// path is judged twice: as spelled, against Dir, and with its symlinks
// followed, against Real, so neither `..` nor a link planted under the root
// reaches a file outside it.
type Root struct {
	// Dir is the root as the server was given it, made absolute and clean.
	Dir string
	// Real is Dir with its symlinks resolved.
	Real string
}

// NewRoot makes the root of dir, which must exist: a relative dir is taken
// from the working directory.
func NewRoot(dir string) (Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Root{}, fmt.Errorf("resolving the workspace root %s: %w", dir, err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Root{}, fmt.Errorf("resolving the workspace root %s: %w", dir, err)
	}
	return Root{Dir: abs, Real: real}, nil
}

// Resolved is a path Root.Resolve accepted.
type Resolved struct {
	// Path is the path as spelled, absolute and clean: a relative path joined
	// to the root. It names the document to a tool and an index.
	Path string
	// Real is Path with its symlinks followed, the file an operation reaches;
	// for a path not yet on disk, the resolved nearest existing ancestor with
	// the rest of the path appended.
	Real string
}

// Option adjusts one resolution.
type Option func(*options)

type options struct {
	exempt []string
	role   NameRole
}

// Exempt lets a path pass when, symlinks followed, it lies under one of dirs
// (likewise followed), wherever they are: the library roots an installed
// project is read from, which live outside a workspace.
func Exempt(dirs ...string) Option {
	return func(o *options) { o.exempt = append(o.exempt, dirs...) }
}

// NameRole is what a tool reads from a path's name, which a symlink must then
// agree with its target about: a link spelled `.kerml` to a `.sysml` file would
// otherwise be read as the wrong language.
type NameRole int

const (
	// NameIgnored reads nothing from the name.
	NameIgnored NameRole = iota
	// NameLanguage reads the notation language (SysML or KerML) from the name.
	NameLanguage
	// NameFormat reads the model format and, for notation, the language.
	NameFormat
)

// NameMeans refuses a symlink whose name means another format or language
// than its target's for the role.
func NameMeans(role NameRole) Option {
	return func(o *options) { o.role = role }
}

// Resolve turns a tool's path argument into the file it names, which must lie
// under the root: as spelled, so `..` and an absolute path elsewhere are
// refused, and once the symlinks on the way are followed, so a link out of the
// root is refused too, a dangling one by where it points. A path that does not
// exist yet is checked through its nearest existing ancestor. A relative path
// is taken from the root.
func (r Root) Resolve(path string, opts ...Option) (Resolved, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return Resolved{}, errors.New("a path is required")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.Dir, path)
	}
	path = filepath.Clean(path)
	real, err := realPath(path)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolving %s: %w", path, err)
	}
	if o.role != NameIgnored && !sameFormat(path, real, o.role) {
		return Resolved{}, fmt.Errorf("%s is a link to %s, whose extension names another format; name the file itself or a link with its extension", path, real)
	}
	res := Resolved{Path: path, Real: real}
	for _, dir := range o.exempt {
		if realDir, err := filepath.EvalSymlinks(dir); err == nil && Under(realDir, real) {
			return res, nil
		}
	}
	if !Under(r.Dir, path) {
		return Resolved{}, fmt.Errorf("%s is outside the workspace root %s", path, r.Dir)
	}
	if !Under(r.Real, real) {
		return Resolved{}, fmt.Errorf("%s leads through a symlink to %s, outside the workspace root %s", path, real, r.Dir)
	}
	return res, nil
}

// Under reports whether path lies under root (or is root), both absolute and
// clean. It judges the spelling only: follow symlinks first where they matter.
func Under(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// maxLinkHops bounds the dangling links followed, as the symlink resolvers of
// the operating systems bound theirs.
const maxLinkHops = 40

// realPath follows the symlinks of the longest existing prefix of path and
// keeps the rest as written, so a file not yet created is placed as well, and
// so is a member named beneath an archive file. A dangling link on the way is
// followed to where it points, since a write through it lands there.
func realPath(path string) (string, error) {
	return realPathHops(path, 0)
}

func realPathHops(path string, hops int) (string, error) {
	rest := ""
	for p := path; ; {
		real, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(real, rest), nil
		}
		parent := filepath.Dir(p)
		if !(errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)) || parent == p {
			return "", err
		}
		if info, lerr := os.Lstat(p); lerr == nil && info.Mode()&fs.ModeSymlink != 0 {
			if hops >= maxLinkHops {
				return "", fmt.Errorf("%s: too many links", path)
			}
			target, rerr := os.Readlink(p)
			if rerr != nil {
				return "", rerr
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(parent, target)
			}
			return realPathHops(filepath.Join(target, rest), hops+1)
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// sameFormat reports whether two names mean the same thing for the role: the
// source kind the parser reads from each (a name with neither model extension
// is its own kind) and, for NameFormat, the format convert infers from each
// (or the same extension when it infers nothing).
func sameFormat(a, b string, role NameRole) bool {
	if source.KindOf(a) != source.KindOf(b) {
		return false
	}
	if role == NameLanguage {
		return true
	}
	fa, errA := convert.FormatOfPath(a)
	fb, errB := convert.FormatOfPath(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(filepath.Ext(a), filepath.Ext(b))
	}
	return fa == fb
}
