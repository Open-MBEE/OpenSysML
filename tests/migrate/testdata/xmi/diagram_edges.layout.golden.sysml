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
                @DiagramLayout::Canvas { unit = "px"; width = 140; height = 220; }
                metadata DiagramLayout::Layout about gain { x = 10; y = 60; width = 100; height = 40; }
                metadata DiagramLayout::Layout about 'set speed' { x = 10; y = 140; width = 100; height = 40; }
                metadata DiagramLayout::Route about 'start to gain' { points = (60, 60, 60, 30); }
                metadata DiagramLayout::Route about finish { points = (60, 220, 60, 180); }
                metadata DiagramLayout::Route about 'gain.result to set speed.value' { points = (110, 160, 140, 160, 140, 80, 110, 80); }
                render Views::asInterconnectionDiagram;
            }
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
        }
        satisfy requirement 'satisfy Mass Limit' : Requirements::'Mass Limit';
        satisfy requirement : Requirements::'Speed Limit';
        view 'Vehicle Internals' {
            expose engine;
            expose wheel;
            expose mass;
            expose limit;
            expose 'engine to wheel';
            expose 'engine to wheel 2';
            expose drive;
            expose 'mass = limit';
            @DiagramLayout::Canvas { unit = "px"; width = 300; height = 120; }
            metadata DiagramLayout::Layout about engine { x = 20; y = 10; width = 100; height = 40; }
            metadata DiagramLayout::Layout about wheel { x = 200; y = 10; width = 100; height = 40; }
            metadata DiagramLayout::Layout about mass { x = 20; y = 100; width = 100; height = 20; }
            metadata DiagramLayout::Layout about limit { x = 200; y = 100; width = 100; height = 20; }
            metadata DiagramLayout::Route about 'engine to wheel' { points = (200, 20, 120, 20); }
            metadata DiagramLayout::Route about 'engine to wheel 2' { points = (200, 40, 160, 60, 120, 40); }
            metadata DiagramLayout::Route about drive { points = (200, 30, 120, 30); }
            metadata DiagramLayout::Route about 'mass = limit' { points = (200, 110, 120, 110); }
            render Views::asInterconnectionDiagram;
        }
    }
    part def Engine :> Component;
    part def Wheel;
    dependency 'Vehicle to Engine' from Vehicle to Engine;
    dependency Vehicle to Wheel;
    view Structure {
        expose Component;
        expose Vehicle;
        expose Engine;
        expose Wheel;
        expose 'Vehicle to Engine';
        @DiagramLayout::Canvas { unit = "px"; width = 330; height = 240; }
        metadata DiagramLayout::Layout about Component { x = 120; y = 20; width = 100; height = 60; }
        metadata DiagramLayout::Layout about Vehicle { x = 10; y = 160; width = 100; height = 80; }
        metadata DiagramLayout::Layout about Engine { x = 120; y = 160; width = 100; height = 80; }
        metadata DiagramLayout::Layout about Wheel { x = 230; y = 160; width = 100; height = 80; }
        render Views::asTreeDiagram;
    }
}
package Behavior {
    item def Go;
    item def Stop;
    state def Modes {
        view Modes : StandardViewDefinitions::StateTransitionView {
            expose Behavior::Modes;
            @DiagramLayout::Canvas { unit = "px"; width = 300; height = 140; }
            metadata DiagramLayout::Layout about Behavior::Modes::Idle { x = 10; y = 10; width = 100; height = 40; }
            metadata DiagramLayout::Layout about Behavior::Modes::Running { x = 200; y = 10; width = 100; height = 40; }
            metadata DiagramLayout::Layout about Behavior::Modes::Halted { x = 200; y = 100; width = 100; height = 40; }
            metadata DiagramLayout::Route about 'Idle accept Go then Running' { points = (200, 30, 110, 30); }
            metadata DiagramLayout::Route about halt { points = (250, 100, 250, 50); }
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
        @DiagramLayout::Canvas { unit = "px"; width = 330; height = 140; }
        metadata DiagramLayout::Layout about 'Mass Limit' { x = 120; y = 10; width = 100; height = 40; }
        metadata DiagramLayout::Layout about Structure::Vehicle { x = 10; y = 100; width = 100; height = 40; }
        metadata DiagramLayout::Layout about 'Mass Test' { x = 230; y = 100; width = 100; height = 40; }
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
        @DiagramLayout::Canvas { unit = "px"; width = 300; height = 50; }
        metadata DiagramLayout::Layout about 'Drive Vehicle' { x = 10; y = 10; width = 100; height = 40; }
        metadata DiagramLayout::Layout about 'Start Engine' { x = 200; y = 10; width = 100; height = 40; }
        render Views::asTextualNotation;
    }
}
