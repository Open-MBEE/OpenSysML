// A reflective read of an enumeration-valued property is the library's literal, a
// value of its enumeration: KerML's reflective model maps each MOF enumeration into
// a DataType whose literals are member features (KerML 1.1 §9.2.17), SysML's into an
// EnumerationDefinition (SysML v2 §9.2.22).
package test {
    private import ScalarValues::*;

    action def A { in x : Real; out y : Real; inout z : Real; }
    part def D { snapshot s; }
    part def V { attribute pu; }
    requirement def R { assume constraint a { true } }
    state def S { entry action e; }

    part def Report {
        attribute direction = (A::x meta KerML::Feature).direction;
        attribute inIsLiteral = (A::x meta KerML::Feature).direction == KerML::Core::FeatureDirectionKind::'in';
        attribute outIsLiteral = (A::y meta SysML::Usage).direction == KerML::Core::FeatureDirectionKind::out;
        attribute inoutIsNotIn = (A::z meta KerML::Feature).direction == KerML::Core::FeatureDirectionKind::'in';
        attribute directionIsKind = (A::x meta KerML::Feature).direction istype KerML::Core::FeatureDirectionKind;
        attribute literal = KerML::Core::FeatureDirectionKind::inout;
        attribute directions [*] = all KerML::Core::FeatureDirectionKind;
        attribute portion = (D::s meta SysML::OccurrenceUsage).portionKind == SysML::Systems::PortionKind::snapshot;
        attribute visibility = (V::pu meta KerML::Feature).owningMembership.visibility == KerML::Root::VisibilityKind::'public';
        attribute visibilityIsKind = (V::pu meta KerML::Feature).owningMembership.visibility istype KerML::Root::VisibilityKind;
        attribute constraintKind = (R::a meta SysML::Usage).owningMembership.kind == SysML::Systems::RequirementConstraintKind::assumption;
        attribute subactionKind = (S::e meta SysML::Usage).owningMembership.kind == SysML::Systems::StateSubactionKind::entry;
    }
}
