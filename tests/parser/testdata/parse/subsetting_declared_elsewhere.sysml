// The values of a subsetting feature are values of the feature it subsets
// wherever the subsetting feature is declared (KerML 1.1 §7.3.4.4): nested
// under a usage of the object, or naming a feature of a nested object by a chain.
package test {
    private import ScalarValues::Real;
    private import ScalarValues::Integer;
    private import RealFunctions::*;
    private import SequenceFunctions::size;

    part def Sub {
        attribute mass : Real default 1.0;
    }

    part def Tray {
        part items : Sub[0..*];
        attribute held : Integer = size(items);
    }

    part def Vehicle {
        part subsystem : Sub[0..*];
        attribute count : Integer = size(subsystem);
        attribute totalMass : Real = sum(subsystem.mass);
        part tray : Tray;
        attribute onTray : Integer = tray.held;

        part direct : Sub :> subsystem;
        part stowed : Sub :> tray.items;
        part frame {
            part beam : Sub :> subsystem {
                attribute :>> mass = 2.5;
            }
            part joint {
                part pin : Sub :> subsystem {
                    attribute :>> mass = 0.5;
                }
            }
        }
        part wheels[2] {
            part hub : Sub :> subsystem;
        }
    }

    part vehicle : Vehicle;
}
