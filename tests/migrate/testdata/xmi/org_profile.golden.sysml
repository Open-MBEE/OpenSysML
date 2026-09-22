package 'Org Profile' {
    doc /* The organisation's tailoring of SysML. */
    enum def Criticality {
        high;
        low;
    }
    metadata def 'Org Requirement' {
        doc /* A requirement with the organisation's rationale. */
        attribute Rationale : ScalarValues::String;
        attribute criticality : Criticality;
        attribute weight : ScalarValues::Real;
        ref owner;
    }
    metadata def 'Key Requirement' :> 'Org Requirement' {
        attribute rank : ScalarValues::Integer;
    }
    metadata def 'Org Block' {
        attribute partNumber : ScalarValues::String;
        ref supplier;
    }
    metadata def 'Org Value' {
        attribute precision : ScalarValues::Integer;
    }
    metadata def 'Org Satisfy' {
        attribute verified : ScalarValues::Boolean;
    }
}
package Review {
    attribute def Priority :> ScalarValues::Integer;
    metadata def Reviewed {
        attribute reviewer : ScalarValues::String;
        attribute round : ScalarValues::Integer;
        attribute approved : ScalarValues::Boolean;
        attribute priority : Priority;
        attribute notes : ScalarValues::String[0..*];
    }
}
package Legacy {
    metadata def Requirement {
        attribute Text : ScalarValues::String;
    }
}
package 'Pump System' {
    attribute def FlowRate {
        @'Org Profile'::'Org Value' {
            precision = 3;
        }
    }
    requirement def <'REQ-1'> 'Flow Requirement' {
        doc /* The pump shall deliver 10 l/s. */
        @'Org Profile'::'Org Requirement' {
            Rationale = "Sized for the peak load.";
            criticality = 'Org Profile'::Criticality::high;
            owner = Acme;
            weight = 2.0;
        }
    }
    requirement def <'REQ-2'> 'Safety Requirement' {
        doc /* The pump shall stop on overpressure. */
        @'Org Profile'::'Key Requirement' {
            criticality = 'Org Profile'::Criticality::low;
            rank = 1;
        }
    }
    part def Acme;
    part def Pump {
        attribute rate : FlowRate {
            @Review::Reviewed {
                approved = false;
                priority = 2;
                reviewer = "QA";
                round = 1;
            }
        }
        part motor : Motor;
        part controller : Controller;
        connection wire connect motor to controller {
            @Review::Reviewed {
                approved = true;
                reviewer = "EE";
            }
        }
        satisfy requirement : 'Flow Requirement' {
            @'Org Profile'::'Org Satisfy' {
                verified = true;
            }
        }
        @'Org Profile'::'Org Block' {
            partNumber = "P-100";
            supplier = Acme;
        }
        @Review::Reviewed {
            approved = true;
            notes = ("First pass.", "Second pass.");
            reviewer = "QA";
            round = 2;
        }
    }
    part def Motor;
    part def Controller;
    part def 'Design Note' {
        @Legacy::Requirement {
            Text = "Kept for reference only.";
        }
    }
    dependency derivedFrom from Motor to Pump {
        @Review::Reviewed {
            approved = true;
            reviewer = "Lead";
        }
    }
}
