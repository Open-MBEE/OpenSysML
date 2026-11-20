// A requirement feature nothing binds has no value though the satisfying object carries one spelled as it:
// the subject's values reach it through the subject or a binding (SysML v2 §7.21.1), so this is undecided.
package test {
    part def Lander {
        attribute verticalSpeed;
    }

    requirement def TouchdownRequirement {
        attribute verticalSpeed;
        attribute maxVerticalSpeed;
        require constraint {
            verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }

    part lander01 : Lander {
        attribute :>> verticalSpeed = 1.2;
    }

    part analysisContext {
        assert satisfy touchdown by lander01;
    }
}
