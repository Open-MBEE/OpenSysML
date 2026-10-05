package runtime

import (
	"testing"
)

// connectorEndSrc connects two ports through unnamed connector ends written as
// feature chains, and redefines, subsets and references features of a plain
// definition.
const connectorEndSrc = `
package test {
	private import ScalarValues::*;
	port def P { attribute value : Real; }
	part def Source { port y : P; }
	part def Sink { port u : P; }
	connection def C { end source[1] : P; end target[1] : P; }
	part def Asm {
		part s : Source;
		part k : Sink;
		connection c1 : C connect [1] s.y to [1] k.u;
	}
	part def Base {
		attribute x : Real;
		attribute y : Real;
		part sub { attribute w : Real; }
	}
	part def Derived :> Base {
		attribute :>> x = 1.0;
		attribute z : Real :> y;
		ref attribute r ::> y;
		attribute deep :>> sub.w;
	}
}
`

// reflectText evaluates expr over connectorEndSrc and formats what it yields.
func reflectText(t *testing.T, expr string) string {
	t.Helper()
	_, got, err := evalDeclaredExpr(t, connectorEndSrc, expr)
	if err != nil {
		t.Fatalf("%s failed: %v", expr, err)
	}
	return FormatValue(got)
}

// TestReflectiveConnectorEndsAreOwned reads a connection usage's unnamed ends
// through its metaobject: they are the ReferenceUsages it owns as members, elements,
// features and end features (SysML v2 8.2.2.13.1), each owning the
// ReferenceSubsetting its attachment states, whose target is the chain written,
// with the chain's features as chainingFeature (KerML 8.3.4.8.15).
func TestReflectiveConnectorEndsAreOwned(t *testing.T) {
	const c1 = "(test::Asm::c1 meta SysML::ConnectionUsage)"
	const ends = "[meta(test::Asm::c1::<unnamed> : SysML::Systems::ReferenceUsage), meta(test::Asm::c1::<unnamed> : SysML::Systems::ReferenceUsage)]"
	for _, property := range []string{"ownedMember", "ownedElement", "ownedFeature", "ownedEndFeature", "endFeature"} {
		if got := reflectText(t, c1+"."+property); got != ends {
			t.Errorf("c1.%s = %s, want %s", property, got, ends)
		}
	}
	if got := reflectText(t, c1+".ownedEndFeature.owner"); got != "[meta(test::Asm::c1 : SysML::Systems::ConnectionUsage), meta(test::Asm::c1 : SysML::Systems::ConnectionUsage)]" {
		t.Errorf("c1.ownedEndFeature.owner = %s, want c1 twice", got)
	}
	if got := reflectText(t, c1+".ownedSubsetting"); got != "[]" {
		t.Errorf("c1.ownedSubsetting = %s, want none: the attachments belong to the ends", got)
	}
	if got := reflectText(t, c1+".ownedTyping.target"); got != "[meta(test::C : SysML::Systems::ConnectionDefinition)]" {
		t.Errorf("c1.ownedTyping.target = %s, want C", got)
	}
	const subsettings = "[meta(test::Asm::c1::<unnamed>::<unnamed> : KerML::Core::ReferenceSubsetting), meta(test::Asm::c1::<unnamed>::<unnamed> : KerML::Core::ReferenceSubsetting)]"
	for _, property := range []string{"ownedReferenceSubsetting", "ownedSubsetting"} {
		if got := reflectText(t, c1+".ownedEndFeature."+property); got != subsettings {
			t.Errorf("c1.ownedEndFeature.%s = %s, want %s", property, got, subsettings)
		}
	}
	const first = c1 + ".ownedEndFeature#(1).ownedReferenceSubsetting"
	for property, want := range map[string]string{
		"referencedFeature":                 "meta(test::Asm::c1::<unnamed>::<unnamed>::<unnamed> : KerML::Core::Feature)",
		"subsettedFeature":                  "meta(test::Asm::c1::<unnamed>::<unnamed>::<unnamed> : KerML::Core::Feature)",
		"target":                            "[meta(test::Asm::c1::<unnamed>::<unnamed>::<unnamed> : KerML::Core::Feature)]",
		"ownedRelatedElement":               "[meta(test::Asm::c1::<unnamed>::<unnamed>::<unnamed> : KerML::Core::Feature)]",
		"referencingFeature":                "meta(test::Asm::c1::<unnamed> : SysML::Systems::ReferenceUsage)",
		"subsettingFeature":                 "meta(test::Asm::c1::<unnamed> : SysML::Systems::ReferenceUsage)",
		"source":                            "[meta(test::Asm::c1::<unnamed> : SysML::Systems::ReferenceUsage)]",
		"owningRelatedElement":              "meta(test::Asm::c1::<unnamed> : SysML::Systems::ReferenceUsage)",
		"isImplied":                         "false",
		"referencedFeature.chainingFeature": "[meta(test::Asm::s : SysML::Systems::PartUsage), meta(test::Source::y : SysML::Systems::PortUsage)]",
		"referencedFeature.owner":           "meta(test::Asm::c1::<unnamed>::<unnamed> : KerML::Core::ReferenceSubsetting)",
	} {
		if got := reflectText(t, first+"."+property); got != want {
			t.Errorf("first end's reference subsetting %s = %s, want %s", property, got, want)
		}
	}
	const second = c1 + ".ownedEndFeature#(2).ownedReferenceSubsetting"
	if got, want := reflectText(t, second+".referencedFeature.chainingFeature"), "[meta(test::Asm::k : SysML::Systems::PartUsage), meta(test::Sink::u : SysML::Systems::PortUsage)]"; got != want {
		t.Errorf("second end's chainingFeature = %s, want %s", got, want)
	}
	if got, want := reflectText(t, c1+".relatedFeature"), "[meta(test::Source::y : SysML::Systems::PortUsage), meta(test::Sink::u : SysML::Systems::PortUsage)]"; got != want {
		t.Errorf("c1.relatedFeature = %s, want %s", got, want)
	}
}

