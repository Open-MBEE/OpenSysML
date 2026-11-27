package main

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// testBuilds holds the commands this package's tests build and drive as processes.
var testBuilds = gobuild.Scratch{Pattern: "sysml-mcp-builds"}

// TestMain removes the directory the commands under test were built into.
func TestMain(m *testing.M) { testBuilds.Main(m) }
