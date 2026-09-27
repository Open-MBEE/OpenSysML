package 'My Profile' {
    metadata def Tracked {
        attribute ticket : ScalarValues::String;
    }
}
package System {
    part def Pump {
        ref mass;
        @'My Profile'::Tracked {
            ticket = "T-42";
        }
    }
}
package Customizations;
package Mockups {
    part def Clock {
        ref ticks;
        state def Ticking {
            entry; then Running;
            state Running;
        }
        exhibit state ticking : Ticking;
        /* applied stereotype «Frame»: title = Clock */
    }
    part def Gauge {
        /* applied stereotype «Panel» */
    }
}
package Runs {
    action def 'Run pump' {
        @Simulation::Configuration;
        part target : System::Pump;
    }
}
