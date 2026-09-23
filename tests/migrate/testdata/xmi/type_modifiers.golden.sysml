package 'Shapes Profile' {
    metadata def typeModifier {
        attribute typeModifier : ScalarValues::String;
    }
}
package Signals {
    part def Actuator;
    part def Sensor;
    part def Filter {
        attribute samples : ScalarValues::Real[0..*] ordered nonunique;
        attribute row : ScalarValues::Real[3] ordered nonunique;
        attribute grid : ScalarValues::Real {
            /* applied stereotype «typeModifier»: typeModifier = [][] */
        }
        attribute rect : ScalarValues::Real {
            /* applied stereotype «typeModifier»: typeModifier = [2*3] */
        }
        attribute history : ScalarValues::Real[0..*] {
            /* applied stereotype «typeModifier»: typeModifier = [] */
        }
        attribute stray : ScalarValues::Real {
            /* applied stereotype «typeModifier»: typeModifier = [] */
        }
        attribute taps : ScalarValues::Real[4] ordered nonunique;
        attribute level : ScalarValues::Real {
            /* applied stereotype «typeModifier»: typeModifier = * */
        }
        attribute count : ScalarValues::Integer {
            @'Shapes Profile'::typeModifier {
                typeModifier = "[]";
            }
        }
        attribute blank : ScalarValues::Real {
            /* applied stereotype «typeModifier»: typeModifier =  */
        }
        attribute odd : ScalarValues::Real {
            /* applied stereotype «typeModifier»: typeModifier = [2x] */
        }
        ref part drive : Actuator;
        ref part probe : Sensor;
        abstract action def smooth {
            in values : ScalarValues::Real[0..*] ordered nonunique;
            in window : ScalarValues::Real[4] ordered nonunique;
            in handle : ScalarValues::Real {
                /* applied stereotype «typeModifier»: typeModifier = * */
            }
            out result : ScalarValues::Real[0..*] ordered nonunique;
        }
        action 'smooth 2' : smooth;
    }
}
