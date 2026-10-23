package convert_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

func TestCameoV5BlockMigrationUsesStandardNamespace(t *testing.T) {
	for _, tc := range []struct {
		ns       string
		standard bool
	}{
		{"http:///SysML/Blocks.ecore", true},
		{"http:///Org/Blocks.ecore", false},
		{"https://evil.example/SysML/Blocks.ecore", false},
	} {
		data := fmt.Sprintf(`<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmlns:blocks="%s"><uml:Model xmi:id="m"><packagedElement xmi:type="uml:Class" xmi:id="c" name="Controller"/></uml:Model><blocks:Block xmi:id="b" base_Class="c"/></xmi:XMI>`, tc.ns)
		result, err := convert.Migrate("cameo-v5.xmi", []byte(data), convert.FormatSysML, convert.MigrateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(string(result.Output), "part def Controller"); got != tc.standard {
			t.Errorf("namespace %q: standard=%v output=%s", tc.ns, tc.standard, result.Output)
		}
	}
}
