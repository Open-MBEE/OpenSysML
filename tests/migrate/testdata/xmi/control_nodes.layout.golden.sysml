package Control {
    part def Controller {
        action def Run {
            view Run : StandardViewDefinitions::ActionFlowView {
                expose Control::Controller::Run;
                @DiagramLayout::Canvas { unit = "px"; width = 190; height = 280; }
                metadata DiagramLayout::Layout about 'fork' { x = 40; y = 60; width = 120; height = 6; }
                metadata DiagramLayout::Layout about heat { x = 10; y = 100; width = 80; height = 40; }
                metadata DiagramLayout::Layout about cool { x = 110; y = 100; width = 80; height = 40; }
                metadata DiagramLayout::Layout about 'join' { x = 40; y = 170; width = 120; height = 6; }
                metadata DiagramLayout::Layout about check { x = 90; y = 200; width = 20; height = 20; }
                metadata DiagramLayout::Layout about final { x = 40; y = 260; width = 20; height = 20; }
                metadata DiagramLayout::Route about 'start to fork' { points = (100, 30, 100, 60); }
                metadata DiagramLayout::Route about 'fork to heat' { points = (50, 66, 50, 100); }
                metadata DiagramLayout::Route about 'fork to cool' { points = (150, 66, 150, 100); }
                metadata DiagramLayout::Route about 'heat to join' { points = (50, 140, 50, 170); }
                metadata DiagramLayout::Route about 'cool to join' { points = (150, 140, 150, 170); }
                metadata DiagramLayout::Route about 'join to check' { points = (100, 176, 100, 200); }
                metadata DiagramLayout::Route about 'check to final' { points = (90, 210, 50, 210, 50, 260); }
                metadata DiagramLayout::Route about 'check to done' { points = (110, 210, 150, 210, 150, 260); }
                render Views::asInterconnectionDiagram;
            }
            succession 'start to fork' first start then 'fork';
            fork 'fork';
            succession 'fork to heat' first 'fork' then heat;
            succession 'fork to cool' first 'fork' then cool;
            action heat {
                /* body not migrated (the body is empty) */
            }
            succession 'heat to join' first heat then 'join';
            action cool {
                /* body not migrated (the body is empty) */
            }
            succession 'cool to join' first cool then 'join';
            join 'join';
            succession 'join to check' first 'join' then check;
            decide check;
            succession 'check to final' first check then final;
            succession 'check to done' first check if false then done;
            action final terminate;
            metadata MigrationMetadata::SynthesizedName about 'start to fork', 'fork', 'fork to heat', 'fork to cool', 'heat to join', 'cool to join', 'join', 'join to check', 'check to final', 'check to done', final;
        }
    }
}
