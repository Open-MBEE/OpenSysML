package migrate

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

func TestMetaclassFiltersExcludeNewConformingTypes(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{
			name: "Class",
			want: `Except(source = WhereType(source = row, type = ("PartDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage", "OccurrenceDefinition")), exclude = Except(source = WhereType(source = row, type = ("ItemDefinition")), exclude = WhereType(source = row, type = ("PartDefinition", "ConstraintDefinition", "PortDefinition", "VerificationCaseDefinition", "ActionDefinition", "StateDefinition", "CalculationDefinition", "ViewUsage", "ViewpointUsage"))))`,
		},
		{
			name: "Property",
			want: `Except(source = WhereType(source = row, type = ("AttributeUsage", "PartUsage", "ItemUsage", "ReferenceUsage", "PortUsage", "ConstraintUsage", "RequirementUsage", "ActionUsage", "StateUsage", "CalculationUsage", "UseCaseUsage", "OccurrenceUsage")), exclude = Except(source = WhereType(source = row, type = ("EventOccurrenceUsage")), exclude = WhereType(source = row, type = ("AttributeUsage", "PartUsage", "ItemUsage", "ReferenceUsage", "PortUsage", "ConstraintUsage", "RequirementUsage", "ActionUsage", "StateUsage", "CalculationUsage", "UseCaseUsage"))))`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := metaclassFilter(tc.name).query(qlit("row")).text("")
			if got != tc.want {
				t.Errorf("filter query = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBlockStereotypeFilterRetainsPlainOccurrenceDefinitions(t *testing.T) {
	got := stereotypeTypes["Block"].types
	if !slices.Equal(got, []string{typePartDef, typeOccurrenceDef}) {
		t.Errorf("Block filter types = %v, want PartDefinition and OccurrenceDefinition", got)
	}
}

func TestPartDefinitionFilterRetainsPlainOccurrenceDefinitions(t *testing.T) {
	filter := fromTypes("«Block»", stereotypeTypes["Block"])
	want := `Except(source = WhereType(source = row, type = ("PartDefinition", "OccurrenceDefinition")), exclude = Except(source = WhereType(source = row, type = ("ItemDefinition")), exclude = WhereType(source = row, type = ("PartDefinition"))))`
	if got := filter.query(qlit("row")).text(""); got != want {
		t.Errorf("Block query = %s, want %s", got, want)
	}
}

func TestMergedTypeFiltersCombineExclusions(t *testing.T) {
	merged := mergeTypeFilters([]typeFilter{
		metaclassFilter("Class"),
		metaclassFilter("Property"),
		fromTypes("«Block»", stereotypeTypes["Block"]),
	})
	wantTypes := []string{
		typePartDef, typeConstraintDef, typePortDef, typeVerificationDef,
		typeActionDef, typeStateDef, typeCalcDef, typeViewUsage, typeViewpointUsage,
		typeOccurrenceDef, typeAttributeUsage, typePartUsage, typeItemUsage, typeReferenceUsage,
		typePortUsage, typeConstraintUsage, typeRequirementUse, typeActionUsage, typeStateUsage,
		typeCalcUsage, typeUseCaseUsage, typeOccurrenceUsage,
	}
	if !slices.Equal(merged.types, wantTypes) {
		t.Errorf("merged types = %v, want %v", merged.types, wantTypes)
	}
	if merged.excluding == nil {
		t.Fatal("merged filter has no exclusions")
	}
	if want := []string{typeItemDef, typeEventOccurrenceUsage}; !slices.Equal(merged.excluding.source, want) {
		t.Errorf("merged exclusion source = %v, want %v", merged.excluding.source, want)
	}
	wantKeep := []string{
		typePartDef, typeConstraintDef, typePortDef, typeVerificationDef,
		typeActionDef, typeStateDef, typeCalcDef, typeViewUsage, typeViewpointUsage,
		typeAttributeUsage, typePartUsage, typeItemUsage, typeReferenceUsage, typePortUsage,
		typeConstraintUsage, typeRequirementUse, typeActionUsage, typeStateUsage,
		typeCalcUsage, typeUseCaseUsage,
	}
	if !slices.Equal(merged.excluding.keep, wantKeep) {
		t.Errorf("merged exclusion keep = %v, want %v", merged.excluding.keep, wantKeep)
	}
}

func TestMergedTypeFiltersWithoutExclusionsStayUnfiltered(t *testing.T) {
	merged := mergeTypeFilters([]typeFilter{
		metaclassFilter("Association"),
		metaclassFilter("Port"),
	})
	if merged.excluding != nil {
		t.Errorf("merged filter has unexpected exclusion: %+v", merged.excluding)
	}
}

func TestMergedExclusionsRetainUnfilteredTypes(t *testing.T) {
	merged := mergeTypeFilters([]typeFilter{
		fromTypes("«Block»", stereotypeTypes["Block"]),
		{types: []string{typeItemDef}},
	})
	if merged.excluding == nil {
		t.Fatal("merged filter has no exclusions")
	}
	if want := []string{typePartDef, typeItemDef}; !slices.Equal(merged.excluding.keep, want) {
		t.Errorf("merged exclusion keep = %v, want %v", merged.excluding.keep, want)
	}
}

func TestToolBlockStereotypeFilterIsTheBlockFilter(t *testing.T) {
	block := fromTypes("«Block»", stereotypeTypes["Block"])
	got := toolBlockFilter("Subsystem")
	if got.label != "«Subsystem»" || !slices.Equal(got.types, block.types) || got.query(qlit("row")).text("") != block.query(qlit("row")).text("") {
		t.Errorf("Subsystem filter = %+v, want the Block filter labelled «Subsystem»", got)
	}
	if got.note == "" {
		t.Error("a filter by the tool table carries no note")
	}
}

func TestSpecializedFilterFollowsModuleGenerals(t *testing.T) {
	sysml := "http://www.omg.org/spec/SysML/20181001/SysML"
	ancestors := []sysmlv1.StereotypeRef{
		{ID: "_nne", Name: "NonNormative", Namespace: sysml},
		{ID: "_user", Name: "Block", Namespace: "http://example.com/schemas/User.xmi"},
		{ID: "_block", Name: "Block", Namespace: sysml},
	}
	got, ok := specializedFilter("Subsystem", ancestors)
	if !ok || !slices.Equal(got.types, []string{typePartDef, typeOccurrenceDef}) {
		t.Errorf("Subsystem :> Block filter = %+v, %v; want the Block types", got, ok)
	}
	if _, ok := specializedFilter("Tag", ancestors[:2]); ok {
		t.Error("a stereotype specializing only a user «Block» filters as a standard block")
	}
}
