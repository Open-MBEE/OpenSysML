package Structure {
    part def Component;
    part def Vehicle {
        part engine : Engine;
        part wheel : Wheel;
        attribute mass : ScalarValues::Real;
        attribute limit : ScalarValues::Real;
        attribute speed : ScalarValues::Real;
        connection 'engine to wheel' connect engine to wheel;
        connection 'engine to wheel 2' connect engine to wheel;
        connect engine to wheel;
        connection drive connect engine to wheel;
        binding 'mass = limit' bind mass = limit;
        action def Drive {
            view Driving : StandardViewDefinitions::ActionFlowView {
                expose Drive;
                expose 'set speed.value = target';
                render Views::asInterconnectionDiagram;
            }
            in target : ScalarValues::Real;
            succession 'start to gain' first start then gain;
            action gain {
                out result = 2.5;
            }
            first gain then 'set speed';
            action 'set speed' {
                in value;
                assign this.speed := value;
            }
            succession finish first 'set speed' then final;
            action final terminate;
            flow 'gain.result to set speed.value' from gain.result to 'set speed'.value;
            binding 'set speed.value = target' bind 'set speed'.value = target;
        }
        satisfy requirement 'satisfy Mass Limit' : Requirements::'Mass Limit';
        satisfy requirement : Requirements::'Speed Limit';
        satisfy requirement 'satisfy Speed Limit' : Requirements::'Speed Limit';
        view 'Vehicle Internals' {
            expose engine;
            expose wheel;
            expose mass;
            expose limit;
            expose 'engine to wheel';
            expose 'engine to wheel 2';
            expose drive;
            expose 'mass = limit';
            render Views::asInterconnectionDiagram;
        }
    }
    part def Engine :> Component {
        satisfy requirement 'satisfy Speed Limit' : Requirements::'Speed Limit';
    }
    part def Wheel;
    dependency 'Vehicle to Engine' from Vehicle to Engine;
    dependency Vehicle to Wheel;
    dependency 'Engine to Component' from Engine to Component;
    dependency 'Wheel to Component' from Wheel to Component;
    view Structure {
        expose Component;
        expose Vehicle;
        expose Engine;
        expose Wheel;
        expose 'Vehicle to Engine';
        expose 'Engine to Component';
        expose 'Wheel to Component';
        render Views::asTreeDiagram;
    }
}
package Behavior {
    item def Go;
    item def Stop;
    item def Resume;
    state def Modes {
        view Modes : StandardViewDefinitions::StateTransitionView {
            expose Behavior::Modes;
            render Views::asInterconnectionDiagram;
        }
        entry; then Idle;
        state Idle;
        state Running {
            do action spin {
                /* body not migrated (the statement "keep turning" is not an assignment of a v2 expression (language English)) {English}:
                 * keep turning
                 */
            }
        }
        state Halted;
        transition 'Idle accept Go then Running' first Idle accept Go then Running;
        transition halt first Running accept Stop then Halted;
        transition first Halted then Idle;
        transition 'Halted accept Go then Running' first Halted accept Go then Running;
        transition 'Halted accept Resume then Running' first Halted accept Resume then Running;
    }
}
package Requirements {
    requirement def <R1> 'Mass Limit' {
        doc /* The vehicle shall weigh less than 1500 kg. */
    }
    requirement def <R2> 'Speed Limit' {
        doc /* The vehicle shall not exceed 120 km/h. */
    }
    verification def 'Mass Test' {
        objective 'objective' {
            verify requirement 'verify Mass Limit' : 'Mass Limit';
        }
    }
    view Traceability {
        expose 'Mass Limit';
        expose Structure::Vehicle;
        expose 'Mass Test';
        expose Structure::Vehicle::'satisfy Mass Limit';
        expose Requirements::'Mass Test'::'objective'::'verify Mass Limit';
        expose Structure::Vehicle::'satisfy Speed Limit';
        expose Structure::Engine::'satisfy Speed Limit';
        render Views::asTreeDiagram;
    }
}
package Usage {
    use case def 'Drive Vehicle' {
        include use case 'start Engine' : 'Start Engine';
        include use case 'stop Engine' : 'Stop Engine';
    }
    use case def 'Start Engine';
    use case def 'Stop Engine';
    view 'Driving Cases' {
        expose 'Drive Vehicle';
        expose 'Start Engine';
        expose Usage::'Drive Vehicle'::'start Engine';
        render Views::asTextualNotation;
    }
}
