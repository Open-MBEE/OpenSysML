package libs

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
)

// RecordCacheEnvVar switches the interface-record cache off when set to 0 (or
// false/off/no): every document is then parsed and held loaded, and none of
// their records is written.
const RecordCacheEnvVar = "OPENSYSML_RECORD_CACHE"

// RecordCacheFromEnv reports whether the environment leaves the record cache
// on, which it does unless RecordCacheEnvVar switches it off.
func RecordCacheFromEnv() bool {
	return recordCacheFromValue(envvar.Lookup(RecordCacheEnvVar))
}

// recordCacheFromValue reads the switch's value: unset or empty leaves it on.
func recordCacheFromValue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// OpenRecordCache is the record cache a command holds its closed documents
// through when the flag and the environment leave it on: the shared cache
// directory, nil when either switches it off or the directory cannot be made
// (the reason returned), in which case every document is held loaded.
func OpenRecordCache(noCache bool) (*Cache, error) {
	if noCache || !RecordCacheFromEnv() {
		return nil, nil
	}
	return NewCache()
}