// TestReflectivePlainUsageRelationships reads the relationships a usage's own
// clauses state as Relationship metaobjects: `:>> x` is one Redefinition whose
// redefinedFeature is x, `:> y` a Subsetting, `::> y` a ReferenceSubsetting, each
// also among ownedSubsetting, with the usage as source and the feature as target.
func TestReflectivePlainUsageRelationships(t *testing.T) {
	for expr, want := range map[string]string{
		"(test::Derived::x meta SysML::AttributeUsage).ownedRedefinition":                                     "[meta(test::Derived::x::<unnamed> : KerML::Core::Redefinition)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedRedefinition.redefinedFeature":                    "[meta(test::Base::x : SysML::Systems::AttributeUsage)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedRedefinition.redefiningFeature":                   "[meta(test::Derived::x : SysML::Systems::AttributeUsage)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedRedefinition.subsettedFeature":                    "[meta(test::Base::x : SysML::Systems::AttributeUsage)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedRedefinition.target":                              "[meta(test::Base::x : SysML::Systems::AttributeUsage)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedRedefinition.source":                              "[meta(test::Derived::x : SysML::Systems::AttributeUsage)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedSubsetting":                                       "[meta(test::Derived::x::<unnamed> : KerML::Core::Redefinition)]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedReferenceSubsetting":                              "[]",
		"(test::Derived::x meta SysML::AttributeUsage).ownedTyping":                                           "[]",
		"(test::Derived::z meta SysML::AttributeUsage).ownedSubsetting.subsettedFeature":                      "[meta(test::Base::y : SysML::Systems::AttributeUsage)]",
		"(test::Derived::z meta SysML::AttributeUsage).ownedSubsetting.subsettingFeature":                     "[meta(test::Derived::z : SysML::Systems::AttributeUsage)]",
		"(test::Derived::z meta SysML::AttributeUsage).ownedRedefinition":                                     "[]",
		"(test::Derived::z meta SysML::AttributeUsage).ownedTyping.type":                                      "[meta(ScalarValues::Real : KerML::Kernel::DataType)]",
		"(test::Derived::z meta SysML::AttributeUsage).ownedTyping.typedFeature":                              "[meta(test::Derived::z : SysML::Systems::AttributeUsage)]",
		"(test::Derived::z meta SysML::AttributeUsage).ownedTyping.isImplied":                                 "[false]",
		"(test::Derived::r meta SysML::AttributeUsage).ownedReferenceSubsetting.referencedFeature":            "[meta(test::Base::y : SysML::Systems::AttributeUsage)]",
		"(test::Derived::r meta SysML::AttributeUsage).ownedReferenceSubsetting.referencingFeature":           "[meta(test::Derived::r : SysML::Systems::AttributeUsage)]",
		"(test::Derived::r meta SysML::AttributeUsage).ownedSubsetting.subsettedFeature":                      "[meta(test::Base::y : SysML::Systems::AttributeUsage)]",
		"(test::Derived::deep meta SysML::AttributeUsage).ownedRedefinition.redefinedFeature.chainingFeature": "[meta(test::Base::sub : SysML::Systems::PartUsage), meta(test::Base::sub::w : SysML::Systems::AttributeUsage)]",
		"(test::Derived::deep meta SysML::AttributeUsage).ownedRedefinition.relatedElement":                   "[meta(test::Derived::deep : SysML::Systems::AttributeUsage), meta(test::Derived::deep::<unnamed>::<unnamed> : KerML::Core::Feature)]",
		"(test::Derived::deep meta SysML::AttributeUsage).ownedElement":                                       "[meta(test::Derived::deep::<unnamed>::<unnamed> : KerML::Core::Feature)]",
		"(test::Derived meta SysML::PartDefinition).ownedSubclassification.general":                           "[meta(test::Base : SysML::Systems::PartDefinition)]",
		"(test::Derived meta SysML::PartDefinition).ownedSpecialization.specific":                             "[meta(test::Derived : SysML::Systems::PartDefinition)]",
	} {
		if got := reflectText(t, expr); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
