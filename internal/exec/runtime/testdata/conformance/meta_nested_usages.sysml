// The metaclasses' `nested*`/`owned*` derive an element's owned usages of the
// metaclass the suffix names (SysML v2 §8.3): a part usage's `nestedPart`
// reads the composite parts — the interface among them, a ConnectorAsUsage —
// and a part definition's `ownedPart`/`ownedPort` its declared ones.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def Wheel;
    part def Engine { port fuelIn; }
    part def Tank { port fuelOut; }

    part vehicle {
        part wheels : Wheel[4];
        part engine : Engine;
        part tank : Tank;
        interface fuelLine connect engine.fuelIn to tank.fuelOut;
        attribute partCount : Natural = subparts->size();
    }

    part def Report {
        part engine : Engine;
        port reportIn;
        attribute nestedParts = (vehicle meta SysML::PartUsage).nestedPart->size();
        attribute nestedUsages = (vehicle meta SysML::PartUsage).nestedUsage->size();
        attribute nestedInterfaces = (vehicle meta SysML::PartUsage).nestedInterface->size();
        attribute nestedAttributes = (vehicle meta SysML::PartUsage).nestedAttribute->size();
        attribute ownedParts = (Report meta SysML::PartDefinition).ownedPart->size();
        attribute ownedPorts = (Report meta SysML::PartDefinition).ownedPort->size();
    }
}
