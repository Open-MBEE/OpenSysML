package Signals {
    item def SetLevel {
        attribute value : ScalarValues::Real;
    }
    item def Stop;
    item def Reset;
    item def Boost;
}
part def Heater {
    attribute level : ScalarValues::Real default = 0.0;
    action def SetLevel {
        first start then receive;
        action receive accept setLevel : Signals::SetLevel;
        first receive then run;
        action run : 'Apply Level' { in value = setLevel.value; }
        first run then receive;
    }
    perform action setLevel : SetLevel;
    action def Stop {
        first start then receive;
        action receive accept stop : Signals::Stop;
        first receive then receive;
    }
    perform action stop : Stop;
    action def Reset {
        first start then receive;
        action receive accept reset : Signals::Reset;
        first receive then receive;
    }
    perform action reset : Reset;
    action def Boost {
        first start then receive;
        action receive accept boost : Signals::Boost;
        first receive then receive;
    }
    perform action boost : Boost;
    comment /* reception 'Away' */
    action def 'Apply Level' {
        in value : ScalarValues::Real;
        in gain : ScalarValues::Real default = 1.0;
        in slack : ScalarValues::Real[0..1];
        first start then 'set level';
        action 'set level' {
            in value;
            assign this.level := value;
        }
        bind 'set level'.value = value;
    }
    action def Boosting {
        in amount : ScalarValues::Real;
        first start then 'set level';
        action 'set level' {
            in value;
            assign this.level := value;
        }
        bind 'set level'.value = amount;
    }
    state def Resetting {
        /* the region has no initial pseudostate: nothing enters it */
        state Idle;
    }
}
