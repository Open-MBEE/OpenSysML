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
    action def SetLevel {
        in ref context : Heater;
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept setLevel : Signals::SetLevel;
        first receive then run;
        action run : 'Apply Level' { in value = setLevel.value; in ref :>> context = SetLevel::context; }
        first run then receive;
        first spread then 'receive via rx';
        action 'receive via rx' accept 'setLevel via rx' : Signals::SetLevel via context.rx;
        first 'receive via rx' then 'run via rx';
        action 'run via rx' : 'Apply Level' { in value = 'setLevel via rx'.value; in ref :>> context = SetLevel::context; }
        first 'run via rx' then 'receive via rx';
    }
    perform action setLevel : SetLevel { in value; in extra; in ref :>> context = this; }
    action def Stop {
        in ref context : Heater;
        first start then spread;
        fork spread;
        first spread then receive;
        action receive accept stop : Signals::Stop;
        first receive then receive;
        first spread then 'receive via rx';
        action 'receive via rx' accept 'stop via rx' : Signals::Stop via context.rx;
        first 'receive via rx' then 'receive via rx';
    }
    perform action stop : Stop { in ref :>> context = this; }
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
    action def OnNudge {
        in ref context : Heater;
        first start then receive;
        action receive accept nudge : Signals::Nudge;
        first receive then run;
        action run : Nudge { in delta = nudge.delta; in ref :>> context = OnNudge::context; }
        first run then receive;
    }
    perform action onNudge : OnNudge { in ref :>> context = this; }
    comment /* reception 'Away' */
    action def Nudge {
        in delta : ScalarValues::Real;
        in ref context : Heater;
        first start then 'set level';
        action 'set level' {
            in value;
            assign context.level := value;
        }
        bind 'set level'.value = delta;
    }
    action nudge : Nudge { in delta; in ref :>> context = this; }
    action def 'Apply Level' {
        in value : ScalarValues::Real;
        in gain : ScalarValues::Real default = 1.0;
        in slack : ScalarValues::Real[0..1];
        in ref context : Heater;
        first start then 'set level';
        action 'set level' {
            in value;
            assign context.level := value;
        }
        bind 'set level'.value = value;
    }
    action def Boosting {
        in amount : ScalarValues::Real;
        in ref context : Heater;
        first start then 'set level';
        action 'set level' {
            in value;
            assign context.level := value;
        }
        bind 'set level'.value = amount;
    }
    action def Naming {
        in name : ScalarValues::Integer;
        in ref context : Heater;
        first start then 'set level';
        action 'set level' {
            in value;
            assign context.level := value;
        }
        bind 'set level'.value = name;
    }
    action def Leveling {
        in values : ScalarValues::Real;
        in ref context : Heater;
        first start then 'set level';
        action 'set level' {
            in value;
            assign context.level := value;
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
        in ref context : Thermostat;
        first start then seven;
        action seven {
            out result = 7.25;
        }
        first seven then 'send level';
        action 'send level' {
            in value;
            send new Signals::SetLevel(value) via context.tx;
        }
        first 'send level' then 'send stop';
        action 'send stop' {
            send new Signals::Stop() via context.tx;
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
