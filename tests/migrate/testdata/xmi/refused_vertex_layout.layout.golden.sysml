part def Controller {
    state def Modes {
        view Modes : StandardViewDefinitions::StateTransitionView {
            expose Controller::Modes;
            @DiagramLayout::Canvas { unit = "px"; width = 340; height = 140; }
            metadata DiagramLayout::Layout about Controller::Modes::Idle { x = 0; y = -0; width = 100; height = 40; }
            metadata DiagramLayout::Layout about Controller::Modes::junction { x = 120; y = 50; width = 100; height = 40; }
            metadata DiagramLayout::Layout about Controller::Modes::choice { x = 240; y = 100; width = 100; height = 40; }
            metadata DiagramLayout::Route about 'Idle then junction' { points = (100, 20, 120, 70); }
            metadata DiagramLayout::Route about 'junction then choice' { points = (220, 70, 240, 120); }
            metadata DiagramLayout::Route about 'choice then Idle' { points = (340, 120, 100, 20); }
            metadata DiagramLayout::Style about Controller::Modes::junction { fill = "#E1E1C3"; }
            metadata DiagramLayout::Note about Controller::Modes::junction { text = "Junction note"; x = 350; y = 40; width = 120; height = 30; }
            render Views::asInterconnectionDiagram;
        }
        entry; then Idle;
        state Idle;
        #StateMachines::junction state junction;
        #StateMachines::choice state choice;
        transition 'Idle then junction' first Idle then junction;
        transition 'junction then choice' first junction then choice;
        transition 'choice then Idle' first choice then Idle;
        metadata MigrationMetadata::SynthesizedName about 'Idle then junction', 'junction then choice', 'choice then Idle';
    }
}
private import StateMachines::*;
