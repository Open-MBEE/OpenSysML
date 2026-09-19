package Signals {
    item def SetLevel {
        attribute value : ScalarValues::Real;
    }
    item def Stop;
    item def Reset;
    item def Boost;
    item def SetName {
        attribute name : ScalarValues::String;
    }
    item def SetLevels {
        attribute values : ScalarValues::Real[0..*];
    }
}
part def Heater {
    attribute level : ScalarValues::Real default = 0.0;
    port rx;
    port aux;
    action def SetLevel {
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept setLevel : Signals::SetLevel;
        first receive then run;
        action run : 'Apply Level' { in value = setLevel.value; }
        first run then receive;
        first spread then 'receive via rx';
        action 'receive via rx' accept 'setLevel via rx' : Signals::SetLevel via rx;
        first 'receive via rx' then 'run via rx';
        action 'run via rx' : 'Apply Level' { in value = 'setLevel via rx'.value; }
        first 'run via rx' then 'receive via rx';
    }
    perform action setLevel : SetLevel;
    action def Stop {
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept stop : Signals::Stop;
        first receive then receive;
        first spread then 'receive via rx';
        action 'receive via rx' accept 'stop via rx' : Signals::Stop via rx;
        first 'receive via rx' then 'receive via rx';
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
    action def SetName {
        first start then receive;
        action receive accept setName : Signals::SetName;
        first receive then receive;
    }
    perform action setName : SetName;
    action def SetLevels {
        first start then receive;
        action receive accept setLevels : Signals::SetLevels;
        first receive then receive;
    }
    perform action setLevels : SetLevels;
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
    action def Naming {
        in name : ScalarValues::Integer;
        first start then 'set level';
        action 'set level' {
            in value;
            assign this.level := value;
        }
        bind 'set level'.value = name;
    }
    action def Leveling {
        in values : ScalarValues::Real;
        first start then 'set level';
        action 'set level' {
            in value;
            assign this.level := value;
        }
        bind 'set level'.value = values;
    }
    state def Resetting {
        /* the region has no initial pseudostate: nothing enters it */
        state Idle;
    }
}
part def Thermostat {
    port tx;
    action def 'Turn Up' {
        first start then seven;
        action seven {
            out result = 7.25;
        }
        first seven then 'send level';
        action 'send level' {
            in value;
            send new Signals::SetLevel(value) via this.tx;
        }
        first 'send level' then 'send stop';
        action 'send stop' {
            send new Signals::Stop() via this.tx;
        }
        first 'send stop' then final;
        action final terminate;
        flow seven.result to 'send level'.value;
    }
}
part def Room {
    part t : Thermostat;
    part h : Heater;
    connect t.tx to h.rx;
}
