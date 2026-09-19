package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestSysMLMetadataExample(t *testing.T) {
	ws := NewWorkspace()

	// From training example "39. Metadata/Metadata Example-1.sysml"
	src := `package Test {
		metadata def SecurityFeature {
			:> annotatedElement : SysML::PartDefinition;
			:> annotatedElement : SysML::PartUsage;
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	var unresolvedCount int
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			t.Logf("ERROR: %s", d.Message)
			if d.Message == "unresolved reference: SysML::PartDefinition" ||
				d.Message == "unresolved reference: SysML::PartUsage" ||
				d.Message == "unresolved reference: SysML::Usage" {
				unresolvedCount++
			}
		}
	}

	if unresolvedCount > 0 {
		t.Errorf("Found %d unresolved SysML metaobject references", unresolvedCount)
	}
}
