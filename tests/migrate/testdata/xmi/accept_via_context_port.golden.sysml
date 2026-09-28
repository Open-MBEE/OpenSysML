item def Go;
item def Ping;
part def Receiver {
    port p;
    perform action ping {
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept ping : Ping;
        first receive then receive;
        first spread then 'receive via p';
        action 'receive via p' accept 'ping via p' : Ping via p;
        first 'receive via p' then 'receive via p';
    }
    state def Life {
        in ref context : Receiver;
        entry; then Waiting;
        state Waiting {
            @MigrationMetadata::DeferredEvent { ref :>> signal : Ping; }
            item deferred : Ping[*] ordered;
            do action buffer {
                first start then 'receive via p';
                action 'receive via p' accept 'kept via p' : Ping via context.p;
                then action 'keep via p' { assign deferred := SequenceFunctions::including(deferred, 'receive via p'.'kept via p'); }
                then 'receive via p';
                metadata MigrationMetadata::SynthesizedName about 'receive via p', 'keep via p';
            }
            exit action flush {
                for kept in deferred { send kept to self; }
                then action clear { assign deferred := (); }
                metadata MigrationMetadata::SynthesizedName about clear;
            }
            metadata MigrationMetadata::SynthesizedName about deferred, buffer, flush;
        }
        state Done;
        transition first Waiting accept Go via context.p then Done;
    }
    exhibit state life : Life { in ref :>> context = this; }
}
port def ActivityReceiver {
    port p;
    action def Await {
        in ref context : ActivityReceiver;
        first start then receive;
        action receive accept Ping via context.p;
        first receive then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
    perform action await : Await { in ref :>> context = this; }
}
part def Sender {
    port 'out';
    action 'send' {
        first start then 'send ping';
        action 'send ping' {
            send new Ping() via 'out';
        }
        first 'send ping' then 'send go';
        action 'send go' {
            send new Go() via 'out';
        }
        first 'send go' then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
part def Rig {
    part sender : Sender;
    part receiver : Receiver;
    connect sender.'out' to receiver.p;
}
