package libs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordCacheFromValueIsOnUnlessSwitchedOff(t *testing.T) {
	for _, raw := range []string{"", " ", "1", "true", "on", "yes", "anything"} {
		if !recordCacheFromValue(raw) {
			t.Errorf("%q switches the record cache off, want on", raw)
		}
	}
	for _, raw := range []string{"0", "false", "off", "no", " OFF ", "No", "FALSE"} {
		if recordCacheFromValue(raw) {
			t.Errorf("%q leaves the record cache on, want off", raw)
		}
	}
}

func TestOpenRecordCacheHonoursTheFlagAndTheEnvironment(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv(RecordCacheEnvVar, "")
	cache, err := OpenRecordCache(false)
	if err != nil {
		t.Fatal(err)
	}
	if cache == nil {
		t.Fatal("flag and environment leave the record cache on, got none")
	}
	if want := filepath.Join(base, "sysml-ls", "libs"); cache.dir != want {
		t.Errorf("record cache directory %q, want the library cache's %q", cache.dir, want)
	}
	if cache, err := OpenRecordCache(true); err != nil || cache != nil {
		t.Errorf("-no-record-cache gave cache %v, err %v; want none, nil", cache, err)
	}
	t.Setenv(RecordCacheEnvVar, "0")
	if cache, err := OpenRecordCache(false); err != nil || cache != nil {
		t.Errorf("%s=0 gave cache %v, err %v; want none, nil", RecordCacheEnvVar, cache, err)
	}
}

// Without a cache root — nothing names one — the command holds every file
// loaded and says nothing: that is how the process was started, not a failure.
func TestOpenRecordCacheIsQuietWithoutACacheRoot(t *testing.T) {
	t.Setenv(RecordCacheEnvVar, "")
	for _, name := range []string{"XDG_CACHE_HOME", "HOME", "LocalAppData", "home"} {
		t.Setenv(name, "")
	}
	if _, err := NewCache(); !errors.Is(err, ErrNoCacheRoot) {
		t.Fatalf("NewCache without a root: %v, want ErrNoCacheRoot", err)
	}
	cache, err := OpenRecordCache(false)
	if err != nil || cache != nil {
		t.Errorf("no cache root gave cache %v, err %v; want none, nil", cache, err)
	}
}

// A root that exists and refuses the directory is a failure the command reports.
func TestOpenRecordCacheReportsARootThatRefuses(t *testing.T) {
	t.Setenv(RecordCacheEnvVar, "")
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", file)
	cache, err := OpenRecordCache(false)
	if err == nil || cache != nil {
		t.Fatalf("a root that refuses gave cache %v, err %v; want none and the reason", cache, err)
	}
	if errors.Is(err, ErrNoCacheRoot) {
		t.Errorf("a root that refuses was taken for no root: %v", err)
	}
}
