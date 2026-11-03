package libs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// buildID identifies the code that produced a record; it is in every cache key,
// so a record of other code is a miss. Hashed because it ends up in a file name.
var buildID = sync.OnceValue(func() string {
	sum := sha256.Sum256([]byte(currentBuildID()))
	return hex.EncodeToString(sum[:8])
})

// currentBuildID identifies the running build from its metadata and the build ID
// the linker wrote into the executable; see buildIdentity for the order.
func currentBuildID() string {
	info, hasInfo := debug.ReadBuildInfo()
	if !hasInfo {
		info = &debug.BuildInfo{}
	}
	exe, exeErr := os.Executable()
	var linkerID string
	if exeErr == nil {
		linkerID, _ = linkerBuildID(exe)
	}
	if id, ok := buildIdentity(info, linkerID); ok {
		return id
	}
	// Unstamped and unreadable: the executable's own size and build time stand in.
	if exeErr == nil {
		if fi, err := os.Stat(exe); err == nil {
			return "x" + strconv.FormatInt(fi.Size(), 10) + "-" + strconv.FormatInt(fi.ModTime().UnixNano(), 10)
		}
	}
	// Provenance unknown: a per-process value, so no record is ever reused.
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "p" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "p" + hex.EncodeToString(b[:])
}

// buildIdentity is a clean checkout's revision, shared by its builds whatever the link
// stamped; else the linker's build ID, which any compiled input or link flag changes.
func buildIdentity(info *debug.BuildInfo, linkerID string) (string, bool) {
	if rev, ok := cleanRevision(info); ok {
		return "g" + rev, true
	}
	if linkerID != "" {
		return "l" + linkerID, true
	}
	if v, ok := releasedVersion(info); ok {
		return "m" + v, true
	}
	return "", false
}

// buildIDFromInfo identifies a build from its metadata alone.
func buildIDFromInfo(info *debug.BuildInfo) (string, bool) {
	return buildIdentity(info, "")
}

// cleanRevision is the VCS revision of an unmodified working tree. A modified
// tree has none: its revision is shared by every edit of that commit.
func cleanRevision(info *debug.BuildInfo) (string, bool) {
	var revision string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				return "", false
			}
		}
	}
	return revision, revision != ""
}

// releasedVersion is the main module's version when it names a release; a
// `<pseudo-version>+dirty` stamp is shared by every edit of a commit and refused.
func releasedVersion(info *debug.BuildInfo) (string, bool) {
	v := info.Main.Version
	if v == "" || v == "(devel)" || strings.HasSuffix(v, "+dirty") {
		return "", false
	}
	return v, true
}
