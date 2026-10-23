package Model {
    package Requirements {
        requirement <'R-1'> 'Flow Requirement' {
            doc /* The pump keeps the flow above the minimum. */
            requirement <'R-1.1'> 'Flow Margin' {
                doc /* The flow keeps a margin over the minimum. */
            }
        }
        requirement <'R-2'> 'Seal Requirement' {
            doc /* Every valve seals when closed. */
        }
        package Mass {
            requirement <'R-3'> 'Mass Requirement' {
                doc /* The pump mass stays under the limit. */
            }
        }
        part def Pump;
        view 'Requirement Table' {
            expose 'Flow Requirement';
            expose Requirements::'Flow Requirement'::'Flow Margin';
            expose 'Seal Requirement';
            expose Requirements::Mass::'Mass Requirement';
            render Views::asElementTable;
        }
    }
    package 'Use Cases' {
        use case 'Pump Fluid';
        use case 'Seal Valve';
        dependency 'Seal Valve traces Mass Requirement' from 'Seal Valve' to Requirements::Mass::'Mass Requirement' {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "Trace";
            }
        }
        part def Valve {
            attribute pressure : ScalarValues::Real;
        }
        dependency 'pressure refines Seal Requirement' from 'Use Cases'::Valve::pressure to Requirements::'Seal Requirement' {
            @ModelingMetadata::Refinement;
        }
        view 'Refine Requirement Matrix' {
            expose 'Pump Fluid refines Flow Requirement';
            expose 'Pump Fluid';
            expose Requirements::'Flow Requirement';
            expose 'Seal Valve refines Seal Requirement';
            expose 'Seal Valve';
            expose Requirements::'Seal Requirement';
            expose 'pressure refines Seal Requirement';
            render Views::asElementTable;
        }
        view 'Use Case Requirements' {
            render Views::asElementTable;
        }
        metadata MigrationMetadata::SynthesizedName about 'Seal Valve traces Mass Requirement', 'pressure refines Seal Requirement';
    }
    connection 'Pump Fluid refines Flow Requirement' connect 'Use Cases'::'Pump Fluid' to Requirements::'Flow Requirement';
    connection 'Seal Valve refines Seal Requirement' connect 'Use Cases'::'Seal Valve' to Requirements::'Seal Requirement';
}
