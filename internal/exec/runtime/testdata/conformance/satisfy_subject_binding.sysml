// A satisfaction assertion binds the requirement's subject to the part named by
// `by`, so the requirement's condition reads that part's own values. Unit-free
// values: quantity expressions such as `1.5 [m/s]` are ROADMAP A1a.
package test {
    part def Lander {
        attribute verticalSpeed;
    }

    requirement def TouchdownRequirement {
        subject lander : Lander;
        attribute maxVerticalSpeed;
        require constraint {
            lander.verticalSpeed <= maxVerticalSpeed
        }
    }

    requirement touchdown : TouchdownRequirement {
        attribute :>> maxVerticalSpeed = 1.5;
    }

    part slowLander : Lander {
        attribute :>> verticalSpeed = 1.2;
    }

    part fastLander : Lander {
        attribute :>> verticalSpeed = 2.4;
    }

    part analysisContext {
        assert satisfy touchdown by slowLander;
        assert satisfy touchdown by fastLander;
    }
}
