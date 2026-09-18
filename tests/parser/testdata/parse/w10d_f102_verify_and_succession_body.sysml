// A verification's requirement reference reaches a nested requirement through a
// '.' chain (SysML.xtext:2119 ReferenceSubsetting, KerML.xtext:699), and an
// action target succession ends in a body (SysML.xtext:1698).
package VerifyAndSuccessionBody {
    requirement def MassRequirement;
    part def Specification {
        requirement vehicleMassRequirement : MassRequirement;
    }
    verification def MassTest {
        part vehicleSpecification : Specification;
        objective massObjective {
            verify vehicleSpecification.vehicleMassRequirement {
                doc /* the verified requirement is reached through a chain */
            }
        }
    }
    action def Sequence {
        action a;
        action starting;
        first a;
        then starting {
            doc /* a target succession may carry a body */
        }
    }
}
