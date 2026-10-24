package Mission {
    private import ScalarValues::*;
    part def Rover {
        port power : PowerPort;
    }
    port def PowerPort { in attribute volts : Real; }
    requirement def MassReq {
        doc /* The rover shall be light. */
        attribute massLimit : Real;
        subject rover : Rover;
        require constraint { massLimit > 0.0 }
    }
    requirement massReq : MassReq {
        doc /* Mass requirement for the rover. */
        require constraint { massLimit < 100.0 }
    }
    requirement textless : MassReq;
    verification def MassTest {
        subject rover : Rover;
        objective { verify massReq; }
    }
    verification massTest : MassTest;
    verification def Idle;
}
package Rules {
    private import SequenceFunctions::*;
    constraint def verificationVerifiesSomething {
        doc /* Every verification case verifies at least one requirement. */
        in v : SysML::VerificationCaseDefinition;
        not v.verifiedRequirement->isEmpty()
    }
    constraint def requirementHasText {
        doc /* Every requirement usage has text. */
        in r : SysML::RequirementUsage;
        not r.text->isEmpty()
    }
    constraint def requirementHasRequiredConstraint {
        in r : SysML::RequirementUsage;
        not r.requiredConstraint->isEmpty()
    }
    constraint def portHasRelationship {
        in p : SysML::PortUsage;
        not p.ownedRelationship->isEmpty()
    }
    constraint def elementAnnotationsAreOwned {
        in e : KerML::Element;
        e.ownedAnnotation->isEmpty() or not e.ownedAnnotation->isEmpty()
    }
    constraint def namespaceOwnsItsMembers {
        in n : KerML::Namespace;
        n.ownedMembership->size() == n.ownedMember->size()
    }
}
