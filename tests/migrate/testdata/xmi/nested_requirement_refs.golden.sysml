package Model {
    package Requirements {
        comment about Requirements::'Outer Requirement'::'Nested Requirement'::'Innermost Requirement' /* The innermost requirement is annotated. */
        requirement <R1> 'Outer Requirement' {
            doc /* The system shall. */
            requirement <'R1.1'> 'Nested Requirement' {
                doc /* The subsystem shall. */
                requirement <'R1.1.1'> 'Innermost Requirement' {
                    doc /* The component shall. */
                }
            }
        }
        part def Host {
            part block : Block;
        }
        part def Block;
        dependency 'Outer Requirement.Nested Requirement.Innermost Requirement refines block' from Requirements::'Outer Requirement'::'Nested Requirement'::'Innermost Requirement' to Requirements::Host::block {
            @ModelingMetadata::Refinement;
        }
        metadata MigrationMetadata::SynthesizedName about 'Outer Requirement.Nested Requirement.Innermost Requirement refines block';
    }
    package Cases {
        use case Operate;
        view 'Operating Cases' {
            expose Operate;
            expose Requirements::'Outer Requirement'::'Nested Requirement'::'Innermost Requirement';
            expose 'Operate refines Innermost Requirement';
            render Views::asInterconnectionDiagram;
        }
    }
    connection 'Operate refines Innermost Requirement' connect Cases::Operate to Requirements::'Outer Requirement'.'Nested Requirement'.'Innermost Requirement';
}
