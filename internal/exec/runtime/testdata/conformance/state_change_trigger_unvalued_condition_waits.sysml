// The machine of an object whose change condition reads a feature carrying no
// value rests in the state the condition leaves: the condition has no result,
// so it has not become true, and nothing else moves the machine.
package test {
    private import ScalarValues::*;
    part def Relay {
        attribute coilVoltage : Real;
        exhibit state contacts {
            entry; then open;
            state open;
            accept when coilVoltage >= 19.6 then closed;
            state closed;
        }
    }
    part relay : Relay;
}
