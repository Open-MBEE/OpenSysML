// `all E` over an enumeration is the sequence of the values it declares, in
// declaration order, whether or not each declares a value of its own.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;
    private import IntegerFunctions::sum;

    enum def Color { red; green; blue; }

    enum def Level :> Integer {
        low = 1;
        mid = 5;
        high = 10;
    }

    part sensor {
        attribute colors : Color[*] = all Color;
        attribute colorCount : Natural = size(all Color);
        attribute levels : Level[*] = all Level;
        attribute levelSum : Integer = sum(all Level);
        attribute topLevel : Level = (all Level)#(3);
    }
}
