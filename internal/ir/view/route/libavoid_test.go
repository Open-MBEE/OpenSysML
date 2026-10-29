package route

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// The embedded module is the one scripts/build-libavoid-wasm.sh builds from
// the pinned libavoid-js release, and the script still pins that release.
func TestEmbeddedLibavoidIsThePinnedBuild(t *testing.T) {
	const want = "c11a35dbd53a0e571b04d4786a397b520249f223fb73dc3ce59b77af1d97a76a"
	sum := sha256.Sum256(libavoidWasm)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("libavoid.wasm sha256 %s, want %s; rebuild it with scripts/build-libavoid-wasm.sh and update both", got, want)
	}
	script, err := os.ReadFile("../../../../scripts/build-libavoid-wasm.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, pin := range []string{`LIBAVOID_JS_VERSION="0.4.5"`, `BINARYEN_VERSION="132.0.0"`, "--translate-to-exnref"} {
		if !strings.Contains(string(script), pin) {
			t.Errorf("build-libavoid-wasm.sh no longer states %s", pin)
		}
	}
}

// A module named by OPENSYSML_LIBAVOID_WASM that is not a libavoid build is refused, not run.
func TestOverriddenModuleMustCompile(t *testing.T) {
	path := t.TempDir() + "/not-wasm"
	if err := os.WriteFile(path, []byte("not a module"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(WasmPathEnv, path)
	if _, err := newEngine(t.Context()); err == nil {
		t.Fatal("a non-module loaded")
	}
}
