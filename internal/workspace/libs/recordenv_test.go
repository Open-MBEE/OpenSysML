package libs

import (
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
