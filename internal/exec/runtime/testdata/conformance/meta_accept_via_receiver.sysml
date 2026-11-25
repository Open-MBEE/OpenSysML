// An `accept … via <port>` trigger's AcceptActionUsage owns, after its payload, a receiver parameter
// valued by a FeatureReferenceExpression to the port, which is its receiverArgument (SysML v2.1 §8.3.17.2).
package test {
    item def Sig;
    port def P { in item s : Sig; }
    part def Machine {
        port p : P;
        state sm {
            entry; then idle;
            state idle; state busy;
            transition t1 first idle accept Sig via p then busy;
            transition t2 first busy accept s : Sig via p then idle;
            transition t3 first idle accept Sig then busy;
        }
    }

    part def Report {
        attribute receiverIsReference = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.receiverArgument istype SysML::FeatureReferenceExpression;
        attribute receiverPort = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.receiverArgument.referent.declaredName;
        attribute namedPayloadReceiverPort = (Machine::sm::t2 meta SysML::TransitionUsage).triggerAction.receiverArgument.referent.declaredName;
        attribute parameters = SequenceFunctions::size((Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.parameter);
        attribute ownedFeatures = SequenceFunctions::size((Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.ownedFeature);
        attribute receiverDirection = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.parameter#(2).direction;
        attribute receiverIsParameter = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.parameter#(2).owningMembership istype SysML::ParameterMembership;
        attribute receiverValueIsArgument = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.parameter#(2) == (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.receiverArgument.owningMembership.featureWithValue;
        attribute parametersDistinct = (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.parameter#(1) != (Machine::sm::t1 meta SysML::TransitionUsage).triggerAction.parameter#(2);
        attribute noViaParameters = SequenceFunctions::size((Machine::sm::t3 meta SysML::TransitionUsage).triggerAction.parameter);
        attribute noViaReceivers = SequenceFunctions::size((Machine::sm::t3 meta SysML::TransitionUsage).triggerAction.receiverArgument);
    }
}
