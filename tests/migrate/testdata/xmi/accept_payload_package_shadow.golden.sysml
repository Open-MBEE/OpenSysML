package Alarm {
    item def Alarm;
    action def Receive {
        first start then receive;
        action receive accept Alarm : $::Alarm::Alarm;
        first receive then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
