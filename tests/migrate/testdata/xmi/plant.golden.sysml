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
    action fill {
        first start then 'open valve';
        action 'open valve' {
            rep language "JavaScript" /* open = true;*/
            assign tank.valve.open := true;
        }
        first 'open valve' then 'fill tank';
        action 'fill tank' {
            rep language "JavaScript" /* volume = volume * 2 + (valve.open ? 1 : 0);*/
            assign tank.volume := tank.volume * 2 + (if tank.valve.open ? 1 else 0);
        }
        first 'fill tank' then torn;
        action torn {
            rep language "JavaScript" /* volume = 0;*/
        }
        /* guard not migrated: [{JavaScript} on] — the name "on" resolves to nothing readable: nothing visible from Plant::Fill::<ControlFlow> is called on; its source 'torn' is in the partitions 'Tank' (this.tank) and 'Pump' (this.pump), which represent different objects, so names resolve through no partition */
        first torn then prime;
        action prime {
            rep language "JavaScript" /* on = true;*/
            assign pump.on := true;
        }
        first prime then twice;
        action twice {
            rep language "JavaScript" /* volume = volume + 1;*/
            assign tank.volume := tank.volume + 1;
        }
        first twice then mine;
        action mine {
            rep language "JavaScript" /* this.level = 1;*/
        }
        first mine then 'count run';
        action 'count run' {
            rep language "JavaScript" /* runs += 1;*/
            assign runs := runs + 1;
        }
        first 'count run' then 'note level';
        action 'note level' {
            rep language "JavaScript" /* level = tank.volume;*/
            assign level := tank.volume;
        }
        first 'note level' then lost;
        action lost {
            rep language "JavaScript" /* level = level + 1;*/
            assign level := level + 1;
        }
        first lost then free;
        action free {
            rep language "JavaScript" /* level = level + tank.volume;*/
            assign level := level + tank.volume;
        }
        first free then arm;
        action arm {
            rep language "JavaScript" /* this.status = true;*/
            assign site.control.rack.controller.status := true;
        }
        first arm then light;
        action light {
            rep language "JavaScript" /* lit = true;*/
            assign site.control.rack.controller.led.lit := true;
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
