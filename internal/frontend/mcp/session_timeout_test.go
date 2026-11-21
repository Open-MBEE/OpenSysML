package mcp

import (
	"testing"
	"time"
)

func TestHTTPConfigSessionTimeout(t *testing.T) {
	for _, tc := range []struct{ set, want time.Duration }{
		{0, DefaultSessionTimeout},
		{time.Minute, time.Minute},
		{-1, 0},
	} {
		if got := (HTTPConfig{SessionTimeout: tc.set}).sessionTimeout(); got != tc.want {
			t.Errorf("SessionTimeout %s: handed the SDK %s, want %s", tc.set, got, tc.want)
		}
	}
	if DefaultSessionTimeout <= 0 {
		t.Error("the default leaves idle sessions open forever")
	}
}
