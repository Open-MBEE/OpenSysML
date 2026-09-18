part def Run {
    /* applied stereotype «SimulationConfig»: timeVariableName = t_sim */
}
calc def Energy {
    in E_0 : ScalarValues::Real;
    in P : ScalarValues::Real;
    in dt : ScalarValues::Real;
    out result : ScalarValues::Real;
    RealFunctions::max(E_0 - P * dt, 0)
}
part def Reactor {
    attribute power : ScalarValues::Real default = 3.0;
    attribute limit : ScalarValues::Real default = RealFunctions::min(power * 2, 5);
    attribute started : ScalarValues::Real default = 0.0;
    attribute label : ScalarValues::String;
    action def Cycle {
        first start then mark;
        action mark {
            assign this.started := localClock.currentTime;
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
            assign this.power := this.power + 1;
        }
        first python then partial;
        action partial {
            /* body not migrated (the call "label.trim" is not in the translated function table) {JavaScript}:
             * power = power + 1; label = label.trim();
             */
        }
        first partial then final;
        action final terminate;
    }
}
