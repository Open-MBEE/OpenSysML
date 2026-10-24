package Model {
    package Alarm {
        item def Beep;
    }
    action def Receive {
        first start then receive;
        action receive accept Alarm : Model::Alarm::Beep;
        first receive then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
