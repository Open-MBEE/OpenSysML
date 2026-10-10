package RequirementsModel {
    comment about RequirementsModel::'Outer Requirement'::'Nested Requirement'::'Innermost Requirement' /* The innermost requirement is annotated. */
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
    dependency RequirementsModel::'Outer Requirement'::'Nested Requirement'::'Innermost Requirement' to RequirementsModel::Host::block {
        @ModelingMetadata::Refinement;
    }
}
metadata MigrationMetadata::LibraryNameAvoided about RequirementsModel { sourceName = "Requirements"; }
