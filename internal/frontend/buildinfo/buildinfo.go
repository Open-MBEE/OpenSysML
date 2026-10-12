// Package buildinfo resolves what a binary reports about its own build: the
// version, commit, build time and Go version. The linker's -X stamps win when a
// build passed them (the Makefile and the release build do); otherwise the
// module version and VCS metadata the Go toolchain embeds stand in, which is
// what a `go install` build has; and a build with neither reports the defaults.
package buildinfo

import (
	"fmt"
	"regexp"
	"runtime"
	"runtime/debug"
	"time"
)

const (
	// Unversioned is the version of a build no release tag identifies.
	Unversioned = "dev"
	// Unknown is a commit, build time or Go version nothing recorded.
	Unknown = "unknown"
	// Dirty is appended to the commit of a build from a modified working tree,
	// as `git describe --dirty` appends it to the Makefile's version.
	Dirty = "-dirty"
	// buildTimeLayout is the Makefile's BUILD_TIME format.
	buildTimeLayout = "2006-01-02_15:04:05"
	// shortRevision is the length a commit hash is reported at: the twelve
	// characters a Go pseudo-version carries.
	shortRevision = 12
)

// Stamps are the values the linker's -X flags wrote into a main package's
// Version, Commit, BuildTime and GoVersion, or their defaults when a build
// passed none; the defaults, like an empty string, count as unset.
type Stamps struct {
	Version   string
	Commit    string
	BuildTime string
	GoVersion string
}

// Info is what a binary reports about its build.
type Info struct {
	// Version is the release tag the build is of, or Unversioned.
	Version string
	// Commit is the short commit hash, Dirty appended for a modified tree, or Unknown.
	Commit string
	// BuildTime is when the build happened, or the commit's time when only that
	// is recorded, in the Makefile's format; Unknown when neither is.
	BuildTime string
	// GoVersion is the toolchain that built the binary.
	GoVersion string
}

// Resolve reports the running binary's build from stamps and the build
// information the toolchain embedded in it.
func Resolve(stamps Stamps) Info {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	return ResolveFrom(stamps, info)
}

// ResolveFrom reports a build from stamps and the build information embedded in
// it, which may be nil for a binary the toolchain recorded nothing about. A
// stamp that is set wins; what the build information says fills the rest.
func ResolveFrom(stamps Stamps, info *debug.BuildInfo) Info {
	out := Info{Version: Unversioned, Commit: Unknown, BuildTime: Unknown, GoVersion: runtime.Version()}
	if info != nil {
		if Released(info.Main.Version) {
			out.Version = info.Main.Version
		}
		var revision, when string
		modified := false
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.time":
				when = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
		if revision != "" {
			out.Commit = ShortRevision(revision)
			if modified {
				out.Commit += Dirty
			}
		}
		if when != "" {
			out.BuildTime = formatTime(when)
		}
		if info.GoVersion != "" {
			out.GoVersion = info.GoVersion
		}
	}
	if set(stamps.Version, Unversioned) {
		out.Version = stamps.Version
	}
	if set(stamps.Commit, Unknown) {
		out.Commit = stamps.Commit
	}
	if set(stamps.BuildTime, Unknown) {
		out.BuildTime = stamps.BuildTime
	}
	if set(stamps.GoVersion, Unknown) {
		out.GoVersion = stamps.GoVersion
	}
	return out
}

// Report is the version block a command prints for -version: name and version
// on the first line, the commit, build time and Go version aligned beneath.
func (i Info) Report(name string) string {
	return fmt.Sprintf("%s %s\n  Commit:     %s\n  Build time: %s\n  Go version: %s\n",
		name, i.Version, i.Commit, i.BuildTime, i.GoVersion)
}

// set reports whether a stamp carries a value: neither empty nor its default.
func set(stamp, fallback string) bool {
	return stamp != "" && stamp != fallback
}

// Stamped reports whether the toolchain recorded a version for a module: it
// leaves "" for a build outside module mode and "(devel)" for the main module of
// an unversioned build or a directory replacement.
func Stamped(version string) bool {
	return version != "" && version != "(devel)"
}

// pseudoVersion matches the versions the toolchain derives from a commit, in
// the shape golang.org/x/mod/module documents: a base version, a timestamp and
// a revision.
var pseudoVersion = regexp.MustCompile(`^v[0-9]+\.(0\.0-|\d+\.\d+-([^+]*\.)?0\.)\d{14}-[A-Za-z0-9]+(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// semanticVersion matches a tag's version: vMAJOR.MINOR.PATCH with an optional
// pre-release and build metadata, "+dirty" included.
var semanticVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// Released reports whether a module version names a release tag: one the
// toolchain stamped that is a semantic version and not a pseudo-version, which
// identifies a commit rather than a release.
func Released(version string) bool {
	return Stamped(version) && semanticVersion.MatchString(version) && !pseudoVersion.MatchString(version)
}

// ShortRevision abbreviates a commit hash to the length it is reported at.
func ShortRevision(revision string) string {
	if len(revision) > shortRevision {
		return revision[:shortRevision]
	}
	return revision
}

// formatTime rewrites the RFC 3339 time the toolchain records in the Makefile's
// format, UTC; a value in another shape is reported as it is.
func formatTime(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format(buildTimeLayout)
}
