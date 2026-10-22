// A concern stating no condition is a requirement check over no constraint, which
// is true (Requirements::RequirementConstraintCheck::result over an empty
// `constraints`): an exposed element satisfies it vacuously.
package test {
    part def Vehicle {
        attribute mass;
    }

    concern def Documented {
        subject vehicle : Vehicle;
    }

    viewpoint def DocumentationPerspective {
        frame concern documented : Documented;
    }

    part vehicle : Vehicle {
        attribute :>> mass = 1200.0;
    }

    part analysisContext {
        assert satisfy DocumentationPerspective::documented by vehicle;
    }
}
