package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestModifierOnlyUsageTakesDefaultValue covers a usage declared by a modifier
// alone whose name is followed straight by a default value (SysML v2 FeatureValue):
// `ref x default = 4;` is a reference usage with a default, not a stray name.
func TestModifierOnlyUsageTakesDefaultValue(t *testing.T) {
	for _, body := range []string{
		"ref x default = 4;",
		"ref x default := 4;",
		"ref x default 4;",
		"derived y default = 4;",
	} {
		t.Run(body, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte("part def A { "+body+" }")))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			name := "x"
			if body[0] == 'd' {
				name = "y"
			}
			u := findUsageNamed(root, name)
			if u == nil || u.Value == nil || !u.ValueIsDefault {
				t.Fatalf("usage %q = %+v; want a usage with a default value", name, u)
			}
		})
	}
}
