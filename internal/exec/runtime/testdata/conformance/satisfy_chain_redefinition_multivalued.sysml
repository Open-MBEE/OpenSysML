// A chain redefinition on the satisfying feature walking a multi-valued
// intermediate (`redefines vehicle1.wheels.pressure = 32`) applies to each
// element the intermediate holds while the requirement is checked.
package MultiProbe {
    private import ScalarValues::Integer;
    part def Wheel {
        attribute pressure : Integer default 0;
    }
    part def Vehicle {
        part wheels : Wheel[4];
    }
    part vehicle1 : Vehicle;
    requirement def Inflated {
        subject vehicle : Vehicle;
        require constraint { vehicle.wheels#(1).pressure == 32 and vehicle.wheels#(4).pressure == 32 }
    }
    satisfy requirement check : Inflated by vehicle1 {
        attribute redefines vehicle1.wheels.pressure = 32;
    }
}
