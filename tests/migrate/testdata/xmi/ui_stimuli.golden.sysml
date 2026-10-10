item def Start;
item def Stop;
item def Finish;
item def Reset;
item def 'Hard Reset' :> Reset;
item def Pause;
item def 'Long Pause' :> Pause;
item def Resume {
    attribute delay : ScalarValues::Real;
}
part def Controller {
    action def Run {
        first start then 'send stop';
        action 'send stop' {
            send new Stop();
        }
        first 'send stop' then 'send hard reset';
        action 'send hard reset' {
            send new 'Hard Reset'();
        }
        first 'send hard reset' then 'send resume';
        action 'send resume' {
            /* not migrated: SendSignalAction 'send resume' — the send passes no argument for the attribute delay of Resume, which must hold a value; v1 sends the signal without it, which v2 does not admit, so the action carries the token and performs nothing */
        }
        first 'send resume' then final;
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
        transition first Running accept Reset then Idle;
        transition first Running accept Pause then Idle;
        transition first Idle accept Resume then Running;
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
    @MigrationMetadata::AppliedStereotype {
        stereotype = "SimulationConfig";
        tags = "UI = Gui::Console";
    }
}
