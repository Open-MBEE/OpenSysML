// A concern stating no condition cannot be evaluated of an exposed element: the
// evaluation says so rather than reporting a vacuous conformance.
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
