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

// buildID identifies the code that produced a record. It is part of every cache
// key, so a record built by different code is a miss rather than a stale hit. It
// is hashed because it ends up in a file name.
var buildID = sync.OnceValue(func() string {
	sum := sha256.Sum256([]byte(currentBuildID()))
	return hex.EncodeToString(sum[:8])
})

// currentBuildID prefers the build ID the linker wrote into the running
// executable: the toolchain derives it from every compiled input, the embedded
// library included, so builds of the same code share it and any other build —
// a dirty tree, a worktree at the same commit, a patched resolver — gets another.
// Without one it falls back to an identity from build metadata, then to one that
// changes with each build.
func currentBuildID() string {
	exe, exeErr := os.Executable()
	if exeErr == nil {
		if id, err := linkerBuildID(exe); err == nil && id != "" {
			return "l" + id
		}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if id, ok := buildIDFromInfo(info); ok {
			return id
		}
	}
	// A dirty or unstamped build: the executable's own size and build time stand
	// in, so a rebuild is always a distinct key.
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

// buildIDFromInfo derives an identity from build metadata, reporting false when
// the build is not identified by it. A modified working tree is never identified:
// its revision, and the `<pseudo-version>+dirty` module version the toolchain
// stamps from it, are shared by every edit of that commit, which is exactly the
// stale hit the identity exists to prevent.
func buildIDFromInfo(info *debug.BuildInfo) (string, bool) {
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
	if revision != "" {
		return "g" + revision, true
	}
	if v := info.Main.Version; v != "" && v != "(devel)" && !strings.HasSuffix(v, "+dirty") {
		return "m" + v, true
	}
	return "", false
}
