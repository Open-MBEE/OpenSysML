package model

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// diagnoseSource opens src in a stdlib-loaded workspace and returns its
// diagnostic messages.
func diagnoseSource(t *testing.T, uri, src string) []string {
	t.Helper()
	ws := NewWorkspace()
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)

	var msgs []string
	for _, d := range ws.Diagnostics(uri) {
		msgs = append(msgs, d.Message)
	}
	return msgs
}

// TestMetadataDefinitionInheritsAnnotatedElement covers SysML v2 §7.27.2: a
// metadata definition implicitly specializes Metadata::MetadataItem, so the
// `annotatedElement` feature it inherits from Metaobjects::Metaobject can be
// restricted by subsetting it.
func TestMetadataDefinitionInheritsAnnotatedElement(t *testing.T) {
	src := `package P {
		metadata def SecurityFeature {
			:> annotatedElement : SysML::PartDefinition;
		}
	}`
	if got := diagnoseSource(t, "file:///metadata.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}

	got := implicitBaseOf(t, src, "P", "SecurityFeature")
	if len(got) != 1 || got[0] != "Metadata::MetadataItem" {
		t.Fatalf("supertypes of the metadata definition = %v, want [Metadata::MetadataItem]", got)
	}
}

// TestExplicitSpecializationReplacesMetadataBase pins that the implicit base
// applies only to a metadata definition that declares no generalization of its
// own.
func TestExplicitSpecializationReplacesMetadataBase(t *testing.T) {
	src := `package P {
		metadata def Base;
		metadata def Derived :> Base;
	}`
	got := implicitBaseOf(t, src, "P", "Derived")
	if len(got) != 1 || got[0] != "P::Base" {
		t.Fatalf("supertypes of the derived metadata definition = %v, want [P::Base]", got)
	}
}

// semanticMetadataSource declares a semantic metadata definition `wheel` whose
// baseType is the usage `wheels`, per SysML v2 §7.27.3.
const semanticMetadataSource = `package P {
	private import Metaobjects::SemanticMetadata;

	part def Wheel {
		attribute pressure;
	}
	part wheels : Wheel[*];

	metadata def wheel :> SemanticMetadata {
		:>> baseType = wheels meta SysML::Usage;
	}

	part def Car {
		#wheel frontLeft {
			:>> pressure = 1;
		}
	}

	#wheel def SportsWheel;
}`

// TestSemanticMetadataKeywordSubsetsBaseType covers SysML v2 §7.27.4 and
// §7.27.3: a usage prefixed by a semantic metadata keyword implicitly subsets
// the keyword's baseType usage, so that usage's members are inherited.
func TestSemanticMetadataKeywordSubsetsBaseType(t *testing.T) {
	if got := diagnoseSource(t, "file:///semantic-metadata.sysml", semanticMetadataSource); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}

	got := implicitBaseOf(t, semanticMetadataSource, "P", "Car", "frontLeft")
	if len(got) != 1 || got[0] != "P::wheels" {
		t.Fatalf("supertypes of the annotated usage = %v, want [P::wheels]", got)
	}
}

// TestSemanticMetadataKeywordSubclassifiesBaseTypeOfDefinition covers the
// definition case of SysML v2 §7.27.3: a definition annotated with a usage
// baseType subclassifies the definitions that usage is typed by.
func TestSemanticMetadataKeywordSubclassifiesBaseTypeOfDefinition(t *testing.T) {
	got := implicitBaseOf(t, semanticMetadataSource, "P", "SportsWheel")
	if len(got) != 1 || got[0] != "P::Wheel" {
		t.Fatalf("supertypes of the annotated definition = %v, want [P::Wheel]", got)
	}
}

