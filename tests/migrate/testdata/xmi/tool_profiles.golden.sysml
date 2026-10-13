package 'My Profile' {
    metadata def Tracked {
        attribute ticket : ScalarValues::String;
    }
}
package System {
    part def Pump {
        attribute mass;
        @'My Profile'::Tracked {
            ticket = "T-42";
        }
    }
}
package Customizations;
package Mockups {
    occurrence def Clock {
        attribute ticks;
        state def Ticking {
            entry; then Running;
            state Running;
        }
        exhibit state ticking : Ticking;
        @MigrationMetadata::AppliedStereotype {
            stereotype = "Frame";
            tags = "title = Clock";
        }
    }
    part def Gauge {
        @MigrationMetadata::AppliedStereotype {
            stereotype = "Panel";
        }
    }
}
package Runs {
    action def 'Run pump' {
        @Simulation::Configuration;
        part target : System::Pump;
    }
}
