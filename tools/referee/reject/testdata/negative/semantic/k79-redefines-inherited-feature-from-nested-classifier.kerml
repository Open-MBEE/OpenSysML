// Invalid: a classifier nested in a type is not featured by it, so a feature of the nested classifier cannot access, and so cannot redefine, a feature the enclosing type inherits (KerML 8.3.3.3.10 validateSubsettingFeaturingTypes; pilot validateSubsettingFeaturingTypes).
package K79RedefinesInheritedFeatureFromNestedClassifier {
    class Vehicle {
        feature mass;
    }
    class Truck specializes Vehicle {
        class Trailer {
            feature trailerMass redefines mass;
        }
    }
}
