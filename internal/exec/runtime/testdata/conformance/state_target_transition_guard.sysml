// The guard shorthand `if … then` and the `transition if … do … then` form leave
// the state declared before them (SysML v2 §7.18.3): the first leaves coasting
// with its effect, the second leaves decelerating and its guard does not hold.
package test {
    private import ScalarValues::*;

    state def Descent {
        attribute alt : Real = 10.0;
        attribute fired : Integer = 0;

        entry; then coasting;
        state coasting;
        transition if alt < 20.0 do assign fired := fired + 1 then decelerating;
        state decelerating;
        if alt < 5.0 then landed;
        state landed;
    }
}
