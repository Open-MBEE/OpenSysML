item def Ping;
action def Make {
    in ref context : Host;
    first start then one;
    action one {
        out result = 1;
    }
    first one then 'send ping';
    action 'send ping' {
        send new Ping() via context.tx;
    }
    first 'send ping' then final;
    action final terminate;
    metadata MigrationMetadata::SynthesizedName about final;
}
part def Host {
    port tx;
    action def Run {
        in ref context : Host;
        first start then make;
        action make : Make { in ref :>> context = Run::context; }
        first make then final;
        action final terminate;
        /* partition 'host' represents Host: make */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
