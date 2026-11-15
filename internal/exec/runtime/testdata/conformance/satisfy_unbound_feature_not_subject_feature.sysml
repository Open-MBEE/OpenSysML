// A requirement feature nothing binds has no value, though the satisfying object
// carries a feature spelled as it is: the name in the condition is the
// requirement's own feature (KerML §8.2.3.5), which a subject's values reach only
// through the subject parameter or a binding (SysML v2 §7.21.1), so the
// satisfaction is undecided naming the feature rather than judged over the
// object's value.
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
