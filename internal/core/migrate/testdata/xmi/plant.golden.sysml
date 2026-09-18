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
part def Plant {
    part tank : Tank;
    part pump : Pump;
    attribute level : ScalarValues::Real default = 0.0;
    attribute runs : ScalarValues::Integer default = 0;
    action def Fill {
        first start then 'open valve';
        action 'open valve' {
            assign this.tank.valve.open := true;
        }
        first 'open valve' then 'fill tank';
        action 'fill tank' {
            assign this.tank.volume := this.tank.volume * 2 + (if this.tank.valve.open ? 1 else 0);
        }
        first 'fill tank' then torn;
        action torn {
            /* body not migrated (the name "volume" resolves to nothing readable: nothing visible from Plant::Fill::torn is called volume; it is in the partitions 'Tank' (this.tank) and 'Pump' (this.pump), which represent different objects, so names resolve through no partition) {JavaScript}:
             * volume = 0;
             */
        }
        first torn then prime;
        action prime {
            assign this.pump.on := true;
        }
        first prime then twice;
        action twice {
            assign this.tank.volume := this.tank.volume + 1;
        }
        first twice then 'count run';
        action 'count run' {
            assign this.runs := this.runs + 1;
        }
        first 'count run' then 'note level';
        action 'note level' {
            assign this.level := this.tank.volume;
        }
        first 'note level' then lost;
        action lost {
            assign this.level := this.level + 1;
        }
        first lost then free;
        action free {
            assign this.level := this.level + this.tank.volume;
        }
        first free then final;
        action final terminate;
        /* partition 'Tank' represents Plant::tank: fill tank, torn, twice */
        /* partition 'Valve' in 'Tank' represents Tank::valve: open valve */
        /* partition 'Plant' represents Plant: count run */
        /* partition 'Operator': prime, note level */
        /* partition 'Elsewhere': lost */
        /* partition 'Idle' represents Plant::tank: twice */
        /* partition 'Spare' represents Plant::tank */
        /* partition 'Pump' represents Plant::pump: torn, prime */
    }
}
