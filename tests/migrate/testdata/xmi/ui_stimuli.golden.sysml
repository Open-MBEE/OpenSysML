item def Start;
item def Stop;
item def Finish;
part def Controller {
    action def Run {
        first start then 'send stop';
        action 'send stop' {
            send new Stop();
        }
        first 'send stop' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    state def 'Controller Behavior' {
        entry; then Idle;
        state Idle;
        state Running {
            do action : Run;
        }
        transition first Idle accept Start then Running;
        transition first Running accept Stop then Idle;
        transition first Idle accept Finish then done;
    }
    exhibit state 'controller Behavior' : 'Controller Behavior';
}
action def 'Await Start' {
    first start then started;
    action started accept Start;
    first started then final;
    action final terminate;
    metadata MigrationMetadata::SynthesizedName about final;
}
package Gui;
action def Interactive {
    @Simulation::Configuration;
    part target : Controller;
    /* «SimulationConfig» settings of the simulation tool: UI = Gui::Console */
}
