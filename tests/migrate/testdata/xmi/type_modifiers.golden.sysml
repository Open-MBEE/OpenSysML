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
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = [][]";
            }
        }
        attribute rect : ScalarValues::Real {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = [2*3]";
            }
        }
        attribute history : ScalarValues::Real[0..*] {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = []";
            }
        }
        attribute stray : ScalarValues::Real {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = []";
            }
        }
        attribute taps : ScalarValues::Real[4] ordered nonunique;
        attribute level : ScalarValues::Real {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = *";
            }
        }
        attribute count : ScalarValues::Integer {
            @'Shapes Profile'::typeModifier {
                typeModifier = "[]";
            }
        }
        attribute blank : ScalarValues::Real {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = ";
            }
        }
        attribute odd : ScalarValues::Real {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "typeModifier";
                tags = "typeModifier = [2x]";
            }
        }
        ref part drive : Actuator;
        ref part probe : Sensor;
        abstract action def smooth {
            in values : ScalarValues::Real[0..*] ordered nonunique;
            in window : ScalarValues::Real[4] ordered nonunique;
            in handle : ScalarValues::Real[1] {
                @MigrationMetadata::AppliedStereotype {
                    stereotype = "typeModifier";
                    tags = "typeModifier = *";
                }
            }
            out result : ScalarValues::Real[0..*] ordered nonunique;
        }
        action 'smooth 2' : smooth;
    }
}
