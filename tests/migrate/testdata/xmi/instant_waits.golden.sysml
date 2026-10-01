item def 'Cmd On';
item def 'Cmd Off';
part def Stimulus {
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
        transition first On accept 'Cmd Off' then Standby;
    }
    exhibit state 'stimulus Behavior' : 'Stimulus Behavior' { in ref :>> context = this; }
}
