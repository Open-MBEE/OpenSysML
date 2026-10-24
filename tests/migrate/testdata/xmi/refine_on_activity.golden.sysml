package Model {
    package Ops {
        use case 'Fill Tank' {
            action def Fill {
                view Fill : StandardViewDefinitions::ActionFlowView {
                    expose Ops::'Fill Tank'::Fill;
                    expose 'Fill Tank';
                    expose 'Fill Rate';
                    expose 'Fill Tank refines Fill Rate';
                    expose 'pump refines Fill Rate';
                    @DiagramLayout::Canvas { unit = "px"; width = 420; height = 300; }
                    metadata DiagramLayout::Layout about Ops::'Fill Tank'::Fill::start { x = 90; y = 20; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about 'open valve' { x = 50; y = 70; width = 100; height = 40; }
                    metadata DiagramLayout::Layout about pump { x = 50; y = 150; width = 100; height = 40; }
                    metadata DiagramLayout::Layout about final { x = 90; y = 240; width = 20; height = 20; }
                    metadata DiagramLayout::Layout about 'Fill Tank' { x = 250; y = 20; width = 140; height = 50; }
                    metadata DiagramLayout::Layout about 'Fill Rate' { x = 250; y = 150; width = 140; height = 60; }
                    metadata DiagramLayout::Route about 'start to open valve' { points = (100, 40, 100, 70); }
                    metadata DiagramLayout::Route about 'open valve to pump' { points = (100, 110, 100, 150); }
                    metadata DiagramLayout::Route about 'pump to final' { points = (100, 190, 100, 240); }
                    metadata DiagramLayout::Route about 'Fill Tank refines Fill Rate' { points = (320, 70, 320, 150); }
                    metadata DiagramLayout::Route about 'pump refines Fill Rate' { points = (150, 170, 250, 170); }
                    render Views::asInterconnectionDiagram;
                }
                succession 'start to open valve' first start then 'open valve';
                action 'open valve';
                succession 'open valve to pump' first 'open valve' then pump;
                action pump;
                succession 'pump to final' first pump then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about 'start to open valve', 'open valve to pump', 'pump to final', final;
            }
        }
        requirement <R1> 'Fill Rate' {
            doc /* The tank shall fill within 30 s. */
        }
        dependency 'pump refines Fill Rate' from Ops::'Fill Tank'::Fill::pump to 'Fill Rate' {
            @ModelingMetadata::Refinement;
        }
        connection 'Fill Tank refines Fill Rate' connect 'Fill Tank' to 'Fill Rate';
        metadata MigrationMetadata::SynthesizedName about 'pump refines Fill Rate';
    }
}
