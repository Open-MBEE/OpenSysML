package Tailoring {
    metadata def Base {
        attribute origin : ScalarValues::String;
    }
    metadata def Left :> Base;
    metadata def Right :> Base {
        attribute side : ScalarValues::Integer;
    }
    metadata def Diamond :> Left, Right;
    metadata def Audited {
        attribute auditor : ScalarValues::String;
    }
    metadata def 'Audited Requirement' :> Audited;
    metadata def 'Ring A' :> 'Ring B';
    metadata def 'Ring B';
    metadata def Lost;
    metadata def Foreign;
    metadata def 'Checked By';
    metadata def 'Detailed By';
    metadata def 'Linked To';
    metadata def 'Derived From';
    metadata def 'Assigned To';
}
package System {
    part def Gear {
        @Tailoring::Diamond {
            origin = "catalogue";
            side = 2;
        }
    }
    part def Shaft {
        @Tailoring::Right {
            side = 1;
        }
    }
    requirement def <'R-1'> 'Torque Requirement' {
        doc /* Torque shall exceed 10 Nm. */
        @Tailoring::'Audited Requirement' {
            auditor = "QA";
        }
    }
    requirement def <'R-2'> 'Speed Requirement' {
        doc /* Speed shall exceed 100 rpm. */
    }
    part def 'Torque Test' {
        verification def run {
            objective {
                verify requirement : 'Torque Requirement' {
                    @Tailoring::'Checked By';
                }
            }
        }
    }
    action def Spin;
    attribute def Torque {
        @Tailoring::'Ring A';
    }
    part def 'Lost Thing' {
        @Tailoring::Lost;
    }
    part def 'Foreign Thing' {
        @Tailoring::Foreign;
    }
    dependency Spin to 'Speed Requirement' {
        @ModelingMetadata::Refinement;
        @Tailoring::'Detailed By';
    }
    dependency 'Speed Requirement' to Gear {
        /* «Trace» */
        @Tailoring::'Linked To';
    }
    connection def 'Derive Speed Requirement' :> RequirementDerivation::Derivation {
        end #RequirementDerivation::original originalRequirement : 'Torque Requirement';
        end #RequirementDerivation::derive derivedRequirement : 'Speed Requirement';
        @Tailoring::'Derived From';
    }
    allocate Spin to Shaft {
        @Tailoring::'Assigned To';
    }
}
