// The FeatureValue and Expression metaclass families read by specification
// name: an attribute's bound value, the operator expression it binds and the
// literal it computes from.
package ValueFamily {
    private import ScalarValues::*;
    part def Tank {
        attribute capacity : Real = 40.0;
        attribute reserve : Real default = capacity * 0.1;
        attribute full : Boolean = reserve < capacity;
    }
}
