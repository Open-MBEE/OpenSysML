package libs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
)

// TestEmbedLoadEndToEnd: default embedded source + real cache dir under a temp
// XDG_CACHE_HOME loads the bundled library and answers qualified lookups.
func TestEmbedLoadEndToEnd(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cache, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	ld := NewLoader(DefaultSource(), cache)
	idx := symbols.NewIndex()
	if err := ld.load("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml", idx); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(idx.LookupQualified("ScalarValues::Real")) != 1 {
		t.Fatal("ScalarValues::Real not indexed via embedded end-to-end load")
	}
}

// TestSysmlLibraryPathOverride: OPENSYSML_LIBRARY_PATH points DefaultSource at an
// on-disk directory, and the loader indexes that custom library instead.
func TestSysmlLibraryPathOverride(t *testing.T) {
	libDir, err := filepath.Abs("testdata/customlib")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(LibraryPathEnvVar, libDir)
	cache := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), cache)
	idx := symbols.NewIndex()
	if err := ld.load("Custom.kerml", idx); err != nil {
		t.Fatalf("Load custom: %v", err)
	}
	if len(idx.LookupQualified("Custom::Widget")) != 1 {
		t.Fatal("OPENSYSML_LIBRARY_PATH override did not load Custom::Widget")
	}
	// The bundled library must NOT be visible under the override source.
	if _, err := DefaultSource().Read("Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"); err == nil {
		t.Fatal("override source unexpectedly served bundled ScalarValues.kerml")
	}
}

// TestSysmlLibraryPathLegacyName: the legacy SYSML_LIBRARY_PATH name still
// points DefaultSource at an on-disk directory, and the OPENSYSML_ name wins
// when both are set.
func TestSysmlLibraryPathLegacyName(t *testing.T) {
	libDir, err := filepath.Abs("testdata/customlib")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("legacy name", func(t *testing.T) {
		t.Setenv(LibraryPathEnvVar, "")
		t.Setenv(envvar.Legacy(LibraryPathEnvVar), libDir)
		cache := &Cache{dir: t.TempDir()}
		ld := NewLoader(DefaultSource(), cache)
		idx := symbols.NewIndex()
		if err := ld.load("Custom.kerml", idx); err != nil {
			t.Fatalf("Load custom: %v", err)
		}
		if len(idx.LookupQualified("Custom::Widget")) != 1 {
			t.Fatal("SYSML_LIBRARY_PATH fallback did not load Custom::Widget")
		}
	})

	t.Run("both set, new name wins", func(t *testing.T) {
		t.Setenv(LibraryPathEnvVar, libDir)
		t.Setenv(envvar.Legacy(LibraryPathEnvVar), t.TempDir())
		if _, err := DefaultSource().Read("Custom.kerml"); err != nil {
			t.Fatalf("OPENSYSML_LIBRARY_PATH did not win over the legacy name: %v", err)
		}
	})
}

// TestCacheStaleContentReparsedNotServed: a cache entry stored under stale
// content must not be served when the source content differs; the loader
// reparses and produces the correct current symbols.
func TestCacheStaleContentReparsedNotServed(t *testing.T) {
	libDir := t.TempDir()
	libFile := filepath.Join(libDir, "Evolving.kerml")
	if err := os.WriteFile(libFile, []byte("package Evolving { namespace First; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(LibraryPathEnvVar, libDir)
	cache := &Cache{dir: t.TempDir()}
	ld := NewLoader(DefaultSource(), cache)

	idx1 := symbols.NewIndex()
	if err := ld.load("Evolving.kerml", idx1); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if len(idx1.LookupQualified("Evolving::First")) != 1 {
		t.Fatal("first load missing Evolving::First")
	}

	// Change the file content: the old cache key no longer matches, so the
	// loader must reparse and index the NEW member, not serve the stale one.
	if err := os.WriteFile(libFile, []byte("package Evolving { namespace Second; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx2 := symbols.NewIndex()
	if err := ld.load("Evolving.kerml", idx2); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(idx2.LookupQualified("Evolving::Second")) != 1 {
		t.Fatal("stale cache served: Evolving::Second not indexed after content change")
	}
	if len(idx2.LookupQualified("Evolving::First")) != 0 {
		t.Fatal("stale cache served: Evolving::First should be gone after content change")
	}
}

// Two builds may disagree on a fact, as two resolvers patched differently do, so
// a record one build wrote is served to that build only: the other misses it
// and derives the fact from the source, with both reading one cache directory.
func TestRecordOfAnotherBuildIsNotServed(t *testing.T) {
	libDir := t.TempDir()
	content := []byte("package Lib { part def Engine; part def Motor; part e : Engine; }")
	if err := os.WriteFile(filepath.Join(libDir, "lib.sysml"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cache, err := NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	restore := buildID
	t.Cleanup(func() { buildID = restore })
	superOfE := func() string {
		idx := loadWholeLibrary(t, libDir, cache.dir)
		e := idx.LookupQualified("Lib::e")
		if len(e) != 1 || e[0].Facts == nil || len(e[0].Facts.Supers) != 1 {
			t.Fatalf("Lib::e = %+v, want one symbol with one supertype", e)
		}
		return e[0].Facts.Supers[0].FQN
	}

	buildID = func() string { return "build-a" }
	if got := superOfE(); got != "Lib::Engine" {
		t.Fatalf("cold load: e : %s, want Lib::Engine", got)
	}
	// Stand in for a build whose resolver answers differently: rewrite the fact
	// its record holds, which is what a warm load of that build then answers.
	key := cache.keyFor("lib.sysml", content, NewLoader(NewDirSource(libDir), nil).setDigest())
	rec, ok := cache.Load(key)
	if !ok {
		t.Fatal("the cold load stored no record for lib.sysml")
	}
	for i := range rec.Facts {
		if rec.Facts[i].FQN == "Lib::e" {
			rec.Facts[i].Supers[0].FQN = "Lib::Motor"
		}
	}
	if err := cache.Store(key, rec); err != nil {
		t.Fatalf("store: %v", err)
	}
	if got := superOfE(); got != "Lib::Motor" {
		t.Fatalf("warm load of the writing build: e : %s, want its record's Lib::Motor", got)
	}

	buildID = func() string { return "build-b" }
	if got := superOfE(); got != "Lib::Engine" {
		t.Fatalf("another build answered with the first build's record: e : %s", got)
	}
}
