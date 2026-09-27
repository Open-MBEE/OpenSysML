part def Valve {
    attribute open : ScalarValues::Boolean default = false;
}
part def Tank {
    attribute volume : ScalarValues::Real default = 1.0;
    part valve : Valve;
}
part def Pump {
    attribute on : ScalarValues::Boolean default = false;
}
part def Led {
    attribute lit : ScalarValues::Boolean default = false;
}
part def Controller {
    attribute status : ScalarValues::Boolean default = false;
    part led : Led;
}
part def Rack {
    part controller : Controller;
}
part def Control {
    part rack : Rack;
}
part def Site {
    part control : Control;
}
part def Plant {
    part site : Site;
    part tank : Tank;
    part pump : Pump;
    attribute level : ScalarValues::Real default = 0.0;
    attribute runs : ScalarValues::Integer default = 0;
    action def Fill {
        in ref context : $::Plant;
        first start then 'open valve';
        action 'open valve' {
            assign context.tank.valve.open := true;
        }
        first 'open valve' then 'fill tank';
        action 'fill tank' {
            assign context.tank.volume := context.tank.volume * 2 + (if context.tank.valve.open ? 1 else 0);
        }
        first 'fill tank' then torn;
        action torn {
            /* body not migrated (the name "volume" resolves to nothing readable: nothing visible from Plant::Fill::torn is called volume; it is in the partitions 'Tank' (this.tank) and 'Pump' (this.pump), which represent different objects, so names resolve through no partition) {JavaScript}:
             * volume = 0;
             */
        }
        /* guard not migrated: [{JavaScript} on] — the name "on" resolves to nothing readable: nothing visible from Plant::Fill::<ControlFlow> is called on; its source 'torn' is in the partitions 'Tank' (this.tank) and 'Pump' (this.pump), which represent different objects, so names resolve through no partition */
        first torn then prime;
        action prime {
            assign context.pump.on := true;
        }
        first prime then twice;
        action twice {
            assign context.tank.volume := context.tank.volume + 1;
        }
        first twice then mine;
        action mine {
            /* body not migrated (the name "this.level" resolves to nothing readable: Tank has no feature level) {JavaScript}:
             * this.level = 1;
             */
        }
        first mine then 'count run';
        action 'count run' {
            assign context.runs := context.runs + 1;
        }
        first 'count run' then 'note level';
        action 'note level' {
            assign context.level := context.tank.volume;
        }
        first 'note level' then lost;
        action lost {
            assign context.level := context.level + 1;
        }
        first lost then free;
        action free {
            assign context.level := context.level + context.tank.volume;
        }
        first free then arm;
        action arm {
            assign context.site.control.rack.controller.status := true;
        }
        first arm then light;
        action light {
            assign context.site.control.rack.controller.led.lit := true;
        }
        first light then final;
        action final terminate;
        /* partition 'Tank' represents Plant::tank: fill tank, torn, twice, mine */
        /* partition 'Valve' in 'Tank' represents Tank::valve: open valve */
        /* partition 'Plant' represents Plant: count run */
        /* partition 'Operator': prime, note level */
        /* partition 'Elsewhere': lost */
        /* partition 'Idle' represents Plant::tank: twice */
        /* partition 'Spare' represents Plant::tank */
        /* partition 'Pump' represents Plant::pump: torn, prime */
        /* partition 'Controller' represents Controller: arm */
        /* partition 'Led' represents Controller::led: light */
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
