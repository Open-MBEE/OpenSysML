package 'Vehicle Profile' {
    metadata def PartProperty;
}
package Fleet {
    part def Vehicle {
        attribute mass : ScalarValues::Real;
        attribute mode : Mode;
        part engine : Engine;
        ref part spare : Wheel;
        ref part lead : Wheel;
        constraint limit : MaxSpeed;
        part odd : Wheel {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "ValueProperty";
            }
        }
        part axle : Wheel {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "PartProperty";
                tags = "ordering = rear";
            }
        }
        part cabin : Engine {
            @'Vehicle Profile'::PartProperty;
        }
        attribute load;
        part trailer;
        ref depot;
        ref driver;
        constraint rule;
        attribute color;
        part chassis;
        attribute serial : ScalarValues::String {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "ValueProperty";
                profile = "http://example.com/legacy/customization";
            }
        }
    }
    part def Engine;
    part def Wheel;
    enum def Mode {
        eco;
        sport;
    }
    constraint def MaxSpeed {
        in attribute v : ScalarValues::Real[1];
        in ref part wheel : Wheel[1];
        in ref part hub : Wheel[1] {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "ValueProperty";
            }
        }
        in attribute k : ScalarValues::Real[1] {
            @MigrationMetadata::AppliedStereotype {
                stereotype = "ReferenceProperty";
            }
        }
        v <= 120.0
    }
}
