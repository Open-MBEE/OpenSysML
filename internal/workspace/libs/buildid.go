package libs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime/debug"
	"strconv"
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

// currentBuildID prefers an identity shared by every build of the same code — a
// clean VCS revision or a released module version — and falls back to one that
// changes with each build.
func currentBuildID() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if id, ok := buildIDFromInfo(info); ok {
			return id
		}
	}
	// A dirty or unstamped build: the executable's own size and build time stand
	// in, so a rebuild is always a distinct key.
	if exe, err := os.Executable(); err == nil {
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
// the build is not identified by it: a modified working tree reuses one revision
// for every edit, which is exactly the stale hit the identity exists to prevent.
func buildIDFromInfo(info *debug.BuildInfo) (string, bool) {
	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision != "" && !modified {
		return "g" + revision, true
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return "m" + v, true
	}
	return "", false
}
