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
    RealFunctions::max(E_0 - P * dt, 0)
}
action def Scale {
    in x : ScalarValues::Real[1];
    out y : ScalarValues::Real[1];
    /* body not migrated (the construct "x" is outside the translated subset: an in parameter is not assigned) {JavaScript}:
     * x = x * 2; y = x;
     */
}
action def Shift {
    in a : ScalarValues::Real[1];
    out b : ScalarValues::Real[1];
    /* body not migrated (the construct "a" is outside the translated subset: an in parameter is not assigned) {JavaScript}:
     * const step = 1; a = a + step; b = a;
     */
}
action def Double {
    inout n : ScalarValues::Real[1];
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
    first start then body;
    action body {
        assign d := t_sim * 2;
    }
    first body then done;
}
calc def Floor {
    in x : ScalarValues::Real[1];
    out whole : ScalarValues::Integer[1];
    RealFunctions::floor(x)
}
calc def Ceil {
    in x : ScalarValues::Real[1];
    out whole : ScalarValues::Integer[1];
    OpenSysMLMathFunctions::ceiling(x)
}
calc def Round {
    in x : ScalarValues::Real[1];
    out whole : ScalarValues::Real[1];
    RealFunctions::floor(x + 0.5)
}
action def Root {
    in x : ScalarValues::Real[1];
    out ok : ScalarValues::Boolean[1];
    /* body not migrated (as the result expression, the types at "Math.sqrt(x)" disagree: the expression is a Real, not the Boolean wanted; as statements, the call "Math.sqrt" is not in the translated function table: a call is not a statement of the subset) {JavaScript}:
     * Math.sqrt(x)
     */
}
action def Split {
    in x : ScalarValues::Real[1];
    out lo : ScalarValues::Real[1];
    out hi : ScalarValues::Real[1];
    /* body not migrated (as the result expression, the behavior has 2 output parameters; one result expression can stand for none of them; as statements, the construct "x" is outside the translated subset: an expression that assigns nothing is not a statement of the subset) {JavaScript}:
     * x / 2
     */
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
            assign started := localClock.currentTime;
        }
        first mark then 'loop';
        action 'loop' {
            /* body not migrated (the construct "for" is outside the translated subset) {JavaScript}:
             * for (var k = 0; k < 3; k++) { power = power + 1; }
             */
        }
        first 'loop' then alloc;
        action alloc {
            /* body not migrated (the construct "new" is outside the translated subset) {JavaScript}:
             * power = new Number(4);
             */
        }
        first alloc then trim;
        action trim {
            /* body not migrated (the call "label.trim" is not in the translated function table) {JavaScript}:
             * label = label.trim();
             */
        }
        first trim then self2;
        action self2 {
            /* body not migrated (the name "this.owner.power" resolves to nothing readable: Reactor has no feature owner) {JavaScript}:
             * power = this.owner.power;
             */
        }
        first self2 then stray;
        action stray {
            /* body not migrated (the name "nowhere" resolves to nothing readable: nothing visible from Reactor::Cycle::stray is called nowhere) {JavaScript}:
             * power = nowhere + 1;
             */
        }
        first stray then mixed;
        action mixed {
            /* body not migrated (the construct "+" is outside the translated subset: string concatenation has no v2 form in the subset) {JavaScript}:
             * power = label + 1;
             */
        }
        first mixed then python;
        action python {
            assign power := power + 1;
        }
        first python then partial;
        action partial {
            /* body not migrated (the call "label.trim" is not in the translated function table) {JavaScript}:
             * power = power + 1; label = label.trim();
             */
        }
        first partial then shadow;
        action shadow {
            /* body not migrated (the construct "var power" is outside the translated subset: power is already a feature here, which a declaration would shadow) {JavaScript}:
             * var power = 1; power += 1;
             */
        }
        first shadow then pinned;
        action pinned {
            in x : ScalarValues::Real[1] = 1.0;
            /* body not migrated (the construct "var x" is outside the translated subset: x is already a feature here, which a declaration would shadow) {JavaScript}:
             * var x = 1; x += 1;
             */
        }
        first pinned then relay;
        action relay {
            in x : ScalarValues::Real[1] = 1.0;
            out y : ScalarValues::Real[1];
            /* body not migrated (the construct "x" is outside the translated subset: an input pin is not assigned) {JavaScript}:
             * y = x * 2; x = y;
             */
        }
        first relay then gain;
        action gain {
            in x : ScalarValues::Real[1] = 1.5;
            out y : ScalarValues::Real[1];
            assign y := x * power;
        }
        first gain then bare;
        action bare {
            in raw[1] = 1.0;
            /* body not migrated (the name "raw.level" resolves to nothing readable: Reactor::Cycle::bare::raw has no type, so no feature level) {JavaScript}:
             * power = raw.level;
             */
        }
        first bare then final;
        action final terminate;
        metadata MigrationMetadata::SynthesizedName about final;
    }
}
