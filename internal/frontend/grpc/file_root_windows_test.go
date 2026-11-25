package grpc

import (
	"slices"
	"testing"
)

// A root may be configured by a name on one drive that leads to a directory on
// another, so a path is matched against each name on that name's own drive.
func TestElementsBelowARootOnItsOwnDrive(t *testing.T) {
	for _, tc := range []struct {
		root, path string
		rest       []string
		ok         bool
	}{
		{`D:\models`, `D:\models\m.sysml`, []string{"m.sysml"}, true},
		{`D:\models`, `D:\\models\sub\m.sysml`, []string{"sub", "m.sysml"}, true},
		{`C:\data\models`, `D:\models\m.sysml`, nil, false},
		{`D:\models`, `C:\models\m.sysml`, nil, false},
		{`D:\models`, `D:\modelsx\m.sysml`, nil, false},
		{`\\host\share\models`, `\\host\share\models\m.sysml`, []string{"m.sysml"}, true},
		{`\\host\share\models`, `\\host\other\models\m.sysml`, nil, false},
	} {
		rest, ok := elementsBelow(tc.root, tc.path)
		if ok != tc.ok || !slices.Equal(rest, tc.rest) {
			t.Errorf("elementsBelow(%q, %q) = %q, %v, want %q, %v", tc.root, tc.path, rest, ok, tc.rest, tc.ok)
		}
	}
}
