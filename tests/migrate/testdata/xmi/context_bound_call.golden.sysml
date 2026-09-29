item def Ping;
action def Make {
    out integer : ScalarValues::Integer[1];
    in ref context : Host[1];
    first start then one;
    action one {
        out result[1] = 1;
    }
    first one then 'send ping';
    action 'send ping' {
        send new Ping() via context.tx;
    }
    first 'send ping' then final;
    action final terminate;
    bind integer = one.result;
    metadata MigrationMetadata::SynthesizedName about final;
    metadata MigrationMetadata::SynthesizedName about integer;
}
action def Consume {
    in value : ScalarValues::Integer[1];
}
part def Host {
    port tx;
    action def Run {
        in ref context : Host[1];
        first start then make;
        action make : Make { out integer[1]; in ref :>> context = Run::context; }
        first make then consume;
        action consume : Consume;
        first consume then final;
        action final terminate;
        flow make.integer to consume.value;
        /* partition 'host' represents Host: make */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
