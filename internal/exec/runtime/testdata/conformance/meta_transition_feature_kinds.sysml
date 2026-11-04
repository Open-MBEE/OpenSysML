// A transition owns its trigger's accept, its guard and its effect through TransitionFeatureMemberships
// whose kind is a TransitionFeatureKind literal, and a time or change trigger's payload is a
// TriggerInvocationExpression whose kind is a TriggerKind literal (SysML v2 §8.3.17).
package test {
    private import ScalarValues::*;
    private import Time::*;

    attribute def Sig;
    part def Machine {
        attribute x : Integer = 1;
        state sm {
            entry; then idle;
            state idle; state busy; state fin; state late;
            transition t1 first idle accept Sig if x > 0 do action e1 then busy;
            transition t2 first busy accept when x > 5 then fin;
            transition t3 first fin accept at TimeOf(this) then late;
            transition t4 first late accept after 2 [SI::s] then idle;
        }
    }

    part def Report {
        attribute triggerKind = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.owningMembership.kind;
        attribute triggerIsLiteral = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.owningMembership.kind == SysML::Systems::TransitionFeatureKind::trigger;
        attribute guardIsLiteral = (Machine::sm::t1 meta SysML::TransitionUsage).guardExpression.owningMembership.kind == SysML::Systems::TransitionFeatureKind::guard;
        attribute effectIsLiteral = (Machine::sm::t1 meta SysML::TransitionUsage).effectAction.owningMembership.kind == SysML::Systems::TransitionFeatureKind::effect;
        attribute guardIsNotEffect = (Machine::sm::t1 meta SysML::TransitionUsage).guardExpression.owningMembership.kind == SysML::Systems::TransitionFeatureKind::effect;
        attribute membershipsAreTransitionFeatures = (Machine::sm::t1 meta SysML::TransitionUsage).guardExpression.owningMembership istype SysML::TransitionFeatureMembership;
        attribute kindIsTransitionFeatureKind = (Machine::sm::t1 meta SysML::TransitionUsage).guardExpression.owningMembership.kind istype SysML::Systems::TransitionFeatureKind;
        attribute whenKind = (Machine::sm::t2 meta SysML::TransitionUsage).triggerAction.payloadArgument.kind;
        attribute whenIsLiteral = (Machine::sm::t2 meta SysML::TransitionUsage).triggerAction.payloadArgument.kind == SysML::Systems::TriggerKind::'when';
        attribute atIsLiteral = (Machine::sm::t3 meta SysML::TransitionUsage).triggerAction.payloadArgument.kind == SysML::Systems::TriggerKind::'at';
        attribute afterIsLiteral = (Machine::sm::t4 meta SysML::TransitionUsage).triggerAction.payloadArgument.kind == SysML::Systems::TriggerKind::'after';
        attribute argumentIsTrigger = (Machine::sm::t4 meta SysML::TransitionUsage).triggerAction.payloadArgument istype SysML::TriggerInvocationExpression;
    }
}
