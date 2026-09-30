item def Go {
    attribute n : ScalarValues::Integer;
}
item def Done;
part def Motor {
    attribute speed : ScalarValues::Real default = 0.0;
    action spin {
        in rpm : ScalarValues::Real[1];
        in times : ScalarValues::Integer[1] default = 1;
        out result : ScalarValues::Real[1];
        first start then 'set speed';
        action 'set speed' {
            in value[1];
            assign speed := value;
        }
        first 'set speed' then 'read speed';
        action 'read speed' {
            out result[1] = speed;
        }
        bind 'set speed'.value = rpm;
        bind result = 'read speed'.result;
    }
    abstract action def Brake {
        in force : ScalarValues::Real[1];
        in hold : ScalarValues::Integer[0..1];
    }
    action brake : Brake;
}
part def Drive {
    part motor : Motor;
}
part def Controller {
    attribute got : ScalarValues::Real default = 0.0;
}
part def Other {
    part motor : Motor;
}
part def Twin {
    part left : Drive;
    part right : Drive;
    /* not migrated: Interaction 'Twinned' — the lifeline 'm' stands for 'motor', which Twin reaches as both left.motor and right.motor */
}
part def Rig {
    part ctrl : Controller;
    part drive : Drive;
    attribute mode : ScalarValues::Integer default = 1;
    action spinup {
        perform action spin ::> drive.motor.spin { in rpm[1] = 30.0; in times[1] = 2; }
        first start then spin;
        action spun {
            assign ctrl.got := spin.result;
        }
        first spin then spun;
        action alt {
            if mode == 1 {
                action altOp1 {
                    action go send new Go(n = 3) to drive.motor;
                    first start then go;
                    first go then done;
                }
            }
            else {
                action altOp2 {
                    perform action brake : Motor::Brake ::> drive.motor.brake { in force[1] = 0.5; }
                    first start then brake;
                    first brake then done;
                }
            }
        }
        first spun then alt;
        action 'loop' {
            for i in 1..2 {
                action loopOp1 {
                    perform action callSpin ::> drive.motor.spin { in rpm[1] = 40.0; }
                    first start then callSpin;
                    first callSpin then done;
                }
            }
        }
        first alt then 'loop';
        fork par;
        first 'loop' then par;
        action parOp12 {
            action parEnd send new Go(n = 4) to drive.motor;
            first start then parEnd;
            first parEnd then done;
        }
        first par then parOp12;
        first parOp12 then parEnd2;
        action parOp2 {
            action parOp1 send new Done() to ctrl;
            first start then parOp1;
            first parOp1 then done;
        }
        first par then parOp2;
        first parOp2 then parEnd2;
        join parEnd2;
        action opt {
            if ctrl.got > 10.0 {
                action optOp1 {
                    /* not migrated: Message 'new' — the message creates the object drive.motor, which exists for as long as its owner does */
                    first start then done;
                }
            }
        }
        first parEnd2 then opt;
        first opt then done;
    }
    /* not migrated: Interaction 'Astray' — the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches */
    /* not migrated: Interaction 'Spinup at 12.01' — the interaction has no message: it records 2 state invariant(s) under 2 time constraint(s), a timing trace, which no scenario step performs */
    verification def 'Spinup Test' {
        subject context : Rig;
        perform action spin ::> context.drive.motor.spin { in rpm[1] = 12.0; }
        first start then spin;
        first spin then done;
    }
}
