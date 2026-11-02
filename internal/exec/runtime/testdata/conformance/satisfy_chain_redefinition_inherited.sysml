// A chain redefinition a `requirement def` general declares through its actor
// parameter (`attribute redefines driver.age = 21;` in LegalAdult) is inherited
// by the satisfy usage typed by it and holds on the actor the usage binds.
package InheritedProbe {
    private import ScalarValues::Integer;
    part def Vehicle;
    part def Person {
        attribute age : Integer default 0;
    }
    part vehicle1 : Vehicle;
    part driver1 : Person;
    requirement def Legal {
        subject vehicle : Vehicle;
        actor driver : Person;
        require constraint { driver.age == 21 }
    }
    requirement def LegalAdult :> Legal {
        attribute redefines driver.age = 21;
    }
    satisfy requirement check : LegalAdult by vehicle1 {
        actor driver = driver1;
    }
}
