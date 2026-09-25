port def MotorCmd;
part def Motor {
    attribute speed : ScalarValues::Real default = 0.0;
    port cmd : MotorCmd;
    action def Spin {
        in rpm : ScalarValues::Real;
        first start then 'set speed';
        action 'set speed' {
            in value;
            assign this.speed := value;
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
    action def Run {
        first start then thirty;
        action thirty {
            out result = 30.0;
        }
        first thirty then 'spin over p';
        perform action 'spin over p' ::> motor.spin;
        first 'spin over p' then 'read motor';
        action 'read motor' {
            out result = this.motor;
        }
        first 'read motor' then 'fork';
        fork 'fork';
        first 'fork' then forty;
        first 'fork' then 'join';
        action forty {
            out result = 40.0;
        }
        first forty then 'join';
        join 'join';
        first 'join' then 'spin over cmd';
        perform action 'spin over cmd' ::> motor.spin;
        first 'spin over cmd' then final;
        action final terminate;
        flow thirty.result to 'spin over p'.rpm;
        flow forty.result to 'spin over cmd'.rpm;
        metadata MigrationMetadata::SynthesizedName about 'fork', 'join', final;
    }
    action def Stray {
        first start then 'fork';
        fork 'fork';
        first 'fork' then five;
        first 'fork' then six;
        action five {
            out result = 5.0;
        }
        first five then 'spin over loose';
        action 'spin over loose' : Motor::Spin;
        action six {
            out result = 6.0;
        }
        first six then 'spin over split';
        action 'spin over split' : Motor::Spin;
        flow five.result to 'spin over loose'.rpm;
        flow six.result to 'spin over split'.rpm;
        metadata MigrationMetadata::SynthesizedName about 'fork';
    }
}
