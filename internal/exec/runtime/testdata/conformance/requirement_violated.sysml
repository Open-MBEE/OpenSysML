// A requirement whose condition is false is a verdict, not a failure to evaluate.
package test {
    requirement def TouchdownRequirement {
        attribute actualVerticalSpeed;
        attribute maxVerticalSpeed;
        require constraint {
            actualVerticalSpeed <= maxVerticalSpeed
        }
    }

    requirement hardLanding : TouchdownRequirement {
        attribute :>> actualVerticalSpeed = 2.4;
        attribute :>> maxVerticalSpeed = 1.5;
    }
}
