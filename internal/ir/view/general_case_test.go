package view

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// caseRouteModel declares one view over use cases; the import and the view's
// definition are filled in so a GeneralView and a CaseView share every line.
const caseRouteModel = `package VehicleUseCases {
	private import %s::*;

	part def Vehicle;
	part def Person;
	use case def 'Add Fuel' {
		subject vehicle : Vehicle;
		actor fueler : Person;
	}
	use case def 'Enter Vehicle' {
		subject vehicle : Vehicle;
		actor driver : Person;
	}
	use case 'Provide Transportation' : 'Enter Vehicle' {
		subject vehicle : Vehicle;
		actor driver : Person;
		objective {
			doc /* Transport the vehicle and its passengers safely. */
		}
		include use case addFuel : 'Add Fuel'[0..*] {
			subject vehicle;
			actor fueler = driver;
		}
		use case 'Drive Vehicle' {
			subject vehicle;
			actor driver = 'Provide Transportation'::driver;
		}
	}
	analysis def FuelAnalysis {
		subject vehicle : Vehicle;
		objective {
			doc /* Estimate the fuel remaining after a journey. */
		}
	}
	view diagram : %s {
		%s
	}
}`

// renderCaseRoute renders VehicleUseCases::diagram declared as viewDef with
// members, importing the library that declares viewDef.
func renderCaseRoute(t *testing.T, library, viewDef, members string) (*Rendering, Options) {
	t.Helper()
	text := fmt.Sprintf(caseRouteModel, library, viewDef, members)
	r, idx := loadSources(t, []string{"cases.sysml"}, [][]byte{[]byte(text)})
	rendering, err := r.Render(lookup(t, idx, "VehicleUseCases::diagram"))
	if err != nil {
		t.Fatalf("render %s: %v", viewDef, err)
	}
	return rendering, Options{Links: Links{
		Template: "https://example.test/src/{file}#L{line}",
		Sites: r.Sites(FileLocator(r.model, func(string) *source.LineIndex {
			return source.NewLineIndex([]byte(text))
		})),
	}}
}

// A GeneralView whose filter names a case metaclass is the case rendering a
// CaseView exposing the same elements draws: every form writes the same bytes,
// links included, but for the provenance each states.
func TestGeneralViewCaseRouteMatchesCaseView(t *testing.T) {
	cases := []struct {
		name, general, caseView string
	}{
		{
			"filtered use case",
			"filter @SysML::UseCaseUsage; expose VehicleUseCases::'Provide Transportation';",
			"expose VehicleUseCases::'Provide Transportation';",
		},
		{
			"filtered expose",
			"expose VehicleUseCases::*[@SysML::CaseUsage or @SysML::CaseDefinition];",
			"expose VehicleUseCases::*[@SysML::CaseUsage or @SysML::CaseDefinition];",
		},
		{
			"specialized metaclass",
			"filter @SysML::AnalysisCaseDefinition; expose VehicleUseCases::FuelAnalysis;",
			"expose VehicleUseCases::FuelAnalysis;",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			general, generalLinks := renderCaseRoute(t, "StandardViewDefinitions", "GeneralView", tc.general)
			caseView, caseLinks := renderCaseRoute(t, "OpenSysMLRenderings", "CaseView", tc.caseView)
			if general.Kind != KindCase || caseView.Kind != KindCase {
				t.Fatalf("kinds = %q and %q, want %q", general.Kind, caseView.Kind, KindCase)
			}
			if !strings.HasPrefix(general.Stated, "view def GeneralView, filter @") || caseView.Stated == general.Stated {
				t.Fatalf("stated = %q beside %q", general.Stated, caseView.Stated)
			}
			for _, form := range []Form{FormText, FormMermaid, FormDot, FormPlantUML} {
				g, err := general.WriteWith(form, generalLinks)
				if err != nil {
					t.Fatal(err)
				}
				c, err := caseView.WriteWith(form, caseLinks)
				if err != nil {
					t.Fatal(err)
				}
				if form != FormText && !strings.Contains(g, "https://example.test/src/cases.sysml#L") {
					t.Errorf("%s has no source links:\n%s", form, g)
				}
				if !strings.Contains(g, general.Stated) {
					t.Errorf("%s does not state %q:\n%s", form, general.Stated, g)
				}
				if got := strings.ReplaceAll(g, general.Stated, caseView.Stated); got != c {
					t.Errorf("%s differs beyond the stated provenance\nGeneralView:\n%s\nCaseView:\n%s", form, g, c)
				}
			}
		})
	}
}
