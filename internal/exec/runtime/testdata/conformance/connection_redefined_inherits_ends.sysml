// A connection usage that redefines another and declares no end of its own
// inherits the redefined usage's ends (KerML 1.0 §7.3.2.4, §7.4.6): its object
// attaches to the features the inherited `connect` clause names.
package test {
    private import ScalarValues::Integer;

    part def Module {
        attribute number : Integer;
    }

    connection def Installation {
        end slot : Slot;
        end module : Module;
    }

    part def Slot {
        ref installedModule : Module [0..1];
        connection installation : Installation [0..1]
            connect self to installedModule;
    }

    part def NarrowedSlot :> Slot {
        connection :>> installation [0..1];
    }

    part def System {
        part module1 : Module { :>> number = 101; }
        part module2 : Module { :>> number = 102; }
        part slot1 : Slot {
            :>> installedModule = module1;
            connection :>> installation [1];
        }
        // The ends pass through a redefinition that declares none either.
        part slot2 : NarrowedSlot {
            :>> installedModule = module2;
            connection fitted :>> installation [1];
        }
    }

    part system : System;
}
