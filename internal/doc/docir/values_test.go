package docir

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

const valueDocument = `
	private import ISQ::*;
	private import SI::*;
	enum def Phase { design; flight; }
	part def Mount { attribute mass :> ISQ::mass = 80 [kg]; attribute 'dry mass' :> ISQ::mass = 70 [kg]; }
	part def Telescope {
		part mount : Mount;
		attribute aperture : Real = 2.4;
		attribute designation : String = "Hubble";
		attribute flying : Boolean = true;
		attribute phase : Phase = Phase::flight;
		attribute bands : String[0..*] = ("optical", "ultraviolet");
		attribute margin : Real;
	}
	part telescope : Telescope;
	part def Report :> Document {
		attribute redefines title = "Report";
		rep language "markdown" /* Aperture [](sysml:value:telescope.aperture), named [](sysml:value:telescope.designation),
		 * flying [](sysml:value:telescope.flying), in [](sysml:value:telescope.phase),
		 * [Mount mass:](sysml:value:telescope.mount.mass), bands [](sysml:value:telescope.bands),
		 * dry [](<sysml:value:telescope.mount.'dry mass'>), via [](sysml:value:telescope.mount::mass).
		 *
		 * - [Nominal aperture](sysml:value:Telescope::aperture)
		 */
	}
	part def Empty :> Document {
		attribute redefines title = "Empty";
		rep language "markdown" /* Margin [](sysml:value:telescope.margin). */
	}
	part def Car { attribute rating : Real; }
	part def Fleet { part cars : Car[0..*]; }
	part fleet : Fleet;
	part car : Car;
	part spare : Car;
	part def FleetReport :> Document {
		attribute redefines title = "Fleet";
		rep language "markdown" /* Ratings [](sysml:value:fleet.cars.rating). */
	}
`

// A value run renders the feature as the same feature renders in a table cell:
// quantities with their unit, strings, booleans, enumeration literals and
// multi-valued features joined, behind the run's label when it states one.
func TestEvaluateValueRuns(t *testing.T) {
	fixture := loadEvaluationFixture(t, valueDocument)
	document := fixture.mustEvaluate(t, "Report")
	content := document.Content()
	if len(content) != 2 {
		t.Fatalf("content = %d blocks, want a paragraph and a list", len(content))
	}
	var b strings.Builder
	for _, run := range content[0].Runs() {
		if run.Kind() != RunPlain {
			t.Errorf("run %+v, want plain text", run)
		}
		b.WriteString(run.Text())
	}
	want := "Aperture 2.4, named Hubble, flying true, in flight, Mount mass: 80 [kg], bands optical, ultraviolet, dry 70 [kg], via 80 [kg]."
	if got := b.String(); got != want {
		t.Errorf("paragraph = %q, want %q", got, want)
	}
	items := content[1].Items()
	if len(items) != 1 || len(items[0].Runs()) != 1 || items[0].Runs()[0].Text() != "Nominal aperture 2.4" {
		t.Errorf("list items = %+v", items)
	}
	for _, run := range content[0].Runs() {
		if !run.Origin().Located() {
			t.Errorf("run %q has no origin", run.Text())
		}
	}
}

// A feature with no value is a typed error naming the paragraph's run and the
// feature, never a blank.
func TestEvaluateValueRunEmpty(t *testing.T) {
	fixture := loadEvaluationFixture(t, valueDocument)
	_, err := fixture.evaluate(t, "Empty")
	var evaluation *Error
	if !errors.As(err, &evaluation) || evaluation.Kind != ErrorEmptyValue {
		t.Fatalf("error = %v, want %s", err, ErrorEmptyValue)
	}
	if !strings.Contains(err.Error(), "Observatory::Telescope::margin, which has no value") || !evaluation.Origin.Located() {
		t.Errorf("error %v lacks the feature or an origin", err)
	}
}

// Over a session holding the chain's root, a value run reads the object's
// current value, as an Objects row does.
func TestEvaluateValueRunHeldObject(t *testing.T) {
	fixture := loadEvaluationFixture(t, valueDocument)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	telescope, err := ctx.Instantiate(fixture.symbol(t, "telescope"))
	if err != nil {
		t.Fatalf("Instantiate telescope: %v", err)
	}
	err = telescope.SetFeatureValue(ctx, "aperture", runtime.Value{
		Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 6.5},
	})
	if err != nil {
		t.Fatalf("SetFeatureValue: %v", err)
	}
	held := fixture.context()
	held.Runtime = ctx
	held.Roots = []queryexec.Root{{Label: "telescope", Object: telescope}}
	document, err := Evaluate(fixture.plan(t, "Report"), held, queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate over the session: %v", err)
	}
	if got := document.Content()[0].Runs()[1].Text(); got != "6.5" {
		t.Errorf("held aperture = %q, want 6.5", got)
	}
}

// A chain through a feature holding several objects joins the values those
// objects hold; one holding none for the member contributes nothing.
func TestEvaluateValueRunHeldObjects(t *testing.T) {
	fixture := loadEvaluationFixture(t, valueDocument)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	instantiate := func(name string) *runtime.Instance {
		inst, err := ctx.Instantiate(fixture.symbol(t, name))
		if err != nil {
			t.Fatalf("Instantiate %s: %v", name, err)
		}
		return inst
	}
	fleet, car, spare := instantiate("fleet"), instantiate("car"), instantiate("spare")
	err := car.SetFeatureValue(ctx, "rating", runtime.Value{
		Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 5},
	})
	if err != nil {
		t.Fatalf("SetFeatureValue rating: %v", err)
	}
	cars := runtime.NewSequence()
	cars.Append(runtime.Value{Kind: runtime.ValInstance, Instance: car.ID})
	cars.Append(runtime.Value{Kind: runtime.ValInstance, Instance: spare.ID})
	if err := fleet.SetFeatureValue(ctx, "cars", runtime.NewSequenceValue(cars)); err != nil {
		t.Fatalf("SetFeatureValue cars: %v", err)
	}
	held := fixture.context()
	held.Runtime = ctx
	held.Roots = []queryexec.Root{{Label: "fleet", Object: fleet}}
	document, err := Evaluate(fixture.plan(t, "FleetReport"), held, queryexec.Options{}, nil)
	if err != nil {
		t.Fatalf("evaluate over the session: %v", err)
	}
	if got := document.Content()[0].Runs()[1].Text(); got != "5" {
		t.Errorf("ratings = %q, want the car's 5", got)
	}
}
