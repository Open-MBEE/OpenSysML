package main

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

func TestNewSessionTracingFromFlag(t *testing.T) {
	if newSession().Tracing() {
		t.Error("tracing is on without -trace")
	}
	traceMode = true
	defer func() { traceMode = false }()
	if !newSession().Tracing() {
		t.Error("-trace did not turn tracing on")
	}
}

func TestNewSessionVerbosityFromFlags(t *testing.T) {
	tests := []struct {
		name         string
		debug, quiet bool
		want         repl.Verbosity
	}{
		{"default", false, false, repl.VerbosityNormal},
		{"debug", true, false, repl.VerbosityDebug},
		{"quiet", false, true, repl.VerbosityQuiet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			debugMode, quietMode = tt.debug, tt.quiet
			defer func() { debugMode, quietMode = false, false }()
			if got := newSession().Verbosity(); got != tt.want {
				t.Errorf("verbosity = %v, want %v", got, tt.want)
			}
		})
	}
}
