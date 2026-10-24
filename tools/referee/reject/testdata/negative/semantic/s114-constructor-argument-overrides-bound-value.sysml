// Invalid: a constructor argument binds a feature whose value is written with `=`, which every feature redefining it inherits; only a `default =` value may be overridden (KerML 8.3.4.10.2 validateFeatureValueOverriding; pilot validateFeatureValueOverriding).
package S114ConstructorArgumentOverridesBoundValue {
    private import ScalarValues::Integer;
    item def A {
        attribute x : Integer = 1;
    }
    part def Receiver;
    part receiver : Receiver;
    action def Make {
        action send1 send new A(x = 2) to receiver;
        action send2 send new A(3) to receiver;
    }
}
