package analysis

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// testBuilds holds the commands this package's tests build and drive as processes.
var testBuilds = gobuild.Scratch{Pattern: "analysis-builds"}

// TestMain removes the directory the commands under test were built into.
func TestMain(m *testing.M) { testBuilds.Main(m) }
