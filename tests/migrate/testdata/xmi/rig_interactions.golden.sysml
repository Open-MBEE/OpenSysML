package Model {
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
        abstract action brake {
            in force : ScalarValues::Real[1];
            in hold : ScalarValues::Integer[0..1];
        }
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
        occurrence def Spinup {
            ref part c : Controller {
                event occurrence spinSent;
                then event occurrence spunReceived;
                then event occurrence goSent;
                then event occurrence brakeSent;
                then event occurrence callSpinSent;
                then event occurrence parEndSent;
                then event occurrence parOp1Received;
                then event occurrence newSent;
            }
            ref part m : Motor {
                event occurrence spinReceived {
                    perform action spinCall ::> Motor::spin { in rpm[1] = 30.0; in times[1] = 2; }
                }
                then event occurrence spunSent;
                then event occurrence goReceived;
                then event occurrence brakeReceived {
                    perform action brakeCall ::> Motor::brake { in force[1] = 0.5; }
                }
                then event occurrence callSpinReceived {
                    perform action callSpin ::> Motor::spin { in rpm[1] = 40.0; }
                }
                then event occurrence parEndReceived;
                then event occurrence parOp1Sent;
                then event occurrence newReceived;
            }
            message spin from c.spinSent to m.spinReceived;
            message spun of result : ScalarValues::Real = m.spinReceived.spinCall.result from m.spunSent to c.spunReceived;
            /* alt [mode == 1] */
            message go of goPayload : Go = new Go(n = 3) from c.goSent to m.goReceived;
            /* else */
            message brake from c.brakeSent to m.brakeReceived;
            /* end alt */
            /* loop (2, 2) */
            message callSpin from c.callSpinSent to m.callSpinReceived;
            /* end loop */
            /* par */
            message parEnd of go : Go = new Go(n = 4) from c.parEndSent to m.parEndReceived;
            /* and */
            message parOp1 of Done[1] from m.parOp1Sent to c.parOp1Received;
            /* end par */
            /* opt [ctrl.got > 10.0] */
            message 'new' from c.newSent to m.newReceived;
            /* end opt */
        }
        /* not migrated: Interaction 'Astray' — the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches */
        occurrence def 'Spinup at 12.01' {
            ref part motor : Motor;
            /* not migrated: TimeConstraint (_tc1) — a TimeConstraint has no form in v2 sequence notation; the events it spans are ordered but not timed */
            /* not migrated: TimeConstraint (_tc2) — a TimeConstraint has no form in v2 sequence notation; the events it spans are ordered but not timed */
        }
        verification def 'Spinup Test' {
            subject context : Rig;
            ref part m :> context.drive.motor {
                event occurrence spinReceived {
                    perform action spinCall ::> Motor::spin { in rpm[1] = 12.0; }
                }
            }
            message spin;
        }
        occurrence spinup : Spinup {
            ref part :>> c = ctrl;
            ref part :>> m = drive.motor;
        }
        occurrence 'spinup at 12.01' : 'Spinup at 12.01' {
            ref part :>> motor = drive.motor;
        }
        metadata MigrationMetadata::SynthesizedName about spinup, 'spinup at 12.01';
    }
}
