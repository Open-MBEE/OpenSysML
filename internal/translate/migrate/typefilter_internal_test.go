package migrate

import (
	"slices"
	"testing"
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
