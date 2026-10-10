action def Run {
    @Simulation::Configuration {
        timeVariable = "t_sim";
    }
}
calc def Energy {
    in E_0 : ScalarValues::Real[1];
    in P : ScalarValues::Real[1];
    in dt : ScalarValues::Real[1];
    out result : ScalarValues::Real[1];
    rep language "JavaScript" /* Math.max(E_0 - P * dt, 0)*/
    RealFunctions::max(E_0 - P * dt, 0)
}
action def Scale {
    in x : ScalarValues::Real[1];
    out y : ScalarValues::Real[1];
    rep language "JavaScript" /* x = x * 2; y = x;*/
}
action def Shift {
    in a : ScalarValues::Real[1];
    out b : ScalarValues::Real[1];
    rep language "JavaScript" /* const step = 1; a = a + step; b = a;*/
}
action def Double {
    inout n : ScalarValues::Real[1];
    rep language "JavaScript" /* const k = 2; n = n * k;*/
    first start then body;
    action body {
        attribute k : ScalarValues::Integer;
        assign k := 2;
        assign n := n * k;
    }
    first body then done;
}
action def Delay {
    in t_sim : ScalarValues::Real[1];
    out d : ScalarValues::Real[1];
    rep language "JavaScript" /* d = t_sim * 2;*/
    first start then body;
    action body {
        assign d := t_sim * 2;
    }
    first body then done;
}
calc def Floor {
    in x : ScalarValues::Real[1];
    out whole : ScalarValues::Integer[1];
    rep language "JavaScript" /* Math.floor(x)*/
    RealFunctions::floor(x)
}
calc def Ceil {
    in x : ScalarValues::Real[1];
    out whole : ScalarValues::Integer[1];
    rep language "JavaScript" /* Math.ceil(x)*/
    OpenSysMLMathFunctions::ceiling(x)
}
calc def Round {
    in x : ScalarValues::Real[1];
    out whole : ScalarValues::Real[1];
    rep language "JavaScript" /* Math.round(x)*/
    RealFunctions::floor(x + 0.5)
}
action def Root {
    in x : ScalarValues::Real[1];
    out ok : ScalarValues::Boolean[1];
    rep language "JavaScript" /* Math.sqrt(x)*/
}
action def Split {
    in x : ScalarValues::Real[1];
    out lo : ScalarValues::Real[1];
    out hi : ScalarValues::Real[1];
    rep language "JavaScript" /* x / 2*/
}
part def Gauge {
    attribute t_sim : ScalarValues::Real default = 4.0;
    attribute reading : ScalarValues::Real default = t_sim + 1;
}
part def Reactor {
    attribute power : ScalarValues::Real default = 3.0;
    attribute limit : ScalarValues::Real default = RealFunctions::min(power * 2, 5);
    attribute started : ScalarValues::Real default = 0.0;
    attribute label : ScalarValues::String;
    action cycle {
        first start then mark;
        action mark {
            rep language "JavaScript" /* started = t_sim;*/
            assign started := localClock.currentTime;
        }
        first mark then 'loop';
        action 'loop' {
            rep language "JavaScript" /* for (var k = 0; k < 3; k++) { power = power + 1; }*/
        }
        first 'loop' then alloc;
        action alloc {
            rep language "JavaScript" /* power = new Number(4);*/
        }
        first alloc then trim;
        action trim {
            rep language "JavaScript" /* label = label.trim();*/
        }
        first trim then self2;
        action self2 {
            rep language "JavaScript" /* power = this.owner.power;*/
        }
        first self2 then stray;
        action stray {
            rep language "JavaScript" /* power = nowhere + 1;*/
        }
        first stray then mixed;
        action mixed {
            rep language "JavaScript" /* power = label + 1;*/
        }
        first mixed then python;
        action python {
            rep language "Python" /* power = power + 1*/
            assign power := power + 1;
        }
        first python then partial;
        action partial {
            rep language "JavaScript" /* power = power + 1; label = label.trim();*/
        }
        first partial then shadow;
        action shadow {
            rep language "JavaScript" /* var power = 1; power += 1;*/
        }
        first shadow then pinned;
        action pinned {
            in x : ScalarValues::Real[1] = 1.0;
            rep language "JavaScript" /* var x = 1; x += 1;*/
        }
        first pinned then relay;
        action relay {
            in x : ScalarValues::Real[1] = 1.0;
            out y : ScalarValues::Real[1];
            rep language "JavaScript" /* y = x * 2; x = y;*/
        }
        first relay then gain;
        action gain {
            in x : ScalarValues::Real[1] = 1.5;
            out y : ScalarValues::Real[1];
            rep language "JavaScript" /* y = x * power;*/
            assign y := x * power;
        }
        first gain then bare;
        action bare {
            in raw[1] = 1.0;
            rep language "JavaScript" /* power = raw.level;*/
        }
        first bare then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
