package Vehicle {
    private import ScalarValues::Real;

    part def Vehicle {
        attribute topSpeedKilometresPerHour : Real;
        attribute rangeKilometres : Real;
    }

    requirement def PerformanceRequirement {
        doc /* The vehicle shall meet its performance targets. */
        subject vehicle : Vehicle;
    }

    requirement def TopSpeedRequirement :> PerformanceRequirement {
        doc /* The vehicle shall reach at least 120 km/h. */
        require constraint { vehicle.topSpeedKilometresPerHour >= 120.0 }
    }

    requirement def RangeRequirement :> PerformanceRequirement {
        doc /* The vehicle shall travel at least 400 km on one charge. */
        require constraint { vehicle.rangeKilometres >= 400.0 }
    }

    requirement vehiclePerformance : PerformanceRequirement {
        requirement topSpeed : TopSpeedRequirement;
        requirement range : RangeRequirement;
    }
}
