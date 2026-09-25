package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestRequirementWithDoc(t *testing.T) {
	input := `
	package Test {
		abstract requirement def R {
			subject subj : Thing[1] {
				doc /* subject doc */
			}
			
			abstract requirement subrequirements[0..*] {
				doc /* nested requirement doc */
			}
		}
	}
	`

	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	root := p.ParseFile()

	if root == nil {
		t.Fatal("ParseFile returned nil")
	}

	if len(p.Diagnostics) > 0 {
		t.Errorf("Expected no diagnostics, got %d:", len(p.Diagnostics))
		for _, d := range p.Diagnostics {
			t.Logf("  %s", d.Message)
		}
	}
}
