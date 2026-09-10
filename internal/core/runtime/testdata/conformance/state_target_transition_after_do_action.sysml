// A transition written without a source leaves the member declared before it
// (SysML v2 §7.18.3). Here that member is the machine's do action rather than a
// state, so the machine is refused with a typed error naming it; the pilot
// rejects the same line with a syntax error (`no viable alternative at input 'accept'`).
package test {
    attribute def Go;

    part def Lander {
        exhibit state flightMode {
            entry; then coasting;
            state coasting;
            do action monitor;
            accept Go then decelerating;
            state decelerating;
        }
    }

    part scout : Lander;
}
