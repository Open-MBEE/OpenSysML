// A chain redefinition through an actor parameter the satisfy usage binds
// (`actor driver = driver1; attribute redefines driver.age = 21;`) holds on the
// bound object while the requirement is checked, as one through the satisfying
// feature does.
package ActorProbe {
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
    satisfy requirement check : Legal by vehicle1 {
        actor driver = driver1;
        attribute redefines driver.age = 21;
    }
}
