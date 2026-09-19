// A condition stated through an anonymous nested constraint is evaluated, and the
// values a typed usage rebinds are the ones it sees.
package test {
    requirement def TouchdownRequirement {
        attribute actualVerticalSpeed;
        attribute maxVerticalSpeed;
        require constraint {
            actualVerticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        attribute :>> actualVerticalSpeed = 1.2;
        attribute :>> maxVerticalSpeed = 1.5;
    }
}