// A named binding is a usage like any other, so a semantic metadata keyword on
// it subsets the baseType usage rather than subclassifying what that usage is
// typed by (SysML v2 §7.27.3).
func TestSemanticMetadataKeywordSubsetsBaseTypeOfBinding(t *testing.T) {
	src := `package P {
		private import Metaobjects::SemanticMetadata;

		part def D { attribute a; attribute b; }
		part d : D;
		binding baseBinding bind d.a = d.b;
		metadata def link :> SemanticMetadata {
			:>> baseType = baseBinding meta SysML::Usage;
		}

		part def Car {
			part e : D;
			#link binding tied bind e.a = e.b;
		}
	}`
	if got := diagnoseSource(t, "file:///semantic-metadata-binding.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}
	if got := implicitBaseOf(t, src, "P", "Car", "tied"); len(got) != 1 || got[0] != "P::baseBinding" {
		t.Fatalf("supertypes of the annotated binding = %v, want [P::baseBinding]", got)
	}
}

// TestPlainMetadataKeywordAddsNoSpecialization pins the other side of §7.27.3:
// a keyword naming a metadata definition that is not semantic metadata
// annotates the declaration without specializing anything: the usage keeps the
// Anything base every kindless usage has (pilot ImplicitGeneralizationMap).
func TestPlainMetadataKeywordAddsNoSpecialization(t *testing.T) {
	src := `package P {
		metadata def Approved;
		part def Design {
			#Approved wheel;
			plain;
		}
	}`
	got := implicitBaseOf(t, src, "P", "Design", "wheel")
	if want := []string{"Base::Anything", "Base::things"}; !slices.Equal(got, want) {
		t.Fatalf("supertypes of the annotated usage = %v, want %v", got, want)
	}
	if plain := implicitBaseOf(t, src, "P", "Design", "plain"); !slices.Equal(plain, got) {
		t.Fatalf("supertypes of the plain usage = %v, want %v as the annotated one", plain, got)
	}
}

// TestSemanticMetadataMemberIsNotInvented guards the resolution path added for
// inherited members: a name the baseType does not declare still fails.
func TestSemanticMetadataMemberIsNotInvented(t *testing.T) {
	src := strings.Replace(semanticMetadataSource, ":>> pressure = 1;", ":>> notAMember = 1;", 1)
	got := diagnoseSource(t, "file:///semantic-metadata-negative.sysml", src)
	if len(got) != 1 || !strings.Contains(got[0], "notAMember") {
		t.Fatalf("diagnostics = %v, want one about notAMember", got)
	}
}

// A metadata usage written as a member of its own names a type, so a name that
// refers to nothing is reported the way the `#` prefix spelling is.
func TestMetadataUsageMemberResolvesItsType(t *testing.T) {
	src := `package P {
		metadata def Safety;
		@Safety;
		part def Car {
			@Safety;
		}
	}`
	if got := diagnoseSource(t, "file:///metadata-usage.sysml", src); len(got) != 0 {
		t.Fatalf("expected no diagnostics, got %v", got)
	}

	got := diagnoseSource(t, "file:///metadata-usage-bad.sysml", `package P {
		metadata def Safety;
		@Securty;
		part def Car {
			@Securty;
		}
	}`)
	if len(got) != 2 {
		t.Fatalf("expected both usages to be reported, got %v", got)
	}
	for _, msg := range got {
		if !strings.Contains(msg, "unresolved reference: Securty") {
			t.Errorf("unexpected diagnostic %q", msg)
		}
	}
}

// An undeclared semantic metadata keyword leaves the usage's supertypes settled;
// opening its declaration later recomputes them and its baseType appears.
func TestSemanticMetadataTypeDeclaredLaterContributesItsBase(t *testing.T) {
	const car, wheels = "file:///car.sysml", "file:///wheels.sysml"
	ws := NewWorkspace()
	ws.Open(car, []byte(`package P {
		private import Q::*;
		part def Car { #wheel frontLeft; }
	}`), 1)
	defer ws.Close(car)
	supertypesOf := func() []string {
		t.Helper()
		ws.Diagnostics(car)
		matches := ws.index.LookupQualified("P::Car::frontLeft")
		if len(matches) != 1 {
			t.Fatalf("P::Car::frontLeft resolves to %d symbols, want 1", len(matches))
		}
		var names []string
		for _, sup := range ws.model.AllSupertypes(matches[0]) {
			names = append(names, symbols.FQNOf(sup))
		}
		if ws.model.SupertypesProvisional(matches[0]) {
			t.Fatalf("supertypes of frontLeft %v are provisional, want settled", names)
		}
		return names
	}
	if got := supertypesOf(); slices.Contains(got, "Q::wheels") {
		t.Fatalf("supertypes of frontLeft before wheel is declared = %v", got)
	}

	ws.Open(wheels, []byte(`package Q {
		private import Metaobjects::SemanticMetadata;
		part def Wheel;
		part wheels : Wheel[*];
		metadata def wheel :> SemanticMetadata { :>> baseType = wheels meta SysML::Usage; }
	}`), 1)
	defer ws.Close(wheels)
	if got := supertypesOf(); !slices.Contains(got, "Q::wheels") {
		t.Fatalf("supertypes of frontLeft once wheel is declared = %v, want Q::wheels among them", got)
	}
}
