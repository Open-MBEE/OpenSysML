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
    item def Nudge {
        attribute delta : ScalarValues::Real;
    }
}
part def Heater {
    attribute level : ScalarValues::Real default = 0.0;
    port rx;
    port aux;
    perform action setLevel {
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept setLevel : Signals::SetLevel;
        first receive then run;
        perform action run ::> 'apply Level' { in value = receive.setLevel.value; }
        first run then receive;
        first spread then 'receive via rx';
        action 'receive via rx' accept 'setLevel via rx' : Signals::SetLevel via rx;
        first 'receive via rx' then 'run via rx';
        perform action 'run via rx' ::> 'apply Level' { in value = 'receive via rx'.'setLevel via rx'.value; }
        first 'run via rx' then 'receive via rx';
    }
    perform action stop {
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept stop : Signals::Stop;
        first receive then receive;
        first spread then 'receive via rx';
        action 'receive via rx' accept 'stop via rx' : Signals::Stop via rx;
        first 'receive via rx' then 'receive via rx';
    }
    action def Reset {
        first start then receive;
        action receive accept reset : Signals::Reset;
        first receive then receive;
    }
    perform action reset : Reset;
    perform action boost {
        first start then receive;
        action receive accept boost : Signals::Boost;
        first receive then receive;
    }
    perform action setName {
        first start then receive;
        action receive accept setName : Signals::SetName;
        first receive then receive;
    }
    perform action setLevels {
        first start then receive;
        action receive accept setLevels : Signals::SetLevels;
        first receive then receive;
    }
    perform action onNudge {
        first start then receive;
        action receive accept nudge2 : Signals::Nudge;
        first receive then run;
        perform action run ::> nudge { in delta = receive.nudge2.delta; }
        first run then receive;
    }
    comment /* reception 'Away' */
    action nudge {
        in delta : ScalarValues::Real;
        first start then 'set level';
        action 'set level' {
            in value;
            assign level := value;
        }
        bind 'set level'.value = delta;
    }
    action 'apply Level' {
        in value : ScalarValues::Real;
        in gain : ScalarValues::Real default = 1.0;
        in slack : ScalarValues::Real[0..1];
        first start then 'set level';
        action 'set level' {
            in value;
            assign level := value;
        }
        bind 'set level'.value = value;
    }
    action boosting {
        in amount : ScalarValues::Real;
        first start then 'set level';
        action 'set level' {
            in value;
            assign level := value;
        }
        bind 'set level'.value = amount;
    }
    action naming {
        in name : ScalarValues::Integer;
        first start then 'set level';
        action 'set level' {
            in value;
            assign level := value;
        }
        bind 'set level'.value = name;
    }
    action leveling {
        in values : ScalarValues::Real;
        first start then 'set level';
        action 'set level' {
            in value;
            assign level := value;
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
    action 'turn Up' {
        first start then seven;
        action seven {
            out result = 7.25;
        }
        first seven then 'send level';
        action 'send level' {
            in value;
            send new Signals::SetLevel(value) via tx;
        }
        first 'send level' then 'send stop';
        action 'send stop' {
            send new Signals::Stop() via tx;
        }
        first 'send stop' then final;
        action final terminate;
        flow seven.result to 'send level'.value;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
part def Room {
    part t : Thermostat;
    part h : Heater;
    connect t.tx to h.rx;
}
