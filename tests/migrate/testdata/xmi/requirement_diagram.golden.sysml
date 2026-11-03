package Model {
    package Requirements {
        requirement <R1> Mission {
            doc /* The mission shall deliver the payload. */
            requirement <'R1.1'> 'Payload Mass' {
                doc /* The payload shall weigh at most 50 kg. */
            }
            requirement <'R1.2'> 'Payload Power' {
                doc /* The payload shall draw at most 100 W. */
            }
        }
        part def Bus {
            satisfy requirement 'satisfy Mission.Payload Mass' :> Requirements::Mission.'Payload Mass';
            metadata MigrationMetadata::SynthesizedName about 'satisfy Mission.Payload Mass';
        }
        connection def 'Derive Payload Power' :> RequirementDerivation::Derivation {
            end #RequirementDerivation::original originalRequirement :> Requirements::Mission.'Payload Mass';
            end #RequirementDerivation::derive derivedRequirement :> Requirements::Mission.'Payload Power';
        }
        view 'Mission Requirements' {
            expose Mission;
            expose Requirements::Mission::'Payload Mass';
            expose Requirements::Mission::'Payload Power';
            expose Bus;
            expose 'Derive Payload Power';
            expose Requirements::Bus::'satisfy Mission.Payload Mass';
            @DiagramLayout::Canvas { unit = "px"; width = 440; height = 300; }
            metadata DiagramLayout::Layout about Mission { x = 150; y = 20; width = 140; height = 60; }
            metadata DiagramLayout::Layout about Requirements::Mission::'Payload Mass' { x = 40; y = 140; width = 140; height = 60; }
            metadata DiagramLayout::Layout about Requirements::Mission::'Payload Power' { x = 260; y = 140; width = 140; height = 60; }
            metadata DiagramLayout::Layout about Bus { x = 40; y = 240; width = 140; height = 50; }
            metadata DiagramLayout::Route about 'Derive Payload Power' { points = (260, 170, 180, 170); }
            metadata DiagramLayout::Route about Requirements::Bus::'satisfy Mission.Payload Mass' { points = (110, 240, 110, 200); }
            render Views::asTreeDiagram;
        }
    }
}
