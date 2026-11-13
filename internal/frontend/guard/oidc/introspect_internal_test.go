package oidc

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/guard"
)

func TestIntrospectionCacheDropsExpiredTokens(t *testing.T) {
	now := time.Now()
	i := newIntrospector(Config{Now: func() time.Time { return now }})
	key := func(n int) [sha256.Size]byte { return sha256.Sum256(fmt.Appendf(nil, "token-%d", n)) }
	for n := 0; n < sweepFloor-1; n++ {
		i.keep(key(n), guard.Principal{Subject: "s", Expires: now.Add(time.Minute)}, now)
	}
	if len(i.cache) != sweepFloor-1 {
		t.Fatalf("cache holds %d, want %d", len(i.cache), sweepFloor-1)
	}
	// Every token so far has expired; the entry that fills the cache sweeps them.
	now = now.Add(2 * time.Minute)
	i.keep(key(sweepFloor), guard.Principal{Subject: "s", Expires: now.Add(time.Minute)}, now)
	if len(i.cache) != 1 {
		t.Fatalf("cache holds %d after the sweep, want 1", len(i.cache))
	}
	// Live entries survive a sweep and set the next one at twice their count.
	for n := 1; n < sweepFloor; n++ {
		i.keep(key(1000+n), guard.Principal{Subject: "s", Expires: now.Add(time.Hour)}, now)
	}
	if len(i.cache) != sweepFloor || i.sweepAt != 2*sweepFloor {
		t.Fatalf("cache holds %d, next sweep at %d", len(i.cache), i.sweepAt)
	}
}
