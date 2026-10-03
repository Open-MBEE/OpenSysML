port def MotorCmd;
part def Motor {
    attribute speed : ScalarValues::Real default = 0.0;
    port cmd : MotorCmd;
    action def Spin {
        in ref context : Motor[1];
        in rpm : ScalarValues::Real[1];
        first start then 'set speed';
        action 'set speed' {
            in value[1];
            assign context.speed := value;
        }
        bind 'set speed'.value = rpm;
    }
    action spin : Spin;
}
part def Drive {
    port p : MotorCmd;
    port loose : MotorCmd;
    port split : MotorCmd;
    part motor : Motor;
    part spare : Motor;
    bind p = motor.cmd;
    bind split = motor.cmd;
    bind split = spare.cmd;
    action run {
        first start then thirty;
        action thirty {
            out result[1] = 30.0;
        }
        perform action 'spin over p' ::> motor.spin;
        first 'spin over p' then 'read motor';
        action 'read motor' {
            out result[1] = motor;
        }
        first 'read motor' then 'fork';
        fork 'fork';
        first 'fork' then forty;
        first 'fork' then 'spin over cmd';
        action forty {
            out result[1] = 40.0;
        }
        perform action 'spin over cmd' ::> motor.spin;
        first 'spin over cmd' then final;
        action final terminate;
        succession flow thirty.result to 'spin over p'.rpm;
        succession flow forty.result to 'spin over cmd'.rpm;
        metadata MigrationMetadata::SynthesizedName about 'fork', final;
        metadata MigrationMetadata::StandIn about 'fork';
    }
    action stray {
        first start then 'fork';
        fork 'fork';
        first 'fork' then five;
        first 'fork' then six;
        action five {
            out result[1] = 5.0;
        }
        action 'spin over loose' : Motor::Spin;
        action six {
            out result[1] = 6.0;
        }
        action 'spin over split' : Motor::Spin;
        succession flow five.result to 'spin over loose'.rpm;
        succession flow six.result to 'spin over split'.rpm;
        metadata MigrationMetadata::SynthesizedName about 'fork';
        metadata MigrationMetadata::StandIn about 'fork';
    }
}
