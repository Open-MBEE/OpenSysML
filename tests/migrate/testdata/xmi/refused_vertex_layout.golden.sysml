part def Controller {
    state def Modes {
        view Modes : StandardViewDefinitions::StateTransitionView {
            expose Controller::Modes;
            render Views::asInterconnectionDiagram;
        }
        entry; then Idle;
        state Idle;
        junction junction;
        choice choice;
        transition 'Idle then junction' first Idle then junction;
        transition 'junction then choice' first junction then choice;
        transition 'choice then Idle' first choice then Idle;
        metadata MigrationMetadata::SynthesizedName about 'Idle then junction', 'junction then choice', 'choice then Idle';
    }
}
