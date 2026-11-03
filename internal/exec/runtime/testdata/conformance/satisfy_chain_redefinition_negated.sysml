// A `not satisfy` usage declaring a chain redefinition on its satisfying feature
// is judged with the chain in effect too: the requirement holds under the
// redefined value, so the negated assertion fails.
package NegatedProbe {
    private import ScalarValues::Integer;
    part def Vehicle {
        attribute fuel : Integer default 0;
    }
    part vehicle1 : Vehicle;
    requirement def Full {
        subject vehicle : Vehicle;
        attribute target : Integer default 1300;
        require constraint { vehicle.fuel == target }
    }
    not satisfy requirement check : Full by vehicle1 {
        attribute redefines vehicle1.fuel = 1300;
    }
}
