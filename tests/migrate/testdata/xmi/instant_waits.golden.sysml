item def 'Cmd On';
item def 'Cmd Off';
item def 'Cmd Hold';
item def 'Cmd Settle';
item def 'Cmd Pause';
part def Relay {
    attribute gain : ScalarValues::Real default = 1.0;
    action def Settle {
        in ref context : Relay[1];
        first start then 'read gain';
        action 'read gain' {
            out result[1] = context.gain;
        }
        first 'read gain' then settled;
        action settled accept after 1.0 [SI::s];
        first settled then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
part def Stimulus {
    action def Hold {
        /* duration constraint on 'Hold' not migrated — the interval has no max, so the interval is open above and no one wait of at least 1.0 s stands for it */
        first start then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def 'Send Ack On' {
        first start then wait;
        action wait accept after RandomFunctions::uniform(1.0, 3.0) [SI::s];
        first wait then 'Turn On';
        action 'Turn On';
        first 'Turn On' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about wait, final;
        metadata MigrationMetadata::StandIn about wait;
    }
    action def 'Warm Up' {
        first start then warmed;
        action warmed accept after 2.0 [SI::s];
        first warmed then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action def 'Cool Down' {
        first start then wait;
        action wait accept after RandomFunctions::uniform(1.0, 3.0) [SI::s];
        first wait then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about wait, final;
        metadata MigrationMetadata::StandIn about wait;
    }
    action def Delay {
        first start then delayed;
        action delayed accept after 2.0 [SI::s];
        first delayed then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    action delay : Delay;
    action def Blink {
        first start then blinked;
        action blinked accept after 1.0 [SI::s];
        first blinked then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    state def 'Stimulus Behavior' {
        in ref context : Stimulus[1];
        entry; then Standby;
        state Standby;
        state On {
            entry action : 'Warm Up';
            do action : Blink;
            exit action : 'Cool Down';
        }
        transition first Standby accept 'Cmd On'
            do action 'Send Ack' {
                first start then call;
                action call : 'Send Ack On';
                first call then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about call, final;
            }
            then On;
        transition first Standby accept 'Cmd Hold'
            do action 'Send Ack Hold' {
                first start then call;
                action call : Hold;
                first call then someday;
                action someday {
                    /* not migrated: AcceptEventAction 'someday' — the time event's time is not written: the instant "someday" is neither a number with a time unit nor an expression: names someday, which nothing visible from Stimulus::Stimulus Behavior::<Region>::<Transition>::Send Ack Hold is called */
                }
                first someday then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about call, final;
            }
            then Standby;
        transition first Standby accept 'Cmd Settle'
            do action 'Send Ack Settle' {
                first start then call;
                action call {
                    /* not migrated: CallBehaviorAction (_sasCall) — the behavior acts on a Relay through its parameter context, which is left unbound: the caller is a Stimulus, which is no Relay and has no part that is one; v1 runs Relay::Settle on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing */
                }
                first call then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about call, final;
            }
            then Standby;
        transition first Standby accept 'Cmd Pause'
            do action 'Send Ack Pause' {
                /* not migrated: ControlFlow (_sapE1) — the edge leads to (_sapCall), which never fires: no value reaches its input pin 'target' */
                action call : Delay;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about call, final;
            }
            then Standby;
        transition first On accept 'Cmd Off'
            do action 'Send Ack Off' {
                first start then call;
                action call : Delay;
                first call then final;
                action final terminate;
                metadata MigrationMetadata::SynthesizedName about call, final;
            }
            then Standby;
    }
    exhibit state 'stimulus Behavior' : 'Stimulus Behavior' { in ref :>> context = this; }
}
