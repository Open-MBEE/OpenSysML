// A nested redefinition written beside a connection usage (`:>> link.rating = 7;`)
// redefines a feature of the connection's object (SysML v2 §7.6.4, KerML 1.0
// §7.3.4.5), whether the usage states its ends or inherits them.
package test {
    private import ScalarValues::Integer;

    part def Module;
    connection def Installation {
        end slot : Slot;
        end module : Module;
        attribute rating : Integer default = 1;
    }
    part def Slot {
        ref installedModule : Module [0..1];
        connection installation : Installation [0..1]
            connect self to installedModule;
    }

    part def System {
        part module1 : Module;
        part stated : Slot {
            :>> installedModule = module1;
            connection :>> installation [1] connect self to installedModule;
            :>> installation.rating = 7;
        }
        part inherited : Slot {
            :>> installedModule = module1;
            connection :>> installation [1];
            :>> installation.rating = 8;
        }
        part plain : Slot {
            :>> installedModule = module1;
            connection :>> installation [1];
        }
    }
}
